package handlers

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"

	"wikikeeper-backend/internal/config"
	"wikikeeper-backend/internal/repository"
)

// StatsHandler handles stats requests
type StatsHandler struct {
	db     *gorm.DB
	config *config.Config
}

// NewStatsHandler creates a new stats handler
func NewStatsHandler(db *gorm.DB, cfg *config.Config) *StatsHandler {
	return &StatsHandler{db: db, config: cfg}
}

// Summary handles GET /api/stats/summary
func (h *StatsHandler) Summary(c echo.Context) error {
	wikiRepo := repository.NewWikiRepository(h.db)
	ctx := c.Request().Context()

	stats, err := wikiRepo.GetSummaryStats(ctx)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"detail": err.Error()})
	}

	return c.JSON(http.StatusOK, stats)
}
