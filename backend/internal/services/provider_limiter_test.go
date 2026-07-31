package services

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"wikikeeper-backend/internal/models"
	"wikikeeper-backend/internal/repository"
)

func TestProviderLimiterRestoresPersistedCooldownAfterRestart(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.ProviderRateLimit{}))
	repo := repository.NewProviderRateLimitRepository(db)

	now := time.Date(2026, time.July, 31, 16, 0, 0, 0, time.UTC)
	newLimiter := func() *ProviderLimiter {
		return newProviderLimiter(
			repo,
			0,
			0,
			func() time.Time { return now },
			sleepContext,
			func(time.Duration) time.Duration { return 0 },
		)
	}

	rateLimitErr := &HTTPStatusError{StatusCode: http.StatusTooManyRequests, RetryAfter: "120"}
	err = newLimiter().Run(context.Background(), "https://one.fandom.com", func() error { return rateLimitErr })
	limitErr, ok := asProviderLimitError(err)
	require.True(t, ok)
	require.True(t, limitErr.Attempted)
	require.Equal(t, now.Add(2*time.Minute), limitErr.RetryAt)

	called := false
	restarted := newLimiter()
	err = restarted.Run(context.Background(), "https://two.wikia.com", func() error {
		called = true
		return nil
	})
	require.False(t, called)
	limitErr, ok = asProviderLimitError(err)
	require.True(t, ok)
	require.False(t, limitErr.Attempted)
	require.Equal(t, now.Add(2*time.Minute), limitErr.RetryAt)

	now = now.Add(2 * time.Minute)
	require.NoError(t, restarted.Run(context.Background(), "https://two.wikia.com", func() error {
		called = true
		return nil
	}))
	require.True(t, called)

	_, err = repo.Get(context.Background(), "fandom.com")
	require.True(t, errors.Is(err, gorm.ErrRecordNotFound))
}
