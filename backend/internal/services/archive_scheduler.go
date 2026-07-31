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

const (
	archiveCheckInterval    = 3 * 24 * time.Hour
	archiveBusyPollInterval = time.Second
	archiveIdlePollInterval = time.Minute
)

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
		archiveSchedulerLog.Info("already running")
		return
	}

	s.running = true

	archiveSchedulerLog.Info("started")

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

	archiveSchedulerLog.Info("stopping...")

	close(s.stopCh)
	s.wg.Wait()

	s.running = false
	archiveSchedulerLog.Info("stopped")
}

// run executes a single archive check cycle
func (s *ArchiveScheduler) run(ctx context.Context) {
	archiveSchedulerLog.Info("starting archive check cycle")

	startTime := time.Now()

	wikiRepo := repository.NewWikiRepository(s.db)
	wikis, err := wikiRepo.GetDueForArchiveCheck(
		ctx,
		s.config.ArchiveCheckBatchSize,
		time.Now().Add(-archiveCheckInterval),
	)
	if err != nil {
		archiveSchedulerLog.Error("failed to get wikis", "error", err)
		return
	}

	if len(wikis) == 0 {
		archiveSchedulerLog.Info("no due wikis found for archive checking")
		return
	}

	successCount := 0
	errorCount := 0

	for i, wiki := range wikis {
		// Check if we should stop
		select {
		case <-s.stopCh:
			archiveSchedulerLog.Warn("archive check cycle interrupted")
			return
		default:
		}

		archiveSchedulerLog.Info("checking wiki", "index", i+1, "total", len(wikis), "url", wiki.URL)

		// Check archives for this wiki
		apiURL := *wiki.APIURL
		indexURL := ""
		if wiki.IndexURL != nil {
			indexURL = *wiki.IndexURL
		}

		found, imported, updated, err := s.archiveService.CollectArchives(ctx, s.db, wiki.ID, apiURL, indexURL)
		if err != nil {
			archiveSchedulerLog.Info("failed to check wiki", "wiki_id", wiki.ID, "error", err)
			s.archiveService.UpdateWikiArchiveError(ctx, s.db, wiki.ID, err)
			errorCount++
		} else {
			archiveSchedulerLog.Info("archive check completed", "found", found, "imported", imported, "updated", updated)
			successCount++
		}
	}

	elapsed := time.Since(startTime)
	archiveSchedulerLog.Info("archive check cycle completed",
		"success", successCount, "errors", errorCount, "duration", elapsed.Round(time.Second))
}

// periodicRun runs archive checks continuously with backoff based on archive_last_check_at
func (s *ArchiveScheduler) periodicRun(ctx context.Context) {
	defer s.wg.Done()
	if s.config.ArchiveCheckBatchSize <= 0 {
		archiveSchedulerLog.Info("archive collection disabled", "batch_size", s.config.ArchiveCheckBatchSize)
		select {
		case <-s.stopCh:
			archiveSchedulerLog.Info("periodic run stopped")
		case <-ctx.Done():
			archiveSchedulerLog.Info("context cancelled")
		}
		return
	}

	ticker := time.NewTimer(archiveBusyPollInterval)

	for {
		select {
		case <-s.stopCh:
			archiveSchedulerLog.Info("periodic run stopped")
			return
		case <-ctx.Done():
			archiveSchedulerLog.Info("context cancelled")
			return
		case <-ticker.C:
			wikiRepo := repository.NewWikiRepository(s.db)
			wikis, err := wikiRepo.GetDueForArchiveCheck(
				ctx,
				1,
				time.Now().Add(-archiveCheckInterval),
			)
			if err != nil {
				archiveSchedulerLog.Error("failed to list wikis", "error", err, "sleep", archiveIdlePollInterval)
				ticker.Reset(archiveIdlePollInterval)
				continue
			}

			if len(wikis) == 0 {
				archiveSchedulerLog.Info("no due wikis found for archive checking", "sleep", archiveIdlePollInterval)
				ticker.Reset(archiveIdlePollInterval)
				continue
			}

			ticker.Reset(archiveBusyPollInterval)
			s.run(ctx)
		}
	}
}
