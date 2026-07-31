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

func TestCollectorDoesNotRedetectKnownAPIOnRateLimit(t *testing.T) {
	var requests atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		w.Header().Set("Retry-After", "60")
		http.Error(w, "rate limited", http.StatusTooManyRequests)
	}))
	defer server.Close()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Exec(`
		CREATE TABLE wikis (
			id TEXT PRIMARY KEY,
			url TEXT NOT NULL UNIQUE,
			api_url TEXT,
			index_url TEXT,
			wiki_name TEXT,
			sitename TEXT,
			lang TEXT,
			db_type TEXT,
			db_version TEXT,
			media_wiki_version TEXT,
			max_page_id INTEGER,
			status TEXT NOT NULL DEFAULT 'pending',
			has_archive INTEGER NOT NULL DEFAULT 0,
			api_available INTEGER NOT NULL DEFAULT 1,
			last_error TEXT,
			last_error_at DATETIME,
			archive_last_check_at DATETIME,
			archive_last_error TEXT,
			archive_last_error_at DATETIME,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			last_check_at DATETIME,
			is_active INTEGER NOT NULL DEFAULT 1
		)
	`).Error)

	apiURL := server.URL + "/api.php"
	indexURL := server.URL + "/index.php"
	wiki := &models.Wiki{
		ID:        uuid.New(),
		URL:       server.URL,
		APIURL:    &apiURL,
		IndexURL:  &indexURL,
		Status:    models.WikiStatusOK,
		IsActive:  true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	require.NoError(t, db.Create(wiki).Error)

	service := NewMediaWikiService(time.Second, "WikiKeeper-Test/1.0")
	collector := NewCollectorService(db, service, &config.Config{})
	err = collector.CollectSingleWiki(context.Background(), wiki.ID)
	require.Error(t, err)
	require.EqualValues(t, 1, requests.Load(), "429 must not trigger API rediscovery requests")

	var statusErr *HTTPStatusError
	require.True(t, errors.As(err, &statusErr))
	require.Equal(t, http.StatusTooManyRequests, statusErr.StatusCode)
	require.Equal(t, "60", statusErr.RetryAfter)

	var updated models.Wiki
	require.NoError(t, db.First(&updated, "id = ?", wiki.ID).Error)
	require.Equal(t, models.WikiStatusError, updated.Status)
	require.NotNil(t, updated.LastError)
	require.Contains(t, *updated.LastError, "HTTP 429")
}
