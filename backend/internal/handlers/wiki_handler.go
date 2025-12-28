package handlers

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"gorm.io/gorm"

	applogger "wikikeeper-backend/internal/logger"

	"wikikeeper-backend/internal/config"
	"wikikeeper-backend/internal/models"
	"wikikeeper-backend/internal/repository"
	"wikikeeper-backend/internal/services"
)

// WikiHandler handles wiki HTTP requests
type WikiHandler struct {
	db     *gorm.DB
	config *config.Config
}

// NewWikiHandler creates a new wiki handler
func NewWikiHandler(db *gorm.DB, cfg *config.Config) *WikiHandler {
	return &WikiHandler{db: db, config: cfg}
}

// ListWikisRequest represents query parameters for listing wikis
type ListWikisRequest struct {
	Page       int    `query:"page"`
	PageSize   int    `query:"page_size"`
	Status     string `query:"status"`
	HasArchive *bool  `query:"has_archive"`
	Search     string `query:"search"`
	OrderBy    string `query:"order_by"`
}

// WikiCreateRequest represents request body for creating a wiki
type WikiCreateRequest struct {
	URL      string  `json:"url"`
	WikiName *string `json:"wiki_name"`
}

// List handles GET /api/wikis
func (h *WikiHandler) List(c echo.Context) error {
	var req ListWikisRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"detail": "Invalid query parameters"})
	}

	// Set defaults
	if req.Page < 1 {
		req.Page = 1
	}
	if req.PageSize < 1 || req.PageSize > 100 {
		req.PageSize = 10
	}

	wikiRepo := repository.NewWikiRepository(h.db)
	ctx := c.Request().Context()

	// Build list options
	opts := repository.ListOptions{
		Page:     req.Page,
		PageSize: req.PageSize,
		OrderBy:  req.OrderBy,
	}

	if req.Status != "" {
		status := models.WikiStatus(req.Status)
		opts.Status = &status
	}
	if req.HasArchive != nil {
		opts.HasArchive = req.HasArchive
	}
	if req.Search != "" {
		opts.Search = req.Search
	}

	wikis, total, err := wikiRepo.List(ctx, opts)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"detail": err.Error()})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"total":     total,
		"page":      req.Page,
		"page_size": req.PageSize,
		"data":      wikis,
	})
}

// Get handles GET /api/wikis/:id
func (h *WikiHandler) Get(c echo.Context) error {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"detail": "Invalid wiki ID format"})
	}

	wikiRepo := repository.NewWikiRepository(h.db)
	ctx := c.Request().Context()

	wiki, err := wikiRepo.GetByID(ctx, id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return c.JSON(http.StatusNotFound, map[string]string{"detail": "Wiki not found"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"detail": err.Error()})
	}

	return c.JSON(http.StatusOK, wiki)
}

// Create handles POST /api/wikis
func (h *WikiHandler) Create(c echo.Context) error {
	var req WikiCreateRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"detail": "Invalid request body"})
	}

	if req.URL == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"detail": "URL is required"})
	}

	// Normalize URL using service
	normalizedURL := services.NormalizeURL(req.URL)
	if normalizedURL == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"detail": "Invalid URL format"})
	}

	wikiRepo := repository.NewWikiRepository(h.db)
	ctx := c.Request().Context()

	// Check if already exists
	exists, err := wikiRepo.ExistsByURL(ctx, normalizedURL)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"detail": err.Error()})
	}
	if exists {
		return c.JSON(http.StatusBadRequest, map[string]string{"detail": "Wiki already exists"})
	}

	// Create wiki
	wiki := &models.Wiki{
		ID:     uuid.New(),
		URL:    normalizedURL,
		Status: models.WikiStatusPending,
	}
	if req.WikiName != nil {
		wiki.WikiName = req.WikiName
	}

	if err := wikiRepo.Create(ctx, wiki); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"detail": err.Error()})
	}

	// TODO: Trigger background initial check (go h.initialWikiCheck(wiki.ID))

	return c.JSON(http.StatusCreated, wiki)
}

// Delete handles DELETE /api/wikis/:id
func (h *WikiHandler) Delete(c echo.Context) error {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"detail": "Invalid wiki ID format"})
	}

	wikiRepo := repository.NewWikiRepository(h.db)
	ctx := c.Request().Context()

	// Check if exists
	_, err = wikiRepo.GetByID(ctx, id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return c.JSON(http.StatusNotFound, map[string]string{"detail": "Wiki not found"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"detail": err.Error()})
	}

	// Delete (cascade to stats and archives)
	if err := wikiRepo.Delete(ctx, id); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"detail": err.Error()})
	}

	return c.JSON(http.StatusOK, map[string]string{
		"detail":  fmt.Sprintf("Wiki %s deleted", idStr),
		"wiki_id": idStr,
	})
}

// TriggerCheck handles POST /api/wikis/:id/check
func (h *WikiHandler) TriggerCheck(c echo.Context) error {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"detail": "Invalid wiki ID format"})
	}

	wikiRepo := repository.NewWikiRepository(h.db)
	ctx := c.Request().Context()

	// Check if exists
	wiki, err := wikiRepo.GetByID(ctx, id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return c.JSON(http.StatusNotFound, map[string]string{"detail": "Wiki not found"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"detail": err.Error()})
	}

	// Check rate limit for anonymous users (1 check per hour per wiki)
	if !h.isAdmin(c) {
		if wiki.LastCheckAt != nil {
			// Check if last check was less than 1 hour ago
			if time.Since(*wiki.LastCheckAt) < 1*time.Hour {
				remainingTime := 1*time.Hour - time.Since(*wiki.LastCheckAt)
				return c.JSON(http.StatusTooManyRequests, map[string]string{
					"detail":        "Rate limit exceeded. Only 1 check per hour per wiki for anonymous users.",
					"retry_after":   fmt.Sprintf("%.0f", remainingTime.Seconds()),
					"last_check_at": wiki.LastCheckAt.Format(time.RFC3339),
				})
			}
		}
	}

	// Start background collection
	go func() {
		bgCtx := context.Background()
		mwService := services.NewMediaWikiService(
			time.Duration(h.config.HTTPTimeout)*time.Second,
			h.config.HTTPUserAgent,
		)
		collector := services.NewCollectorService(h.db, mwService, h.config)

		if err := collector.CollectSingleWiki(bgCtx, id); err != nil {
			applogger.Log.Info("[Handler] Collection failed for %s: %v", id, err)
		}
	}()

	return c.JSON(http.StatusAccepted, map[string]string{
		"detail":  "Stats collection started",
		"wiki_id": idStr,
	})
}

// GetStats handles GET /api/wikis/:id/stats
func (h *WikiHandler) GetStats(c echo.Context) error {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"detail": "Invalid wiki ID format"})
	}

	// Get days parameter
	daysStr := c.QueryParam("days")
	days := 30 // default
	if daysStr != "" {
		days, err = strconv.Atoi(daysStr)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"detail": "Invalid days parameter"})
		}
	}

	wikiRepo := repository.NewWikiRepository(h.db)
	statsRepo := repository.NewStatsRepository(h.db)
	ctx := c.Request().Context()

	// Check if wiki exists
	_, err = wikiRepo.GetByID(ctx, id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return c.JSON(http.StatusNotFound, map[string]string{"detail": "Wiki not found"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"detail": err.Error()})
	}

	// Get stats
	stats, err := statsRepo.GetByWikiID(ctx, id, days)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"detail": err.Error()})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"wiki_id": idStr,
		"days":    days,
		"data":    stats,
	})
}

// GetArchives handles GET /api/wikis/:id/archives
func (h *WikiHandler) GetArchives(c echo.Context) error {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"detail": "Invalid wiki ID format"})
	}

	wikiRepo := repository.NewWikiRepository(h.db)
	archiveRepo := repository.NewArchiveRepository(h.db)
	ctx := c.Request().Context()

	// Check if wiki exists
	_, err = wikiRepo.GetByID(ctx, id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return c.JSON(http.StatusNotFound, map[string]string{"detail": "Wiki not found"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"detail": err.Error()})
	}

	// Get archives
	archives, err := archiveRepo.GetByWikiID(ctx, id)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"detail": err.Error()})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"wiki_id": idStr,
		"data":    archives,
	})
}

// CheckArchive handles POST /api/wikis/:id/check-archive
func (h *WikiHandler) CheckArchive(c echo.Context) error {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"detail": "Invalid wiki ID format"})
	}

	wikiRepo := repository.NewWikiRepository(h.db)
	ctx := c.Request().Context()

	// Check if wiki exists
	wiki, err := wikiRepo.GetByID(ctx, id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return c.JSON(http.StatusNotFound, map[string]string{"detail": "Wiki not found"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"detail": err.Error()})
	}

	// Check if API URL is available
	if wiki.APIURL == nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"detail": "Wiki API URL not available. Run stats collection first."})
	}

	// Check rate limit for anonymous users (1 check per hour per wiki)
	if !h.isAdmin(c) {
		if wiki.ArchiveLastCheckAt != nil {
			// Check if last check was less than 1 hour ago
			if time.Since(*wiki.ArchiveLastCheckAt) < 1*time.Hour {
				remainingTime := 1*time.Hour - time.Since(*wiki.ArchiveLastCheckAt)
				return c.JSON(http.StatusTooManyRequests, map[string]string{
					"detail":                "Rate limit exceeded. Only 1 archive check per hour per wiki for anonymous users.",
					"retry_after":           fmt.Sprintf("%.0f", remainingTime.Seconds()),
					"archive_last_check_at": wiki.ArchiveLastCheckAt.Format(time.RFC3339),
				})
			}
		}
	}

	// Create Archive service
	archiveService := services.NewArchiveService(
		time.Duration(h.config.HTTPTimeout)*time.Second,
		h.config.HTTPUserAgent,
		h.config.ArchiveCheckDelay,
	)

	// Check Archive.org (async)
	go func() {
		bgCtx := context.Background()
		apiURL := *wiki.APIURL
		indexURL := ""
		if wiki.IndexURL != nil {
			indexURL = *wiki.IndexURL
		}

		found, imported, updated, err := archiveService.CollectArchives(bgCtx, h.db, id, apiURL, indexURL)
		if err != nil {
			applogger.Log.Info("[Handler] Archive check failed for %s: %v", id, err)
			// Update wiki with archive error
			archiveService.UpdateWikiArchiveError(bgCtx, h.db, id, err)
		} else {
			applogger.Log.Info("[Handler] Archive check completed: found=%d, imported=%d, updated=%d", found, imported, updated)
		}
	}()

	return c.JSON(http.StatusAccepted, map[string]interface{}{
		"detail":  "Archive check started",
		"wiki_id": idStr,
	})
}

// GetThumbnail handles GET /api/wikis/:id/thumbnail
func (h *WikiHandler) GetThumbnail(c echo.Context) error {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"detail": "Invalid wiki ID format"})
	}

	wikiRepo := repository.NewWikiRepository(h.db)
	archiveRepo := repository.NewArchiveRepository(h.db)
	ctx := c.Request().Context()

	// Get wiki
	wiki, err := wikiRepo.GetByID(ctx, id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return c.JSON(http.StatusNotFound, map[string]string{"detail": "Wiki not found"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"detail": err.Error()})
	}

	// Try to get the most recent archive for this wiki
	archives, err := archiveRepo.GetByWikiID(ctx, id)
	if err == nil && len(archives) > 0 {
		// Get the most recent archive (archives are ordered by added_date DESC)
		mostRecent := archives[0]
		if mostRecent.IAIdentifier != "" {
			return c.Redirect(http.StatusFound, fmt.Sprintf("https://archive.org/services/img/%s", mostRecent.IAIdentifier))
		}
	}

	// Fallback: try to construct from sitename (for wikis without archives but with sitename)
	if wiki.Sitename != nil && *wiki.Sitename != "" {
		// Try common Archive.org naming patterns
		possiblePatterns := []string{
			fmt.Sprintf("wiki-%s_w", *wiki.Sitename),
			fmt.Sprintf("%s_w", *wiki.Sitename),
			*wiki.Sitename,
		}

		for _, pattern := range possiblePatterns {
			// Check if this pattern exists by trying to redirect
			// Archive.org will return 404 if not found, browser will show broken image
			return c.Redirect(http.StatusFound, fmt.Sprintf("https://archive.org/services/img/%s", pattern))
		}
	}

	// Default placeholder
	return c.Redirect(http.StatusFound, "https://archive.org/services/img/wikiteam.png")
}

// normalizeURL function removed - use services.NormalizeURL instead

// isAdmin checks if the request has a valid admin token
func (h *WikiHandler) isAdmin(c echo.Context) bool {
	// If no admin token configured, no admin protection
	if h.config.AdminToken == "" {
		return false
	}

	// Check for admin token cookie
	cookie, err := c.Cookie("admintoken")
	if err != nil {
		return false
	}

	return cookie.Value == h.config.AdminToken
}
