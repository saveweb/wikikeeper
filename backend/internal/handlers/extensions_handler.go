package handlers

import (
	"net/http"
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
		fromSet = true
	}

	if toStr != "" {
		to, err = time.Parse(time.RFC3339, toStr)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"detail": "Invalid to time format"})
		}
		toSet = true
	}

	// Default time range: last 30 days
	if !fromSet {
		from = time.Now().AddDate(0, 0, -30)
	}
	if !toSet {
		to = time.Now()
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
		"from":      from.Format(time.RFC3339),
		"to":        to.Format(time.RFC3339),
		"snapshots": snapshots,
	})
}
