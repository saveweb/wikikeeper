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
			extension_set_id INTEGER,
			FOREIGN KEY (wiki_id) REFERENCES wikis(id) ON DELETE CASCADE
		)
	`).Error)
	require.NoError(t, db.Exec(`
		CREATE TABLE wiki_extension_sets (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			content_hash BLOB NOT NULL UNIQUE,
			item_count INTEGER NOT NULL
		)
	`).Error)
	require.NoError(t, db.Exec(`
		CREATE TABLE wiki_extension_set_items (
			set_id INTEGER NOT NULL,
			ext_type TEXT NOT NULL,
			name TEXT NOT NULL,
			url TEXT,
			version TEXT,
			license_name TEXT,
			PRIMARY KEY (set_id, ext_type, name),
			FOREIGN KEY (set_id) REFERENCES wiki_extension_sets(id) ON DELETE CASCADE
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
	require.NoError(t, db.Exec(`
		CREATE TABLE extension_storage_state (
			singleton INTEGER PRIMARY KEY,
			legacy_writes INTEGER NOT NULL
		)
	`).Error)
	require.NoError(t, db.Exec(`INSERT INTO extension_storage_state(singleton, legacy_writes) VALUES (1, 1)`).Error)

	wikiID := uuid.New()
	require.NoError(t, db.Create(&models.Wiki{
		ID:       wikiID,
		URL:      "https://example.org",
		Status:   models.WikiStatusOK,
		IsActive: true,
	}).Error)
	return NewExtensionsRepository(db), wikiID
}

func strptr(value string) *string { return &value }

func TestCreateSnapshotReusesContentAddressedSet(t *testing.T) {
	repo, wikiID := setupExtensionsTestDB(t)
	ctx := context.Background()
	version := "MediaWiki 1.43.8"
	firstUntil := time.Now().UTC()
	items := []models.WikiExtensionItem{
		{ExtType: "skin", Name: "FandomDesktop", Version: strptr("1.0")},
		{ExtType: "other", Name: "ParserFunctions", URL: strptr("https://example.org/parser")},
	}
	first := &models.WikiExtensionsSnapshot{
		ID: uuid.New(), WikiID: wikiID, SnapshotAt: firstUntil.Add(-time.Hour),
		ValidUntil: &firstUntil, MediaWikiVersion: &version, Items: items,
	}
	second := &models.WikiExtensionsSnapshot{
		ID: uuid.New(), WikiID: wikiID, SnapshotAt: firstUntil,
		MediaWikiVersion: &version, Items: []models.WikiExtensionItem{items[1], items[0]},
	}
	firstHash, _, err := canonicalExtensionSet(first.Items)
	require.NoError(t, err)
	secondHash, _, err := canonicalExtensionSet(second.Items)
	require.NoError(t, err)
	require.Equal(t, firstHash, secondHash)
	require.NoError(t, repo.CreateSnapshot(ctx, first))
	require.NoError(t, repo.CreateSnapshot(ctx, second))
	require.NotNil(t, first.ExtensionSetID)
	require.NotNil(t, second.ExtensionSetID)
	require.Equal(t, *first.ExtensionSetID, *second.ExtensionSetID)

	var setCount, setItemCount, legacyItemCount int64
	require.NoError(t, repo.db.Table("wiki_extension_sets").Count(&setCount).Error)
	require.NoError(t, repo.db.Table("wiki_extension_set_items").Count(&setItemCount).Error)
	require.NoError(t, repo.db.Table("wiki_extension_items").Count(&legacyItemCount).Error)
	require.EqualValues(t, 1, setCount)
	require.EqualValues(t, 2, setItemCount)
	require.EqualValues(t, 4, legacyItemCount)

	require.NoError(t, repo.db.Exec(`UPDATE extension_storage_state SET legacy_writes = 0`).Error)
	require.NoError(t, repo.db.Exec(`DELETE FROM wiki_extension_items`).Error)
	got, err := repo.GetLatestSnapshot(ctx, wikiID)
	require.NoError(t, err)
	require.Len(t, got.Items, 2)
	require.Equal(t, "ParserFunctions", got.Items[0].Name)
}

func TestBackfillExtensionSetsIsResumable(t *testing.T) {
	repo, wikiID := setupExtensionsTestDB(t)
	ctx := context.Background()
	snapshotID := uuid.New()
	require.NoError(t, repo.db.Exec(`
		INSERT INTO wiki_extensions_snapshots(id, wiki_id, snapshot_at)
		VALUES (?, ?, ?)
	`, snapshotID, wikiID, time.Now().UTC()).Error)
	require.NoError(t, repo.db.Exec(`
		INSERT INTO wiki_extension_items(snapshot_id, ext_type, name)
		VALUES (?, 'other', 'Cite')
	`, snapshotID).Error)

	result, err := repo.BackfillExtensionSets(ctx, 10)
	require.NoError(t, err)
	require.Equal(t, 1, result.Snapshots)
	require.Equal(t, 1, result.Items)
	result, err = repo.BackfillExtensionSets(ctx, 10)
	require.NoError(t, err)
	require.Zero(t, result.Snapshots)

	got, err := repo.GetLatestSnapshot(ctx, wikiID)
	require.NoError(t, err)
	require.NotNil(t, got.ExtensionSetID)
	require.Len(t, got.Items, 1)
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
