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

	archiveSchedulerLog.Info("Started")

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

	close(s.stopCh)
	s.wg.Wait()

	s.running = false
	archiveSchedulerLog.Info("Stopped")
}

// run executes a single archive check cycle
func (s *ArchiveScheduler) run(ctx context.Context) {
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
		archiveSchedulerLog.Error("Failed to get wikis", "error", err)
		return
	}

	if len(wikis) == 0 {
		archiveSchedulerLog.Warn("Found wikis to check archives", "total", len(wikis))
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
			archiveSchedulerLog.Warn("Archive check cycle interrupted")
			return
		default:
		}

		// Skip wikis without API URL
		if wiki.APIURL == nil {
			archiveSchedulerLog.Info("Skipping wiki: no API URL", "url", wiki.URL)
			skippedCount++
			continue
		}

		archiveSchedulerLog.Info("Checking wiki", "index", i+1, "total", len(wikis), "url", wiki.URL)

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
	}

	elapsed := time.Since(startTime)
	archiveSchedulerLog.Info("Archive check cycle completed",
		"success", successCount, "errors", errorCount, "skipped", skippedCount, "duration", elapsed.Round(time.Second))
}

// periodicRun runs archive checks continuously with backoff based on archive_last_check_at
func (s *ArchiveScheduler) periodicRun(ctx context.Context) {
	defer s.wg.Done()

	ticker := time.NewTimer(time.Second)

	for {
		select {
		case <-s.stopCh:
			archiveSchedulerLog.Info("Periodic run stopped")
			return
		case <-ctx.Done():
			archiveSchedulerLog.Info("Context cancelled")
			return
		case <-ticker.C:
			// Check the oldest archive_last_check_at before running
			wikiRepo := repository.NewWikiRepository(s.db)
			wikis, _, err := wikiRepo.List(ctx, repository.ListOptions{
				Page:     1,
				PageSize: 1,
				Status:   nil,
				OrderBy:  "archive_last_check_at ASC NULLS FIRST",
			})
			if err != nil {
				archiveSchedulerLog.Error("Failed to list wikis", "error", err, "sleep", time.Minute)
				ticker.Reset(time.Minute)
				continue
			}

			if len(wikis) == 0 {
				archiveSchedulerLog.Info("No wikis found for archive checking", "sleep", time.Minute)
				ticker.Reset(time.Minute)
				continue
			}

			// Check if we need to back off
			if wikis[0].ArchiveLastCheckAt != nil {
				timeSinceLastCheck := time.Since(*wikis[0].ArchiveLastCheckAt)
				backoffThreshold := 3 * 24 * time.Hour // 3 days

				if timeSinceLastCheck < backoffThreshold {
					backoffTime := time.Hour
					archiveSchedulerLog.Info("Backing off, recent update detected",
						"last_check", wikis[0].ArchiveLastCheckAt,
						"since", timeSinceLastCheck,
						"backoff", backoffTime)
					ticker.Reset(backoffTime)
					continue
				}
			}

			ticker.Reset(time.Second)
			s.run(ctx)
		}
	}
}
