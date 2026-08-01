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
	if r.db.Dialector.Name() == "postgres" {
		return r.backfillExtensionSetsSequential(ctx, batchSize)
	}
	return r.backfillExtensionSetsBySnapshot(ctx, batchSize)
}

func (r *ExtensionsRepository) backfillExtensionSetsBySnapshot(ctx context.Context, batchSize int) (ExtensionSetBackfillResult, error) {
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

func (r *ExtensionsRepository) backfillExtensionSetsSequential(ctx context.Context, batchSize int) (ExtensionSetBackfillResult, error) {
	result := ExtensionSetBackfillResult{}
	itemLimit := batchSize * 256
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var cursor int64
		if err := tx.Raw(`
			SELECT backfill_cursor
			FROM extension_storage_state
			WHERE singleton = TRUE
			FOR UPDATE
		`).Scan(&cursor).Error; err != nil {
			return err
		}

		var window []models.WikiExtensionItem
		if err := tx.Select("id", "snapshot_id").Where("id > ?", cursor).
			Order("id").Limit(itemLimit).Find(&window).Error; err != nil {
			return err
		}
		if len(window) == 0 {
			return r.backfillEmptyExtensionSets(tx, batchSize, &result)
		}

		order := make([]uuid.UUID, 0, batchSize)
		seen := make(map[uuid.UUID]bool, batchSize)
		for _, item := range window {
			if !seen[item.SnapshotID] {
				seen[item.SnapshotID] = true
				order = append(order, item.SnapshotID)
			}
		}

		// Snapshots can interleave when collectors insert concurrently. Fetch
		// complete snapshots after discovering them from the sequential window.
		var processItems []models.WikiExtensionItem
		if err := tx.Where("snapshot_id IN ?", order).
			Order("snapshot_id, ext_type, name").Find(&processItems).Error; err != nil {
			return err
		}
		itemsBySnapshot := make(map[uuid.UUID][]models.WikiExtensionItem, len(order))
		for _, item := range processItems {
			itemsBySnapshot[item.SnapshotID] = append(itemsBySnapshot[item.SnapshotID], item)
		}

		type snapshotState struct {
			ID             uuid.UUID
			ExtensionSetID *int64
		}
		var states []snapshotState
		if err := tx.Model(&models.WikiExtensionsSnapshot{}).
			Select("id", "extension_set_id").Where("id IN ?", order).Find(&states).Error; err != nil {
			return err
		}
		needsBackfill := make(map[uuid.UUID]bool, len(states))
		for _, state := range states {
			needsBackfill[state.ID] = state.ExtensionSetID == nil
		}

		updates := make(map[int64][]uuid.UUID)
		for _, snapshotID := range order {
			if !needsBackfill[snapshotID] {
				continue
			}
			setID, err := ensureExtensionSet(tx, itemsBySnapshot[snapshotID])
			if err != nil {
				return fmt.Errorf("snapshot %s: %w", snapshotID, err)
			}
			updates[setID] = append(updates[setID], snapshotID)
			result.Snapshots++
		}
		for setID, snapshotIDs := range updates {
			if err := tx.Model(&models.WikiExtensionsSnapshot{}).
				Where("id IN ? AND extension_set_id IS NULL", snapshotIDs).
				UpdateColumn("extension_set_id", setID).Error; err != nil {
				return err
			}
		}

		result.Items = len(processItems)
		cursor = window[len(window)-1].ID
		return tx.Exec(`
			UPDATE extension_storage_state
			SET backfill_cursor = ?, updated_at = NOW()
			WHERE singleton = TRUE
		`, cursor).Error
	})
	return result, err
}

func (r *ExtensionsRepository) backfillEmptyExtensionSets(tx *gorm.DB, batchSize int, result *ExtensionSetBackfillResult) error {
	var snapshots []models.WikiExtensionsSnapshot
	if err := tx.Select("id").Where("extension_set_id IS NULL").Order("snapshot_at, id").Limit(batchSize).Find(&snapshots).Error; err != nil {
		return err
	}
	if len(snapshots) == 0 {
		return nil
	}
	setID, err := ensureExtensionSet(tx, nil)
	if err != nil {
		return err
	}
	ids := make([]uuid.UUID, 0, len(snapshots))
	for _, snapshot := range snapshots {
		ids = append(ids, snapshot.ID)
	}
	if err := tx.Model(&models.WikiExtensionsSnapshot{}).
		Where("id IN ? AND extension_set_id IS NULL", ids).
		UpdateColumn("extension_set_id", setID).Error; err != nil {
		return err
	}
	result.Snapshots = len(ids)
	return nil
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
