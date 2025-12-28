package services

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"wikikeeper-backend/internal/config"
	applogger "wikikeeper-backend/internal/logger"
	"wikikeeper-backend/internal/models"
	"wikikeeper-backend/internal/repository"
)

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
	applogger.Log.Info("[Collector] Starting collection for wiki %s", wikiID)

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
		applogger.Log.Info("[Collector] Using existing API URL: %s", *wiki.APIURL)
		client = s.mwService.CreateClientWithURL(wiki.URL, *wiki.APIURL, *wiki.IndexURL)

		// Try to fetch siteinfo with existing API URL
		siteinfo, err = s.mwService.FetchSiteinfo(ctx, client)

		// If fetch failed with existing API, try re-detecting
		if err != nil {
			applogger.Log.Info("[Collector] Existing API failed (%v), re-detecting...", err)
			client, err = s.mwService.Initialize(ctx, wiki.URL)
			if err != nil {
				s.UpdateWikiStatus(ctx, wikiID, models.WikiStatusError, err)
				return NewCollectorError("initialize_mediawiki", err)
			}
			siteinfo, err = s.mwService.FetchSiteinfo(ctx, client)
			if err != nil {
				s.UpdateWikiStatus(ctx, wikiID, models.WikiStatusError, err)
				return NewCollectorError("fetch_siteinfo", err)
			}
		}
	} else {
		// No existing API URL, need to detect
		client, err = s.mwService.Initialize(ctx, wiki.URL)
		if err != nil {
			s.UpdateWikiStatus(ctx, wikiID, models.WikiStatusError, err)
			return NewCollectorError("initialize_mediawiki", err)
		}

		siteinfo, err = s.mwService.FetchSiteinfo(ctx, client)
		if err != nil {
			s.UpdateWikiStatus(ctx, wikiID, models.WikiStatusError, err)
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
	wiki.APIAvailable = true
	wiki.LastCheckAt = &now
	wiki.Status = models.WikiStatusOK
	// Clear previous error on successful collection
	wiki.LastError = nil
	wiki.LastErrorAt = nil

	// Check for duplicate API URL
	if client.APIURL != nil {
		if removed, err := s.HandleDuplicateAPIURL(ctx, wiki, *client.APIURL); err != nil {
			applogger.Log.Info("[Collector] Warning: duplicate check failed: %v", err)
		} else if removed {
			applogger.Log.Info("[Collector] Wiki %s deleted as duplicate", wikiID)
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
		WikiID:        wikiID,
		Time:          now,
		Pages:         siteinfo.Statistics.Pages,
		Articles:      siteinfo.Statistics.Articles,
		Edits:         siteinfo.Statistics.Edits,
		Images:        siteinfo.Statistics.Images,
		Users:         siteinfo.Statistics.Users,
		ActiveUsers:   siteinfo.Statistics.ActiveUsers,
		Admins:        siteinfo.Statistics.Admins,
		Jobs:          siteinfo.Statistics.Jobs,
		ResponseTimeMs: &responseTime,
		HTTPStatus:     &httpStatus,
	}

	if err := statsRepo.Create(ctx, stats); err != nil {
		return NewCollectorError("create_stats", err)
	}

	applogger.Log.Info("[Collector] Collection completed for %s: %d pages, %d edits",
		wikiID, siteinfo.Statistics.Pages, siteinfo.Statistics.Edits)

	return nil
}

// UpdateWikiStatus updates wiki status and error information
func (s *CollectorService) UpdateWikiStatus(ctx context.Context, wikiID uuid.UUID, status models.WikiStatus, err error) {
	wikiRepo := repository.NewWikiRepository(s.db)
	wiki, getErr := wikiRepo.GetByID(ctx, wikiID)
	if getErr != nil {
		applogger.Log.Info("[Collector] Failed to get wiki for status update: %v", getErr)
		return
	}

	now := time.Now()
	wiki.Status = status
	wiki.LastCheckAt = &now

	if err != nil && status == models.WikiStatusError {
		errMsg := err.Error()
		wiki.LastError = &errMsg
		wiki.LastErrorAt = &now
		wiki.APIAvailable = false
	}

	if updateErr := wikiRepo.Update(ctx, wiki); updateErr != nil {
		applogger.Log.Info("[Collector] Failed to update wiki status: %v", updateErr)
	}
}

// HandleDuplicateAPIURL checks for and removes duplicate wikis with the same API URL
func (s *CollectorService) HandleDuplicateAPIURL(ctx context.Context, wiki *models.Wiki, apiURL string) (bool, error) {
	wikiRepo := repository.NewWikiRepository(s.db)

	// Find other wikis with the same API URL
	duplicates, _, err := wikiRepo.List(ctx, repository.ListOptions{
		PageSize: 100,
	})
	if err != nil {
		return false, err
	}

	// Check for duplicates (excluding current wiki)
	for _, dup := range duplicates {
		if dup.ID == wiki.ID {
			continue
		}
		if dup.APIURL != nil && *dup.APIURL == apiURL {
			// Found duplicate - remove the one created later
			if dup.CreatedAt.Before(wiki.CreatedAt) {
				// Current wiki is newer, delete it
				applogger.Log.Info("[Collector] Duplicate API URL found: %s already exists (created %v, current %v)",
					apiURL, dup.CreatedAt, wiki.CreatedAt)
				return true, nil
			} else {
				// Duplicate is newer, delete it
				applogger.Log.Info("[Collector] Removing duplicate wiki %s with API URL %s", dup.ID, apiURL)
				if delErr := wikiRepo.Delete(ctx, dup.ID); delErr != nil {
					applogger.Log.Info("[Collector] Failed to delete duplicate: %v", delErr)
				}
			}
		}
	}

	return false, nil
}

// CollectBatch collects stats for multiple active wikis
func (s *CollectorService) CollectBatch(ctx context.Context, limit int, delay time.Duration) ([]*models.WikiStats, error) {
	applogger.Log.Info("[Collector] Starting batch collection (limit=%d, delay=%v)", limit, delay)

	wikiRepo := repository.NewWikiRepository(s.db)

	// Get active wikis
	wikis, total, err := wikiRepo.List(ctx, repository.ListOptions{
		PageSize: limit,
	})
	if err != nil {
		return nil, NewCollectorError("list_wikis", err)
	}

	applogger.Log.Info("[Collector] Found %d active wikis (total: %d)", len(wikis), total)

	var results []*models.WikiStats
	statsRepo := repository.NewStatsRepository(s.db)

	for i, wiki := range wikis {
		if err := s.CollectSingleWiki(ctx, wiki.ID); err != nil {
			applogger.Log.Info("[Collector] Failed to collect %s: %v", wiki.ID, err)
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

	applogger.Log.Info("[Collector] Batch collection completed: %d/%d successful", len(results), len(wikis))
	return results, nil
}
