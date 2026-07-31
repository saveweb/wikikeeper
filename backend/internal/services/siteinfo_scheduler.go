package services

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"wikikeeper-backend/internal/config"
	applogger "wikikeeper-backend/internal/logger"
	"wikikeeper-backend/internal/metrics"
	"wikikeeper-backend/internal/models"
	"wikikeeper-backend/internal/repository"
)

var siteinfoSchedulerLog = applogger.With("component", "siteinfo-scheduler")

// SiteInfoScheduler manages periodic wiki siteinfo collection
type SiteInfoScheduler struct {
	db             *gorm.DB
	collector      *CollectorService
	archiveService *ArchiveService
	config         *config.Config
	stopCh         chan struct{}
	wg             sync.WaitGroup
	mu             sync.Mutex
	cancel         context.CancelFunc
	running        bool
}

// NewSiteInfoScheduler creates a new siteinfo scheduler instance
func NewSiteInfoScheduler(db *gorm.DB, collector *CollectorService, archiveService *ArchiveService, cfg *config.Config) *SiteInfoScheduler {
	return &SiteInfoScheduler{
		db:             db,
		collector:      collector,
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

	runCtx, cancel := context.WithCancel(ctx)
	s.cancel = cancel
	s.running = true

	siteinfoSchedulerLog.Info("siteinfo scheduler started")

	// Start periodic collection
	s.wg.Add(2)
	go s.periodicRun(runCtx)
	go s.refreshExtensionStatsMaterializedView(runCtx)
}

func (s *SiteInfoScheduler) refreshExtensionStatsMaterializedView(ctx context.Context) {
	defer s.wg.Done()

	ticker := time.NewTicker(30 * time.Minute)
	extensionsRepo := repository.NewExtensionsRepository(s.db)

	for {
		select {
		case <-s.stopCh:
			siteinfoSchedulerLog.Info("exiting extension stats materialized view refresher")
			return
		case <-ctx.Done():
			siteinfoSchedulerLog.Info("context cancelled, exiting extension stats materialized view refresher")
			return
		case <-ticker.C:
			siteinfoSchedulerLog.Info("refreshing extension stats materialized view")
			// Refresh materialized view to reflect new snapshots

			start := time.Now()
			if err := extensionsRepo.RefreshExtensionStatsMaterializedView(ctx); err != nil {
				siteinfoSchedulerLog.Info("Failed to refresh extension stats materialized view", "err", err)
			}
			siteinfoSchedulerLog.Info("refreshed extension stats materialized view", "duration", time.Since(start))
		}
	}
}

// Stop gracefully stops the scheduler
func (s *SiteInfoScheduler) Stop() {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return
	}

	siteinfoSchedulerLog.Info("stopping siteinfo scheduler")

	if s.cancel != nil {
		s.cancel()
	}
	close(s.stopCh)
	s.mu.Unlock()
	s.wg.Wait()

	s.mu.Lock()
	s.running = false
	s.mu.Unlock()
	siteinfoSchedulerLog.Info("siteinfo scheduler stopped")
}

// run executes a single collection cycle
func (s *SiteInfoScheduler) run(ctx context.Context) {
	siteinfoSchedulerLog.Info("starting collection cycle")

	startTime := time.Now()

	// Get active wikis whose persisted collection schedule is due.
	wikiRepo := repository.NewWikiRepository(s.db)
	wikis, err := wikiRepo.GetDueForUpdateFair(ctx, int(s.config.SiteinfoCheckBatchSize), time.Now())
	if err != nil {
		siteinfoSchedulerLog.Error("failed to get wikis", "error", err)
		return
	}

	totalWikis := len(wikis)
	siteinfoSchedulerLog.Info("found active wikis to process", "count", totalWikis)

	if totalWikis == 0 {
		return
	}

	// Process one serial queue per registrable domain. Different providers may
	// run concurrently, but a provider never has multiple in-flight checks.
	var successCount atomic.Int64
	var errorCount atomic.Int64
	var deferredCount atomic.Int64

	type queuedWiki struct {
		wiki  *models.Wiki
		index int
	}
	queues := make(map[string][]queuedWiki)
	for i, wiki := range wikis {
		if !wiki.IsActive {
			continue
		}
		group := requestGroup(wiki.URL)
		queues[group] = append(queues[group], queuedWiki{wiki: wiki, index: i})
	}

	wg := sync.WaitGroup{}
	for group, queue := range queues {
		group := group
		queue := queue
		wg.Add(1)
		go func() {
			defer wg.Done()
			for position, item := range queue {
				if ctx.Err() != nil {
					return
				}

				siteinfoSchedulerLog.Info("processing wiki", "index", item.index+1, "total", totalWikis, "url", item.wiki.URL, "request_group", group)
				err := s.collector.CollectSingleWiki(ctx, item.wiki.ID)
				if err != nil {
					if ctx.Err() != nil {
						return
					}
					if _, rateLimited := asRateLimitError(err); rateLimited {
						attempted := true
						retryAt := time.Time{}
						if limitErr, ok := asProviderLimitError(err); ok {
							attempted = limitErr.Attempted
							retryAt = limitErr.RetryAt
						}
						start := position
						if attempted {
							errorCount.Add(1)
							metrics.CollectionWikisFailed.Inc()
							metrics.CollectionWikisProcessed.Inc()
							start++
						}
						remainingIDs := make([]uuid.UUID, 0, len(queue)-start)
						for _, remaining := range queue[start:] {
							remainingIDs = append(remainingIDs, remaining.wiki.ID)
						}
						if len(remainingIDs) > 0 && !retryAt.IsZero() {
							if deferErr := wikiRepo.DeferCollectionChecks(ctx, remainingIDs, retryAt); deferErr != nil {
								siteinfoSchedulerLog.Error("failed to defer provider queue", "request_group", group, "count", len(remainingIDs), "error", deferErr)
							} else {
								deferredCount.Add(int64(len(remainingIDs)))
							}
						}
						siteinfoSchedulerLog.Warn("provider rate limited; deferred remaining queue",
							"request_group", group,
							"attempted", attempted,
							"deferred", len(remainingIDs),
							"retry_at", retryAt)
						return
					}
					siteinfoSchedulerLog.Error("failed to collect wiki", "id", item.wiki.ID, "url", item.wiki.URL, "error", err)
					errorCount.Add(1)
					metrics.CollectionWikisFailed.Inc()
				} else {
					successCount.Add(1)
				}
				metrics.CollectionWikisProcessed.Inc()
			}
		}()
	}
	wg.Wait()

	// Update metrics
	metrics.CollectionCycleTotal.Inc()
	metrics.CollectionCycleDuration.Observe(time.Since(startTime).Seconds())

	elapsed := time.Since(startTime)
	siteinfoSchedulerLog.Info("collection cycle completed",
		"success", successCount.Load(),
		"errors", errorCount.Load(),
		"deferred", deferredCount.Load(),
		"duration", elapsed.Round(time.Second))
}

// periodicRun runs collection continuously with backoff based on last_check_at
func (s *SiteInfoScheduler) periodicRun(ctx context.Context) {
	defer s.wg.Done()
	if s.config.SiteinfoCheckBatchSize <= 0 {
		siteinfoSchedulerLog.Info("siteinfo collection disabled", "batch_size", s.config.SiteinfoCheckBatchSize)
		select {
		case <-s.stopCh:
			siteinfoSchedulerLog.Info("periodic run stopped")
		case <-ctx.Done():
			siteinfoSchedulerLog.Info("context cancelled")
		}
		return
	}

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
			wikiRepo := repository.NewWikiRepository(s.db)
			wikis, err := wikiRepo.GetDueForUpdate(ctx, 1, time.Now())
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

			ticker.Reset(time.Second)
			s.run(ctx)
		}
	}
}
