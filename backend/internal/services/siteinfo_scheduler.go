package services

import (
	"context"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/net/publicsuffix"
	"golang.org/x/time/rate"
	"gorm.io/gorm"

	"wikikeeper-backend/internal/config"
	applogger "wikikeeper-backend/internal/logger"
	"wikikeeper-backend/internal/metrics"
	"wikikeeper-backend/internal/models"
	"wikikeeper-backend/internal/repository"
)

var siteinfoSchedulerLog = applogger.With("component", "siteinfo-scheduler")

const (
	maxDomainInflight    = 2
	domainRequestsPerSec = 2
)

type domainThrottle struct {
	limiter *rate.Limiter
	slots   chan struct{}
}

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
	s.wg.Add(2)
	go s.periodicRun(ctx)
	go s.refreshExtensionStatsMaterializedView(ctx)
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
	var successCount atomic.Int64
	var errorCount atomic.Int64

	collector := NewCollectorService(s.db, s.mwService, s.config)

	wg := sync.WaitGroup{}
	throttleMu := sync.Mutex{}
	throttles := make(map[string]*domainThrottle)
	throttleFor := func(group string) *domainThrottle {
		throttleMu.Lock()
		defer throttleMu.Unlock()
		if throttle, ok := throttles[group]; ok {
			return throttle
		}
		throttle := &domainThrottle{
			limiter: rate.NewLimiter(rate.Limit(domainRequestsPerSec), 1),
			slots:   make(chan struct{}, maxDomainInflight),
		}
		throttles[group] = throttle
		return throttle
	}

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

		collect := func(wiki *models.Wiki, i int) {
			defer wg.Done()

			group := requestGroup(wiki.URL)
			throttle := throttleFor(group)
			if err := throttle.limiter.Wait(ctx); err != nil {
				siteinfoSchedulerLog.Warn("request pacing interrupted", "group", group, "error", err)
				return
			}

			select {
			case throttle.slots <- struct{}{}:
				defer func() { <-throttle.slots }()
			case <-ctx.Done():
				return
			}

			siteinfoSchedulerLog.Info("processing wiki", "index", i+1, "total", totalWikis, "url", wiki.URL, "request_group", group)

			// Collect siteinfo
			if err := collector.CollectSingleWiki(ctx, wiki.ID); err != nil {
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
