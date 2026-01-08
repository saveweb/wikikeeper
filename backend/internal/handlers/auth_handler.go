package handlers

import (
	"net/http"
	"time"

	"github.com/labstack/echo/v4"

	"wikikeeper-backend/internal/config"
)

// AuthHandler handles authentication HTTP requests
type AuthHandler struct {
	config *config.Config
}

// NewAuthHandler creates a new auth handler
func NewAuthHandler(cfg *config.Config) *AuthHandler {
	return &AuthHandler{config: cfg}
}

// CallbackRequest represents query parameters for auth callback
type CallbackRequest struct {
	Token      string `query:"token"`
	RedirectTo string `query:"redirect_to"`
}

// Callback handles GET /api/auth/callback
// This endpoint is used for cross-domain cookie setting/clearing
// Flow:
// 1. Frontend redirects to API domain: https://api.example.com/api/auth/callback?token=xxx&redirect_to=xxx
// 2a. If token is provided and valid: API sets cookie (same domain)
// 2b. If token is empty: API clears cookie
// 3. API redirects back to frontend
func (h *AuthHandler) Callback(c echo.Context) error {
	var req CallbackRequest
	if err := c.Bind(&req); err != nil {
		return c.String(http.StatusBadRequest, "Invalid request parameters")
	}

	// Validate token (if provided)
	if h.config.AdminToken == "" {
		return c.String(http.StatusInternalServerError, "Admin authentication is not configured")
	}

	var cookie *http.Cookie

	if req.Token != "" {
		// Validate and set cookie
		if req.Token != h.config.AdminToken {
			return c.String(http.StatusUnauthorized, "Invalid token")
		}

	}
	cookie = &http.Cookie{
		Name:     "admintoken",
		Value:    req.Token,
		Path:     "/",
		MaxAge:   int(30 * 24 * time.Hour / time.Second), // 30 days
		HttpOnly: true,
		Secure:   c.Request().TLS != nil || c.Request().Header.Get("X-Forwarded-Proto") == "https",
		SameSite: http.SameSiteNoneMode,
	}
	c.SetCookie(cookie)

	// Redirect back to frontend
	return c.HTML(http.StatusOK, `<html><head><meta http-equiv="refresh" content="0;url=`+req.RedirectTo+`"></head><body></body></html>`)
}

// Check handles GET /api/auth/check
// This endpoint checks if the user has a valid admin token cookie
func (h *AuthHandler) Check(c echo.Context) error {
	if h.config.AdminToken == "" {
		return c.JSON(http.StatusOK, map[string]bool{"authenticated": false})
	}

	cookie, err := c.Cookie("admintoken")
	if err != nil {
		return c.JSON(http.StatusOK, map[string]bool{"authenticated": false})
	}

	isAuthenticated := cookie.Value == h.config.AdminToken
	return c.JSON(http.StatusOK, map[string]bool{"authenticated": isAuthenticated})
}
