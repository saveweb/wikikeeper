package services

import (
	"context"
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
	collectionInterval   = 3 * 24 * time.Hour
	siteFailureThreshold = 3
)

func markWikiCollectionSuccess(wiki *models.Wiki, now time.Time) {
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
}

// NewCollectorService creates a new collector service instance
func NewCollectorService(db *gorm.DB, mwService *MediaWikiService, cfg *config.Config) *CollectorService {
	return &CollectorService{
		db:        db,
		mwService: mwService,
		config:    cfg,
	}
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

	var client *MediaWikiClient
	var siteinfo *SiteInfo

	// If API URL exists, try using it directly first
	if wiki.APIURL != nil && wiki.IndexURL != nil {
		collectorLog.Info("Using existing API URL", "api_url", *wiki.APIURL)
		client = s.mwService.CreateClientWithURL(wiki.URL, *wiki.APIURL, *wiki.IndexURL)

		// Try to fetch siteinfo with existing API URL
		siteinfo, err = s.mwService.FetchSiteinfo(ctx, client)

		// If fetch failed with existing API, try re-detecting
		if err != nil {
			if _, rateLimited := asRateLimitError(err); rateLimited {
				s.UpdateWikiRateLimit(ctx, wikiID, err)
				return NewCollectorError("fetch_siteinfo", err)
			}

			collectorLog.Info("Existing API failed, re-detecting", "err", err)
			client, err = s.mwService.Initialize(ctx, wiki.URL)
			if err != nil {
				s.UpdateWikiCollectionFailure(ctx, wikiID, err)
				return NewCollectorError("initialize_mediawiki", err)
			}
			siteinfo, err = s.mwService.FetchSiteinfo(ctx, client)
			if err != nil {
				s.UpdateWikiCollectionFailure(ctx, wikiID, err)
				return NewCollectorError("fetch_siteinfo", err)
			}
		}
	} else {
		// No existing API URL, need to detect
		client, err = s.mwService.Initialize(ctx, wiki.URL)
		if err != nil {
			s.UpdateWikiCollectionFailure(ctx, wikiID, err)
			return NewCollectorError("initialize_mediawiki", err)
		}

		siteinfo, err = s.mwService.FetchSiteinfo(ctx, client)
		if err != nil {
			s.UpdateWikiCollectionFailure(ctx, wikiID, err)
			return NewCollectorError("fetch_siteinfo", err)
		}
	}

	// Update wiki with siteinfo
	now := time.Now()
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
			collectorLog.Info("Wiki deleted as duplicate", "wiki_id", wikiID)
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
		WikiID:         wikiID,
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
	lastSnapshot, err := extensionsRepo.GetLatestSnapshot(ctx, wikiID)
	if err != nil && err != gorm.ErrRecordNotFound {
		collectorLog.Info("Failed to get last extensions snapshot", "err", err)
		// Continue anyway, we'll create a new snapshot
	}

	// Compare extensions
	diff := CompareExtensionsFromSnapshot(lastSnapshot, &siteinfo.Extensions)

	if diff.HasChanges || lastSnapshot == nil {
		// Extensions changed or first collection, create new snapshot

		// Close old snapshot if exists
		if lastSnapshot != nil {
			if err := extensionsRepo.CloseLatestSnapshot(ctx, wikiID, now); err != nil {
				collectorLog.Info("Failed to close last snapshot", "err", err)
			}
		}

		// Create new snapshot
		items := flattenExtensions(&siteinfo.Extensions)
		snapshot := &models.WikiExtensionsSnapshot{
			WikiID:           wikiID,
			SnapshotAt:       now,
			ValidUntil:       nil,
			MediaWikiVersion: &siteinfo.General.Generator,
			Items:            items,
		}

		if err := extensionsRepo.CreateSnapshot(ctx, snapshot); err != nil {
			collectorLog.Info("Failed to create extensions snapshot", "err", err)
		} else {
			collectorLog.Info("Extensions snapshot created",
				"wiki_id", wikiID,
				"extensions", len(siteinfo.Extensions.Extensions),
				"skins", len(siteinfo.Extensions.Skins),
				"added", len(diff.Added),
				"removed", len(diff.Removed),
				"modified", len(diff.Modified))
		}
	}
	// If no changes, we don't need to update anything (snapshot remains valid)

	collectorLog.Info("Collection completed",
		"wiki_id", wikiID, "pages", siteinfo.Statistics.Pages, "edits", siteinfo.Statistics.Edits)

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

	now := time.Now()
	wiki.CollectionStatus = models.CollectionStatusError
	wiki.LastCheckAt = &now
	nextCheckAt := now.Add(collectionInterval)
	wiki.NextCheckAt = &nextCheckAt
	wiki.ConsecutiveFailures++
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

// UpdateWikiRateLimit records a transient collection failure without changing
// the last verified wiki status or API availability.
func (s *CollectorService) UpdateWikiRateLimit(ctx context.Context, wikiID uuid.UUID, err error) {
	wikiRepo := repository.NewWikiRepository(s.db)
	wiki, getErr := wikiRepo.GetByID(ctx, wikiID)
	if getErr != nil {
		collectorLog.Info("Failed to get wiki for rate-limit update", "err", getErr)
		return
	}

	now := time.Now()
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
