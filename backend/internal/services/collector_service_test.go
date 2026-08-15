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
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"wikikeeper-backend/internal/config"
	"wikikeeper-backend/internal/models"
)

func setupCollectorTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Exec(`
		CREATE TABLE wikis (
			id TEXT PRIMARY KEY,
			url TEXT NOT NULL UNIQUE,
			api_url TEXT,
			index_url TEXT,
			wiki_name TEXT,
			farm TEXT,
			sitename TEXT,
			lang TEXT,
			db_type TEXT,
			db_version TEXT,
			media_wiki_version TEXT,
			max_page_id INTEGER,
			status TEXT NOT NULL DEFAULT 'pending',
			has_archive INTEGER NOT NULL DEFAULT 0,
			api_available INTEGER NOT NULL DEFAULT 1,
			collection_status TEXT NOT NULL DEFAULT 'pending',
			last_error TEXT,
			last_error_at DATETIME,
			last_success_at DATETIME,
			next_check_at DATETIME,
			consecutive_failures INTEGER NOT NULL DEFAULT 0,
			archive_last_check_at DATETIME,
			archive_last_error TEXT,
			archive_last_error_at DATETIME,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			last_check_at DATETIME,
			is_active INTEGER NOT NULL DEFAULT 1
		)
	`).Error)
	return db
}

func TestCollectorDoesNotRedetectKnownAPIOnRateLimit(t *testing.T) {
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
	wiki := &models.Wiki{
		ID:                  uuid.New(),
		URL:                 server.URL,
		APIURL:              &apiURL,
		IndexURL:            &indexURL,
		Status:              models.WikiStatusOK,
		APIAvailable:        true,
		CollectionStatus:    models.CollectionStatusOK,
		ConsecutiveFailures: 2,
		IsActive:            true,
		CreatedAt:           time.Now(),
		UpdatedAt:           time.Now(),
	}
	require.NoError(t, db.Create(wiki).Error)

	service := NewMediaWikiService(time.Second, "WikiKeeper-Test/1.0")
	collector := NewCollectorService(db, service, &config.Config{})
	err := collector.CollectSingleWiki(context.Background(), wiki.ID)
	require.Error(t, err)
	require.EqualValues(t, 1, requests.Load(), "429 must not trigger API rediscovery requests")

	var statusErr *HTTPStatusError
	require.True(t, errors.As(err, &statusErr))
	require.Equal(t, http.StatusTooManyRequests, statusErr.StatusCode)
	require.Equal(t, "60", statusErr.RetryAfter)

	var updated models.Wiki
	require.NoError(t, db.First(&updated, "id = ?", wiki.ID).Error)
	require.Equal(t, models.WikiStatusOK, updated.Status)
	require.True(t, updated.APIAvailable)
	require.Equal(t, models.CollectionStatusRateLimited, updated.CollectionStatus)
	require.Equal(t, 3, updated.ConsecutiveFailures)
	require.NotNil(t, updated.LastCheckAt)
	require.NotNil(t, updated.NextCheckAt)
	require.WithinDuration(t, updated.LastCheckAt.Add(time.Minute), *updated.NextCheckAt, time.Second)
	require.NotNil(t, updated.LastError)
	require.Contains(t, *updated.LastError, "HTTP 429")
}

func TestCollectorDerivesMissingIndexURLFromKnownAPI(t *testing.T) {
	var requestedPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestedPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"query":{"general":{"sitename":"Example Wiki","lang":"en","generator":"MediaWiki 1.42","dbtype":"mysql","dbversion":"8.0"}}}`))
	}))
	defer server.Close()

	db := setupCollectorTestDB(t)
	apiURL := server.URL + "/Wiki/api.php"
	wiki := &models.Wiki{
		ID:               uuid.New(),
		URL:              server.URL + "/Wiki",
		APIURL:           &apiURL,
		Status:           models.WikiStatusPending,
		CollectionStatus: models.CollectionStatusPending,
		IsActive:         true,
	}
	require.NoError(t, db.Create(wiki).Error)

	collector := NewCollectorService(db, NewMediaWikiService(time.Second, "WikiKeeper-Test/1.0"), &config.Config{})
	err := collector.CollectSingleWiki(context.Background(), wiki.ID)
	require.ErrorContains(t, err, "no such table: wiki_stats")
	require.Equal(t, "/Wiki/api.php", requestedPath)

	var updated models.Wiki
	require.NoError(t, db.First(&updated, "id = ?", wiki.ID).Error)
	require.NotNil(t, updated.IndexURL)
	require.Equal(t, server.URL+"/Wiki/index.php", *updated.IndexURL)
}

func TestCollectorDoesNotRedetectMissingKnownFandomAPI(t *testing.T) {
	var requests atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer server.Close()

	db := setupCollectorTestDB(t)
	apiURL := server.URL + "/api.php"
	indexURL := server.URL + "/index.php"
	wiki := &models.Wiki{
		ID:               uuid.New(),
		URL:              "https://missing.fandom.com",
		APIURL:           &apiURL,
		IndexURL:         &indexURL,
		Status:           models.WikiStatusOK,
		APIAvailable:     true,
		CollectionStatus: models.CollectionStatusOK,
		IsActive:         true,
	}
	require.NoError(t, db.Create(wiki).Error)

	collector := NewCollectorService(db, NewMediaWikiService(time.Second, "WikiKeeper-Test/1.0"), &config.Config{})
	err := collector.CollectSingleWiki(context.Background(), wiki.ID)
	require.Error(t, err)
	require.EqualValues(t, 1, requests.Load())

	var updated models.Wiki
	require.NoError(t, db.First(&updated, "id = ?", wiki.ID).Error)
	require.NotNil(t, updated.NextCheckAt)
	require.NotNil(t, updated.LastCheckAt)
	require.WithinDuration(t, updated.LastCheckAt.Add(terminalFailureInterval), *updated.NextCheckAt, time.Second)
}

func TestTerminalHTTPFailureUsesLongRecheckInterval(t *testing.T) {
	wiki := &models.Wiki{
		URL:                 "https://missing.example",
		ConsecutiveFailures: 1,
	}
	err := &HTTPStatusError{StatusCode: http.StatusGone, Body: "gone"}
	require.Equal(t, terminalFailureInterval, collectionFailureBackoff(wiki, err))
}

func TestSharedCollectorBlocksManualCheckDuringProviderCooldown(t *testing.T) {
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
	}
	for _, wiki := range wikis {
		require.NoError(t, db.Create(wiki).Error)
	}

	limiter := newProviderLimiter(nil, 0, 0, time.Now, sleepContext, func(time.Duration) time.Duration { return 0 })
	collector := NewCollectorService(
		db,
		NewMediaWikiService(time.Second, "WikiKeeper-Test/1.0", limiter),
		&config.Config{},
		limiter,
	)
	require.Error(t, collector.CollectSingleWiki(context.Background(), wikis[0].ID))
	require.Error(t, collector.CollectSingleWiki(context.Background(), wikis[1].ID))
	require.EqualValues(t, 1, requests.Load())

	var deferred models.Wiki
	require.NoError(t, db.First(&deferred, "id = ?", wikis[1].ID).Error)
	require.Equal(t, models.CollectionStatusOK, deferred.CollectionStatus)
	require.Nil(t, deferred.LastCheckAt)
	require.Nil(t, deferred.LastError)
}

func TestRateLimitKeepsNeverVerifiedWikiPending(t *testing.T) {
	db := setupCollectorTestDB(t)
	wiki := &models.Wiki{
		ID:               uuid.New(),
		URL:              "https://new.example",
		Status:           models.WikiStatusPending,
		APIAvailable:     false,
		CollectionStatus: models.CollectionStatusPending,
		IsActive:         true,
	}
	require.NoError(t, db.Create(wiki).Error)

	collector := NewCollectorService(db, nil, &config.Config{})
	collector.UpdateWikiRateLimit(context.Background(), wiki.ID, &HTTPStatusError{
		StatusCode: http.StatusTooManyRequests,
		Body:       "rate limited",
	})

	var updated models.Wiki
	require.NoError(t, db.First(&updated, "id = ?", wiki.ID).Error)
	require.Equal(t, models.WikiStatusPending, updated.Status)
	require.False(t, updated.APIAvailable)
	require.Equal(t, models.CollectionStatusRateLimited, updated.CollectionStatus)
	require.Equal(t, 1, updated.ConsecutiveFailures)
	require.Nil(t, updated.LastSuccessAt)
	require.NotNil(t, updated.NextCheckAt)
}

func TestCollectionErrorChangesVerifiedStatusAtThreshold(t *testing.T) {
	db := setupCollectorTestDB(t)
	wiki := &models.Wiki{
		ID:               uuid.New(),
		URL:              "https://flaky.example",
		Status:           models.WikiStatusOK,
		APIAvailable:     true,
		CollectionStatus: models.CollectionStatusOK,
		IsActive:         true,
	}
	require.NoError(t, db.Create(wiki).Error)

	collector := NewCollectorService(db, nil, &config.Config{})
	for attempt := 1; attempt <= siteFailureThreshold; attempt++ {
		collector.UpdateWikiStatus(context.Background(), wiki.ID, models.WikiStatusError, errors.New("connection failed"))

		var updated models.Wiki
		require.NoError(t, db.First(&updated, "id = ?", wiki.ID).Error)
		require.Equal(t, attempt, updated.ConsecutiveFailures)
		require.Equal(t, models.CollectionStatusError, updated.CollectionStatus)
		require.NotNil(t, updated.LastError)
		require.NotNil(t, updated.LastCheckAt)
		require.NotNil(t, updated.NextCheckAt)
		expectedDelay := baseFailureBackoff * time.Duration(1<<(attempt-1))
		require.WithinDuration(t, updated.LastCheckAt.Add(expectedDelay), *updated.NextCheckAt, time.Second)
		if attempt < siteFailureThreshold {
			require.Equal(t, models.WikiStatusOK, updated.Status)
			require.True(t, updated.APIAvailable)
		} else {
			require.Equal(t, models.WikiStatusError, updated.Status)
			require.False(t, updated.APIAvailable)
		}
	}
}

func TestCollectionSuccessClearsBackoffState(t *testing.T) {
	lastError := "HTTP 429"
	previous := time.Date(2026, time.July, 30, 5, 0, 0, 0, time.UTC)
	now := previous.Add(24 * time.Hour)
	wiki := &models.Wiki{
		Status:              models.WikiStatusOK,
		APIAvailable:        true,
		CollectionStatus:    models.CollectionStatusRateLimited,
		LastError:           &lastError,
		LastErrorAt:         &previous,
		LastSuccessAt:       &previous,
		ConsecutiveFailures: 7,
	}

	markWikiCollectionSuccess(wiki, now)

	require.Equal(t, models.WikiStatusOK, wiki.Status)
	require.Equal(t, models.CollectionStatusOK, wiki.CollectionStatus)
	require.True(t, wiki.APIAvailable)
	require.Equal(t, 0, wiki.ConsecutiveFailures)
	require.Nil(t, wiki.LastError)
	require.Nil(t, wiki.LastErrorAt)
	require.Equal(t, now, *wiki.LastSuccessAt)
	require.Equal(t, now.Add(collectionInterval), *wiki.NextCheckAt)
}

func TestFandomCollectionSuccessUsesStandardCadence(t *testing.T) {
	now := time.Date(2026, time.August, 1, 12, 0, 0, 0, time.UTC)
	wiki := &models.Wiki{URL: "https://example.fandom.com"}

	markWikiCollectionSuccess(wiki, now)

	require.Equal(t, now.Add(collectionInterval), *wiki.NextCheckAt)
}

func TestCollectionFailureBackoffIsCapped(t *testing.T) {
	wiki := &models.Wiki{
		URL:                 "https://flaky.example",
		ConsecutiveFailures: 20,
	}
	require.Equal(t, maxFailureBackoff, collectionFailureBackoff(wiki, errors.New("timeout")))
}
