package repository

import (
	"context"
	"crypto/sha256"
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"wikikeeper-backend/internal/models"
)

// ExtensionsRepository handles extensions snapshots data access
type ExtensionsRepository struct {
	db       *gorm.DB
	setMu    sync.RWMutex
	setCache map[[32]byte]int64
}

// NewExtensionsRepository creates a new ExtensionsRepository
func NewExtensionsRepository(db *gorm.DB) *ExtensionsRepository {
	return &ExtensionsRepository{db: db, setCache: make(map[[32]byte]int64)}
}

// CreateSnapshot creates a new extensions snapshot with its items
func (r *ExtensionsRepository) CreateSnapshot(ctx context.Context, snapshot *models.WikiExtensionsSnapshot) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		setID, err := r.ensureExtensionSet(tx, snapshot.Items)
		if err != nil {
			return err
		}
		snapshot.ExtensionSetID = &setID

		if err := tx.Omit("Items").Create(snapshot).Error; err != nil {
			return err
		}

		legacyWrites, err := extensionLegacyWrites(tx)
		if err != nil || !legacyWrites || len(snapshot.Items) == 0 {
			return err
		}
		items := append([]models.WikiExtensionItem(nil), snapshot.Items...)
		for i := range items {
			items[i].ID = 0
			items[i].SnapshotID = snapshot.ID
			items[i].CreatedAt = time.Time{}
		}
		return tx.CreateInBatches(&items, 500).Error
	})
}

// GetLatestSnapshot gets the latest (currently valid) snapshot for a wiki
func (r *ExtensionsRepository) GetLatestSnapshot(ctx context.Context, wikiID uuid.UUID) (*models.WikiExtensionsSnapshot, error) {
	var snapshot models.WikiExtensionsSnapshot
	err := r.db.WithContext(ctx).
		Where("wiki_id = ? AND valid_until IS NULL", wikiID).
		Order("snapshot_at DESC").
		First(&snapshot).Error
	if err != nil {
		return nil, err
	}
	if err := r.loadSnapshotItems(ctx, []*models.WikiExtensionsSnapshot{&snapshot}); err != nil {
		return nil, err
	}
	return &snapshot, nil
}

// GetSnapshotsInTimeRange gets snapshots within a time range for a wiki
func (r *ExtensionsRepository) GetSnapshotsInTimeRange(ctx context.Context, wikiID uuid.UUID, from, to time.Time) ([]*models.WikiExtensionsSnapshot, error) {
	var snapshots []*models.WikiExtensionsSnapshot
	err := r.db.WithContext(ctx).
		Where(
			"wiki_id = ? AND snapshot_at <= ? AND (valid_until IS NULL OR valid_until >= ?)",
			wikiID,
			to,
			from,
		).
		Order("snapshot_at DESC").
		Find(&snapshots).Error
	if err != nil {
		return nil, err
	}
	if err := r.loadSnapshotItems(ctx, snapshots); err != nil {
		return nil, err
	}
	return snapshots, nil
}

// CloseLatestSnapshot closes the latest snapshot by setting valid_until
func (r *ExtensionsRepository) CloseLatestSnapshot(ctx context.Context, wikiID uuid.UUID, validUntil time.Time) error {
	return r.db.WithContext(ctx).
		Model(&models.WikiExtensionsSnapshot{}).
		Where("wiki_id = ? AND valid_until IS NULL", wikiID).
		Update("valid_until", validUntil).Error
}

// SnapshotExists checks if a snapshot exists at a specific time
func (r *ExtensionsRepository) SnapshotExists(ctx context.Context, wikiID uuid.UUID, snapshotAt time.Time) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&models.WikiExtensionsSnapshot{}).
		Where("wiki_id = ? AND snapshot_at = ?", wikiID, snapshotAt).
		Count(&count).Error
	return count > 0, err
}

// GetAllSnapshots gets all snapshots for a wiki, ordered by snapshot_at DESC
func (r *ExtensionsRepository) GetAllSnapshots(ctx context.Context, wikiID uuid.UUID) ([]*models.WikiExtensionsSnapshot, error) {
	var snapshots []*models.WikiExtensionsSnapshot
	err := r.db.WithContext(ctx).
		Where("wiki_id = ?", wikiID).
		Order("snapshot_at DESC").
		Find(&snapshots).Error
	if err != nil {
		return nil, err
	}
	if err := r.loadSnapshotItems(ctx, snapshots); err != nil {
		return nil, err
	}
	return snapshots, nil
}

type canonicalExtensionItem struct {
	ExtType     string  `json:"type"`
	Name        string  `json:"name"`
	URL         *string `json:"url"`
	Version     *string `json:"version"`
	LicenseName *string `json:"license_name"`
}

type extensionSetHash []byte

func (hash extensionSetHash) Value() (driver.Value, error) {
	return []byte(hash), nil
}

func canonicalExtensionSet(items []models.WikiExtensionItem) ([32]byte, []models.WikiExtensionSetItem, error) {
	canonical := make([]canonicalExtensionItem, 0, len(items))
	for _, item := range items {
		canonical = append(canonical, canonicalExtensionItem{
			ExtType: item.ExtType, Name: item.Name, URL: item.URL,
			Version: item.Version, LicenseName: item.LicenseName,
		})
	}
	sort.Slice(canonical, func(i, j int) bool {
		if canonical[i].ExtType != canonical[j].ExtType {
			return canonical[i].ExtType < canonical[j].ExtType
		}
		return canonical[i].Name < canonical[j].Name
	})
	payload, err := json.Marshal(canonical)
	if err != nil {
		return [32]byte{}, nil, err
	}
	hash := sha256.Sum256(payload)
	setItems := make([]models.WikiExtensionSetItem, 0, len(canonical))
	for _, item := range canonical {
		setItems = append(setItems, models.WikiExtensionSetItem{
			ExtType: item.ExtType, Name: item.Name, URL: item.URL,
			Version: item.Version, LicenseName: item.LicenseName,
		})
	}
	return hash, setItems, nil
}

func (r *ExtensionsRepository) ensureExtensionSet(tx *gorm.DB, items []models.WikiExtensionItem) (int64, error) {
	hash, setItems, err := canonicalExtensionSet(items)
	if err != nil {
		return 0, err
	}
	r.setMu.RLock()
	cachedID, cached := r.setCache[hash]
	r.setMu.RUnlock()
	if cached {
		return cachedID, nil
	}

	var setID int64
	inserted := false
	hashValue := extensionSetHash(hash[:])
	if tx.Dialector.Name() == "postgres" {
		rows, err := tx.Raw(`
			INSERT INTO wiki_extension_sets(content_hash, item_count)
			VALUES (?, ?)
			ON CONFLICT (content_hash) DO NOTHING
			RETURNING id
		`, hashValue, len(setItems)).Rows()
		if err != nil {
			return 0, err
		}
		if rows.Next() {
			inserted = true
			err = rows.Scan(&setID)
		}
		rows.Close()
		if err != nil {
			return 0, err
		}
	} else {
		result := tx.Exec(`INSERT OR IGNORE INTO wiki_extension_sets(content_hash, item_count) VALUES (?, ?)`, hashValue, len(setItems))
		if result.Error != nil {
			return 0, result.Error
		}
		inserted = result.RowsAffected == 1
		if err := tx.Raw(`SELECT id FROM wiki_extension_sets WHERE content_hash = ?`, hashValue).Scan(&setID).Error; err != nil {
			return 0, err
		}
	}

	if !inserted {
		var itemCount int
		if err := tx.Raw(`SELECT id, item_count FROM wiki_extension_sets WHERE content_hash = ?`, hashValue).Row().Scan(&setID, &itemCount); err != nil {
			return 0, err
		}
		if itemCount != len(setItems) {
			return 0, fmt.Errorf("extension set hash collision: stored %d items, got %d", itemCount, len(setItems))
		}
		r.setMu.Lock()
		r.setCache[hash] = setID
		r.setMu.Unlock()
		return setID, nil
	}

	for i := range setItems {
		setItems[i].SetID = setID
	}
	if len(setItems) > 0 {
		if err := tx.CreateInBatches(&setItems, 500).Error; err != nil {
			return 0, err
		}
	}
	return setID, nil
}

func extensionLegacyWrites(tx *gorm.DB) (bool, error) {
	var enabled bool
	query := `SELECT legacy_writes FROM extension_storage_state WHERE singleton = ?`
	if tx.Dialector.Name() == "postgres" {
		query += ` FOR SHARE`
	}
	err := tx.Raw(query, true).Scan(&enabled).Error
	return enabled, err
}

func (r *ExtensionsRepository) loadSnapshotItems(ctx context.Context, snapshots []*models.WikiExtensionsSnapshot) error {
	setSnapshots := make(map[int64][]*models.WikiExtensionsSnapshot)
	legacySnapshots := make(map[uuid.UUID]*models.WikiExtensionsSnapshot)
	setIDs := make([]int64, 0)
	legacyIDs := make([]uuid.UUID, 0)
	for _, snapshot := range snapshots {
		if snapshot.ExtensionSetID != nil {
			if _, exists := setSnapshots[*snapshot.ExtensionSetID]; !exists {
				setIDs = append(setIDs, *snapshot.ExtensionSetID)
			}
			setSnapshots[*snapshot.ExtensionSetID] = append(setSnapshots[*snapshot.ExtensionSetID], snapshot)
		} else {
			legacyIDs = append(legacyIDs, snapshot.ID)
			legacySnapshots[snapshot.ID] = snapshot
		}
	}

	if len(setIDs) > 0 {
		var items []models.WikiExtensionSetItem
		if err := r.db.WithContext(ctx).Where("set_id IN ?", setIDs).Order("ext_type, name").Find(&items).Error; err != nil {
			return err
		}
		bySet := make(map[int64][]models.WikiExtensionItem)
		for _, item := range items {
			bySet[item.SetID] = append(bySet[item.SetID], models.WikiExtensionItem{
				ExtType: item.ExtType, Name: item.Name, URL: item.URL,
				Version: item.Version, LicenseName: item.LicenseName,
			})
		}
		for setID, targets := range setSnapshots {
			for _, snapshot := range targets {
				snapshot.Items = append([]models.WikiExtensionItem(nil), bySet[setID]...)
			}
		}
	}

	if len(legacyIDs) > 0 {
		var items []models.WikiExtensionItem
		if err := r.db.WithContext(ctx).Where("snapshot_id IN ?", legacyIDs).Order("ext_type, name").Find(&items).Error; err != nil {
			return err
		}
		for _, item := range items {
			legacySnapshots[item.SnapshotID].Items = append(legacySnapshots[item.SnapshotID].Items, item)
		}
	}
	return nil
}

// ExtensionWikiInfo contains wiki and extension information
type ExtensionWikiInfo struct {
	WikiID           uuid.UUID `json:"wiki_id"`
	WikiName         *string   `json:"wiki_name,omitempty"`
	Sitename         *string   `json:"sitename,omitempty"`
	URL              string    `json:"url"`
	SnapshotAt       time.Time `json:"snapshot_at"`
	ExtensionVersion *string   `json:"version,omitempty"`
}

// ExtensionWikisListOptions pagination options for listing wikis
type ExtensionWikisListOptions struct {
	Page  int
	Limit int
}

// GetWikisUsingExtension gets wikis that are using a specific extension (paginated)
func (r *ExtensionsRepository) GetWikisUsingExtension(
	ctx context.Context,
	extensionName string,
	opts ExtensionWikisListOptions,
) ([]*ExtensionWikiInfo, int64, error) {
	var result []*ExtensionWikiInfo
	var total int64

	// Set defaults
	if opts.Page < 1 {
		opts.Page = 1
	}
	if opts.Limit < 1 || opts.Limit > 100 {
		opts.Limit = 20
	}

	const usageSQL = `
		SELECT w.id AS wiki_id, w.wiki_name, w.sitename, w.url,
		       wes.snapshot_at, item.version AS extension_version
		FROM wiki_extension_set_items item
		JOIN wiki_extensions_snapshots wes ON wes.extension_set_id = item.set_id
		JOIN wikis w ON w.id = wes.wiki_id
		WHERE item.name = ? AND wes.valid_until IS NULL
		UNION ALL
		SELECT w.id AS wiki_id, w.wiki_name, w.sitename, w.url,
		       wes.snapshot_at, item.version AS extension_version
		FROM wiki_extension_items item
		JOIN wiki_extensions_snapshots wes ON wes.id = item.snapshot_id
		JOIN wikis w ON w.id = wes.wiki_id
		WHERE wes.extension_set_id IS NULL
		  AND item.name = ? AND wes.valid_until IS NULL
	`

	if err := r.db.WithContext(ctx).Raw(
		"SELECT COUNT(*) FROM ("+usageSQL+") AS usage",
		extensionName, extensionName,
	).Scan(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (opts.Page - 1) * opts.Limit
	err := r.db.WithContext(ctx).Raw(
		"SELECT * FROM ("+usageSQL+") AS usage "+
			"ORDER BY sitename ASC NULLS LAST, url ASC LIMIT ? OFFSET ?",
		extensionName, extensionName, opts.Limit, offset,
	).Scan(&result).Error

	if err != nil {
		return nil, 0, err
	}

	return result, total, nil
}

// ExtensionVersionStats represents version distribution statistics
type ExtensionVersionStats struct {
	Version string `json:"version"`
	Count   int64  `json:"count"`
}

// GetExtensionVersionDistribution gets the version distribution for an extension
func (r *ExtensionsRepository) GetExtensionVersionDistribution(
	ctx context.Context,
	extensionName string,
) ([]*ExtensionVersionStats, int64, error) {
	var stats []*ExtensionVersionStats

	// Use COALESCE to convert NULL to empty string
	query := `
		WITH usage AS (
			SELECT item.version
			FROM wiki_extension_set_items item
			JOIN wiki_extensions_snapshots wes ON wes.extension_set_id = item.set_id
			WHERE item.name = ? AND wes.valid_until IS NULL
			UNION ALL
			SELECT item.version
			FROM wiki_extension_items item
			JOIN wiki_extensions_snapshots wes ON wes.id = item.snapshot_id
			WHERE wes.extension_set_id IS NULL
			  AND item.name = ? AND wes.valid_until IS NULL
		)
		SELECT
			COALESCE(version, '') as version,
			COUNT(*) as count
		FROM usage
		GROUP BY COALESCE(version, '')
		ORDER BY count DESC
	`

	err := r.db.WithContext(ctx).Raw(query, extensionName, extensionName).Scan(&stats).Error
	if err != nil {
		return nil, 0, err
	}

	// Calculate total
	var total int64
	for _, stat := range stats {
		total += stat.Count
	}

	return stats, total, nil
}

// ExtensionStats represents extension usage statistics
type ExtensionStats struct {
	Name  string `json:"name"`
	Count int64  `json:"count"`
}

// GetAllExtensionsStatsOptions pagination options for GetAllExtensionsStats
type GetAllExtensionsStatsOptions struct {
	Page  int
	Limit int
}

// GetAllExtensionsStats gets statistics for all extensions (paginated)
// Uses materialized view for performance
func (r *ExtensionsRepository) GetAllExtensionsStats(ctx context.Context, opts GetAllExtensionsStatsOptions) ([]*ExtensionStats, int64, error) {
	var stats []*ExtensionStats
	var total int64

	// Set defaults
	if opts.Page < 1 {
		opts.Page = 1
	}
	if opts.Limit < 1 || opts.Limit > 500 {
		opts.Limit = 50
	}

	// Get total count from materialized view
	err := r.db.WithContext(ctx).Raw("SELECT COUNT(*) FROM mv_extension_stats").Scan(&total).Error
	if err != nil {
		return nil, 0, err
	}

	// Get paginated data from materialized view
	offset := (opts.Page - 1) * opts.Limit
	query := `
		SELECT name, count
		FROM mv_extension_stats
		ORDER BY count DESC, name ASC
		LIMIT ? OFFSET ?
	`
	err = r.db.WithContext(ctx).Raw(query, opts.Limit, offset).Scan(&stats).Error
	if err != nil {
		return nil, 0, err
	}

	return stats, total, nil
}

// RefreshExtensionStatsMaterializedView refreshes the extension stats materialized view
// This should be called after extension snapshots are updated
func (r *ExtensionsRepository) RefreshExtensionStatsMaterializedView(ctx context.Context) error {
	return r.db.WithContext(ctx).Exec("REFRESH MATERIALIZED VIEW CONCURRENTLY mv_extension_stats").Error
}
