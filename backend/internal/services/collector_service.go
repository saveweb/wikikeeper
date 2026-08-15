package services

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"wikikeeper-backend/internal/config"
	applogger "wikikeeper-backend/internal/logger"
	"wikikeeper-backend/internal/models"
	"wikikeeper-backend/internal/repository"
)

var collectorLog = applogger.With("component", "collector")

const (
	collectionInterval      = 30 * 24 * time.Hour
	terminalFailureInterval = 90 * 24 * time.Hour
	baseFailureBackoff      = 3 * 24 * time.Hour
	maxFailureBackoff       = 30 * 24 * time.Hour
	siteFailureThreshold    = 3
)

func markWikiCollectionSuccess(wiki *models.Wiki, now time.Time) {
	now = now.UTC()
	wiki.APIAvailable = true
	wiki.LastCheckAt = &now
	wiki.LastSuccessAt = &now
	nextCheckAt := now.Add(collectionInterval)
	wiki.NextCheckAt = &nextCheckAt
	wiki.Status = models.WikiStatusOK
	wiki.CollectionStatus = models.CollectionStatusOK
	wiki.ConsecutiveFailures = 0
	wiki.LastError = nil
	wiki.LastErrorAt = nil
}

// CollectorService coordinates wiki data collection
type CollectorService struct {
	db        *gorm.DB
	mwService *MediaWikiService
	config    *config.Config
	limiter   *ProviderLimiter
}

// NewCollectorService creates a new collector service instance
func NewCollectorService(db *gorm.DB, mwService *MediaWikiService, cfg *config.Config, limiters ...*ProviderLimiter) *CollectorService {
	service := &CollectorService{
		db:        db,
		mwService: mwService,
		config:    cfg,
	}
	if len(limiters) > 0 {
		service.limiter = limiters[0]
	}
	return service
}

func (s *CollectorService) ProviderCooldown(ctx context.Context, rawURL string) (time.Time, bool) {
	if s.limiter == nil {
		return time.Time{}, false
	}
	return s.limiter.Cooldown(ctx, rawURL)
}

// CollectSingleWiki collects stats for a single wiki
func (s *CollectorService) CollectSingleWiki(ctx context.Context, wikiID uuid.UUID) error {
	collectorLog.Info("Starting collection for wiki", "wiki_id", wikiID)

	// Get wiki from database
	wikiRepo := repository.NewWikiRepository(s.db)
	wiki, err := wikiRepo.GetByID(ctx, wikiID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return NewCollectorError("get_wiki", ErrWikiNotFound)
		}
		return NewCollectorError("get_wiki", err)
	}
	return s.collectWiki(ctx, wiki)
}

func (s *CollectorService) recordCollectionError(ctx context.Context, wikiID uuid.UUID, op string, err error) error {
	if limitErr, ok := asProviderLimitError(err); ok && !limitErr.Attempted {
		return NewCollectorError(op, err)
	}
	if _, rateLimited := asRateLimitError(err); rateLimited {
		s.UpdateWikiRateLimit(ctx, wikiID, err)
	} else {
		s.UpdateWikiCollectionFailure(ctx, wikiID, err)
	}
	return NewCollectorError(op, err)
}

func (s *CollectorService) collectWiki(ctx context.Context, wiki *models.Wiki) error {
	var client *MediaWikiClient
	var siteinfo *SiteInfo
	var err error
	wikiRepo := repository.NewWikiRepository(s.db)

	storedIndexURL := wiki.IndexURL
	if wiki.APIURL != nil && storedIndexURL == nil {
		_, _, derivedIndexURL, ok := NormalizeExplicitAPIURL(*wiki.APIURL)
		if ok {
			storedIndexURL = &derivedIndexURL
		}
	}

	// If API URL exists, try using it directly first
	if wiki.APIURL != nil && storedIndexURL != nil {
		collectorLog.Info("Using existing API URL", "api_url", *wiki.APIURL)
		client = s.mwService.CreateClientWithURL(wiki.URL, *wiki.APIURL, *storedIndexURL)

		// Try to fetch siteinfo with existing API URL
		siteinfo, err = s.mwService.FetchSiteinfo(ctx, client)

		// If fetch failed with existing API, try re-detecting
		if err != nil {
			if _, rateLimited := asRateLimitError(err); rateLimited {
				return s.recordCollectionError(ctx, wiki.ID, "fetch_siteinfo", err)
			}
			if requestGroup(wiki.URL) == "fandom.com" && hasHTTPStatus(err, http.StatusNotFound, http.StatusGone) {
				return s.recordCollectionError(ctx, wiki.ID, "fetch_siteinfo", err)
			}

			collectorLog.Info("Existing API failed, re-detecting", "err", err)
			client, err = s.mwService.Initialize(ctx, wiki.URL)
			if err != nil {
				return s.recordCollectionError(ctx, wiki.ID, "initialize_mediawiki", err)
			}
			siteinfo, err = s.mwService.FetchSiteinfo(ctx, client)
			if err != nil {
				return s.recordCollectionError(ctx, wiki.ID, "fetch_siteinfo", err)
			}
		}
	} else {
		// Fandom's API path is deterministic. Construct it directly so a
		// collection does not fetch siteinfo once for detection and again for
		// the actual snapshot.
		if requestGroup(wiki.URL) == "fandom.com" {
			baseURL := NormalizeURL(wiki.URL)
			if baseURL == "" {
				return s.recordCollectionError(ctx, wiki.ID, "normalize_url", ErrInvalidWikiURL)
			}
			if strings.HasPrefix(baseURL, "http://") {
				baseURL = "https://" + strings.TrimPrefix(baseURL, "http://")
			}
			candidates := mediaWikiCandidates(baseURL)
			client = s.mwService.CreateClientWithURL(wiki.URL, candidates[0].apiURL, candidates[0].indexURL)
		} else {
			// Unknown providers still need generic API path detection.
			client, err = s.mwService.Initialize(ctx, wiki.URL)
			if err != nil {
				return s.recordCollectionError(ctx, wiki.ID, "initialize_mediawiki", err)
			}
		}

		siteinfo, err = s.mwService.FetchSiteinfo(ctx, client)
		if err != nil {
			return s.recordCollectionError(ctx, wiki.ID, "fetch_siteinfo", err)
		}
	}

	// Update wiki with siteinfo
	now := time.Now().UTC()
	wiki.Sitename = &siteinfo.General.Sitename
	wiki.Lang = &siteinfo.General.Lang
	wiki.DBType = &siteinfo.General.DBType
	wiki.DBVersion = &siteinfo.General.DBVersion
	wiki.MediaWikiVersion = &siteinfo.General.Generator
	wiki.MaxPageID = siteinfo.General.MaxPageID
	wiki.APIURL = client.APIURL
	wiki.IndexURL = client.IndexURL
	markWikiCollectionSuccess(wiki, now)

	// Check for duplicate API URL
	if client.APIURL != nil {
		if removed, err := s.HandleDuplicateAPIURL(ctx, wiki, *client.APIURL); err != nil {
			collectorLog.Info("Warning: duplicate check failed", "err", err)
		} else if removed {
			collectorLog.Info("Wiki deleted as duplicate", "wiki_id", wiki.ID)
			return NewCollectorError("duplicate_check", ErrWikiDeleted)
		}
	}

	// Update wiki in database
	if err := wikiRepo.Update(ctx, wiki); err != nil {
		return NewCollectorError("update_wiki", err)
	}

	// Create stats record
	statsRepo := repository.NewStatsRepository(s.db)
	responseTime := siteinfo.ResponseTime
	httpStatus := siteinfo.HTTPStatus
	stats := &models.WikiStats{
		WikiID:         wiki.ID,
		Time:           now,
		Pages:          siteinfo.Statistics.Pages,
		Articles:       siteinfo.Statistics.Articles,
		Edits:          siteinfo.Statistics.Edits,
		Images:         siteinfo.Statistics.Images,
		Users:          siteinfo.Statistics.Users,
		ActiveUsers:    siteinfo.Statistics.ActiveUsers,
		Admins:         siteinfo.Statistics.Admins,
		Jobs:           siteinfo.Statistics.Jobs,
		ResponseTimeMs: &responseTime,
		HTTPStatus:     &httpStatus,
	}

	if err := statsRepo.Create(ctx, stats); err != nil {
		return NewCollectorError("create_stats", err)
	}

	// Process extensions snapshot
	extensionsRepo := repository.NewExtensionsRepository(s.db)

	// Get latest snapshot for comparison
	lastSnapshot, err := extensionsRepo.GetLatestSnapshot(ctx, wiki.ID)
	if err != nil && err != gorm.ErrRecordNotFound {
		collectorLog.Info("Failed to get last extensions snapshot", "err", err)
		// Continue anyway, we'll create a new snapshot
	}

	// Compare extensions
	diff := CompareExtensionsFromSnapshot(lastSnapshot, &siteinfo.Extensions)
	versionChanged := extensionSnapshotVersionChanged(lastSnapshot, siteinfo.General.Generator)

	if extensionSnapshotNeedsUpdate(lastSnapshot, siteinfo.General.Generator, diff) {
		// Extensions or MediaWiki changed, or this is the first collection.

		// Close old snapshot if exists
		if lastSnapshot != nil {
			if err := extensionsRepo.CloseLatestSnapshot(ctx, wiki.ID, now); err != nil {
				collectorLog.Info("Failed to close last snapshot", "err", err)
			}
		}

		// Create new snapshot
		items := flattenExtensions(&siteinfo.Extensions)
		snapshot := &models.WikiExtensionsSnapshot{
			WikiID:           wiki.ID,
			SnapshotAt:       now,
			ValidUntil:       nil,
			MediaWikiVersion: &siteinfo.General.Generator,
			Items:            items,
		}

		if err := extensionsRepo.CreateSnapshot(ctx, snapshot); err != nil {
			collectorLog.Info("Failed to create extensions snapshot", "err", err)
		} else {
			collectorLog.Info("Extensions snapshot created",
				"wiki_id", wiki.ID,
				"extensions", len(siteinfo.Extensions.Extensions),
				"skins", len(siteinfo.Extensions.Skins),
				"added", len(diff.Added),
				"removed", len(diff.Removed),
				"modified", len(diff.Modified),
				"mediawiki_version_changed", versionChanged)
		}
	}
	// If no changes, the current snapshot remains valid.

	collectorLog.Info("Collection completed",
		"wiki_id", wiki.ID, "pages", siteinfo.Statistics.Pages, "edits", siteinfo.Statistics.Edits)

	return nil
}

func (s *CollectorService) UpdateWikiCollectionFailure(ctx context.Context, wikiID uuid.UUID, err error) {
	if _, rateLimited := asRateLimitError(err); rateLimited {
		s.UpdateWikiRateLimit(ctx, wikiID, err)
		return
	}
	s.UpdateWikiStatus(ctx, wikiID, models.WikiStatusError, err)
}

// UpdateWikiStatus updates wiki status and error information
func (s *CollectorService) UpdateWikiStatus(ctx context.Context, wikiID uuid.UUID, status models.WikiStatus, err error) {
	wikiRepo := repository.NewWikiRepository(s.db)
	wiki, getErr := wikiRepo.GetByID(ctx, wikiID)
	if getErr != nil {
		collectorLog.Info("Failed to get wiki for status update", "err", getErr)
		return
	}

	now := time.Now().UTC()
	wiki.CollectionStatus = models.CollectionStatusError
	wiki.LastCheckAt = &now
	wiki.ConsecutiveFailures++
	delay := collectionFailureBackoff(wiki, err)
	nextCheckAt := now.Add(delay)
	wiki.NextCheckAt = &nextCheckAt
	if wiki.ConsecutiveFailures >= siteFailureThreshold {
		wiki.Status = status
		wiki.APIAvailable = false
	}

	if err != nil && status == models.WikiStatusError {
		errMsg := err.Error()
		errMsgUTF8Safe := strings.ToValidUTF8(errMsg, `\uFFFD`)
		wiki.LastError = &errMsgUTF8Safe
		wiki.LastErrorAt = &now
	}

	if updateErr := wikiRepo.Update(ctx, wiki); updateErr != nil {
		collectorLog.Info("Failed to update wiki status", "err", updateErr)
	}
}

func hasHTTPStatus(err error, statuses ...int) bool {
	var statusErr *HTTPStatusError
	if !errors.As(err, &statusErr) {
		return false
	}
	for _, status := range statuses {
		if statusErr.StatusCode == status {
			return true
		}
	}
	return false
}

func collectionFailureBackoff(wiki *models.Wiki, err error) time.Duration {
	if hasHTTPStatus(err, http.StatusNotFound, http.StatusGone) {
		return terminalFailureInterval
	}
	failures := wiki.ConsecutiveFailures
	if failures < 1 {
		failures = 1
	}
	shift := failures - 1
	if shift > 10 {
		shift = 10
	}
	delay := baseFailureBackoff * time.Duration(1<<shift)
	if delay > maxFailureBackoff {
		return maxFailureBackoff
	}
	return delay
}

// UpdateWikiRateLimit records a transient collection failure without changing
// the last verified wiki status or API availability.
func (s *CollectorService) UpdateWikiRateLimit(ctx context.Context, wikiID uuid.UUID, err error) {
	wikiRepo := repository.NewWikiRepository(s.db)
	wiki, getErr := wikiRepo.GetByID(ctx, wikiID)
	if getErr != nil {
		collectorLog.Info("Failed to get wiki for rate-limit update", "err", getErr)
		return
	}

	now := time.Now().UTC()
	wiki.CollectionStatus = models.CollectionStatusRateLimited
	wiki.LastCheckAt = &now
	wiki.ConsecutiveFailures++
	delay, _ := rateLimitBackoff(err, wiki.ConsecutiveFailures, now, positiveJitter)
	nextCheckAt := now.Add(delay)
	wiki.NextCheckAt = &nextCheckAt

	if err != nil {
		errMsg := strings.ToValidUTF8(err.Error(), `\uFFFD`)
		wiki.LastError = &errMsg
		wiki.LastErrorAt = &now
	}

	if updateErr := wikiRepo.Update(ctx, wiki); updateErr != nil {
		collectorLog.Info("Failed to record wiki rate limit", "err", updateErr)
	}
}

// HandleDuplicateAPIURL checks for and removes duplicate wikis with the same API URL
func (s *CollectorService) HandleDuplicateAPIURL(ctx context.Context, wiki *models.Wiki, apiURL string) (bool, error) {
	wikiRepo := repository.NewWikiRepository(s.db)

	// Find existing wiki with the same API URL
	existing, err := wikiRepo.GetByAPIURL(ctx, apiURL)
	if err != nil {
		// Not found is ok, means no duplicate
		return false, nil
	}

	// Check if it's a different wiki (not the current one)
	if existing.ID == wiki.ID {
		return false, nil
	}

	// Found duplicate - remove the one created later
	if existing.CreatedAt.Before(wiki.CreatedAt) {
		// Current wiki is newer, delete it directly
		collectorLog.Info("Duplicate API URL found, deleting current wiki",
			"wiki_id", wiki.ID, "api_url", apiURL, "existing_id", existing.ID, "existing_created", existing.CreatedAt, "current_created", wiki.CreatedAt)
		if delErr := wikiRepo.Delete(ctx, wiki.ID); delErr != nil {
			collectorLog.Info("Failed to delete current wiki", "err", delErr)
			return false, delErr
		}
		return true, nil
	} else {
		// Existing wiki is newer, delete it (shouldn't happen normally, but just in case)
		collectorLog.Info("Removing existing duplicate wiki", "wiki_id", existing.ID, "api_url", apiURL)
		if delErr := wikiRepo.Delete(ctx, existing.ID); delErr != nil {
			collectorLog.Info("Failed to delete existing wiki", "err", delErr)
			return false, delErr
		}
		return false, nil
	}
}

// CollectBatch collects stats for multiple active wikis
func (s *CollectorService) CollectBatch(ctx context.Context, limit int, delay time.Duration) ([]*models.WikiStats, error) {
	collectorLog.Info("Starting batch collection", "limit", limit, "delay", delay)

	wikiRepo := repository.NewWikiRepository(s.db)

	// Get active wikis
	wikis, total, err := wikiRepo.List(ctx, repository.ListOptions{
		PageSize: limit,
	})
	if err != nil {
		return nil, NewCollectorError("list_wikis", err)
	}

	collectorLog.Info("Found active wikis", "count", len(wikis), "total", total)

	var results []*models.WikiStats
	statsRepo := repository.NewStatsRepository(s.db)

	for i, wiki := range wikis {
		if err := s.CollectSingleWiki(ctx, wiki.ID); err != nil {
			collectorLog.Info("Failed to collect wiki", "wiki_id", wiki.ID, "err", err)
			continue
		}

		// Get the created stats
		stats, err := statsRepo.GetLatestByWikiID(ctx, wiki.ID)
		if err == nil && stats != nil {
			results = append(results, stats)
		}

		// Delay between requests (except last)
		if i < len(wikis)-1 && delay > 0 {
			time.Sleep(delay)
		}
	}

	collectorLog.Info("Batch collection completed", "successful", len(results), "total", len(wikis))
	return results, nil
}
