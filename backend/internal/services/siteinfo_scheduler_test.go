package services

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"wikikeeper-backend/internal/config"
)

func TestRequestGroupUsesRegistrableDomain(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want string
	}{
		{name: "Fandom subdomain", url: "https://ifmovie.fandom.com", want: "fandom.com"},
		{name: "another Fandom subdomain", url: "https://starwars.fandom.com/wiki/Main_Page", want: "fandom.com"},
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

func TestProviderGateIsSharedAcrossFandomSubdomains(t *testing.T) {
	scheduler := NewSiteInfoScheduler(nil, nil, nil, &config.Config{})
	first, firstGroup := scheduler.providerGateFor("https://first.fandom.com")
	second, secondGroup := scheduler.providerGateFor("https://second.fandom.com")

	require.Equal(t, "fandom.com", firstGroup)
	require.Equal(t, firstGroup, secondGroup)
	require.Same(t, first, second)
}

func TestProviderGateHonorsRetryAfterAndRecovers(t *testing.T) {
	now := time.Date(2026, time.July, 31, 5, 0, 0, 0, time.UTC)
	var sleeps []time.Duration
	gate := newProviderGateWithClock(
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
	attempted, err := gate.run(context.Background(), func() error { return rateLimitErr })
	require.True(t, attempted)
	require.ErrorIs(t, err, rateLimitErr)

	attempted, err = gate.run(context.Background(), func() error { return nil })
	require.False(t, attempted)
	statusErr, rateLimited := asRateLimitError(err)
	require.True(t, rateLimited)
	require.Equal(t, "60", statusErr.RetryAfter)

	now = now.Add(time.Minute)
	attempted, err = gate.run(context.Background(), func() error { return nil })
	require.True(t, attempted)
	require.NoError(t, err)
	attempted, err = gate.run(context.Background(), func() error { return nil })
	require.True(t, attempted)
	require.NoError(t, err)

	require.Equal(t, []time.Duration{providerRequestInterval}, sleeps)
	require.Equal(t, 0, gate.consecutiveRateLimits)
}

func TestProviderGateUsesExponentialBackoff(t *testing.T) {
	now := time.Date(2026, time.July, 31, 5, 0, 0, 0, time.UTC)
	var sleeps []time.Duration
	gate := newProviderGateWithClock(
		func() time.Time { return now },
		func(_ context.Context, delay time.Duration) error {
			sleeps = append(sleeps, delay)
			now = now.Add(delay)
			return nil
		},
		func(time.Duration) time.Duration { return 0 },
	)
	rateLimitErr := &HTTPStatusError{StatusCode: http.StatusTooManyRequests}

	attempted, err := gate.run(context.Background(), func() error { return rateLimitErr })
	require.True(t, attempted)
	require.Error(t, err)
	attempted, err = gate.run(context.Background(), func() error { return rateLimitErr })
	require.False(t, attempted)
	require.Error(t, err)
	now = now.Add(30 * time.Second)
	attempted, err = gate.run(context.Background(), func() error { return rateLimitErr })
	require.True(t, attempted)
	require.Error(t, err)
	attempted, err = gate.run(context.Background(), func() error { return nil })
	require.False(t, attempted)
	require.Error(t, err)

	require.Empty(t, sleeps)
	require.Equal(t, 2, gate.consecutiveRateLimits)
	require.Equal(t, now.Add(time.Minute), gate.nextAllowed)
}

func TestProviderGateWaitIsCancellable(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	gate := newProviderGate()
	<-gate.token

	called := false
	attempted, err := gate.run(ctx, func() error {
		called = true
		return nil
	})
	require.ErrorIs(t, err, context.Canceled)
	require.False(t, attempted)
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
