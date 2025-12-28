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
		applogger.Log.Info("[ArchiveScheduler] Already running")
		return
	}

	s.running = true

	// Calculate interval from config (default 12 hours)
	interval := time.Duration(s.config.ArchiveCheckInterval) * time.Minute
	if interval == 0 {
		interval = 12 * 60 * time.Minute // Default: 12 hours
	}

	s.ticker = time.NewTicker(interval)

	applogger.Log.Info("[ArchiveScheduler] Started with interval: %v", interval)

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

	applogger.Log.Info("[ArchiveScheduler] Stopping...")

	if s.ticker != nil {
		s.ticker.Stop()
	}

	close(s.stopCh)
	s.wg.Wait()

	s.running = false
	applogger.Log.Info("[ArchiveScheduler] Stopped")
}

// run executes a single archive check cycle
func (s *ArchiveScheduler) run(ctx context.Context) {
	defer s.wg.Done()

	applogger.Log.Info("[ArchiveScheduler] Starting archive check cycle")

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
		applogger.Log.Info("[ArchiveScheduler] Failed to get wikis: %v", err)
		return
	}

	totalWikis := len(wikis)
	applogger.Log.Info("[ArchiveScheduler] Found %d wikis to check archives", totalWikis)

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
			applogger.Log.Info("[ArchiveScheduler] Archive check cycle interrupted")
			return
		default:
		}

		// Skip wikis without API URL
		if wiki.APIURL == nil {
			applogger.Log.Info("[ArchiveScheduler] Skipping wiki %s: no API URL", wiki.URL)
			skippedCount++
			continue
		}

		applogger.Log.Info("[ArchiveScheduler] Checking wiki %d/%d: %s", i+1, totalWikis, wiki.URL)

		// Check archives for this wiki
		apiURL := *wiki.APIURL
		indexURL := ""
		if wiki.IndexURL != nil {
			indexURL = *wiki.IndexURL
		}

		found, imported, updated, err := s.archiveService.CollectArchives(ctx, s.db, wiki.ID, apiURL, indexURL)
		if err != nil {
			applogger.Log.Info("[ArchiveScheduler] Failed to check wiki %s: %v", wiki.ID, err)
			s.archiveService.UpdateWikiArchiveError(ctx, s.db, wiki.ID, err)
			errorCount++
		} else {
			applogger.Log.Info("[ArchiveScheduler] Archive check completed: found=%d, imported=%d, updated=%d", found, imported, updated)
			successCount++
		}

		// Rate limiting delay
		if i < totalWikis-1 && s.config.ArchiveCheckDelay > 0 {
			delay := time.Duration(s.config.ArchiveCheckDelay * float64(time.Second))
			applogger.Log.Info("[ArchiveScheduler] Waiting %v before next wiki...", delay)
			select {
			case <-time.After(delay):
			case <-s.stopCh:
				applogger.Log.Info("[ArchiveScheduler] Archive check cycle interrupted during delay")
				return
			}
		}
	}

	elapsed := time.Since(startTime)
	applogger.Log.Info("[ArchiveScheduler] Archive check cycle completed: %d success, %d errors, %d skipped, duration: %v",
		successCount, errorCount, skippedCount, elapsed.Round(time.Second))
}

// periodicRun runs archive checks continuously with backoff based on archive_last_check_at
func (s *ArchiveScheduler) periodicRun(ctx context.Context) {
	defer s.wg.Done()

	for {
		select {
		case <-s.stopCh:
			applogger.Log.Info("[ArchiveScheduler] Periodic run stopped")
			return
		case <-ctx.Done():
			applogger.Log.Info("[ArchiveScheduler] Context cancelled")
			return
		default:
			// Check the oldest archive_last_check_at before running
			wikiRepo := repository.NewWikiRepository(s.db)
			wikis, _, err := wikiRepo.List(ctx, repository.ListOptions{
				Page:     1,
				PageSize: 1,
				Status:   nil,
				OrderBy:  "archive_last_check_at ASC NULLS FIRST",
			})
			if err != nil {
				applogger.Log.Info("[ArchiveScheduler] Failed to check wikis: %v", err)
				time.Sleep(10 * time.Second)
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
					applogger.Log.Info("[ArchiveScheduler] Backing off, recent update detected",
						"last_check", wikis[0].ArchiveLastCheckAt,
						"hours_since", hoursSinceCheck,
						"backoff", backoffTime)
					time.Sleep(backoffTime)
					continue
				}
			}

			applogger.Log.Info("[ArchiveScheduler] Triggering archive check")
			s.run(ctx)

			// Small delay to avoid tight loop
			time.Sleep(1 * time.Second)
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
		applogger.Log.Info("[ArchiveScheduler] Cannot trigger run: scheduler not running")
		return
	}

	applogger.Log.Info("[ArchiveScheduler] Manual archive check triggered")
	s.wg.Add(1)
	go s.run(ctx)
}
