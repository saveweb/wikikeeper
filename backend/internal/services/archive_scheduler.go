package services

import (
	"context"
	"sync"
	"time"

	"gorm.io/gorm"

	"wikikeeper-backend/internal/config"
	applogger "wikikeeper-backend/internal/logger"
	"wikikeeper-backend/internal/repository"
)

var archiveSchedulerLog = applogger.With("component", "archive-scheduler")

// ArchiveScheduler manages periodic archive.org checking
type ArchiveScheduler struct {
	db             *gorm.DB
	archiveService *ArchiveService
	config         *config.Config
	ticker         *time.Ticker
	stopCh         chan struct{}
	wg             sync.WaitGroup
	mu             sync.Mutex
	running        bool
}

// NewArchiveScheduler creates a new archive scheduler instance
func NewArchiveScheduler(db *gorm.DB, archiveService *ArchiveService, cfg *config.Config) *ArchiveScheduler {
	return &ArchiveScheduler{
		db:             db,
		archiveService: archiveService,
		config:         cfg,
		stopCh:         make(chan struct{}),
		running:        false,
	}
}

// Start begins periodic archive checking
func (s *ArchiveScheduler) Start(ctx context.Context) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		archiveSchedulerLog.Info("Already running")
		return
	}

	s.running = true

	// Calculate interval from config (default 12 hours)
	interval := time.Duration(s.config.ArchiveCheckInterval) * time.Minute
	if interval == 0 {
		interval = 12 * 60 * time.Minute // Default: 12 hours
	}

	s.ticker = time.NewTicker(interval)

	archiveSchedulerLog.Info("Started with interval", "interval", interval)

	// Run initial archive check
	s.wg.Add(1)
	go s.run(ctx)

	// Start periodic archive checking
	s.wg.Add(1)
	go s.periodicRun(ctx)
}

// Stop gracefully stops the scheduler
func (s *ArchiveScheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return
	}

	archiveSchedulerLog.Info("Stopping...")

	if s.ticker != nil {
		s.ticker.Stop()
	}

	close(s.stopCh)
	s.wg.Wait()

	s.running = false
	archiveSchedulerLog.Info("Stopped")
}

// run executes a single archive check cycle
func (s *ArchiveScheduler) run(ctx context.Context) {
	defer s.wg.Done()

	archiveSchedulerLog.Info("Starting archive check cycle")

	startTime := time.Now()

	// Get wikis that need archive checking
	// Priority: NULL archive_last_check_at first (never checked), then oldest archive_last_check_at
	wikiRepo := repository.NewWikiRepository(s.db)
	wikis, _, err := wikiRepo.List(ctx, repository.ListOptions{
		Page:     1,
		PageSize: int(s.config.ArchiveCheckBatchSize),
		Status:   nil, // Get all statuses
		// Order by archive_last_check_at ASC (NULL first, then oldest)
		OrderBy: "archive_last_check_at ASC NULLS FIRST",
	})
	if err != nil {
		archiveSchedulerLog.Info("Failed to get wikis", "error", err)
		return
	}

	totalWikis := len(wikis)
	archiveSchedulerLog.Info("Found wikis to check archives", "total", totalWikis)

	if totalWikis == 0 {
		return
	}

	// Process wikis with rate limiting
	successCount := 0
	errorCount := 0
	skippedCount := 0

	for i, wiki := range wikis {
		// Check if we should stop
		select {
		case <-s.stopCh:
			archiveSchedulerLog.Info("Archive check cycle interrupted")
			return
		default:
		}

		// Skip wikis without API URL
		if wiki.APIURL == nil {
			archiveSchedulerLog.Info("Skipping wiki: no API URL", "url", wiki.URL)
			skippedCount++
			continue
		}

		archiveSchedulerLog.Info("Checking wiki", "index", i+1, "total", totalWikis, "url", wiki.URL)

		// Check archives for this wiki
		apiURL := *wiki.APIURL
		indexURL := ""
		if wiki.IndexURL != nil {
			indexURL = *wiki.IndexURL
		}

		found, imported, updated, err := s.archiveService.CollectArchives(ctx, s.db, wiki.ID, apiURL, indexURL)
		if err != nil {
			archiveSchedulerLog.Info("Failed to check wiki", "wiki_id", wiki.ID, "error", err)
			s.archiveService.UpdateWikiArchiveError(ctx, s.db, wiki.ID, err)
			errorCount++
		} else {
			archiveSchedulerLog.Info("Archive check completed", "found", found, "imported", imported, "updated", updated)
			successCount++
		}

		// Rate limiting delay
		if i < totalWikis-1 && s.config.ArchiveCheckDelay > 0 {
			delay := time.Duration(s.config.ArchiveCheckDelay * float64(time.Second))
			archiveSchedulerLog.Info("Waiting before next wiki", "delay", delay)
			select {
			case <-time.After(delay):
			case <-s.stopCh:
				archiveSchedulerLog.Info("Archive check cycle interrupted during delay")
				return
			}
		}
	}

	elapsed := time.Since(startTime)
	archiveSchedulerLog.Info("Archive check cycle completed",
		"success", successCount, "errors", errorCount, "skipped", skippedCount, "duration", elapsed.Round(time.Second))
}

// periodicRun runs archive checks continuously with backoff based on archive_last_check_at
func (s *ArchiveScheduler) periodicRun(ctx context.Context) {
	defer s.wg.Done()

	for {
		select {
		case <-s.stopCh:
			archiveSchedulerLog.Info("Periodic run stopped")
			return
		case <-ctx.Done():
			archiveSchedulerLog.Info("Context cancelled")
			return
		case <-s.ticker.C:
			// Check the oldest archive_last_check_at before running
			wikiRepo := repository.NewWikiRepository(s.db)
			wikis, _, err := wikiRepo.List(ctx, repository.ListOptions{
				Page:     1,
				PageSize: 1,
				Status:   nil,
				OrderBy:  "archive_last_check_at ASC NULLS FIRST",
			})
			if err != nil {
				archiveSchedulerLog.Info("Failed to check wikis", "error", err)
				continue
			}

			// Check if we need to back off
			if len(wikis) > 0 && wikis[0].ArchiveLastCheckAt != nil {
				timeSinceLastCheck := time.Since(*wikis[0].ArchiveLastCheckAt)
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
					archiveSchedulerLog.Info("Backing off, recent update detected",
						"last_check", wikis[0].ArchiveLastCheckAt,
						"hours_since", hoursSinceCheck,
						"backoff", backoffTime)
					// Skip this cycle, will retry after configured interval
					continue
				}
			}

			archiveSchedulerLog.Info("Triggering archive check")
			s.wg.Add(1)
			go s.run(ctx)
		}
	}
}

// IsRunning returns whether the scheduler is currently running
func (s *ArchiveScheduler) IsRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running
}

// TriggerManualRun manually triggers an archive check cycle
func (s *ArchiveScheduler) TriggerManualRun(ctx context.Context) {
	if !s.IsRunning() {
		archiveSchedulerLog.Info("Cannot trigger run: scheduler not running")
		return
	}

	archiveSchedulerLog.Info("Manual archive check triggered")
	s.wg.Add(1)
	go s.run(ctx)
}
