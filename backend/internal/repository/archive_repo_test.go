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

func TestGetLatestDumpDatesByWikiIDs(t *testing.T) {
	db := setupTestDB(t)
	repo := NewArchiveRepository(db)
	ctx := context.Background()
	wikiID := uuid.New()
	otherWikiID := uuid.New()
	for id, rawURL := range map[uuid.UUID]string{
		wikiID:      "https://example.org",
		otherWikiID: "https://other.example.org",
	} {
		require.NoError(t, db.Create(&models.Wiki{
			ID: id, URL: rawURL, Status: models.WikiStatusOK,
		}).Error)
	}

	oldXML := time.Date(2024, time.January, 1, 0, 0, 0, 0, time.UTC)
	latestXML := time.Date(2025, time.January, 1, 0, 0, 0, 0, time.UTC)
	latestImages := time.Date(2025, time.February, 1, 0, 0, 0, 0, time.UTC)
	newerWithoutContent := time.Date(2025, time.March, 1, 0, 0, 0, 0, time.UTC)
	archives := []*models.WikiArchive{
		{ID: uuid.New(), WikiID: wikiID, IAIdentifier: "old-xml", DumpDate: &oldXML, HasXMLCurrent: true},
		{ID: uuid.New(), WikiID: wikiID, IAIdentifier: "latest-xml", DumpDate: &latestXML, HasXMLHistory: true},
		{ID: uuid.New(), WikiID: wikiID, IAIdentifier: "latest-images", DumpDate: &latestImages, HasImagesDump: true},
		{ID: uuid.New(), WikiID: wikiID, IAIdentifier: "newer-without-content", DumpDate: &newerWithoutContent},
		{ID: uuid.New(), WikiID: otherWikiID, IAIdentifier: "other-wiki", DumpDate: &newerWithoutContent, HasImagesDump: true},
	}
	for _, archive := range archives {
		require.NoError(t, repo.Create(ctx, archive))
	}

	dates, err := repo.GetLatestDumpDatesByWikiIDs(ctx, []uuid.UUID{wikiID})
	require.NoError(t, err)
	require.Len(t, dates, 1)
	require.Equal(t, latestXML, *dates[wikiID].LatestXMLDumpAt)
	require.Equal(t, latestImages, *dates[wikiID].LatestImagesDumpAt)

	empty, err := repo.GetLatestDumpDatesByWikiIDs(ctx, nil)
	require.NoError(t, err)
	require.Empty(t, empty)
}
