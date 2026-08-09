package services

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"wikikeeper-backend/internal/models"
	"wikikeeper-backend/internal/repository"
)

func TestScrapeSearchResultOriginalURL(t *testing.T) {
	var result scrapeSearchResult
	require.NoError(t, json.Unmarshal([]byte(`{
		"items": [
			{"identifier": "single", "originalurl": "https://www.altlinux.org/api.php"},
			{"identifier": "multiple", "originalurl": [
				"https://www.altlinux.org/api.php",
				"https://ru.altlinux.org/api.php",
				"https://wiki.altlinux.org/api.php"
			]},
			{"identifier": "missing"},
			{"identifier": "null", "originalurl": null}
		]
	}`), &result))

	require.Equal(t, stringList{"https://www.altlinux.org/api.php"}, result.Items[0].OriginalURL)
	require.Equal(t, stringList{
		"https://www.altlinux.org/api.php",
		"https://ru.altlinux.org/api.php",
		"https://wiki.altlinux.org/api.php",
	}, result.Items[1].OriginalURL)
	require.Nil(t, result.Items[2].OriginalURL)
	require.Nil(t, result.Items[3].OriginalURL)
}

func TestCheckFileContentsDetectsRedirectsList(t *testing.T) {
	var metadata archiveMetadata
	require.NoError(t, json.Unmarshal([]byte(`{
		"files": [{"name": "osm.bio-20260809-redirects.jsonl.zst"}]
	}`), &metadata))

	info := &ArchiveInfo{}
	NewArchiveService(time.Second, "WikiKeeper-Test/1.0").checkFileContents(info, metadata.Files)

	require.True(t, info.HasRedirectsList)
}

func TestCheckFileContentsIgnoresRedirectsChecksum(t *testing.T) {
	var metadata archiveMetadata
	require.NoError(t, json.Unmarshal([]byte(`{
		"files": [{"name": "osm.bio-20260809-redirects.jsonl.zst.sha1"}]
	}`), &metadata))

	info := &ArchiveInfo{}
	NewArchiveService(time.Second, "WikiKeeper-Test/1.0").checkFileContents(info, metadata.Files)

	require.False(t, info.HasRedirectsList)
}

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
