package services

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	applogger "wikikeeper-backend/internal/logger"
	"wikikeeper-backend/internal/models"
	"wikikeeper-backend/internal/repository"
)

// ArchiveService checks Archive.org for wiki backups
type ArchiveService struct {
	timeout     time.Duration
	userAgent   string
	checkDelay  time.Duration // Delay between Archive.org checks
}

// NewArchiveService creates a new Archive service instance
func NewArchiveService(timeout time.Duration, userAgent string, checkDelay float64) *ArchiveService {
	if userAgent == "" {
		userAgent = "WikiKeeper/1.0"
	}
	return &ArchiveService{
		timeout:    timeout,
		userAgent:  userAgent,
		checkDelay: time.Duration(checkDelay * float64(time.Second)),
	}
}

// ArchiveInfo represents an Archive.org item
type ArchiveInfo struct {
	IAIdentifier     string     `json:"ia_identifier"`
	AddedDate        *time.Time `json:"added_date,omitempty"`
	DumpDate         *time.Time `json:"dump_date,omitempty"`
	ItemSize         *int64     `json:"item_size,omitempty"`
	Uploader         *string    `json:"uploader,omitempty"`
	Scanner          *string    `json:"scanner,omitempty"`
	UploadState      *string    `json:"upload_state,omitempty"`
	HasXMLCurrent    bool       `json:"has_xml_current"`
	HasXMLHistory    bool       `json:"has_xml_history"`
	HasImagesDump    bool       `json:"has_images_dump"`
	HasTitlesList    bool       `json:"has_titles_list"`
	HasImagesList    bool       `json:"has_images_list"`
	HasLegacyWikidump bool      `json:"has_legacy_wikidump"`
}

// ArchiveSearchResult represents Archive.org search response
type archiveSearchResult struct {
	Response struct {
		Docs []struct {
			Identifier  string `json:"identifier"`
			AddedDate   string `json:"addeddate"`
			OriginalURL string `json:"originalurl,omitempty"`
		} `json:"docs"`
		NumFound int `json:"numFound"`
	} `json:"response"`
}

// ArchiveMetadata represents Archive.org item metadata
type archiveMetadata struct {
	Metadata struct {
		Uploader    string `json:"uploader"`
		Scanner     string `json:"scanner"`
		UploadState string `json:"upload-state"`
	} `json:"metadata"`
	Files []struct {
		Name string `json:"name"`
		Size interface{} `json:"size"` // Can be int64 or string like "1.2G"
	} `json:"files"`
	ItemSize interface{} `json:"item_size"` // Can be int64 or string
}

// CheckArchive searches Archive.org for wiki backups
func (s *ArchiveService) CheckArchive(ctx context.Context, apiURL, indexURL string) ([]*ArchiveInfo, error) {
	applogger.Log.Info("[Archive] Checking Archive.org for: %s", apiURL)

	if apiURL == "" {
		return nil, fmt.Errorf("API URL is required")
	}

	// Derive index_url if not provided
	if indexURL == "" {
		indexURL = strings.Replace(apiURL, "api.php", "index.php", 1)
	}

	// Build search query
	// Search for items matching either api_url or index_url
	// Try both http and https versions since archive.org might use different protocol
	apiURLHTTP := strings.Replace(apiURL, "https://", "http://", 1)
	apiURLHTTPS := strings.Replace(apiURL, "http://", "https://", 1)
	indexURLHTTP := strings.Replace(indexURL, "https://", "http://", 1)
	indexURLHTTPS := strings.Replace(indexURL, "http://", "https://", 1)

	query := fmt.Sprintf(`(originalurl:"%s" OR originalurl:"%s" OR originalurl:"%s" OR originalurl:"%s")`,
		apiURLHTTP, apiURLHTTPS, indexURLHTTP, indexURLHTTPS)
	searchURL := s.buildSearchURL(query)

	// Make search request
	results, err := s.searchArchive(ctx, searchURL)
	if err != nil {
		return nil, fmt.Errorf("archive search failed: %w", err)
	}

	applogger.Log.Info("[Archive] Found %d results for: %s", len(results), apiURL)

	var archives []*ArchiveInfo

	// Process each result
	for _, result := range results {
		info, err := s.parseArchiveItem(ctx, result)
		if err != nil {
			applogger.Log.Info("[Archive] Failed to parse item %s: %v", result.Identifier, err)
			continue
		}

		if info != nil {
			archives = append(archives, info)
		}
	}

	return archives, nil
}

// CollectArchives checks and stores archive info for a wiki
func (s *ArchiveService) CollectArchives(ctx context.Context, db *gorm.DB, wikiID uuid.UUID, apiURL, indexURL string) (found, imported, updated int, err error) {
	archives, err := s.CheckArchive(ctx, apiURL, indexURL)
	if err != nil {
		return 0, 0, 0, err
	}

	found = len(archives)

	if found == 0 {
		// No archives found, update wiki has_archive to false
		s.updateWikiArchiveStatus(ctx, db, wikiID, false)
		return 0, 0, 0, nil
	}

	archiveRepo := repository.NewArchiveRepository(db)

	// Store each archive
	for _, archiveInfo := range archives {
		// Convert ArchiveInfo to WikiArchive model
		wikiArchive := &models.WikiArchive{
			WikiID:           wikiID,
			IAIdentifier:     archiveInfo.IAIdentifier,
			AddedDate:        archiveInfo.AddedDate,
			DumpDate:         archiveInfo.DumpDate,
			ItemSize:         archiveInfo.ItemSize,
			Uploader:         archiveInfo.Uploader,
			Scanner:          archiveInfo.Scanner,
			UploadState:      archiveInfo.UploadState,
			HasXMLCurrent:    archiveInfo.HasXMLCurrent,
			HasXMLHistory:    archiveInfo.HasXMLHistory,
			HasImagesDump:    archiveInfo.HasImagesDump,
			HasTitlesList:    archiveInfo.HasTitlesList,
			HasImagesList:    archiveInfo.HasImagesList,
			HasLegacyWikidump: archiveInfo.HasLegacyWikidump,
		}

		// Use Upsert to handle both new and existing archives
		if err := archiveRepo.UpsertByWikiAndIAIdentifier(ctx, wikiArchive); err != nil {
			applogger.Log.Info("[Archive] Failed to upsert archive %s: %v", archiveInfo.IAIdentifier, err)
			continue
		}

		// Check if this was a new or existing archive
		exists, _ := archiveRepo.ExistsByWikiAndIAIdentifier(ctx, wikiID, archiveInfo.IAIdentifier)
		if exists {
			updated++
			applogger.Log.Info("[Archive] Updated archive: %s", archiveInfo.IAIdentifier)
		} else {
			imported++
			applogger.Log.Info("[Archive] Imported archive: %s", archiveInfo.IAIdentifier)
		}
	}

	// Update wiki has_archive status
	s.updateWikiArchiveStatus(ctx, db, wikiID, true)

	applogger.Log.Info("[Archive] Archive collection completed: found=%d, imported=%d, updated=%d", found, imported, updated)
	return found, imported, updated, nil
}

// updateWikiArchiveStatus updates the has_archive field for a wiki
func (s *ArchiveService) updateWikiArchiveStatus(ctx context.Context, db *gorm.DB, wikiID uuid.UUID, hasArchive bool) {
	wikiRepo := repository.NewWikiRepository(db)
	wiki, err := wikiRepo.GetByID(ctx, wikiID)
	if err != nil {
		applogger.Log.Info("[Archive] Failed to get wiki for status update: %v", err)
		return
	}

	now := time.Now()
	wiki.HasArchive = hasArchive
	wiki.ArchiveLastCheckAt = &now
	// Clear previous archive error on successful check
	wiki.ArchiveLastError = nil
	wiki.ArchiveLastErrorAt = nil

	if err := wikiRepo.Update(ctx, wiki); err != nil {
		applogger.Log.Info("[Archive] Failed to update wiki has_archive status: %v", err)
	}
}

// UpdateWikiArchiveError records an archive check error (exported for handler use)
func (s *ArchiveService) UpdateWikiArchiveError(ctx context.Context, db *gorm.DB, wikiID uuid.UUID, err error) {
	wikiRepo := repository.NewWikiRepository(db)
	wiki, getErr := wikiRepo.GetByID(ctx, wikiID)
	if getErr != nil {
		applogger.Log.Info("[Archive] Failed to get wiki for error update: %v", getErr)
		return
	}

	now := time.Now()
	errMsg := err.Error()
	wiki.ArchiveLastError = &errMsg
	wiki.ArchiveLastErrorAt = &now
	wiki.ArchiveLastCheckAt = &now

	if updateErr := wikiRepo.Update(ctx, wiki); updateErr != nil {
		applogger.Log.Info("[Archive] Failed to update wiki archive error: %v", updateErr)
	}
}

// buildSearchURL constructs Archive.org Advanced Search URL
func (s *ArchiveService) buildSearchURL(query string) string {
	// URL encode the query
	encodedQuery := url.QueryEscape(query)

	// Build URL manually to preserve [] in parameter names
	return fmt.Sprintf("https://archive.org/advancedsearch.php?q=%s&fl[]=identifier&fl[]=addeddate&fl[]=originalurl&sort[]=addeddate+desc&rows[]=100&output=json",
		encodedQuery)
}

// searchArchive performs Archive.org search
func (s *ArchiveService) searchArchive(ctx context.Context, searchURL string) ([]archiveSearchResultDoc, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", s.userAgent)

	client := &http.Client{Timeout: s.timeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	var result archiveSearchResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("JSON decode failed: %w", err)
	}

	applogger.Log.Info("[Archive] Search result: numFound=%d", result.Response.NumFound)

	// Convert to simple format
	var docs []archiveSearchResultDoc
	for _, doc := range result.Response.Docs {
		docs = append(docs, archiveSearchResultDoc{
			Identifier:  doc.Identifier,
			AddedDate:   doc.AddedDate,
			OriginalURL: doc.OriginalURL,
		})
	}

	return docs, nil
}

type archiveSearchResultDoc struct {
	Identifier  string `json:"identifier"`
	AddedDate   string `json:"addeddate"`
	OriginalURL string `json:"originalurl,omitempty"`
}

// parseArchiveItem parses a single archive item and fetches its metadata
func (s *ArchiveService) parseArchiveItem(ctx context.Context, result archiveSearchResultDoc) (*ArchiveInfo, error) {
	info := &ArchiveInfo{
		IAIdentifier: result.Identifier,
	}

	// Parse added_date
	if result.AddedDate != "" {
		// Try multiple date formats
		formats := []string{
			"2006-01-02T15:04:05Z",
			"2006-01-02T15:04:05.999Z",
			"2006-01-02 15:04:05",
			"2006-01-02",
		}

		for _, format := range formats {
			if t, err := time.Parse(format, result.AddedDate); err == nil {
				info.AddedDate = &t
				break
			}
		}
	}

	// Fetch full metadata
	metadata, err := s.fetchMetadata(ctx, result.Identifier)
	if err != nil {
		applogger.Log.Info("[Archive] Failed to fetch metadata for %s: %v", result.Identifier, err)
		// Return basic info even if metadata fetch fails
		return info, nil
	}

	// Parse metadata
	if metadata.Metadata.Uploader != "" {
		info.Uploader = &metadata.Metadata.Uploader
	}
	if metadata.Metadata.Scanner != "" {
		info.Scanner = &metadata.Metadata.Scanner
	}
	if metadata.Metadata.UploadState != "" {
		info.UploadState = &metadata.Metadata.UploadState
	}

	// Parse item_size (can be int64 or string)
	if metadata.ItemSize != nil {
		switch v := metadata.ItemSize.(type) {
		case float64:
			size := int64(v)
			info.ItemSize = &size
		case int:
			size := int64(v)
			info.ItemSize = &size
		case int64:
			info.ItemSize = &v
		case string:
			// Try to parse size string like "1.2G" or "1234567890"
			if size, err := ParseSize(v); err == nil {
				info.ItemSize = &size
			}
		}
	}

	// Extract dump_date from identifier (YYYYMMDD format)
	re := regexp.MustCompile(`-(\d{8})$`)
	if matches := re.FindStringSubmatch(result.Identifier); len(matches) > 1 {
		if t, err := time.Parse("20060102", matches[1]); err == nil {
			info.DumpDate = &t
		}
	}

	// Fallback to added_date if no dump_date
	if info.DumpDate == nil && info.AddedDate != nil {
		info.DumpDate = info.AddedDate
	}

	// Check file contents
	s.checkFileContents(info, metadata.Files)

	applogger.Log.Info("[Archive] Loaded: %s (xml_current=%v, xml_history=%v)",
		result.Identifier, info.HasXMLCurrent, info.HasXMLHistory)

	return info, nil
}

// fetchMetadata fetches full metadata for an archive item
func (s *ArchiveService) fetchMetadata(ctx context.Context, identifier string) (*archiveMetadata, error) {
	metadataURL := fmt.Sprintf("https://archive.org/metadata/%s", identifier)

	req, err := http.NewRequestWithContext(ctx, "GET", metadataURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", s.userAgent)

	client := &http.Client{Timeout: s.timeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	var metadata archiveMetadata
	if err := json.NewDecoder(resp.Body).Decode(&metadata); err != nil {
		return nil, err
	}

	return &metadata, nil
}

// checkFileContents checks files for dump type indicators
func (s *ArchiveService) checkFileContents(info *ArchiveInfo, files []struct {
	Name string      `json:"name"`
	Size interface{} `json:"size"`
}) {
	for _, file := range files {
		name := strings.ToLower(file.Name)

		switch {
		case strings.Contains(name, "-current.xml"):
			info.HasXMLCurrent = true
		case strings.Contains(name, "-history.xml"):
			info.HasXMLHistory = true
		case strings.Contains(name, "-images.7z") || strings.Contains(name, "-images.tar"):
			info.HasImagesDump = true
		case strings.Contains(name, "-titles.txt") || strings.Contains(name, "-titles.xml"):
			info.HasTitlesList = true
		case strings.Contains(name, "-images.txt") || strings.Contains(name, "-images.xml"):
			info.HasImagesList = true
		case strings.Contains(name, "-wikidump.7z") || strings.Contains(name, "-wikidump.tar"):
			info.HasLegacyWikidump = true
		}
	}
}

// FormatBytes formats bytes as human-readable string
func FormatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// ParseSize parses size string to bytes
func ParseSize(sizeStr string) (int64, error) {
	sizeStr = strings.ToUpper(strings.TrimSpace(sizeStr))

	// Extract number and unit
	re := regexp.MustCompile(`^([\d.]+)\s*([KMGTPE]?B?)?$`)
	matches := re.FindStringSubmatch(sizeStr)
	if len(matches) < 2 {
		return 0, fmt.Errorf("invalid size format: %s", sizeStr)
	}

	value, err := strconv.ParseFloat(matches[1], 64)
	if err != nil {
		return 0, err
	}

	unit := matches[2]
	if unit == "" || unit == "B" {
		return int64(value), nil
	}

	multipliers := map[string]float64{
		"KB": 1 << 10,
		"K":  1 << 10,
		"MB": 1 << 20,
		"M":  1 << 20,
		"GB": 1 << 30,
		"G":  1 << 30,
		"TB": 1 << 40,
		"T":  1 << 40,
	}

	mult, ok := multipliers[unit]
	if !ok {
		return 0, fmt.Errorf("unknown unit: %s", unit)
	}

	return int64(value * mult), nil
}
