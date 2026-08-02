package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"gorm.io/gorm"

	applogger "wikikeeper-backend/internal/logger"
	"wikikeeper-backend/internal/repository"
)

var extensionsLog = applogger.With("component", "extensions-handler")

// ExtensionsHandler handles extensions-related HTTP requests
type ExtensionsHandler struct {
	db *gorm.DB
}

// NewExtensionsHandler creates a new ExtensionsHandler
func NewExtensionsHandler(db *gorm.DB) *ExtensionsHandler {
	return &ExtensionsHandler{db: db}
}

// GetLatestExtensions gets the latest extensions snapshot for a wiki
// GET /api/wikis/:id/extensions
func (h *ExtensionsHandler) GetLatestExtensions(c echo.Context) error {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"detail": "Invalid wiki ID format"})
	}

	wikiRepo := repository.NewWikiRepository(h.db)
	ctx := c.Request().Context()

	// Check if wiki exists
	_, err = wikiRepo.GetByID(ctx, id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return c.JSON(http.StatusNotFound, map[string]string{"detail": "Wiki not found"})
		}
		extensionsLog.Info("Failed to get wiki", "err", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"detail": "Failed to get wiki"})
	}

	extensionsRepo := repository.NewExtensionsRepository(h.db)
	snapshot, err := extensionsRepo.GetLatestSnapshot(ctx, id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return c.JSON(http.StatusNotFound, map[string]string{"detail": "Extensions not found"})
		}
		extensionsLog.Info("Failed to get extensions snapshot", "err", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"detail": "Failed to get extensions"})
	}

	return c.JSON(http.StatusOK, snapshot)
}

// GetExtensionsHistory gets extensions snapshots in a time range
// GET /api/wikis/:id/extensions/history?from=...&to=...
func (h *ExtensionsHandler) GetExtensionsHistory(c echo.Context) error {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"detail": "Invalid wiki ID format"})
	}

	// Parse time range parameters
	fromStr := c.QueryParam("from")
	toStr := c.QueryParam("to")

	var from, to time.Time
	var fromSet, toSet bool

	if fromStr != "" {
		from, err = time.Parse(time.RFC3339, fromStr)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"detail": "Invalid from time format"})
		}
		from = from.UTC()
		fromSet = true
	}

	if toStr != "" {
		to, err = time.Parse(time.RFC3339, toStr)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"detail": "Invalid to time format"})
		}
		to = to.UTC()
		toSet = true
	}

	// Default time range: last 30 days
	if !fromSet {
		from = time.Now().UTC().AddDate(0, 0, -30)
	}
	if !toSet {
		to = time.Now().UTC()
	}
	if from.After(to) {
		return c.JSON(http.StatusBadRequest, map[string]string{"detail": "from time must not be after to time"})
	}

	wikiRepo := repository.NewWikiRepository(h.db)
	extensionsRepo := repository.NewExtensionsRepository(h.db)
	ctx := c.Request().Context()

	// Check if wiki exists
	_, err = wikiRepo.GetByID(ctx, id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return c.JSON(http.StatusNotFound, map[string]string{"detail": "Wiki not found"})
		}
		extensionsLog.Info("Failed to get wiki", "err", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"detail": "Failed to get wiki"})
	}

	snapshots, err := extensionsRepo.GetSnapshotsInTimeRange(ctx, id, from, to)
	if err != nil {
		extensionsLog.Info("Failed to get extensions history", "err", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"detail": "Failed to get extensions history"})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"wiki_id":   idStr,
		"from":      from.UTC().Format(time.RFC3339),
		"to":        to.UTC().Format(time.RFC3339),
		"snapshots": snapshots,
	})
}

// GetExtensionWikisRequest query parameters for GetExtensionWikis
type GetExtensionWikisRequest struct {
	Page  int `query:"page"`
	Limit int `query:"limit"`
}

// GetExtensionWikis handles GET /api/extensions/:name/wikis
func (h *ExtensionsHandler) GetExtensionWikis(c echo.Context) error {
	extensionName := c.Param("name")
	if extensionName == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"detail": "Extension name is required",
		})
	}

	var req GetExtensionWikisRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"detail": "Invalid query parameters",
		})
	}

	// Set defaults
	if req.Page < 1 {
		req.Page = 1
	}
	if req.Limit < 1 || req.Limit > 100 {
		req.Limit = 20
	}

	extensionsRepo := repository.NewExtensionsRepository(h.db)
	ctx := c.Request().Context()

	wikis, total, err := extensionsRepo.GetWikisUsingExtension(
		ctx,
		extensionName,
		repository.ExtensionWikisListOptions{
			Page:  req.Page,
			Limit: req.Limit,
		},
	)

	if err != nil {
		extensionsLog.Info("Failed to get extension wikis", "err", err, "extension", extensionName)
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"detail": "Failed to get extension wikis",
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"extension_name": extensionName,
		"total":          total,
		"page":           req.Page,
		"limit":          req.Limit,
		"data":           wikis,
	})
}

// GetExtensionVersions handles GET /api/extensions/:name/versions
func (h *ExtensionsHandler) GetExtensionVersions(c echo.Context) error {
	extensionName := c.Param("name")
	if extensionName == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"detail": "Extension name is required",
		})
	}

	extensionsRepo := repository.NewExtensionsRepository(h.db)
	ctx := c.Request().Context()

	stats, total, err := extensionsRepo.GetExtensionVersionDistribution(ctx, extensionName)
	if err != nil {
		extensionsLog.Info("Failed to get extension versions", "err", err, "extension", extensionName)
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"detail": "Failed to get extension version distribution",
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"extension_name": extensionName,
		"total_wikis":    total,
		"versions":       stats,
	})
}

// GetAllExtensionsStats handles GET /api/extensions
func (h *ExtensionsHandler) GetAllExtensionsStats(c echo.Context) error {
	// Parse pagination parameters
	page := c.QueryParam("page")
	limit := c.QueryParam("limit")
	search := c.QueryParam("search")

	pageInt := 1
	limitInt := 50

	if page != "" {
		if p, err := strconv.Atoi(page); err == nil && p > 0 {
			pageInt = p
		}
	}

	if limit != "" {
		if l, err := strconv.Atoi(limit); err == nil && l > 0 && l <= 500 {
			limitInt = l
		}
	}

	extensionsRepo := repository.NewExtensionsRepository(h.db)
	ctx := c.Request().Context()

	stats, total, err := extensionsRepo.GetAllExtensionsStats(
		ctx,
		repository.GetAllExtensionsStatsOptions{
			Page:   pageInt,
			Limit:  limitInt,
			Search: search,
		},
	)
	if err != nil {
		extensionsLog.Info("Failed to get extensions stats", "err", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"detail": "Failed to get extensions statistics",
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"extensions": stats,
		"total":      total,
		"page":       pageInt,
		"limit":      limitInt,
		"pages":      (total + int64(limitInt) - 1) / int64(limitInt),
	})
}
