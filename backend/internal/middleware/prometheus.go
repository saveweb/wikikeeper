package middleware

import (
	"time"

	"github.com/labstack/echo/v4"
	"strconv"

	"wikikeeper-backend/internal/metrics"
)

// PrometheusMiddleware tracks HTTP requests with Prometheus metrics
func PrometheusMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			start := time.Now()

			// Process request
			err := next(c)

			// Record metrics
			duration := time.Since(start).Seconds()
			status := c.Response().Status
			method := c.Request().Method
			path := c.Path()

			// Skip metrics endpoint itself
			if path != "/metrics" {
				metrics.HTTPRequestsTotal.WithLabelValues(method, path, strconv.Itoa(status)).Inc()
				metrics.HTTPRequestDuration.WithLabelValues(method, path).Observe(duration)
			}

			return err
		}
	}
}
