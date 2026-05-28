package handlers

import (
	"net/http"

	"wikikeeper-backend/internal/config"

	"github.com/labstack/echo/v4"
)

// HealthHandler handles health check requests
type HealthHandler struct {
	config *config.Config
}

// NewHealthHandler creates a new health handler
func NewHealthHandler(cfg *config.Config) *HealthHandler {
	return &HealthHandler{config: cfg}
}

// Check handles GET /health
func (h *HealthHandler) Check(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]interface{}{
		"status": "ok",
	})
}
