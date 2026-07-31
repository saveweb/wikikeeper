package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"wikikeeper-backend/internal/models"
)

// ExtensionsRepository handles extensions snapshots data access
type ExtensionsRepository struct {
	db *gorm.DB
}

// NewExtensionsRepository creates a new ExtensionsRepository
func NewExtensionsRepository(db *gorm.DB) *ExtensionsRepository {
	return &ExtensionsRepository{db: db}
}

// CreateSnapshot creates a new extensions snapshot with its items
func (r *ExtensionsRepository) CreateSnapshot(ctx context.Context, snapshot *models.WikiExtensionsSnapshot) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// GORM will automatically create the associated items due to the relationship defined in the model
		// We just need to create the snapshot and GORM handles the rest
		if err := tx.Create(snapshot).Error; err != nil {
			return err
		}

		return nil
	})
}

// GetLatestSnapshot gets the latest (currently valid) snapshot for a wiki
func (r *ExtensionsRepository) GetLatestSnapshot(ctx context.Context, wikiID uuid.UUID) (*models.WikiExtensionsSnapshot, error) {
	var snapshot models.WikiExtensionsSnapshot
	err := r.db.WithContext(ctx).
		Preload("Items").
		Where("wiki_id = ? AND valid_until IS NULL", wikiID).
		Order("snapshot_at DESC").
		First(&snapshot).Error
	if err != nil {
		return nil, err
	}
	return &snapshot, nil
}

// GetSnapshotsInTimeRange gets snapshots within a time range for a wiki
func (r *ExtensionsRepository) GetSnapshotsInTimeRange(ctx context.Context, wikiID uuid.UUID, from, to time.Time) ([]*models.WikiExtensionsSnapshot, error) {
	var snapshots []*models.WikiExtensionsSnapshot
	err := r.db.WithContext(ctx).
		Preload("Items").
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
		Preload("Items").
		Where("wiki_id = ?", wikiID).
		Order("snapshot_at DESC").
		Find(&snapshots).Error
	if err != nil {
		return nil, err
	}
	return snapshots, nil
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

	// Count total
	baseQuery := r.db.WithContext(ctx).
		Table("wiki_extension_items wei").
		Joins("JOIN wiki_extensions_snapshots wes ON wei.snapshot_id = wes.id").
		Joins("JOIN wikis w ON wes.wiki_id = w.id").
		Where("wei.name = ? AND wes.valid_until IS NULL", extensionName)

	if err := baseQuery.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Get paginated data
	offset := (opts.Page - 1) * opts.Limit
	err := baseQuery.
		Select(`
			w.id as wiki_id,
			w.wiki_name,
			w.sitename,
			w.url,
			wes.snapshot_at,
			wei.version as extension_version
		`).
		Offset(offset).
		Limit(opts.Limit).
		Order("w.sitename ASC NULLS LAST, w.url ASC").
		Scan(&result).Error

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
		SELECT
			COALESCE(wei.version, '') as version,
			COUNT(*) as count
		FROM wiki_extension_items wei
		JOIN wiki_extensions_snapshots wes ON wei.snapshot_id = wes.id
		WHERE wei.name = ? AND wes.valid_until IS NULL
		GROUP BY COALESCE(wei.version, '')
		ORDER BY count DESC
	`

	err := r.db.WithContext(ctx).Raw(query, extensionName).Scan(&stats).Error
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
