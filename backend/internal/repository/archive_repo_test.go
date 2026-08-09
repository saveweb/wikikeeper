package repository

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"wikikeeper-backend/internal/models"
)

func TestGetLatestArchiveByWikiIDUsesDumpDate(t *testing.T) {
	db := setupTestDB(t)
	repo := NewArchiveRepository(db)
	ctx := context.Background()
	wikiID := uuid.New()
	require.NoError(t, db.Create(&models.Wiki{
		ID: wikiID, URL: "https://example.org", Status: models.WikiStatusOK,
	}).Error)

	olderDump := time.Date(2025, time.January, 1, 0, 0, 0, 0, time.UTC)
	newerDump := time.Date(2025, time.February, 1, 0, 0, 0, 0, time.UTC)
	earlierUpload := newerDump.Add(24 * time.Hour)
	laterUpload := newerDump.Add(48 * time.Hour)
	require.NoError(t, repo.Create(ctx, &models.WikiArchive{
		ID: uuid.New(), WikiID: wikiID, IAIdentifier: "newer-upload",
		AddedDate: &laterUpload, DumpDate: &olderDump,
	}))
	require.NoError(t, repo.Create(ctx, &models.WikiArchive{
		ID: uuid.New(), WikiID: wikiID, IAIdentifier: "newer-dump",
		AddedDate: &earlierUpload, DumpDate: &newerDump,
	}))

	archive, err := repo.GetLatestByWikiID(ctx, wikiID)
	require.NoError(t, err)
	require.Equal(t, "newer-dump", archive.IAIdentifier)

	_, err = repo.GetLatestByWikiID(ctx, uuid.New())
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)
}
