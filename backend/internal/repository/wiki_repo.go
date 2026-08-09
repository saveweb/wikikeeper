package repository

import (
	"context"
	"errors"
	"strings"
	"time"

	"wikikeeper-backend/internal/models"

	"github.com/google/uuid"
	"github.com/jellydator/ttlcache/v3"
	"gorm.io/gorm"
	"wikikeeper-backend/internal/farms"
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
	if wiki.Farm == nil {
		wiki.Farm = farms.Detect(wiki.URL)
	}
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
	Page       int
	PageSize   int
	Status     *models.WikiStatus
	IsActive   *bool
	HasArchive *bool
	Farm       string
	Search     string // Search in sitename
	OrderBy    WikiOrder
}

const WikiFarmIndependentFilter = "_independent"

// WikiOrder is a supported ordering for wiki lists.
type WikiOrder string

const (
	WikiOrderUpdatedDesc       WikiOrder = "updated_at DESC"
	WikiOrderCreatedDesc       WikiOrder = "created_at DESC"
	WikiOrderSitenameAsc       WikiOrder = "sitename ASC"
	WikiOrderLastCheckAscNulls WikiOrder = "stats_last_check_at ASC NULLS FIRST"
)

// ErrInvalidWikiOrder indicates an unsupported wiki list ordering.
var ErrInvalidWikiOrder = errors.New("invalid wiki order")

// ErrInvalidWikiActiveFilter indicates an unsupported activity filter.
var ErrInvalidWikiActiveFilter = errors.New("invalid wiki active filter")

// ErrInvalidWikiStatusFilter indicates an unsupported wiki status filter.
var ErrInvalidWikiStatusFilter = errors.New("invalid wiki status filter")

// ParseWikiStatusFilter maps an external filter to a supported wiki status.
func ParseWikiStatusFilter(value string) (*models.WikiStatus, error) {
	status := models.WikiStatus(value)
	switch status {
	case "":
		return nil, nil
	case models.WikiStatusPending, models.WikiStatusOK, models.WikiStatusError:
		return &status, nil
	default:
		return nil, ErrInvalidWikiStatusFilter
	}
}

// ParseWikiActiveFilter maps an external filter to an optional boolean.
func ParseWikiActiveFilter(value string) (*bool, error) {
	switch value {
	case "":
		return nil, nil
	case "true":
		active := true
		return &active, nil
	case "false":
		active := false
		return &active, nil
	default:
		return nil, ErrInvalidWikiActiveFilter
	}
}

func validateWikiOrder(order WikiOrder) error {
	switch order {
	case "", WikiOrderUpdatedDesc, WikiOrderCreatedDesc, WikiOrderSitenameAsc, WikiOrderLastCheckAscNulls:
		return nil
	default:
		return ErrInvalidWikiOrder
	}
}

// ParseWikiOrder maps an external value to a supported ordering.
func ParseWikiOrder(value string) (WikiOrder, error) {
	order := WikiOrder(value)
	if err := validateWikiOrder(order); err != nil {
		return "", err
	}
	if order == "" {
		order = WikiOrderUpdatedDesc
	}
	return order, nil
}

func applyWikiOrder(query *gorm.DB, order WikiOrder) (*gorm.DB, error) {
	switch order {
	case "", WikiOrderUpdatedDesc:
		return query.Order("updated_at DESC"), nil
	case WikiOrderCreatedDesc:
		return query.Order("created_at DESC"), nil
	case WikiOrderSitenameAsc:
		return query.Order("sitename ASC"), nil
	case WikiOrderLastCheckAscNulls:
		return query.Order("last_check_at ASC NULLS FIRST"), nil
	default:
		return nil, ErrInvalidWikiOrder
	}
}

func (r *WikiRepository) List(ctx context.Context, opts ListOptions) ([]*models.Wiki, int64, error) {
	var wikis []*models.Wiki
	var total int64
	if err := validateWikiOrder(opts.OrderBy); err != nil {
		return nil, 0, err
	}

	query := r.db.WithContext(ctx).Model(&models.Wiki{})

	// Apply filters
	if opts.Status != nil {
		query = query.Where("status = ?", *opts.Status)
	}
	if opts.IsActive != nil {
		query = query.Where("is_active = ?", *opts.IsActive)
	}
	if opts.HasArchive != nil {
		query = query.Where("has_archive = ?", *opts.HasArchive)
	}
	if opts.Farm == WikiFarmIndependentFilter {
		query = query.Where("farm IS NULL")
	} else if opts.Farm != "" {
		query = query.Where("farm = ?", opts.Farm)
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
	query, err := applyWikiOrder(query, opts.OrderBy)
	if err != nil {
		return nil, 0, err
	}

	// Fetch results
	err = query.Offset(offset).Limit(opts.PageSize).Find(&wikis).Error
	if err != nil {
		return nil, 0, err
	}

	return wikis, total, nil
}

// ListFarms returns the farm markers currently present in the wiki catalog.
func (r *WikiRepository) ListFarms(ctx context.Context) ([]models.WikiFarm, error) {
	var farms []models.WikiFarm
	err := r.db.WithContext(ctx).Model(&models.Wiki{}).
		Where("farm IS NOT NULL").
		Distinct().
		Order("farm ASC").
		Pluck("farm", &farms).Error
	return farms, err
}

// Update updates a wiki
func (r *WikiRepository) Update(ctx context.Context, wiki *models.Wiki) error {
	return r.db.WithContext(ctx).Save(wiki).Error
}

// SetActive enables or disables scheduled checks for a wiki.
func (r *WikiRepository) SetActive(ctx context.Context, id uuid.UUID, active bool) error {
	result := r.db.WithContext(ctx).
		Model(&models.Wiki{}).
		Where("id = ?", id).
		Update("is_active", active)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// UpdateArchiveStatus updates only archive-owned fields. UpdateColumns avoids
// model hooks and the generic updated_at timestamp used by stats collection.
func (r *WikiRepository) UpdateArchiveStatus(ctx context.Context, id uuid.UUID, hasArchive bool, checkedAt time.Time) error {
	return r.db.WithContext(ctx).
		Model(&models.Wiki{}).
		Where("id = ?", id).
		UpdateColumns(map[string]interface{}{
			"has_archive":           hasArchive,
			"archive_last_check_at": checkedAt,
			"archive_last_error":    nil,
			"archive_last_error_at": nil,
		}).Error
}

// UpdateArchiveError records an archive failure without changing collection
// state, the last verified wiki state, or the generic updated_at timestamp.
func (r *WikiRepository) UpdateArchiveError(ctx context.Context, id uuid.UUID, errMsg string, checkedAt time.Time) error {
	return r.db.WithContext(ctx).
		Model(&models.Wiki{}).
		Where("id = ?", id).
		UpdateColumns(map[string]interface{}{
			"archive_last_check_at": checkedAt,
			"archive_last_error":    errMsg,
			"archive_last_error_at": checkedAt,
		}).Error
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

// GetDueForUpdate returns active wikis whose persisted collection schedule is due.
func (r *WikiRepository) GetDueForUpdate(ctx context.Context, limit int, now time.Time) ([]*models.Wiki, error) {
	var wikis []*models.Wiki
	err := r.db.WithContext(ctx).
		Where("is_active = ?", true).
		Where("next_check_at IS NULL OR next_check_at <= ?", now).
		Order("next_check_at ASC NULLS FIRST").
		Limit(limit).
		Find(&wikis).Error
	if err != nil {
		return nil, err
	}
	return wikis, nil
}

// GetDueForUpdateFair independently selects healthy, never-successful, and
// failed wikis, then interleaves the queues so no class can starve the others.
func (r *WikiRepository) GetDueForUpdateFair(ctx context.Context, limit int, now time.Time) ([]*models.Wiki, error) {
	if limit <= 0 {
		return nil, nil
	}
	filters := []string{
		"last_success_at IS NOT NULL AND collection_status = 'ok'",
		"last_success_at IS NULL",
		"last_success_at IS NOT NULL AND collection_status <> 'ok'",
	}
	queues := make([][]*models.Wiki, 0, len(filters))
	for _, filter := range filters {
		var queue []*models.Wiki
		err := r.db.WithContext(ctx).
			Where("is_active = ?", true).
			Where("next_check_at IS NULL OR next_check_at <= ?", now).
			Where(filter).
			Order("next_check_at ASC NULLS FIRST").
			Limit(limit).
			Find(&queue).Error
		if err != nil {
			return nil, err
		}
		queues = append(queues, queue)
	}

	wikis := make([]*models.Wiki, 0, limit)
	for position := 0; len(wikis) < limit; position++ {
		added := false
		for _, queue := range queues {
			if position < len(queue) {
				wikis = append(wikis, queue[position])
				added = true
				if len(wikis) == limit {
					break
				}
			}
		}
		if !added {
			break
		}
	}
	return wikis, nil
}

// GetDueForArchiveCheck returns active wikis with a known API whose archive
// check has never run or is older than dueBefore.
func (r *WikiRepository) GetDueForArchiveCheck(ctx context.Context, limit int, dueBefore time.Time) ([]*models.Wiki, error) {
	var wikis []*models.Wiki
	err := r.db.WithContext(ctx).
		Where("is_active = ?", true).
		Where("api_url IS NOT NULL AND api_url <> ''").
		Where("archive_last_check_at IS NULL OR archive_last_check_at <= ?", dueBefore).
		Order("archive_last_check_at ASC NULLS FIRST").
		Limit(limit).
		Find(&wikis).Error
	if err != nil {
		return nil, err
	}
	return wikis, nil
}

// DeferCollectionChecks postpones queued checks without recording an attempt.
func (r *WikiRepository) DeferCollectionChecks(ctx context.Context, ids []uuid.UUID, nextCheckAt time.Time) error {
	if len(ids) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).
		Model(&models.Wiki{}).
		Where("id IN ?", ids).
		UpdateColumn("next_check_at", nextCheckAt).Error
}

// ExistsByURL checks if a wiki with the given URL exists
func (r *WikiRepository) ExistsByURL(ctx context.Context, url string) (bool, error) {
	var count int64
	baseURL := strings.TrimRight(url, "/")
	err := r.db.WithContext(ctx).Model(&models.Wiki{}).
		Where("url IN ?", []string{baseURL, baseURL + "/"}).
		Count(&count).Error
	return count > 0, err
}

// ExistsByAPIURL checks if a wiki with the given API URL exists
func (r *WikiRepository) ExistsByAPIURL(ctx context.Context, apiURL string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&models.Wiki{}).Where("api_url = ?", apiURL).Count(&count).Error
	return count > 0, err
}

type Summary struct {
	TotalWikis       int64
	ArchivedWikis    int64
	ArchivedSize     int64
	StatusOKWikis    int64
	StatusErrorWikis int64
	ActiveWikis      int64
	TotalPages       int64
	TotalEdits       int64
}

var summaryCache = ttlcache.New(
	ttlcache.WithTTL[bool, *Summary](10*time.Second),
	ttlcache.WithDisableTouchOnHit[bool, *Summary](),
)

func init() {
	go summaryCache.Start()
}

// GetSummaryStats returns summary statistics
func (r *WikiRepository) GetSummaryStats(ctx context.Context, bypassCache bool) (map[string]int64, error) {
	var result Summary

	if !bypassCache {
		if c := summaryCache.Get(true); c != nil {
			cached := c.Value()
			return map[string]int64{
				"total_wikis":        cached.TotalWikis,
				"archived_wikis":     cached.ArchivedWikis,
				"archived_size":      cached.ArchivedSize,
				"status_ok_wikis":    cached.StatusOKWikis,
				"status_error_wikis": cached.StatusErrorWikis,
				"active_wikis":       cached.ActiveWikis,
				"total_pages":        cached.TotalPages,
				"total_edits":        cached.TotalEdits,
			}, nil
		}
	}

	// Single query to get all wiki statistics using CASE statements
	query := `
		SELECT
			COUNT(*) as total_wikis,
			COUNT(*) FILTER (WHERE has_archive = true) as archived_wikis,
			(
				SELECT COALESCE(SUM(item_size), 0)
				FROM (
					SELECT MAX(item_size) AS item_size
					FROM wiki_archives
					GROUP BY ia_identifier
				) ia_items
			) as archived_size,
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
	summaryCache.Set(true, &result, ttlcache.DefaultTTL)

	return map[string]int64{
		"total_wikis":        result.TotalWikis,
		"archived_wikis":     result.ArchivedWikis,
		"archived_size":      result.ArchivedSize,
		"status_ok_wikis":    result.StatusOKWikis,
		"status_error_wikis": result.StatusErrorWikis,
		"active_wikis":       result.ActiveWikis,
		"total_pages":        result.TotalPages,
		"total_edits":        result.TotalEdits,
	}, nil
}
