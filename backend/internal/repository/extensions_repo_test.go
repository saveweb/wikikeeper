package repository

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"wikikeeper-backend/internal/models"
)

func setupExtensionsTestDB(t *testing.T) (*ExtensionsRepository, uuid.UUID) {
	t.Helper()
	db := setupTestDB(t)
	require.NoError(t, db.Exec(`
		CREATE TABLE wiki_extensions_snapshots (
			id TEXT PRIMARY KEY,
			wiki_id TEXT NOT NULL,
			snapshot_at DATETIME NOT NULL,
			valid_until DATETIME,
			mediawiki_version TEXT,
			FOREIGN KEY (wiki_id) REFERENCES wikis(id) ON DELETE CASCADE
		)
	`).Error)
	require.NoError(t, db.Exec(`
		CREATE TABLE wiki_extension_items (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			snapshot_id TEXT NOT NULL,
			ext_type TEXT NOT NULL,
			name TEXT NOT NULL,
			url TEXT,
			version TEXT,
			license_name TEXT,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (snapshot_id) REFERENCES wiki_extensions_snapshots(id) ON DELETE CASCADE
		)
	`).Error)

	wikiID := uuid.New()
	require.NoError(t, db.Create(&models.Wiki{
		ID:       wikiID,
		URL:      "https://example.org",
		Status:   models.WikiStatusOK,
		IsActive: true,
	}).Error)
	return NewExtensionsRepository(db), wikiID
}

func TestGetSnapshotsInTimeRangeUsesValidityOverlap(t *testing.T) {
	repo, wikiID := setupExtensionsTestDB(t)
	ctx := context.Background()
	from := time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, time.July, 31, 23, 59, 0, 0, time.UTC)

	beforeRange := from.AddDate(0, -1, 0)
	overlapsUntil := from.Add(14 * 24 * time.Hour)
	endedBeforeRange := from.Add(-time.Hour)
	activeInRange := from.Add(20 * 24 * time.Hour)
	afterRange := to.Add(time.Hour)

	snapshots := []*models.WikiExtensionsSnapshot{
		{ID: uuid.New(), WikiID: wikiID, SnapshotAt: beforeRange.AddDate(0, -1, 0), ValidUntil: &endedBeforeRange},
		{ID: uuid.New(), WikiID: wikiID, SnapshotAt: beforeRange, ValidUntil: &overlapsUntil},
		{ID: uuid.New(), WikiID: wikiID, SnapshotAt: activeInRange},
		{ID: uuid.New(), WikiID: wikiID, SnapshotAt: afterRange},
	}
	for _, snapshot := range snapshots {
		require.NoError(t, repo.CreateSnapshot(ctx, snapshot))
	}

	got, err := repo.GetSnapshotsInTimeRange(ctx, wikiID, from, to)
	require.NoError(t, err)
	require.Len(t, got, 2)
	require.Equal(t, snapshots[2].ID, got[0].ID)
	require.Equal(t, snapshots[1].ID, got[1].ID)
}
