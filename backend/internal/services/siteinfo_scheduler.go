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

var siteinfoSchedulerLog = applogger.With("component", "siteinfo-scheduler")

// SiteInfoScheduler manages periodic wiki siteinfo collection
type SiteInfoScheduler struct {
	db             *gorm.DB
	mwService      *MediaWikiService
	archiveService *ArchiveService
	config         *config.Config
	stopCh         chan struct{}
	wg             sync.WaitGroup
	mu             sync.Mutex
	running        bool
}

// NewSiteInfoScheduler creates a new siteinfo scheduler instance
func NewSiteInfoScheduler(db *gorm.DB, mwService *MediaWikiService, archiveService *ArchiveService, cfg *config.Config) *SiteInfoScheduler {
	return &SiteInfoScheduler{
		db:             db,
		mwService:      mwService,
		archiveService: archiveService,
		config:         cfg,
		stopCh:         make(chan struct{}),
		running:        false,
	}
}

// Start begins periodic collection
func (s *SiteInfoScheduler) Start(ctx context.Context) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		siteinfoSchedulerLog.Warn("siteinfo scheduler already running")
		return
	}

	s.running = true

	siteinfoSchedulerLog.Info("siteinfo scheduler started")

	// Start periodic collection
	s.wg.Add(1)
	go s.periodicRun(ctx)
}

// Stop gracefully stops the scheduler
func (s *SiteInfoScheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return
	}

	siteinfoSchedulerLog.Info("stopping siteinfo scheduler")

	close(s.stopCh)
	s.wg.Wait()

	s.running = false
	siteinfoSchedulerLog.Info("siteinfo scheduler stopped")
}

// run executes a single collection cycle
func (s *SiteInfoScheduler) run(ctx context.Context) {
	siteinfoSchedulerLog.Info("starting collection cycle")

	startTime := time.Now()

	// Get active wikis that need collection
	// Priority: NULL last_check_at first (never checked), then oldest last_check_at
	wikiRepo := repository.NewWikiRepository(s.db)
	wikis, _, err := wikiRepo.List(ctx, repository.ListOptions{
		Page:     1,
		PageSize: int(s.config.SiteinfoCheckBatchSize),
		Status:   nil, // Get all statuses
		// Order by last_check_at ASC (NULL first, then oldest)
		OrderBy: "last_check_at ASC NULLS FIRST",
	})
	if err != nil {
		siteinfoSchedulerLog.Error("failed to get wikis", "error", err)
		return
	}

	totalWikis := len(wikis)
	siteinfoSchedulerLog.Info("found active wikis to process", "count", totalWikis)

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
			siteinfoSchedulerLog.Warn("collection cycle interrupted")
			return
		default:
		}

		// Skip inactive wikis
		if !wiki.IsActive {
			continue
		}

		// Skip recently checked wikis
		if wiki.LastCheckAt != nil {
			timeSinceLastCheck := time.Since(*wiki.LastCheckAt)
			backoffThreshold := 3 * 24 * time.Hour // 3 days

			if timeSinceLastCheck < backoffThreshold {
				siteinfoSchedulerLog.Info("skipping wiki, recent update detected",
					"url", wiki.URL,
					"last_check", wiki.LastCheckAt,
					"since", timeSinceLastCheck)
				continue
			}
		}

		siteinfoSchedulerLog.Info("processing wiki", "index", i+1, "total", totalWikis, "url", wiki.URL)

		// Collect siteinfo
		if err := collector.CollectSingleWiki(ctx, wiki.ID); err != nil {
			siteinfoSchedulerLog.Error("failed to collect wiki", "id", wiki.ID, "url", wiki.URL, "error", err)
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
	siteinfoSchedulerLog.Info("collection cycle completed",
		"success", successCount,
		"errors", errorCount,
		"duration", elapsed.Round(time.Second))
}

// periodicRun runs collection continuously with backoff based on last_check_at
func (s *SiteInfoScheduler) periodicRun(ctx context.Context) {
	defer s.wg.Done()

	ticker := time.NewTimer(time.Second)
	for {
		select {
		case <-s.stopCh:
			siteinfoSchedulerLog.Info("periodic run stopped")
			return
		case <-ctx.Done():
			siteinfoSchedulerLog.Info("context cancelled")
			return
		case <-ticker.C:
			// Check the oldest last_check_at before running
			wikiRepo := repository.NewWikiRepository(s.db)
			wikis, _, err := wikiRepo.List(ctx, repository.ListOptions{
				Page:     1,
				PageSize: 1,
				Status:   nil,
				OrderBy:  "last_check_at ASC NULLS FIRST",
			})
			if err != nil {
				siteinfoSchedulerLog.Error("failed to check wikis", "error", err)
				ticker.Reset(time.Minute)
				continue
			}

			if len(wikis) == 0 {
				siteinfoSchedulerLog.Info("no wikis found for checking", "sleep", time.Minute)
				ticker.Reset(time.Minute)
				continue
			}

			// Check if we need to back off
			if wikis[0].LastCheckAt != nil {
				timeSinceLastCheck := time.Since(*wikis[0].LastCheckAt)
				backoffThreshold := 3 * 24 * time.Hour // 3 days

				if timeSinceLastCheck < backoffThreshold {
					siteinfoSchedulerLog.Info("backing off, recent update detected",
						"last_check", wikis[0].LastCheckAt,
						"since", timeSinceLastCheck)
					ticker.Reset(time.Minute)
					continue
				}
			}

			ticker.Reset(time.Second)
			s.run(ctx)
		}
	}
}
