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
// This endpoint is used for cross-domain cookie setting
// Flow:
// 1. Frontend redirects to API domain: https://api.example.com/api/auth/callback?token=xxx&redirect_to=xxx
// 2. API validates token and sets cookie (same domain)
// 3. API redirects back to frontend
func (h *AuthHandler) Callback(c echo.Context) error {
	var req CallbackRequest
	if err := c.Bind(&req); err != nil {
		return c.String(http.StatusBadRequest, "Invalid request parameters")
	}

	// Validate token
	if h.config.AdminToken == "" {
		return c.String(http.StatusInternalServerError, "Admin authentication is not configured")
	}

	if req.Token == "" {
		return c.String(http.StatusBadRequest, "Token is required")
	}

	if req.Token != h.config.AdminToken {
		return c.String(http.StatusUnauthorized, "Invalid token")
	}

	// Set cookie on API domain
	cookie := &http.Cookie{
		Name:     "admintoken",
		Value:    req.Token,
		Path:     "/",
		MaxAge:   int(30 * 24 * time.Hour / time.Second), // 30 days
		HttpOnly: true,
		Secure:   c.Request().TLS != nil, // Secure only if using HTTPS
		SameSite: http.SameSiteNoneMode,  // None for cross-origin
	}
	c.SetCookie(cookie)

	// Redirect back to frontend
	if req.RedirectTo != "" {
		return c.Redirect(http.StatusFound, req.RedirectTo)
	}

	// Default redirect to root
	return c.Redirect(http.StatusFound, "/")
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

