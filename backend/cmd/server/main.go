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
	"wikikeeper-backend/internal/pages"
	"wikikeeper-backend/internal/services"
)

func main() {
	time.Local = time.UTC

	// Load configuration
	cfg := config.Load()

	// Initialize logger
	applogger.Init(cfg.LogLevel)
	applogger.Log.Info("starting WikiKeeper",
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

	// Run migrations
	applogger.Log.Info("running database migrations")
	if err := database.RunMigrations(db); err != nil {
		applogger.Log.Error("failed to run migrations", "error", err)
		os.Exit(1)
	}
	applogger.Log.Info("database migrations completed")

	// Initialize services
	providerLimiter := services.NewProviderLimiter(db, cfg)
	mwService := services.NewMediaWikiService(
		time.Duration(cfg.HTTPTimeout)*time.Second,
		cfg.HTTPUserAgent,
		providerLimiter,
	)
	archiveService := services.NewArchiveService(
		time.Duration(cfg.HTTPTimeout)*time.Second,
		cfg.HTTPUserAgent,
	)
	collectorService := services.NewCollectorService(db, mwService, cfg, providerLimiter)

	// Start siteinfo scheduler
	siteinfoScheduler := services.NewSiteInfoScheduler(db, collectorService, archiveService, cfg)
	ctx := context.Background()
	siteinfoScheduler.Start(ctx)
	applogger.Log.Info("siteinfo scheduler started")
	defer siteinfoScheduler.Stop()

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
	wikiHandler := handlers.NewWikiHandler(db, cfg, collectorService)
	statsHandler := handlers.NewStatsHandler(db, cfg)
	adminHandler := handlers.NewAdminHandler(db, cfg, collectorService)
	authHandler := handlers.NewAuthHandler(cfg)
	extensionsHandler := handlers.NewExtensionsHandler(db)

	// Initialize page handlers
	pagesHandler := pages.NewPages(db, cfg, collectorService, archiveService)

	// Serve static files
	e.Static("/static", "web/static")

	// Page routes (HTML)
	e.GET("/", pagesHandler.Dashboard)
	e.GET("/wikis", pagesHandler.WikiList)
	e.GET("/wikis/add", pagesHandler.WikiAdd)
	e.POST("/wikis", pagesHandler.WikiAddSubmit)
	e.GET("/wikis/:id", pagesHandler.WikiDetail)
	e.GET("/wikis/:id/extensions/compare", pagesHandler.WikiExtensionsCompare)
	e.GET("/wikis/:id/stats/embed", pagesHandler.WikiStatsEmbed)
	e.POST("/wikis/:id/check", pagesHandler.TriggerCheck)
	e.POST("/wikis/:id/check-archive", pagesHandler.TriggerArchiveCheck)
	e.GET("/extensions", pagesHandler.ExtensionList)
	e.GET("/extensions/:name", pagesHandler.ExtensionDetail)
	e.GET("/login", authHandler.LoginPage)
	e.POST("/login", authHandler.Login)
	e.POST("/logout", authHandler.Logout)

	admin := e.Group("/admin")
	admin.Use(appmiddleware.AdminAuth(cfg))
	admin.DELETE("/wikis/:id", pagesHandler.AdminDeleteWiki)
	admin.POST("/collect-all", pagesHandler.AdminCollectAll)
	admin.POST("/check-all-archives", pagesHandler.AdminCheckAllArchives)

	e.GET("/health", healthHandler.Check)

	// API routes
	api := e.Group("/api")

	// Auth check endpoint (for verifying authentication status)
	api.GET("/auth/check", authHandler.Check)

	// Public stats endpoint (no auth required)
	api.GET("/stats/summary", statsHandler.Summary)

	// Wiki routes - public (GET requests for viewing data)
	api.GET("/wikis", wikiHandler.List)
	api.GET("/wikis/:id", wikiHandler.Get)
	api.GET("/wikis/:id/stats", wikiHandler.GetStats)
	api.GET("/wikis/:id/archives", wikiHandler.GetArchives)
	api.GET("/wikis/:id/thumbnail", wikiHandler.GetThumbnail)
	api.GET("/wikis/:id/extensions", extensionsHandler.GetLatestExtensions)
	api.GET("/wikis/:id/extensions/history", extensionsHandler.GetExtensionsHistory)
	api.GET("/extensions", extensionsHandler.GetAllExtensionsStats)
	api.GET("/extensions/:name/wikis", extensionsHandler.GetExtensionWikis)
	api.GET("/extensions/:name/versions", extensionsHandler.GetExtensionVersions)

	// Wiki routes - public POST with rate limiting
	api.POST("/wikis", wikiHandler.Create)
	api.POST("/wikis/:id/check", wikiHandler.TriggerCheck)
	api.POST("/wikis/:id/check-archive", wikiHandler.CheckArchive)

	// Admin routes - require admin token
	adminAPI := api.Group("/admin")
	adminAPI.Use(appmiddleware.AdminAuth(cfg))

	// Admin wiki management
	adminAPI.DELETE("/wikis/:id", adminHandler.DeleteWiki)
	adminAPI.GET("/wikis/:id/stats", adminHandler.GetWikiStats)

	// Admin bulk operations
	adminAPI.POST("/collect-all", adminHandler.CollectAll)
	adminAPI.POST("/check-all-archives", adminHandler.CheckAllArchives)

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
