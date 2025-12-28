package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"wikikeeper-backend/internal/config"
	"wikikeeper-backend/internal/database"
	"wikikeeper-backend/internal/handlers"
	applogger "wikikeeper-backend/internal/logger"
	appmiddleware "wikikeeper-backend/internal/middleware"
	"wikikeeper-backend/internal/services"
)

func main() {
	// Load configuration
	cfg := config.Load()

	// Initialize logger
	applogger.Init(cfg.LogLevel)
	applogger.Log.Info("starting WikiKeeper",
		"version", cfg.AppVersion,
		"port", cfg.Port,
	)

	// Connect to database
	db, err := database.Connect()
	if err != nil {
		applogger.Log.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer database.Close()

	applogger.Log.Info("database connection successful")

	// Initialize services
	mwService := services.NewMediaWikiService(
		time.Duration(cfg.HTTPTimeout)*time.Second,
		cfg.HTTPUserAgent,
	)
	archiveService := services.NewArchiveService(
		time.Duration(cfg.HTTPTimeout)*time.Second,
		cfg.HTTPUserAgent,
		cfg.ArchiveCheckDelay,
	)

	// Start collection scheduler
	scheduler := services.NewCollectionScheduler(db, mwService, archiveService, cfg)
	ctx := context.Background()
	scheduler.Start(ctx)
	applogger.Log.Info("collection scheduler started")
	defer scheduler.Stop()

	// Start archive check scheduler
	archiveScheduler := services.NewArchiveScheduler(db, archiveService, cfg)
	archiveScheduler.Start(ctx)
	applogger.Log.Info("archive check scheduler started")
	defer archiveScheduler.Stop()

	// Create Echo instance
	e := echo.New()

	// Middleware
	e.Use(middleware.Recover())
	applogger.Log.Info("CORS allowed origins", "origins", cfg.AllowOrigins)
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins:     cfg.AllowOrigins,
		AllowMethods:     []string{echo.GET, echo.POST, echo.DELETE, echo.PUT, echo.OPTIONS},
		AllowHeaders:     []string{echo.HeaderOrigin, echo.HeaderContentType, echo.HeaderAccept, echo.HeaderAuthorization},
		ExposeHeaders:    []string{echo.HeaderContentLength},
		AllowCredentials: true,
	}))
	e.Use(appmiddleware.PrometheusMiddleware())

	// Initialize handlers with database
	healthHandler := handlers.NewHealthHandler(cfg)
	wikiHandler := handlers.NewWikiHandler(db, cfg)
	statsHandler := handlers.NewStatsHandler(db, cfg)
	adminHandler := handlers.NewAdminHandler(db, cfg)

	// Routes
	e.GET("/", func(c echo.Context) error {
		return c.JSON(200, map[string]string{
			"name":    cfg.AppName,
			"version": cfg.AppVersion,
			"docs":    "/docs",
			"health":  "/health",
		})
	})

	e.GET("/health", healthHandler.Check)

	// API routes
	api := e.Group("/api")

	// Public stats endpoint (no auth required)
	api.GET("/stats/summary", statsHandler.Summary)

	// Wiki routes - public (GET requests for viewing data)
	api.GET("/wikis", wikiHandler.List)
	api.GET("/wikis/:id", wikiHandler.Get)
	api.GET("/wikis/:id/stats", wikiHandler.GetStats)
	api.GET("/wikis/:id/archives", wikiHandler.GetArchives)
	api.GET("/wikis/:id/thumbnail", wikiHandler.GetThumbnail)

	// Wiki routes - public POST with rate limiting
	api.POST("/wikis", wikiHandler.Create)
	api.POST("/wikis/:id/check", wikiHandler.TriggerCheck)
	api.POST("/wikis/:id/check-archive", wikiHandler.CheckArchive)

	// Admin routes - require admin token
	admin := api.Group("/admin")
	admin.Use(appmiddleware.AdminAuth(cfg))

	// Admin wiki management
	admin.DELETE("/wikis/:id", adminHandler.DeleteWiki)
	admin.GET("/wikis/:id/stats", adminHandler.GetWikiStats)

	// Admin bulk operations
	admin.POST("/collect-all", adminHandler.CollectAll)
	admin.POST("/check-all-archives", adminHandler.CheckAllArchives)

	// Prometheus metrics endpoint
	e.GET("/metrics", echo.WrapHandler(promhttp.Handler()))

	// Start server
	address := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	applogger.Log.Info("starting server", "address", address)

	// Graceful shutdown
	go func() {
		if err := e.Start(address); err != nil && err != http.ErrServerClosed {
			applogger.Log.Error("server startup failed", "error", err)
			os.Exit(1)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit
	applogger.Log.Info("shutting down server")

	// Shutdown Echo server
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := e.Shutdown(ctx); err != nil {
		applogger.Log.Error("server shutdown failed", "error", err)
	}

	applogger.Log.Info("server exited")
}
