package services

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"wikikeeper-backend/internal/config"
	"wikikeeper-backend/internal/models"
)

func TestRequestGroupUsesRegistrableDomain(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want string
	}{
		{name: "Fandom subdomain", url: "https://ifmovie.fandom.com", want: "fandom.com"},
		{name: "another Fandom subdomain", url: "https://starwars.fandom.com/wiki/Main_Page", want: "fandom.com"},
		{name: "legacy Wikia domain", url: "https://starwars.wikia.com", want: "fandom.com"},
		{name: "Gamepedia domain", url: "https://example.gamepedia.com", want: "fandom.com"},
		{name: "multi-part public suffix", url: "https://wiki.example.co.uk", want: "example.co.uk"},
		{name: "IP address", url: "http://127.0.0.1:8080", want: "127.0.0.1"},
		{name: "invalid URL", url: "://invalid", want: "default"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, requestGroup(tt.url))
		})
	}
}

func TestProviderLimiterIsSharedAcrossFandomDomains(t *testing.T) {
	limiter := NewProviderLimiter(nil, &config.Config{})
	first, firstGroup := limiter.gateFor("https://first.fandom.com")
	second, secondGroup := limiter.gateFor("https://second.wikia.com")

	require.Equal(t, "fandom.com", firstGroup)
	require.Equal(t, firstGroup, secondGroup)
	require.Same(t, first, second)
}

func TestSchedulerStopsProviderQueueAfterOneRateLimit(t *testing.T) {
	var requests atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		w.Header().Set("Retry-After", "60")
		http.Error(w, "rate limited", http.StatusTooManyRequests)
	}))
	defer server.Close()

	db := setupCollectorTestDB(t)
	apiURL := server.URL + "/api.php"
	indexURL := server.URL + "/index.php"
	wikis := []*models.Wiki{
		{ID: uuid.New(), URL: "https://one.fandom.com", APIURL: &apiURL, IndexURL: &indexURL, Status: models.WikiStatusOK, CollectionStatus: models.CollectionStatusOK, APIAvailable: true, IsActive: true},
		{ID: uuid.New(), URL: "https://two.fandom.com", APIURL: &apiURL, IndexURL: &indexURL, Status: models.WikiStatusOK, CollectionStatus: models.CollectionStatusOK, APIAvailable: true, IsActive: true},
		{ID: uuid.New(), URL: "https://three.fandom.com", APIURL: &apiURL, IndexURL: &indexURL, Status: models.WikiStatusOK, CollectionStatus: models.CollectionStatusOK, APIAvailable: true, IsActive: true},
	}
	for _, wiki := range wikis {
		require.NoError(t, db.Create(wiki).Error)
	}

	scheduler := NewSiteInfoScheduler(
		db,
		func() *CollectorService {
			limiter := newProviderLimiter(nil, 0, 0, time.Now, sleepContext, func(time.Duration) time.Duration { return 0 })
			return NewCollectorService(
				db,
				NewMediaWikiService(time.Second, "WikiKeeper-Test/1.0", limiter),
				&config.Config{},
				limiter,
			)
		}(),
		nil,
		&config.Config{SiteinfoCheckBatchSize: 100},
	)
	scheduler.run(context.Background())

	require.EqualValues(t, 1, requests.Load())
	var attempted, deferred int
	for _, wiki := range wikis {
		var updated models.Wiki
		require.NoError(t, db.First(&updated, "id = ?", wiki.ID).Error)
		switch updated.CollectionStatus {
		case models.CollectionStatusRateLimited:
			attempted++
			require.Equal(t, 1, updated.ConsecutiveFailures)
			require.NotNil(t, updated.LastCheckAt)
			require.NotNil(t, updated.LastError)
		case models.CollectionStatusOK:
			deferred++
			require.Equal(t, 0, updated.ConsecutiveFailures)
			require.Nil(t, updated.LastCheckAt)
			require.Nil(t, updated.LastError)
		default:
			t.Fatalf("unexpected collection status %q", updated.CollectionStatus)
		}
		require.NotNil(t, updated.NextCheckAt)
	}
	require.Equal(t, 1, attempted)
	require.Equal(t, 2, deferred)
}

func TestProviderLimiterHonorsRetryAfterAndRecovers(t *testing.T) {
	now := time.Date(2026, time.July, 31, 5, 0, 0, 0, time.UTC)
	var sleeps []time.Duration
	limiter := newProviderLimiter(
		nil,
		500*time.Millisecond,
		5*time.Second,
		func() time.Time { return now },
		func(_ context.Context, delay time.Duration) error {
			sleeps = append(sleeps, delay)
			now = now.Add(delay)
			return nil
		},
		func(time.Duration) time.Duration { return 0 },
	)

	rateLimitErr := &HTTPStatusError{
		StatusCode: http.StatusTooManyRequests,
		RetryAfter: "60",
		Body:       "rate limited",
	}
	err := limiter.Run(context.Background(), "https://one.fandom.com", func() error { return rateLimitErr })
	limitErr, ok := asProviderLimitError(err)
	require.True(t, ok)
	require.True(t, limitErr.Attempted)
	require.ErrorIs(t, err, rateLimitErr)

	err = limiter.Run(context.Background(), "https://two.wikia.com", func() error { return nil })
	limitErr, ok = asProviderLimitError(err)
	require.True(t, ok)
	require.False(t, limitErr.Attempted)
	statusErr, rateLimited := asRateLimitError(err)
	require.True(t, rateLimited)
	require.Equal(t, "60", statusErr.RetryAfter)

	now = now.Add(time.Minute)
	err = limiter.Run(context.Background(), "https://one.fandom.com", func() error { return nil })
	require.NoError(t, err)
	err = limiter.Run(context.Background(), "https://one.fandom.com", func() error { return nil })
	require.NoError(t, err)

	require.Equal(t, []time.Duration{5 * time.Second}, sleeps)
	gate, _ := limiter.gateFor("https://one.fandom.com")
	require.Equal(t, 0, gate.consecutiveRateLimits)
}

func TestProviderLimiterUsesExponentialBackoff(t *testing.T) {
	now := time.Date(2026, time.July, 31, 5, 0, 0, 0, time.UTC)
	var sleeps []time.Duration
	limiter := newProviderLimiter(
		nil,
		0,
		0,
		func() time.Time { return now },
		func(_ context.Context, delay time.Duration) error {
			sleeps = append(sleeps, delay)
			now = now.Add(delay)
			return nil
		},
		func(time.Duration) time.Duration { return 0 },
	)
	rateLimitErr := &HTTPStatusError{StatusCode: http.StatusTooManyRequests}

	err := limiter.Run(context.Background(), "https://one.fandom.com", func() error { return rateLimitErr })
	limitErr, ok := asProviderLimitError(err)
	require.True(t, ok)
	require.True(t, limitErr.Attempted)
	require.Error(t, err)
	err = limiter.Run(context.Background(), "https://one.fandom.com", func() error { return rateLimitErr })
	limitErr, ok = asProviderLimitError(err)
	require.True(t, ok)
	require.False(t, limitErr.Attempted)
	require.Error(t, err)
	now = now.Add(30 * time.Second)
	err = limiter.Run(context.Background(), "https://one.fandom.com", func() error { return rateLimitErr })
	limitErr, ok = asProviderLimitError(err)
	require.True(t, ok)
	require.True(t, limitErr.Attempted)
	require.Error(t, err)
	err = limiter.Run(context.Background(), "https://one.fandom.com", func() error { return nil })
	limitErr, ok = asProviderLimitError(err)
	require.True(t, ok)
	require.False(t, limitErr.Attempted)
	require.Error(t, err)

	require.Empty(t, sleeps)
	gate, _ := limiter.gateFor("https://one.fandom.com")
	require.Equal(t, 2, gate.consecutiveRateLimits)
	require.Equal(t, now.Add(time.Minute), gate.nextAllowed)
}

func TestProviderLimiterWaitIsCancellable(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	limiter := NewProviderLimiter(nil, &config.Config{})
	gate, _ := limiter.gateFor("https://one.fandom.com")
	<-gate.token

	called := false
	err := limiter.Run(ctx, "https://one.fandom.com", func() error {
		called = true
		return nil
	})
	require.ErrorIs(t, err, context.Canceled)
	require.False(t, called)

	gate.token <- struct{}{}
}

func TestRateLimitBackoffParsingAndCap(t *testing.T) {
	now := time.Date(2026, time.July, 31, 5, 0, 0, 0, time.UTC)
	zeroJitter := func(time.Duration) time.Duration { return 0 }

	delay, ok := rateLimitBackoff(&HTTPStatusError{
		StatusCode: http.StatusTooManyRequests,
		RetryAfter: "120",
	}, 1, now, zeroJitter)
	require.True(t, ok)
	require.Equal(t, 2*time.Minute, delay)

	delay, ok = rateLimitBackoff(&HTTPStatusError{
		StatusCode: http.StatusTooManyRequests,
		RetryAfter: now.Add(3 * time.Minute).Format(http.TimeFormat),
	}, 1, now, zeroJitter)
	require.True(t, ok)
	require.Equal(t, 3*time.Minute, delay)

	delay, ok = rateLimitBackoff(&HTTPStatusError{StatusCode: http.StatusTooManyRequests}, 99, now, zeroJitter)
	require.True(t, ok)
	require.Equal(t, maxRateLimitBackoff, delay)

	_, ok = rateLimitBackoff(errors.New("not throttled"), 1, now, zeroJitter)
	require.False(t, ok)
}
