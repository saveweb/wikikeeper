package repository

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"wikikeeper-backend/internal/models"
)

// ArchiveRepository handles wiki_archives database operations
type ArchiveRepository struct {
	db *gorm.DB
}

// NewArchiveRepository creates a new archive repository
func NewArchiveRepository(db *gorm.DB) *ArchiveRepository {
	return &ArchiveRepository{db: db}
}

// Create creates a new archive entry
func (r *ArchiveRepository) Create(ctx context.Context, archive *models.WikiArchive) error {
	return r.db.WithContext(ctx).Create(archive).Error
}

// BatchCreate creates multiple archive entries in a single transaction
func (r *ArchiveRepository) BatchCreate(ctx context.Context, archives []*models.WikiArchive) error {
	if len(archives) == 0 {
		return nil
	}

	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return tx.Create(&archives).Error
	})
}

// GetByID retrieves an archive by ID
func (r *ArchiveRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.WikiArchive, error) {
	var archive models.WikiArchive
	err := r.db.WithContext(ctx).First(&archive, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &archive, nil
}

// GetByWikiID retrieves all archives for a wiki
func (r *ArchiveRepository) GetByWikiID(ctx context.Context, wikiID uuid.UUID) ([]*models.WikiArchive, error) {
	var archives []*models.WikiArchive
	err := r.db.WithContext(ctx).
		Where("wiki_id = ?", wikiID).
		Order("dump_date DESC").
		Find(&archives).Error
	if err != nil {
		return nil, err
	}
	return archives, nil
}

// GetByIAIdentifier retrieves an archive by Archive.org identifier
func (r *ArchiveRepository) GetByIAIdentifier(ctx context.Context, iaIdentifier string) (*models.WikiArchive, error) {
	var archive models.WikiArchive
	err := r.db.WithContext(ctx).
		Where("ia_identifier = ?", iaIdentifier).
		First(&archive).Error
	if err != nil {
		return nil, err
	}
	return &archive, nil
}

// GetByWikiAndIAIdentifier retrieves an archive by wiki ID and IA identifier
func (r *ArchiveRepository) GetByWikiAndIAIdentifier(
	ctx context.Context,
	wikiID uuid.UUID,
	iaIdentifier string,
) (*models.WikiArchive, error) {
	var archive models.WikiArchive
	err := r.db.WithContext(ctx).
		Where("wiki_id = ? AND ia_identifier = ?", wikiID, iaIdentifier).
		First(&archive).Error
	if err != nil {
		return nil, err
	}
	return &archive, nil
}

// Update updates an archive entry
func (r *ArchiveRepository) Update(ctx context.Context, archive *models.WikiArchive) error {
	return r.db.WithContext(ctx).Save(archive).Error
}

// Delete deletes an archive entry
func (r *ArchiveRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Delete(&models.WikiArchive{}, "id = ?", id).Error
}

// DeleteByWikiID deletes all archives for a specific wiki
func (r *ArchiveRepository) DeleteByWikiID(ctx context.Context, wikiID uuid.UUID) error {
	return r.db.WithContext(ctx).
		Where("wiki_id = ?", wikiID).
		Delete(&models.WikiArchive{}).Error
}

// ExistsByWikiAndIAIdentifier checks if an archive exists for a wiki with the given IA identifier
func (r *ArchiveRepository) ExistsByWikiAndIAIdentifier(
	ctx context.Context,
	wikiID uuid.UUID,
	iaIdentifier string,
) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&models.WikiArchive{}).
		Where("wiki_id = ? AND ia_identifier = ?", wikiID, iaIdentifier).
		Count(&count).Error
	return count > 0, err
}

// UpsertByWikiAndIAIdentifier updates an archive if it exists, or creates it if it doesn't
func (r *ArchiveRepository) UpsertByWikiAndIAIdentifier(
	ctx context.Context,
	archive *models.WikiArchive,
) error {
	// Check if exists
	exists, err := r.ExistsByWikiAndIAIdentifier(ctx, archive.WikiID, archive.IAIdentifier)
	if err != nil {
		return err
	}

	if exists {
		// Update existing
		return r.db.WithContext(ctx).
			Model(&models.WikiArchive{}).
			Where("wiki_id = ? AND ia_identifier = ?", archive.WikiID, archive.IAIdentifier).
			Updates(archive).Error
	}

	// Create new
	return r.Create(ctx, archive)
}
