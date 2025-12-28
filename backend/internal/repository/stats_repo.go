package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"wikikeeper-backend/internal/models"
)

// StatsRepository handles wiki_stats database operations
type StatsRepository struct {
	db *gorm.DB
}

// NewStatsRepository creates a new stats repository
func NewStatsRepository(db *gorm.DB) *StatsRepository {
	return &StatsRepository{db: db}
}

// Create creates a new wiki stats entry
func (r *StatsRepository) Create(ctx context.Context, stats *models.WikiStats) error {
	return r.db.WithContext(ctx).Create(stats).Error
}

// BatchCreate creates multiple wiki stats entries in a single transaction
func (r *StatsRepository) BatchCreate(ctx context.Context, stats []*models.WikiStats) error {
	if len(stats) == 0 {
		return nil
	}

	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return tx.Create(&stats).Error
	})
}

// GetByID retrieves a stats entry by ID
func (r *StatsRepository) GetByID(ctx context.Context, id int64) (*models.WikiStats, error) {
	var stats models.WikiStats
	err := r.db.WithContext(ctx).First(&stats, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &stats, nil
}

// GetByWikiID retrieves stats for a wiki within a time range
func (r *StatsRepository) GetByWikiID(ctx context.Context, wikiID uuid.UUID, days int) ([]*models.WikiStats, error) {
	var stats []*models.WikiStats

	query := r.db.WithContext(ctx).Where("wiki_id = ?", wikiID)

	if days > 0 {
		since := time.Now().AddDate(0, 0, -days)
		query = query.Where("time >= ?", since)
	}

	err := query.Order("time DESC").Find(&stats).Error
	if err != nil {
		return nil, err
	}

	return stats, nil
}

// GetLatestByWikiID retrieves the latest stats for a wiki
func (r *StatsRepository) GetLatestByWikiID(ctx context.Context, wikiID uuid.UUID) (*models.WikiStats, error) {
	var stats models.WikiStats
	err := r.db.WithContext(ctx).
		Where("wiki_id = ?", wikiID).
		Order("time DESC").
		First(&stats).Error
	if err != nil {
		return nil, err
	}
	return &stats, nil
}

// GetLatestForAllWikis retrieves the latest stats for all active wikis
func (r *StatsRepository) GetLatestForAllWikis(ctx context.Context) ([]*models.WikiStats, error) {
	var stats []*models.WikiStats

	// Subquery to find the latest stats for each wiki
	err := r.db.WithContext(ctx).Raw(`
		SELECT ws.* FROM wiki_stats ws
		INNER JOIN (
			SELECT wiki_id, MAX(time) as max_time
			FROM wiki_stats
			GROUP BY wiki_id
		) latest ON ws.wiki_id = latest.wiki_id AND ws.time = latest.max_time
		INNER JOIN wikis w ON ws.wiki_id = w.id
		WHERE w.is_active = true
		ORDER BY ws.time DESC
	`).Find(&stats).Error

	if err != nil {
		return nil, err
	}

	return stats, nil
}

// DeleteOlderThan deletes stats entries older than the given days
func (r *StatsRepository) DeleteOlderThan(ctx context.Context, days int) error {
	if days <= 0 {
		return nil
	}

	cutoff := time.Now().AddDate(0, 0, -days)
	return r.db.WithContext(ctx).
		Where("time < ?", cutoff).
		Delete(&models.WikiStats{}).Error
}

// DeleteByWikiID deletes all stats for a specific wiki
func (r *StatsRepository) DeleteByWikiID(ctx context.Context, wikiID uuid.UUID) error {
	return r.db.WithContext(ctx).
		Where("wiki_id = ?", wikiID).
		Delete(&models.WikiStats{}).Error
}

// CountByWikiID returns the number of stats entries for a wiki
func (r *StatsRepository) CountByWikiID(ctx context.Context, wikiID uuid.UUID) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&models.WikiStats{}).
		Where("wiki_id = ?", wikiID).
		Count(&count).Error
	return count, err
}
