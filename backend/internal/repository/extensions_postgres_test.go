package repository

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"wikikeeper-backend/internal/database"
	"wikikeeper-backend/internal/models"
)

func TestExtensionSetMigrationPostgres(t *testing.T) {
	dsn := os.Getenv("WIKIKEEPER_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("set WIKIKEEPER_TEST_POSTGRES_DSN to run PostgreSQL migration tests")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, database.RunMigrations(db))
	repo := NewExtensionsRepository(db)
	ctx := context.Background()
	marker := uuid.NewString()
	items := []models.WikiExtensionItem{
		{ExtType: "other", Name: "Concurrent-" + marker, URL: strptr("https://example.org/extension")},
		{ExtType: "skin", Name: "Skin-" + marker, Version: strptr("1.0")},
	}

	const workers = 8
	var wg sync.WaitGroup
	errs := make(chan error, workers)
	for i := range workers {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			wikiID := uuid.New()
			if err := db.Create(&models.Wiki{
				ID: wikiID, URL: fmt.Sprintf("https://%d-%s.example.org", i, marker),
				Status: models.WikiStatusOK, IsActive: true,
			}).Error; err != nil {
				errs <- err
				return
			}
			version := "MediaWiki 1.43.8"
			errs <- repo.CreateSnapshot(ctx, &models.WikiExtensionsSnapshot{
				ID: uuid.New(), WikiID: wikiID, SnapshotAt: time.Now().UTC(),
				MediaWikiVersion: &version, Items: items,
			})
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}

	hash, _, err := canonicalExtensionSet(items)
	require.NoError(t, err)
	var matchingSets int64
	require.NoError(t, db.Table("wiki_extension_sets").
		Where("content_hash = ?", extensionSetHash(hash[:])).Count(&matchingSets).Error)
	require.EqualValues(t, 1, matchingSets)

	legacyWikiID := uuid.New()
	legacySnapshotID := uuid.New()
	require.NoError(t, db.Create(&models.Wiki{
		ID: legacyWikiID, URL: "https://legacy-" + marker + ".example.org",
		Status: models.WikiStatusOK, IsActive: true,
	}).Error)
	require.NoError(t, db.Exec(`
		INSERT INTO wiki_extensions_snapshots(id, wiki_id, snapshot_at)
		VALUES (?, ?, NOW())
	`, legacySnapshotID, legacyWikiID).Error)
	require.NoError(t, db.Exec(`
		INSERT INTO wiki_extension_items(snapshot_id, ext_type, name)
		VALUES (?, 'other', ?)
	`, legacySnapshotID, "Legacy-"+marker).Error)
	interleavedIDs := []uuid.UUID{uuid.New(), uuid.New()}
	for i, snapshotID := range interleavedIDs {
		wikiID := uuid.New()
		require.NoError(t, db.Create(&models.Wiki{
			ID: wikiID, URL: fmt.Sprintf("https://interleaved-%d-%s.example.org", i, marker),
			Status: models.WikiStatusOK, IsActive: true,
		}).Error)
		require.NoError(t, db.Exec(`
			INSERT INTO wiki_extensions_snapshots(id, wiki_id, snapshot_at)
			VALUES (?, ?, NOW())
		`, snapshotID, wikiID).Error)
	}
	require.NoError(t, db.Exec(`
		INSERT INTO wiki_extension_items(snapshot_id, ext_type, name)
		SELECT CASE WHEN number % 2 = 0 THEN ?::uuid ELSE ?::uuid END,
		       'other', 'Interleaved-' || ((number + 1) / 2)::text
		FROM generate_series(1, 600) AS number
	`, interleavedIDs[0], interleavedIDs[1]).Error)

	var migrated int
	for range 20 {
		result, err := repo.BackfillExtensionSets(ctx, 1)
		require.NoError(t, err)
		migrated += result.Snapshots
		if result.Snapshots == 0 && result.Items == 0 {
			break
		}
	}
	require.GreaterOrEqual(t, migrated, 3)
	remaining, err := repo.RemainingLegacyExtensionSnapshots(ctx)
	require.NoError(t, err)
	require.Zero(t, remaining)

	require.NoError(t, repo.FinalizeExtensionSetMigration(ctx))
	var legacyItems int64
	require.NoError(t, db.Table("wiki_extension_items").Count(&legacyItems).Error)
	require.Zero(t, legacyItems)
	var legacyWrites bool
	require.NoError(t, db.Raw(`SELECT legacy_writes FROM extension_storage_state WHERE singleton`).Scan(&legacyWrites).Error)
	require.False(t, legacyWrites)

	postWikiID := uuid.New()
	require.NoError(t, db.Create(&models.Wiki{
		ID: postWikiID, URL: "https://post-" + marker + ".example.org",
		Status: models.WikiStatusOK, IsActive: true,
	}).Error)
	version := "MediaWiki 1.43.8"
	require.NoError(t, repo.CreateSnapshot(ctx, &models.WikiExtensionsSnapshot{
		ID: uuid.New(), WikiID: postWikiID, SnapshotAt: time.Now().UTC(),
		MediaWikiVersion: &version, Items: items,
	}))
	require.NoError(t, db.Table("wiki_extension_items").Count(&legacyItems).Error)
	require.Zero(t, legacyItems)

	wikis, total, err := repo.GetWikisUsingExtension(ctx, "Concurrent-"+marker, ExtensionWikisListOptions{Page: 1, Limit: 20})
	require.NoError(t, err)
	require.EqualValues(t, workers+1, total)
	require.Len(t, wikis, workers+1)
	versions, versionTotal, err := repo.GetExtensionVersionDistribution(ctx, "Concurrent-"+marker)
	require.NoError(t, err)
	require.EqualValues(t, workers+1, versionTotal)
	require.Len(t, versions, 1)
	require.NoError(t, repo.RefreshExtensionStatsMaterializedView(ctx))
	stats, _, err := repo.GetAllExtensionsStats(ctx, GetAllExtensionsStatsOptions{Page: 1, Limit: 500})
	require.NoError(t, err)
	found := false
	for _, stat := range stats {
		if stat.Name == "Concurrent-"+marker {
			found = true
			require.EqualValues(t, workers+1, stat.Count)
		}
	}
	require.True(t, found)
}
