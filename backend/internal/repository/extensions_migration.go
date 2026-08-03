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

type extensionSetBackfillGroup struct {
	items       []models.WikiExtensionItem
	snapshotIDs []uuid.UUID
}

func groupExtensionSnapshots(order []uuid.UUID, itemsBySnapshot map[uuid.UUID][]models.WikiExtensionItem, include func(uuid.UUID) bool) (map[[32]byte]*extensionSetBackfillGroup, error) {
	groups := make(map[[32]byte]*extensionSetBackfillGroup)
	for _, snapshotID := range order {
		if !include(snapshotID) {
			continue
		}
		items := itemsBySnapshot[snapshotID]
		hash, _, err := canonicalExtensionSet(items)
		if err != nil {
			return nil, err
		}
		group := groups[hash]
		if group == nil {
			group = &extensionSetBackfillGroup{items: items}
			groups[hash] = group
		}
		group.snapshotIDs = append(group.snapshotIDs, snapshotID)
	}
	return groups, nil
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

		groups, err := groupExtensionSnapshots(ids, itemsBySnapshot, func(uuid.UUID) bool { return true })
		if err != nil {
			return err
		}
		updates := make(map[int64][]uuid.UUID)
		for _, group := range groups {
			setID, err := r.ensureExtensionSet(tx, group.items)
			if err != nil {
				return err
			}
			updates[setID] = append(updates[setID], group.snapshotIDs...)
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
		// Serialize migrators without locking extension_storage_state, whose
		// legacy_writes flag is read by the live collector.
		if err := tx.Exec(`SELECT pg_advisory_xact_lock(77110301)`).Error; err != nil {
			return err
		}
		var cursor int64
		if err := tx.Raw(`
			SELECT backfill_cursor
			FROM extension_storage_state
			WHERE singleton = TRUE
		`).Scan(&cursor).Error; err != nil {
			return err
		}

		var window []models.WikiExtensionItem
		if err := tx.Where("id > ?", cursor).
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

		itemsBySnapshot := make(map[uuid.UUID][]models.WikiExtensionItem, len(order))
		for _, item := range window {
			itemsBySnapshot[item.SnapshotID] = append(itemsBySnapshot[item.SnapshotID], item)
		}

		// Snapshot inserts can interleave. The snapshot_id index can count each
		// complete set without reading the large heap; only sets crossing this
		// window need a random heap lookup.
		type snapshotItemCount struct {
			SnapshotID uuid.UUID
			Count      int
		}
		var counts []snapshotItemCount
		if err := tx.Model(&models.WikiExtensionItem{}).
			Select("snapshot_id", "COUNT(*) AS count").
			Where("snapshot_id IN ?", order).
			Group("snapshot_id").Scan(&counts).Error; err != nil {
			return err
		}
		var incomplete []uuid.UUID
		for _, count := range counts {
			if len(itemsBySnapshot[count.SnapshotID]) != count.Count {
				incomplete = append(incomplete, count.SnapshotID)
			}
		}
		var boundaryItems []models.WikiExtensionItem
		if len(incomplete) > 0 {
			if err := tx.Where("snapshot_id IN ?", incomplete).
				Order("snapshot_id, ext_type, name").Find(&boundaryItems).Error; err != nil {
				return err
			}
			for _, snapshotID := range incomplete {
				itemsBySnapshot[snapshotID] = nil
			}
			for _, item := range boundaryItems {
				itemsBySnapshot[item.SnapshotID] = append(itemsBySnapshot[item.SnapshotID], item)
			}
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

		groups, err := groupExtensionSnapshots(order, itemsBySnapshot, func(snapshotID uuid.UUID) bool {
			return needsBackfill[snapshotID]
		})
		if err != nil {
			return err
		}
		updates := make(map[int64][]uuid.UUID)
		for _, group := range groups {
			setID, err := r.ensureExtensionSet(tx, group.items)
			if err != nil {
				return err
			}
			updates[setID] = append(updates[setID], group.snapshotIDs...)
			result.Snapshots += len(group.snapshotIDs)
		}
		for setID, snapshotIDs := range updates {
			if err := tx.Model(&models.WikiExtensionsSnapshot{}).
				Where("id IN ? AND extension_set_id IS NULL", snapshotIDs).
				UpdateColumn("extension_set_id", setID).Error; err != nil {
				return err
			}
		}

		result.Items = len(window) + len(boundaryItems)
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
	setID, err := r.ensureExtensionSet(tx, nil)
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

// FinalizeExtensionSetMigration prevents further legacy writes, swaps in
// statistics built from extension sets, and releases the legacy table storage.
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

	// Building the view can take minutes in production. Do it without holding
	// the legacy_writes row lock so live collectors can continue dual-writing.
	if err := r.db.WithContext(ctx).Exec(`
		DROP MATERIALIZED VIEW IF EXISTS mv_extension_stats_next;
		CREATE MATERIALIZED VIEW mv_extension_stats_next AS
		SELECT item.name,
		       COUNT(*) FILTER (WHERE wiki.farm IS NULL) AS count,
		       COUNT(*) AS all_count
		FROM wiki_extension_set_items item
		JOIN wiki_extensions_snapshots snapshot
		  ON snapshot.extension_set_id = item.set_id
		JOIN wikis wiki ON wiki.id = snapshot.wiki_id
		WHERE snapshot.valid_until IS NULL
		GROUP BY item.name
		WITH DATA;
		CREATE UNIQUE INDEX idx_mv_extension_stats_next_name
		ON mv_extension_stats_next(name);
		CREATE INDEX idx_mv_extension_stats_next_count
		ON mv_extension_stats_next(count DESC, name);
		CREATE INDEX idx_mv_extension_stats_next_all_count
		ON mv_extension_stats_next(all_count DESC, name);
	`).Error; err != nil {
		return err
	}

	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec(`SELECT pg_advisory_xact_lock(77110301)`).Error; err != nil {
			return err
		}
		if err := tx.Exec(`
			DROP MATERIALIZED VIEW mv_extension_stats;
			ALTER MATERIALIZED VIEW mv_extension_stats_next
			RENAME TO mv_extension_stats;
			ALTER INDEX idx_mv_extension_stats_next_name
			RENAME TO idx_mv_extension_stats_name;
			ALTER INDEX idx_mv_extension_stats_next_count
			RENAME TO idx_mv_extension_stats_count;
			ALTER INDEX idx_mv_extension_stats_next_all_count
			RENAME TO idx_mv_extension_stats_all_count;
		`).Error; err != nil {
			return err
		}
		// Existing collectors holding FOR SHARE finish first; new collectors
		// wait here and observe legacy_writes=false after commit.
		if err := tx.Exec(`
			UPDATE extension_storage_state
			SET legacy_writes = FALSE, updated_at = NOW()
			WHERE singleton = TRUE
		`).Error; err != nil {
			return err
		}
		return tx.Exec(`TRUNCATE TABLE wiki_extension_items RESTART IDENTITY`).Error
	})
}
