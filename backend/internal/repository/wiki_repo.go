package repository

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"wikikeeper-backend/internal/models"
)

// WikiRepository handles wiki database operations
type WikiRepository struct {
	db *gorm.DB
}

// NewWikiRepository creates a new wiki repository
func NewWikiRepository(db *gorm.DB) *WikiRepository {
	return &WikiRepository{db: db}
}

// Create creates a new wiki
func (r *WikiRepository) Create(ctx context.Context, wiki *models.Wiki) error {
	return r.db.WithContext(ctx).Create(wiki).Error
}

// GetByID retrieves a wiki by ID
func (r *WikiRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Wiki, error) {
	var wiki models.Wiki
	err := r.db.WithContext(ctx).First(&wiki, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &wiki, nil
}

// GetByURL retrieves a wiki by URL
func (r *WikiRepository) GetByURL(ctx context.Context, url string) (*models.Wiki, error) {
	var wiki models.Wiki
	err := r.db.WithContext(ctx).First(&wiki, "url = ?", url).Error
	if err != nil {
		return nil, err
	}
	return &wiki, nil
}

// GetByAPIURL retrieves a wiki by API URL
func (r *WikiRepository) GetByAPIURL(ctx context.Context, apiURL string) (*models.Wiki, error) {
	var wiki models.Wiki
	err := r.db.WithContext(ctx).First(&wiki, "api_url = ?", apiURL).Error
	if err != nil {
		return nil, err
	}
	return &wiki, nil
}

// List retrieves wikis with pagination and filtering
type ListOptions struct {
	Page      int
	PageSize  int
	Status    *models.WikiStatus
	HasArchive *bool
	Search    string // Search in sitename
	OrderBy   string // e.g., "updated_at DESC"
}

func (r *WikiRepository) List(ctx context.Context, opts ListOptions) ([]*models.Wiki, int64, error) {
	var wikis []*models.Wiki
	var total int64

	query := r.db.WithContext(ctx).Model(&models.Wiki{})

	// Apply filters
	if opts.Status != nil {
		query = query.Where("status = ?", *opts.Status)
	}
	if opts.HasArchive != nil {
		query = query.Where("has_archive = ?", *opts.HasArchive)
	}
	if opts.Search != "" {
		// Remove protocol from search term to match URLs with or without http/https
		cleanSearch := strings.TrimPrefix(opts.Search, "http://")
		cleanSearch = strings.TrimPrefix(cleanSearch, "https://")
		cleanSearch = strings.TrimPrefix(cleanSearch, "www.")

		// Search in sitename or URL (with or without protocol)
		searchPattern := "%" + opts.Search + "%"
		cleanPattern := "%" + cleanSearch + "%"
		query = query.Where("sitename ILIKE ? OR url ILIKE ? OR url ILIKE ?",
			searchPattern, searchPattern, cleanPattern)
	}

	// Count total
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Apply pagination
	if opts.Page < 1 {
		opts.Page = 1
	}
	if opts.PageSize < 1 {
		opts.PageSize = 10
	}
	offset := (opts.Page - 1) * opts.PageSize

	// Apply ordering
	if opts.OrderBy != "" {
		query = query.Order(opts.OrderBy)
	} else {
		query = query.Order("updated_at DESC")
	}

	// Fetch results
	err := query.Offset(offset).Limit(opts.PageSize).Find(&wikis).Error
	if err != nil {
		return nil, 0, err
	}

	return wikis, total, nil
}

// Update updates a wiki
func (r *WikiRepository) Update(ctx context.Context, wiki *models.Wiki) error {
	return r.db.WithContext(ctx).Save(wiki).Error
}

// Delete deletes a wiki (cascades to stats and archives)
func (r *WikiRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Delete(&models.Wiki{}, "id = ?", id).Error
}

// GetPendingForUpdate retrieves wikis that need to be checked, ordered by last_check_at
func (r *WikiRepository) GetPendingForUpdate(ctx context.Context, limit int) ([]*models.Wiki, error) {
	var wikis []*models.Wiki
	err := r.db.WithContext(ctx).
		Where("is_active = ?", true).
		Order("last_check_at ASC NULLS FIRST").
		Limit(limit).
		Find(&wikis).Error
	if err != nil {
		return nil, err
	}
	return wikis, nil
}

// ExistsByURL checks if a wiki with the given URL exists
func (r *WikiRepository) ExistsByURL(ctx context.Context, url string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&models.Wiki{}).Where("url = ?", url).Count(&count).Error
	return count > 0, err
}

// ExistsByAPIURL checks if a wiki with the given API URL exists
func (r *WikiRepository) ExistsByAPIURL(ctx context.Context, apiURL string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&models.Wiki{}).Where("api_url = ?", apiURL).Count(&count).Error
	return count > 0, err
}

// GetSummaryStats returns summary statistics
func (r *WikiRepository) GetSummaryStats(ctx context.Context) (map[string]int64, error) {
	var result struct {
		TotalWikis      int64
		ArchivedWikis   int64
		StatusOKWikis   int64
		StatusErrorWikis int64
		ActiveWikis     int64
		TotalPages      int64
		TotalEdits      int64
	}

	// Single query to get all wiki statistics using CASE statements
	query := `
		SELECT 
			COUNT(*) as total_wikis,
			COUNT(*) FILTER (WHERE has_archive = true) as archived_wikis,
			COUNT(*) FILTER (WHERE status = 'ok') as status_ok_wikis,
			COUNT(*) FILTER (WHERE status = 'error') as status_error_wikis,
			COUNT(*) FILTER (WHERE is_active = true) as active_wikis,
			COALESCE(SUM(pages), 0) as total_pages,
			COALESCE(SUM(edits), 0) as total_edits
		FROM (
			SELECT 
				w.has_archive,
				w.status,
				w.is_active,
				ws.pages,
				ws.edits
			FROM wikis w
			LEFT JOIN LATERAL (
				SELECT pages, edits
				FROM wiki_stats
				WHERE wiki_id = w.id
				ORDER BY time DESC
				LIMIT 1
			) ws ON true
		) subquery
	`

	err := r.db.WithContext(ctx).Raw(query).Scan(&result).Error
	if err != nil {
		return nil, err
	}

	return map[string]int64{
		"total_wikis":       result.TotalWikis,
		"archived_wikis":    result.ArchivedWikis,
		"status_ok_wikis":   result.StatusOKWikis,
		"status_error_wikis": result.StatusErrorWikis,
		"active_wikis":      result.ActiveWikis,
		"total_pages":       result.TotalPages,
		"total_edits":       result.TotalEdits,
	}, nil
}
