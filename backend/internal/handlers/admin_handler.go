package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"gorm.io/gorm"

	applogger "wikikeeper-backend/internal/logger"

	"wikikeeper-backend/internal/config"
	"wikikeeper-backend/internal/repository"
	"wikikeeper-backend/internal/services"
)

// AdminHandler handles admin-only requests
type AdminHandler struct {
	db     *gorm.DB
	config *config.Config
}

// NewAdminHandler creates a new admin handler
func NewAdminHandler(db *gorm.DB, cfg *config.Config) *AdminHandler {
	return &AdminHandler{db: db, config: cfg}
}

// DeleteWiki handles DELETE /api/admin/wikis/:id
func (h *AdminHandler) DeleteWiki(c echo.Context) error {
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

	// Delete wiki (cascades to stats and archives)
	if err := wikiRepo.Delete(ctx, id); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"detail": err.Error()})
	}

	applogger.Log.Info("[Admin] Wiki deleted", "id", id)

	return c.JSON(http.StatusOK, map[string]string{
		"detail":  "Wiki deleted",
		"wiki_id": idStr,
	})
}

// CollectAll handles POST /api/admin/collect-all
// Triggers collection for all active wikis
func (h *AdminHandler) CollectAll(c echo.Context) error {
	// Start background collection for all wikis
	go func() {
		ctx := context.Background()
		wikiRepo := repository.NewWikiRepository(h.db)

		// Get all active wikis
		wikis, total, err := wikiRepo.List(ctx, repository.ListOptions{
			PageSize: 10000, // Get all
		})
		if err != nil {
			applogger.Log.Info("[Admin] Failed to get wikis for collection", "err", err)
			return
		}

		applogger.Log.Info("[Admin] Starting collection for wikis", "count", len(wikis), "total", total)

		mwService := services.NewMediaWikiService(
			time.Duration(h.config.HTTPTimeout)*time.Second,
			h.config.HTTPUserAgent,
		)
		collector := services.NewCollectorService(h.db, mwService, h.config)

		successCount := 0
		errorCount := 0

		for i, wiki := range wikis {
			if !wiki.IsActive {
				continue
			}

			applogger.Log.Info("[Admin] Collecting wiki", "index", i+1, "total", len(wikis), "url", wiki.URL)

			if err := collector.CollectSingleWiki(ctx, wiki.ID); err != nil {
				applogger.Log.Info("[Admin] Failed to collect wiki", "wiki_id", wiki.ID, "err", err)
				errorCount++
			} else {
				successCount++
			}
		}

		applogger.Log.Info("[Admin] Collection completed", "success", successCount, "errors", errorCount)
	}()

	return c.JSON(http.StatusAccepted, map[string]string{
		"detail": "Full collection started for all active wikis",
	})
}

// CheckAllArchives handles POST /api/admin/check-all-archives
// Triggers archive check for all wikis
func (h *AdminHandler) CheckAllArchives(c echo.Context) error {
	// Start background archive check for all wikis
	go func() {
		ctx := context.Background()
		wikiRepo := repository.NewWikiRepository(h.db)

		// Get all wikis
		wikis, total, err := wikiRepo.List(ctx, repository.ListOptions{
			PageSize: 10000, // Get all
		})
		if err != nil {
			applogger.Log.Info("[Admin] Failed to get wikis for archive check", "err", err)
			return
		}

		applogger.Log.Info("[Admin] Starting archive check for wikis", "count", len(wikis), "total", total)

		archiveService := services.NewArchiveService(
			time.Duration(h.config.HTTPTimeout)*time.Second,
			h.config.HTTPUserAgent,
		)

		successCount := 0
		errorCount := 0
		skippedCount := 0

		for i, wiki := range wikis {
			applogger.Log.Info("[Admin] Checking wiki", "index", i+1, "total", len(wikis), "url", wiki.URL)

			// Skip wikis without API URL
			if wiki.APIURL == nil {
				applogger.Log.Info("[Admin] Skipping wiki: no API URL", "url", wiki.URL)
				skippedCount++
				continue
			}

			apiURL := *wiki.APIURL
			indexURL := ""
			if wiki.IndexURL != nil {
				indexURL = *wiki.IndexURL
			}

			found, imported, updated, err := archiveService.CollectArchives(ctx, h.db, wiki.ID, apiURL, indexURL)
			if err != nil {
				applogger.Log.Info("[Admin] Failed to check wiki", "wiki_id", wiki.ID, "err", err)
				archiveService.UpdateWikiArchiveError(ctx, h.db, wiki.ID, err)
				errorCount++
			} else {
				applogger.Log.Info("[Admin] Archive check completed", "found", found, "imported", imported, "updated", updated)
				successCount++
			}
		}

		applogger.Log.Info("[Admin] Archive check completed", "success", successCount, "errors", errorCount, "skipped", skippedCount)
	}()

	return c.JSON(http.StatusAccepted, map[string]string{
		"detail": "Archive check started for all wikis",
	})
}

// GetWikiStats handles GET /api/admin/wiki/:id/stats
// Returns detailed stats including status information
func (h *AdminHandler) GetWikiStats(c echo.Context) error {
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

	return c.JSON(http.StatusOK, map[string]interface{}{
		"wiki_id":               idStr,
		"url":                   wiki.URL,
		"sitename":              wiki.Sitename,
		"status":                wiki.Status,
		"is_active":             wiki.IsActive,
		"last_check_at":         wiki.LastCheckAt,
		"last_error":            wiki.LastError,
		"last_error_at":         wiki.LastErrorAt,
		"archive_last_check_at": wiki.ArchiveLastCheckAt,
		"archive_last_error":    wiki.ArchiveLastError,
		"archive_last_error_at": wiki.ArchiveLastErrorAt,
		"has_archive":           wiki.HasArchive,
		"api_available":         wiki.APIAvailable,
	})
}
