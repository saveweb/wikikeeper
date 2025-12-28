package middleware

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"wikikeeper-backend/internal/config"
)

// AdminAuth creates middleware that checks for admin token in cookie
func AdminAuth(cfg *config.Config) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// If no admin token configured, allow all
			if cfg.AdminToken == "" {
				return next(c)
			}

			// Get token from cookie
			cookie, err := c.Cookie("admintoken")
			if err != nil {
				return c.JSON(http.StatusUnauthorized, map[string]string{
					"detail": "Admin token required. Set 'admintoken' cookie.",
				})
			}

			// Validate token
			if cookie.Value != cfg.AdminToken {
				return c.JSON(http.StatusUnauthorized, map[string]string{
					"detail": "Invalid admin token",
				})
			}

			return next(c)
		}
	}
}

// CheckRateLimit creates middleware for rate limiting check endpoints
// Allows 1 check per hour per wiki for anonymous users
func CheckRateLimit(checkType string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// If user has admin token, skip rate limit
			if cfg := c.Get("config").(*config.Config); cfg != nil {
				if cookie, err := c.Cookie("admintoken"); err == nil {
					if cookie.Value == cfg.AdminToken && cfg.AdminToken != "" {
						return next(c)
					}
				}
			}

			// For anonymous users, check if rate limited
			// This is handled in the handler itself using database timestamps
			return next(c)
		}
	}
}
