package repository

import (
	"context"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"wikikeeper-backend/internal/models"
)

type ProviderRateLimitRepository struct {
	db *gorm.DB
}

func NewProviderRateLimitRepository(db *gorm.DB) *ProviderRateLimitRepository {
	return &ProviderRateLimitRepository{db: db}
}

func (r *ProviderRateLimitRepository) Get(ctx context.Context, provider string) (*models.ProviderRateLimit, error) {
	var state models.ProviderRateLimit
	if err := r.db.WithContext(ctx).First(&state, "provider = ?", provider).Error; err != nil {
		return nil, err
	}
	return &state, nil
}

func (r *ProviderRateLimitRepository) Upsert(ctx context.Context, state *models.ProviderRateLimit) error {
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "provider"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"retry_at",
			"consecutive_rate_limits",
			"updated_at",
		}),
	}).Create(state).Error
}

func (r *ProviderRateLimitRepository) Delete(ctx context.Context, provider string) error {
	return r.db.WithContext(ctx).Delete(&models.ProviderRateLimit{}, "provider = ?", provider).Error
}
