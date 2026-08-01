package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"wikikeeper-backend/internal/models"
)

type ExtensionSetBackfillResult struct {
	Snapshots int
	Items     int
}

// BackfillExtensionSets migrates one resumable batch of legacy snapshots.
func (r *ExtensionsRepository) BackfillExtensionSets(ctx context.Context, batchSize int) (ExtensionSetBackfillResult, error) {
	if batchSize < 1 {
		return ExtensionSetBackfillResult{}, fmt.Errorf("batch size must be positive")
	}

	result := ExtensionSetBackfillResult{}
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var snapshots []models.WikiExtensionsSnapshot
		if err := tx.Select("id").
			Where("extension_set_id IS NULL").
			// Snapshot creation time tracks the physical insertion order of legacy
			// members. Keeping each batch local avoids random reads across the
			// 100+ GB legacy table.
			Order("snapshot_at, id").
			Limit(batchSize).
			Find(&snapshots).Error; err != nil {
			return err
		}
		if len(snapshots) == 0 {
			return nil
		}

		ids := make([]uuid.UUID, 0, len(snapshots))
		itemsBySnapshot := make(map[uuid.UUID][]models.WikiExtensionItem, len(snapshots))
		for _, snapshot := range snapshots {
			ids = append(ids, snapshot.ID)
			itemsBySnapshot[snapshot.ID] = nil
		}
		var items []models.WikiExtensionItem
		if err := tx.Where("snapshot_id IN ?", ids).
			Order("snapshot_id, ext_type, name").
			Find(&items).Error; err != nil {
			return err
		}
		for _, item := range items {
			itemsBySnapshot[item.SnapshotID] = append(itemsBySnapshot[item.SnapshotID], item)
		}

		updates := make(map[int64][]uuid.UUID)
		for _, snapshot := range snapshots {
			setID, err := ensureExtensionSet(tx, itemsBySnapshot[snapshot.ID])
			if err != nil {
				return fmt.Errorf("snapshot %s: %w", snapshot.ID, err)
			}
			updates[setID] = append(updates[setID], snapshot.ID)
		}
		for setID, snapshotIDs := range updates {
			if err := tx.Model(&models.WikiExtensionsSnapshot{}).
				Where("id IN ? AND extension_set_id IS NULL", snapshotIDs).
				UpdateColumn("extension_set_id", setID).Error; err != nil {
				return err
			}
		}

		result.Snapshots = len(snapshots)
		result.Items = len(items)
		return nil
	})
	return result, err
}

func (r *ExtensionsRepository) RemainingLegacyExtensionSnapshots(ctx context.Context) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&models.WikiExtensionsSnapshot{}).
		Where("extension_set_id IS NULL").Count(&count).Error
	return count, err
}

func (r *ExtensionsRepository) LegacyExtensionWritesEnabled(ctx context.Context) (bool, error) {
	return extensionLegacyWrites(r.db.WithContext(ctx))
}

// FinalizeExtensionSetMigration prevents further legacy writes, rebuilds the
// statistics view from extension sets, and releases the legacy table storage.
func (r *ExtensionsRepository) FinalizeExtensionSetMigration(ctx context.Context) error {
	remaining, err := r.RemainingLegacyExtensionSnapshots(ctx)
	if err != nil {
		return err
	}
	if remaining != 0 {
		return fmt.Errorf("cannot finalize extension sets: %d legacy snapshots remain", remaining)
	}
	if r.db.Dialector.Name() != "postgres" {
		return fmt.Errorf("extension set finalization requires PostgreSQL")
	}

	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec(`
			UPDATE extension_storage_state
			SET legacy_writes = FALSE, updated_at = NOW()
			WHERE singleton = TRUE
		`).Error; err != nil {
			return err
		}
		if err := tx.Exec(`
			DROP MATERIALIZED VIEW IF EXISTS mv_extension_stats;
			CREATE MATERIALIZED VIEW mv_extension_stats AS
			SELECT item.name, COUNT(*) AS count
			FROM wiki_extension_set_items item
			JOIN wiki_extensions_snapshots snapshot
			  ON snapshot.extension_set_id = item.set_id
			WHERE snapshot.valid_until IS NULL
			GROUP BY item.name
			WITH DATA;
			CREATE UNIQUE INDEX idx_mv_extension_stats_name
			ON mv_extension_stats(name);
			CREATE INDEX idx_mv_extension_stats_count
			ON mv_extension_stats(count DESC, name);
		`).Error; err != nil {
			return err
		}
		return tx.Exec(`TRUNCATE TABLE wiki_extension_items RESTART IDENTITY`).Error
	})
}
