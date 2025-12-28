package services

import (
	"context"
	"sync"
	"time"

	"gorm.io/gorm"

	"wikikeeper-backend/internal/config"
	applogger "wikikeeper-backend/internal/logger"
	"wikikeeper-backend/internal/metrics"
	"wikikeeper-backend/internal/repository"
)

// CollectionScheduler manages periodic wiki data collection
type CollectionScheduler struct {
	db         *gorm.DB
	mwService  *MediaWikiService
	archiveService *ArchiveService
	config     *config.Config
	ticker     *time.Ticker
	stopCh     chan struct{}
	wg         sync.WaitGroup
	mu         sync.Mutex
	running    bool
	nextRun    time.Time
}

// NewCollectionScheduler creates a new scheduler instance
func NewCollectionScheduler(db *gorm.DB, mwService *MediaWikiService, archiveService *ArchiveService, cfg *config.Config) *CollectionScheduler {
	return &CollectionScheduler{
		db:         db,
		mwService:  mwService,
		archiveService: archiveService,
		config:     cfg,
		stopCh:     make(chan struct{}),
		running:    false,
	}
}

// Start begins periodic collection
func (s *CollectionScheduler) Start(ctx context.Context) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		applogger.Log.Warn("scheduler already running")
		return
	}

	s.running = true

	// Calculate interval from config (default 1 hour)
	interval := time.Duration(s.config.CollectInterval) * time.Minute
	if interval == 0 {
		interval = 60 * time.Minute // Default: 1 hour
	}

	s.ticker = time.NewTicker(interval)
	s.nextRun = time.Now().Add(interval)

	applogger.Log.Info("scheduler started", "interval", interval)

	// Run initial collection
	s.wg.Add(1)
	go s.run(ctx)

	// Start periodic collection
	s.wg.Add(1)
	go s.periodicRun(ctx)
}

// Stop gracefully stops the scheduler
func (s *CollectionScheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return
	}

	applogger.Log.Info("stopping scheduler")

	if s.ticker != nil {
		s.ticker.Stop()
	}

	close(s.stopCh)
	s.wg.Wait()

	s.running = false
	applogger.Log.Info("scheduler stopped")
}

// run executes a single collection cycle
func (s *CollectionScheduler) run(ctx context.Context) {
	defer s.wg.Done()

	applogger.Log.Info("starting collection cycle")

	startTime := time.Now()

	// Get active wikis that need collection
	// Priority: NULL last_check_at first (never checked), then oldest last_check_at
	wikiRepo := repository.NewWikiRepository(s.db)
	wikis, _, err := wikiRepo.List(ctx, repository.ListOptions{
		Page:     1,
		PageSize: int(s.config.CollectBatchSize),
		Status:   nil, // Get all statuses
		// Order by last_check_at ASC (NULL first, then oldest)
		OrderBy:  "last_check_at ASC NULLS FIRST",
	})
	if err != nil {
		applogger.Log.Error("failed to get wikis", "error", err)
		return
	}

	totalWikis := len(wikis)
	applogger.Log.Info("found active wikis to process", "count", totalWikis)

	if totalWikis == 0 {
		return
	}

	// Process wikis with rate limiting
	successCount := 0
	errorCount := 0

	collector := NewCollectorService(s.db, s.mwService, s.config)

	for i, wiki := range wikis {
		// Check if we should stop
		select {
		case <-s.stopCh:
			applogger.Log.Warn("collection cycle interrupted")
			return
		default:
		}

		// Skip inactive wikis
		if !wiki.IsActive {
			continue
		}

		applogger.Log.Info("processing wiki", "index", i+1, "total", totalWikis, "url", wiki.URL)

		// Collect siteinfo
		if err := collector.CollectSingleWiki(ctx, wiki.ID); err != nil {
			applogger.Log.Error("failed to collect wiki", "id", wiki.ID, "url", wiki.URL, "error", err)
			errorCount++
			metrics.CollectionWikisFailed.Inc()
		} else {
			successCount++
		}
		metrics.CollectionWikisProcessed.Inc()
	}

	// Update metrics
	metrics.CollectionCycleTotal.Inc()
	metrics.CollectionCycleDuration.Observe(time.Since(startTime).Seconds())

	elapsed := time.Since(startTime)
	applogger.Log.Info("collection cycle completed",
		"success", successCount,
		"errors", errorCount,
		"duration", elapsed.Round(time.Second))
}

// periodicRun runs collection continuously with backoff based on last_check_at
func (s *CollectionScheduler) periodicRun(ctx context.Context) {
	defer s.wg.Done()

	for {
		select {
		case <-s.stopCh:
			applogger.Log.Info("periodic run stopped")
			return
		case <-ctx.Done():
			applogger.Log.Info("context cancelled")
			return
		default:
			// Check the oldest last_check_at before running
			wikiRepo := repository.NewWikiRepository(s.db)
			wikis, _, err := wikiRepo.List(ctx, repository.ListOptions{
				Page:     1,
				PageSize: 1,
				Status:   nil,
				OrderBy:  "last_check_at ASC NULLS FIRST",
			})
			if err != nil {
				applogger.Log.Error("failed to check wikis", "error", err)
				time.Sleep(10 * time.Second)
				continue
			}

			// Check if we need to back off
			if len(wikis) > 0 && wikis[0].LastCheckAt != nil {
				timeSinceLastCheck := time.Since(*wikis[0].LastCheckAt)
				backoffThreshold := 3 * 24 * time.Hour // 3 days

				if timeSinceLastCheck < backoffThreshold {
					// Calculate backoff time based on how recent the last check was
					// More recent = longer backoff (up to 60s max)
					hoursSinceCheck := timeSinceLastCheck.Hours()
					var backoffTime time.Duration
					if hoursSinceCheck < 24 {
						backoffTime = 60 * time.Second // checked within 24h, max backoff
					} else if hoursSinceCheck < 48 {
						backoffTime = 45 * time.Second // checked within 48h
					} else {
						backoffTime = 30 * time.Second // checked within 72h
					}
					applogger.Log.Info("backing off, recent update detected",
						"last_check", wikis[0].LastCheckAt,
						"hours_since", hoursSinceCheck,
						"backoff", backoffTime)
					time.Sleep(backoffTime)
					continue
				}
			}

			applogger.Log.Info("triggering collection")
			s.run(ctx)

			// Small delay to avoid tight loop
			time.Sleep(1 * time.Second)
		}
	}
}

// IsRunning returns whether the scheduler is currently running
func (s *CollectionScheduler) IsRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running
}

// GetNextRun returns the next scheduled run time
func (s *CollectionScheduler) GetNextRun() time.Time {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.nextRun
}

// TriggerManualRun manually triggers a collection cycle
func (s *CollectionScheduler) TriggerManualRun(ctx context.Context) {
	if !s.IsRunning() {
		applogger.Log.Warn("cannot trigger run: scheduler not running")
		return
	}

	applogger.Log.Info("manual collection triggered")
	s.wg.Add(1)
	go s.run(ctx)
}
