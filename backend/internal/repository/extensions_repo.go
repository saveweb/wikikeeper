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
		Where("wiki_id = ? AND snapshot_at >= ? AND snapshot_at <= ?", wikiID, from, to).
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
