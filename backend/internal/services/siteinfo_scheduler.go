package services

import (
	"context"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/net/publicsuffix"
	"gorm.io/gorm"

	"wikikeeper-backend/internal/config"
	applogger "wikikeeper-backend/internal/logger"
	"wikikeeper-backend/internal/metrics"
	"wikikeeper-backend/internal/models"
	"wikikeeper-backend/internal/repository"
)

var siteinfoSchedulerLog = applogger.With("component", "siteinfo-scheduler")

func requestGroup(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "default"
	}

	host := strings.TrimSuffix(strings.ToLower(parsed.Hostname()), ".")
	if host == "" {
		return "default"
	}
	if domain, err := publicsuffix.EffectiveTLDPlusOne(host); err == nil {
		return domain
	}
	return host
}

// SiteInfoScheduler manages periodic wiki siteinfo collection
type SiteInfoScheduler struct {
	db             *gorm.DB
	mwService      *MediaWikiService
	archiveService *ArchiveService
	config         *config.Config
	stopCh         chan struct{}
	wg             sync.WaitGroup
	mu             sync.Mutex
	cancel         context.CancelFunc
	running        bool
	providerMu     sync.Mutex
	providerGates  map[string]*providerGate
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
		providerGates:  make(map[string]*providerGate),
	}
}

func (s *SiteInfoScheduler) providerGateFor(rawURL string) (*providerGate, string) {
	group := requestGroup(rawURL)
	s.providerMu.Lock()
	defer s.providerMu.Unlock()
	if gate, ok := s.providerGates[group]; ok {
		return gate, group
	}
	gate := newProviderGate()
	s.providerGates[group] = gate
	return gate, group
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
	wikis, err := wikiRepo.GetDueForUpdate(ctx, int(s.config.SiteinfoCheckBatchSize), time.Now())
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
	var successCount atomic.Int64
	var errorCount atomic.Int64

	collector := NewCollectorService(s.db, s.mwService, s.config)

	wg := sync.WaitGroup{}

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

		collect := func(wiki *models.Wiki, i int) {
			defer wg.Done()

			gate, group := s.providerGateFor(wiki.URL)
			attempted, err := gate.run(ctx, func() error {
				siteinfoSchedulerLog.Info("processing wiki", "index", i+1, "total", totalWikis, "url", wiki.URL, "request_group", group)
				return collector.CollectSingleWiki(ctx, wiki.ID)
			})
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				if !attempted {
					if _, rateLimited := asRateLimitError(err); rateLimited {
						collector.UpdateWikiRateLimit(ctx, wiki.ID, err)
					}
				}
				siteinfoSchedulerLog.Error("failed to collect wiki", "id", wiki.ID, "url", wiki.URL, "error", err)
				errorCount.Add(1)
				metrics.CollectionWikisFailed.Inc()
			} else {
				successCount.Add(1)
			}
			metrics.CollectionWikisProcessed.Inc()
		}
		wg.Add(1)
		go collect(wiki, i)
	}
	wg.Wait()

	// Update metrics
	metrics.CollectionCycleTotal.Inc()
	metrics.CollectionCycleDuration.Observe(time.Since(startTime).Seconds())

	elapsed := time.Since(startTime)
	siteinfoSchedulerLog.Info("collection cycle completed",
		"success", successCount.Load(),
		"errors", errorCount.Load(),
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
