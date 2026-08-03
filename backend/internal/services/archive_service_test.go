package services

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"wikikeeper-backend/internal/models"
	"wikikeeper-backend/internal/repository"
)

func TestCollectArchivesRecordsFailure(t *testing.T) {
	db := setupCollectorTestDB(t)
	ctx := context.Background()
	statsSuccess := time.Date(2026, time.June, 16, 3, 38, 50, 0, time.UTC)
	wiki := &models.Wiki{
		ID:            uuid.New(),
		URL:           "https://example.org",
		Status:        models.WikiStatusOK,
		LastSuccessAt: &statsSuccess,
		IsActive:      true,
	}
	require.NoError(t, db.Create(wiki).Error)

	service := NewArchiveService(time.Second, "WikiKeeper-Test/1.0")
	_, _, _, err := service.CollectArchives(ctx, db, wiki.ID, "", "")
	require.ErrorContains(t, err, "API URL is required")

	stored, err := repository.NewWikiRepository(db).GetByID(ctx, wiki.ID)
	require.NoError(t, err)
	require.NotNil(t, stored.ArchiveLastCheckAt)
	require.NotNil(t, stored.ArchiveLastErrorAt)
	require.Equal(t, stored.ArchiveLastCheckAt, stored.ArchiveLastErrorAt)
	require.Equal(t, "API URL is required", *stored.ArchiveLastError)
	require.Equal(t, statsSuccess, *stored.LastSuccessAt)
}
