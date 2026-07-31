package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	applogger "wikikeeper-backend/internal/logger"
)

var mediaWikiLog = applogger.With("component", "mediawiki")

// MediaWikiService handles MediaWiki API interactions
type MediaWikiService struct {
	timeout   time.Duration
	userAgent string
	limiter   *ProviderLimiter
}

// HTTPStatusError preserves the response status so callers can distinguish
// throttling from a missing or moved MediaWiki API.
type HTTPStatusError struct {
	StatusCode int
	RetryAfter string
	Body       string
}

func (e *HTTPStatusError) Error() string {
	if e.RetryAfter != "" {
		return fmt.Sprintf("HTTP %d (retry after %s): %s", e.StatusCode, e.RetryAfter, e.Body)
	}
	return fmt.Sprintf("HTTP %d: %s", e.StatusCode, e.Body)
}

// NewMediaWikiService creates a new MediaWiki service instance
func NewMediaWikiService(timeout time.Duration, userAgent string, limiters ...*ProviderLimiter) *MediaWikiService {
	if userAgent == "" {
		userAgent = "WikiKeeper/1.0"
	}
	service := &MediaWikiService{
		timeout:   timeout,
		userAgent: userAgent,
	}
	if len(limiters) > 0 {
		service.limiter = limiters[0]
	}
	return service
}

// MediaWikiClient represents a detected MediaWiki installation
type MediaWikiClient struct {
	URL           string  // Original URL
	APIURL        *string // Detected API URL
	IndexURL      *string // Detected index URL
	WasRedirected bool    // Whether URL was permanently redirected
}

type mediaWikiCandidate struct {
	apiURL   string
	indexURL string
}

// SiteInfo contains site information and statistics
type SiteInfo struct {
	General      SiteInfoGeneral
	Statistics   SiteInfoStatistics
	Extensions   SiteInfoExtensions
	ResponseTime int // Response time in milliseconds
	HTTPStatus   int // HTTP status code
}

// SiteInfoGeneral contains general site information from siteinfo
type SiteInfoGeneral struct {
	Sitename  string `json:"sitename"`
	Lang      string `json:"lang"`
	DBType    string `json:"dbtype"`
	DBVersion string `json:"dbversion"`
	Generator string `json:"generator"`
	BaseURL   string `json:"baseurl"`
	MainPage  string `json:"mainpage"`
	MaxPageID *int   `json:"maxpageid,omitempty"`
}

// SiteInfoStatistics contains wiki statistics from siteinfo
type SiteInfoStatistics struct {
	Pages       int `json:"pages"`
	Articles    int `json:"articles"`
	Edits       int `json:"edits"`
	Images      int `json:"images"`
	Users       int `json:"users"`
	ActiveUsers int `json:"activeusers"`
	Admins      int `json:"admins"`
	Jobs        int `json:"jobs"`
}

// SiteInfoExtensions contains extensions and skins
type SiteInfoExtensions struct {
	Extensions []ExtensionInfo
	Skins      []ExtensionInfo
}

// ExtensionInfo represents a single extension or skin
type ExtensionInfo struct {
	Type        string  `json:"type"` // "extension", "skin", "parserhook", etc.
	Name        string  `json:"name"`
	URL         *string `json:"url,omitempty"`
	Version     *string `json:"version,omitempty"`
	LicenseName *string `json:"license-name,omitempty"`
}

// API response structures
type mediawikiResponse struct {
	Query struct {
		General    map[string]interface{} `json:"general"`
		Statistics map[string]interface{} `json:"statistics"`
		Extensions []interface{}          `json:"extensions"`
	} `json:"query"`
	Error *struct {
		Code string `json:"code"`
		Info string `json:"info"`
	} `json:"error"`
}

// Initialize detects and validates the MediaWiki API for a given URL
func (s *MediaWikiService) Initialize(ctx context.Context, wikiURL string) (*MediaWikiClient, error) {
	mediaWikiLog.Info("Initializing", "wiki_url", wikiURL)

	normalizedURL := NormalizeURL(wikiURL)
	if normalizedURL == "" {
		return nil, NewMediaWikiError("normalize_url", wikiURL, ErrInvalidWikiURL)
	}

	// Try to detect if the base URL needs scheme upgrade (http -> https)
	normalizedURL, wasRedirected, err := s.detectSchemeUpgrade(ctx, normalizedURL)
	if err != nil {
		return nil, NewMediaWikiError("detect_scheme", normalizedURL, err)
	}

	// Detect API URL
	apiURL, indexURL, err := s.detectAPIURL(ctx, normalizedURL)
	if err != nil {
		return nil, NewMediaWikiError("detect_api", normalizedURL, err)
	}

	client := &MediaWikiClient{
		URL:           wikiURL,
		APIURL:        &apiURL,
		IndexURL:      &indexURL,
		WasRedirected: wasRedirected,
	}

	mediaWikiLog.Info("API found", "api_url", apiURL, "redirected", wasRedirected)
	return client, nil
}

// CreateClientWithURL creates a MediaWikiClient with pre-known API and Index URLs
func (s *MediaWikiService) CreateClientWithURL(wikiURL, apiURL, indexURL string) *MediaWikiClient {
	mediaWikiLog.Info("Creating client with known API", "api_url", apiURL)

	return &MediaWikiClient{
		URL:           wikiURL,
		APIURL:        &apiURL,
		IndexURL:      &indexURL,
		WasRedirected: false,
	}
}

// FetchSiteinfo retrieves site information and statistics from the MediaWiki API
func (s *MediaWikiService) FetchSiteinfo(ctx context.Context, client *MediaWikiClient) (*SiteInfo, error) {
	if client.APIURL == nil {
		return nil, NewMediaWikiError("fetch_siteinfo", client.URL, ErrMediaWikiNotFound)
	}

	// Build API request URL with general, statistics, and extensions
	apiURL := *client.APIURL
	reqURL := fmt.Sprintf("%s?action=query&meta=siteinfo&siprop=general|statistics|extensions&format=json", apiURL)

	start := time.Now()
	resp, err := s.makeRequest(ctx, reqURL)
	if err != nil {
		return nil, NewMediaWikiError("fetch_siteinfo", client.URL, err)
	}
	defer resp.Body.Close()
	elapsed := time.Since(start)

	// Parse response
	var mwResp mediawikiResponse
	if err := json.NewDecoder(resp.Body).Decode(&mwResp); err != nil {
		return nil, NewMediaWikiError("parse_response", client.URL, fmt.Errorf("JSON decode: %w", err))
	}

	// Check for API errors
	if mwResp.Error != nil {
		return nil, NewMediaWikiError("api_error", client.URL, fmt.Errorf("%s: %s", mwResp.Error.Code, mwResp.Error.Info))
	}

	// Parse general info
	general, err := parseSiteInfoGeneral(mwResp.Query.General)
	if err != nil {
		return nil, NewMediaWikiError("parse_general", client.URL, err)
	}

	// Parse statistics
	stats, err := parseSiteInfoStatistics(mwResp.Query.Statistics)
	if err != nil {
		return nil, NewMediaWikiError("parse_statistics", client.URL, err)
	}

	// Parse extensions
	extensions, err := parseExtensions(mwResp.Query.Extensions)
	if err != nil {
		// Don't fail on extensions parse error, just log and continue with empty extensions
		mediaWikiLog.Info("Failed to parse extensions", "err", err)
		extensions = &SiteInfoExtensions{
			Extensions: []ExtensionInfo{},
			Skins:      []ExtensionInfo{},
		}
	}

	siteinfo := &SiteInfo{
		General:      *general,
		Statistics:   *stats,
		Extensions:   *extensions,
		ResponseTime: int(elapsed.Milliseconds()),
		HTTPStatus:   resp.StatusCode,
	}

	mediaWikiLog.Info("Fetched siteinfo", "sitename", general.Sitename, "pages", stats.Pages, "edits", stats.Edits, "response_time_ms", siteinfo.ResponseTime)

	return siteinfo, nil
}

// detectAPIURL tries common MediaWiki API paths
// It intelligently follows scheme/host redirects but ignores path redirects
func (s *MediaWikiService) detectAPIURL(ctx context.Context, baseURL string) (apiURL, indexURL string, err error) {
	// Remove trailing slash for consistent path joining
	baseURL = strings.TrimSuffix(baseURL, "/")

	candidates := mediaWikiCandidates(baseURL)

	// Track last error details for better error reporting
	var lastErr error
	var lastHTTPStatus int
	var lastRespBody string

	for _, candidate := range candidates {
		// Fandom has a stable API layout, including localized paths such as
		// /pl/api.php. Avoid spending an extra request on a redirect probe.
		if requestGroup(candidate.apiURL) == "fandom.com" {
			resp, requestErr := s.makeRequest(ctx, candidate.apiURL+"?action=query&meta=siteinfo&format=json")
			if requestErr != nil {
				if _, rateLimited := asRateLimitError(requestErr); rateLimited {
					return "", "", requestErr
				}
				lastErr = requestErr
				continue
			}
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			var result map[string]interface{}
			if json.Unmarshal(body, &result) == nil {
				if _, ok := result["query"]; ok {
					return candidate.apiURL, candidate.indexURL, nil
				}
			}
			lastHTTPStatus = resp.StatusCode
			lastRespBody = string(body)
			continue
		}

		// Check for permanent redirects on the API URL
		redirectedAPI, hasRedirect, checkErr := s.checkRedirect(ctx, candidate.apiURL)
		if _, rateLimited := asRateLimitError(checkErr); rateLimited {
			return "", "", checkErr
		}
		if checkErr == nil && hasRedirect {
			// Check if this is a scheme/host-only redirect (path unchanged)
			if isSchemeOrHostRedirect(candidate.apiURL, redirectedAPI) {
				mediaWikiLog.Info("Testing redirect for API", "api_url", candidate.apiURL, "redirected_api", redirectedAPI)

				// Test if the redirected URL actually works as a MediaWiki API
				testURL := redirectedAPI + "?action=query&meta=siteinfo&format=json"
				resp, testErr := s.makeRequest(ctx, testURL)
				if _, rateLimited := asRateLimitError(testErr); rateLimited {
					return "", "", testErr
				}
				if testErr == nil {
					defer resp.Body.Close()

					// Check if response is valid MediaWiki API
					var result map[string]interface{}
					body, _ := io.ReadAll(resp.Body)
					if json.Unmarshal(body, &result) == nil {
						if _, ok := result["query"]; ok {
							// Redirected URL works! Use it
							mediaWikiLog.Info("Using redirected API", "api_url", redirectedAPI)
							apiURL = redirectedAPI

							// Also upgrade index URL to match the redirect target
							redirectedURL, _ := url.Parse(redirectedAPI)
							originalIndexURL, _ := url.Parse(candidate.indexURL)

							// Construct new index URL with redirected scheme+host and original path
							newIndexURL := &url.URL{
								Scheme: redirectedURL.Scheme,
								Host:   redirectedURL.Host,
								Path:   originalIndexURL.Path,
							}
							indexURL = newIndexURL.String()

							return apiURL, indexURL, nil
						}
					}
				}

				// Redirected URL doesn't work as MediaWiki API, fall through to test original
				mediaWikiLog.Info("Redirected URL doesn't work, trying original", "api_url", candidate.apiURL)
			} else {
				// Path changed - skip this candidate entirely
				mediaWikiLog.Info("Skipping candidate due to path redirect", "api_url", candidate.apiURL, "redirected_api", redirectedAPI)
				continue
			}
		}

		// Test API URL (either original or if redirect didn't work)
		testURL := candidate.apiURL + "?action=query&meta=siteinfo&format=json"
		resp, err := s.makeRequest(ctx, testURL)
		if err != nil {
			if _, rateLimited := asRateLimitError(err); rateLimited {
				return "", "", err
			}
			lastErr = err
			continue
		}
		defer resp.Body.Close()

		// Store response details for error reporting
		lastHTTPStatus = resp.StatusCode
		body, _ := io.ReadAll(resp.Body)
		lastRespBody = string(body)

		// Check if response is valid JSON
		var result map[string]interface{}
		if err := json.Unmarshal(body, &result); err != nil {
			continue
		}

		// Check for "query" key (valid MediaWiki API response)
		if _, ok := result["query"]; ok {
			return candidate.apiURL, candidate.indexURL, nil
		}
	}

	// Build detailed error message
	errMsg := fmt.Sprintf("API not found (tried %d candidates", len(candidates))
	if lastHTTPStatus > 0 {
		// Include HTTP status and response preview (first 120 chars)
		respPreview := lastRespBody
		if len(respPreview) > 120 {
			respPreview = respPreview[:120] + "..."
		}
		// Clean up the preview for readability
		respPreview = strings.ReplaceAll(respPreview, "\n", " ")
		respPreview = strings.ReplaceAll(respPreview, "\r", " ")
		respPreview = strings.TrimSpace(respPreview)

		errMsg = fmt.Sprintf("%s, last HTTP %d: %s", errMsg, lastHTTPStatus, respPreview)
	} else if lastErr != nil {
		errMsg = fmt.Sprintf("%s, last error: %v", errMsg, lastErr)
	}
	errMsg += ")"

	if lastErr != nil {
		return "", "", NewMediaWikiError("detect_api", baseURL, fmt.Errorf("%s: %w", errMsg, lastErr))
	}
	return "", "", NewMediaWikiError("detect_api", baseURL, errors.New(errMsg))
}

func mediaWikiCandidates(baseURL string) []mediaWikiCandidate {
	baseURL = strings.TrimSuffix(baseURL, "/")
	if requestGroup(baseURL) == "fandom.com" {
		return []mediaWikiCandidate{{
			apiURL:   baseURL + "/api.php",
			indexURL: baseURL + "/index.php",
		}}
	}
	return []mediaWikiCandidate{
		{apiURL: baseURL + "/w/api.php", indexURL: baseURL + "/w/index.php"},
		{apiURL: baseURL + "/api.php", indexURL: baseURL + "/index.php"},
		{apiURL: baseURL + "/wiki/api.php", indexURL: baseURL + "/wiki/index.php"},
	}
}

// checkRedirect checks for permanent redirect (301/308)
func (s *MediaWikiService) checkRedirect(ctx context.Context, url string) (string, bool, error) {
	req, err := http.NewRequestWithContext(ctx, "HEAD", url, nil)
	if err != nil {
		return "", false, err
	}

	req.Header.Set("User-Agent", s.userAgent)

	client := &http.Client{
		Timeout: s.timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			// Don't follow redirects automatically
			return http.ErrUseLastResponse
		},
	}

	resp, err := s.execute(ctx, url, func() (*http.Response, error) { return client.Do(req) })
	if err != nil {
		return "", false, err
	}
	defer resp.Body.Close()

	// Check for permanent redirect
	if resp.StatusCode == 301 || resp.StatusCode == 308 {
		location := resp.Header.Get("Location")
		if location != "" {
			mediaWikiLog.Info("Permanent redirect", "url", url, "location", location)
			return location, true, nil
		}
	}

	return url, false, nil
}

// detectSchemeUpgrade checks if the URL should be upgraded from http to https
// Returns the normalized URL and whether a redirect occurred
func (s *MediaWikiService) detectSchemeUpgrade(ctx context.Context, rawURL string) (string, bool, error) {
	// Only check http URLs
	if !strings.HasPrefix(rawURL, "http://") {
		return rawURL, false, nil
	}

	// Inspect the HTTP endpoint's redirect without following it. The HTTPS
	// landing page may itself return a WAF challenge, which is unrelated to API
	// availability and must not prevent a scheme upgrade.
	testURL := strings.TrimSuffix(rawURL, "/") + "/"
	req, err := http.NewRequestWithContext(ctx, "HEAD", testURL, nil)
	if err != nil {
		return rawURL, false, nil
	}

	req.Header.Set("User-Agent", s.userAgent)

	client := &http.Client{
		Timeout: 10 * time.Second,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	resp, err := s.execute(ctx, testURL, func() (*http.Response, error) { return client.Do(req) })
	if err != nil {
		if _, rateLimited := asRateLimitError(err); rateLimited {
			return rawURL, false, err
		}
		return rawURL, false, nil
	}
	defer resp.Body.Close()

	if !isRedirectStatus(resp.StatusCode) {
		return rawURL, false, nil
	}
	location, err := resp.Location()
	if err != nil || location.Scheme != "https" {
		return rawURL, false, nil
	}

	original, err := url.Parse(rawURL)
	if err != nil {
		return rawURL, false, nil
	}
	original.Scheme = "https"
	original.Host = location.Host
	httpsURL := original.String()
	mediaWikiLog.Info("Scheme upgrade", "url", rawURL, "https_url", httpsURL)
	return httpsURL, true, nil
}

func isRedirectStatus(status int) bool {
	switch status {
	case http.StatusMovedPermanently,
		http.StatusFound,
		http.StatusSeeOther,
		http.StatusTemporaryRedirect,
		http.StatusPermanentRedirect:
		return true
	default:
		return false
	}
}

// isSchemeOrHostRedirect checks if a redirect only changed the scheme or host (but not path)
// This allows following http->https upgrades and domain changes, while ignoring path redirects
func isSchemeOrHostRedirect(originalURL, redirectURL string) bool {
	origParsed, err1 := url.Parse(originalURL)
	if err1 != nil {
		return false
	}

	redirectParsed, err2 := url.Parse(redirectURL)
	if err2 != nil {
		return false
	}

	// Check if path is the same (ignore scheme and host differences)
	return origParsed.Path == redirectParsed.Path
}

// makeRequest makes an HTTP request with proper headers and timeout
func (s *MediaWikiService) makeRequest(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", s.userAgent)
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: s.timeout}
	resp, err := s.execute(ctx, url, func() (*http.Response, error) { return client.Do(req) })
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}

	// Check HTTP status
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		// Truncate response body for error messages (max 120 chars)
		bodyStr := string(body)
		if len(bodyStr) > 120 {
			bodyStr = bodyStr[:120] + "..."
		}
		// Clean up for readability
		bodyStr = strings.ReplaceAll(bodyStr, "\n", " ")
		bodyStr = strings.ReplaceAll(bodyStr, "\r", " ")
		bodyStr = strings.TrimSpace(bodyStr)
		return nil, &HTTPStatusError{
			StatusCode: resp.StatusCode,
			RetryAfter: resp.Header.Get("Retry-After"),
			Body:       bodyStr,
		}
	}

	return resp, nil
}

func (s *MediaWikiService) execute(
	ctx context.Context,
	rawURL string,
	request func() (*http.Response, error),
) (*http.Response, error) {
	var resp *http.Response
	perform := func() error {
		var err error
		resp, err = request()
		if err != nil {
			return err
		}
		if resp.StatusCode == http.StatusTooManyRequests {
			return responseStatusError(resp)
		}
		return nil
	}

	var err error
	if s.limiter == nil {
		err = perform()
	} else {
		err = s.limiter.Run(ctx, rawURL, perform)
	}
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func responseStatusError(resp *http.Response) *HTTPStatusError {
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	bodyStr := string(body)
	if len(bodyStr) > 120 {
		bodyStr = bodyStr[:120] + "..."
	}
	bodyStr = strings.ReplaceAll(bodyStr, "\n", " ")
	bodyStr = strings.ReplaceAll(bodyStr, "\r", " ")
	bodyStr = strings.TrimSpace(bodyStr)
	return &HTTPStatusError{
		StatusCode: resp.StatusCode,
		RetryAfter: resp.Header.Get("Retry-After"),
		Body:       bodyStr,
	}
}

// parseSiteInfoGeneral parses general site information from API response
func parseSiteInfoGeneral(data map[string]interface{}) (*SiteInfoGeneral, error) {
	general := &SiteInfoGeneral{}

	// Helper to safely get string values
	getString := func(key string) string {
		if v, ok := data[key]; ok {
			if s, ok := v.(string); ok {
				return s
			}
		}
		return ""
	}

	// Helper to safely get int values
	getInt := func(key string) *int {
		if v, ok := data[key]; ok {
			switch val := v.(type) {
			case float64:
				i := int(val)
				return &i
			case int:
				return &val
			}
		}
		return nil
	}

	general.Sitename = getString("sitename")
	general.Lang = getString("lang")
	general.DBType = getString("dbtype")
	general.DBVersion = getString("dbversion")
	general.Generator = getString("generator")
	general.BaseURL = getString("baseurl")
	general.MainPage = getString("mainpage")
	general.MaxPageID = getInt("maxpageid")

	return general, nil
}

// parseSiteInfoStatistics parses statistics from API response
func parseSiteInfoStatistics(data map[string]interface{}) (*SiteInfoStatistics, error) {
	stats := &SiteInfoStatistics{}

	getInt := func(key string) int {
		if v, ok := data[key]; ok {
			switch val := v.(type) {
			case float64:
				return int(val)
			case int:
				return val
			case string:
				// Try to parse as int
				var i int
				fmt.Sscanf(val, "%d", &i)
				return i
			}
		}
		return 0
	}

	stats.Pages = getInt("pages")
	stats.Articles = getInt("articles")
	stats.Edits = getInt("edits")
	stats.Images = getInt("images")
	stats.Users = getInt("users")
	stats.ActiveUsers = getInt("activeusers")
	stats.Admins = getInt("admins")
	stats.Jobs = getInt("jobs")

	return stats, nil
}

// parseExtensions parses extensions from API response
func parseExtensions(data []interface{}) (*SiteInfoExtensions, error) {
	extensions := &SiteInfoExtensions{
		Extensions: []ExtensionInfo{},
		Skins:      []ExtensionInfo{},
	}

	// MediaWiki API returns extensions as a list
	if len(data) == 0 {
		// No extensions data, return empty lists
		return extensions, nil
	}

	extList := data

	// Track seen extensions to avoid duplicates from API
	seenExtensions := make(map[string]bool)
	seenSkins := make(map[string]bool)
	duplicatesCount := 0

	for _, extItem := range extList {
		extMap, ok := extItem.(map[string]interface{})
		if !ok {
			continue
		}

		info := ExtensionInfo{}

		// Parse type (required)
		if v, ok := extMap["type"].(string); ok {
			info.Type = v
		} else {
			// Skip if type is missing
			continue
		}

		// Parse name (required)
		if v, ok := extMap["name"].(string); ok {
			info.Name = v
		} else {
			// Skip if name is missing
			continue
		}

		// Parse optional fields
		if v, ok := extMap["url"].(string); ok && v != "" {
			info.URL = &v
		}
		if v, ok := extMap["version"].(string); ok && v != "" {
			info.Version = &v
		}
		if v, ok := extMap["license-name"].(string); ok && v != "" {
			info.LicenseName = &v
		}

		// Categorize by type and deduplicate
		key := info.Type + ":" + info.Name
		if info.Type == "skin" {
			if !seenSkins[key] {
				seenSkins[key] = true
				extensions.Skins = append(extensions.Skins, info)
			} else {
				duplicatesCount++
				mediaWikiLog.Info("Duplicate skin found", "type", info.Type, "name", info.Name)
			}
		} else {
			// All other types (extension, parserhook, etc.) go to extensions
			if !seenExtensions[key] {
				seenExtensions[key] = true
				extensions.Extensions = append(extensions.Extensions, info)
			} else {
				duplicatesCount++
				mediaWikiLog.Info("Duplicate extension found", "type", info.Type, "name", info.Name)
			}
		}
	}

	if duplicatesCount > 0 {
		mediaWikiLog.Info("Removed duplicates from API", "duplicates", duplicatesCount,
			"unique_extensions", len(extensions.Extensions),
			"unique_skins", len(extensions.Skins))
	}

	return extensions, nil
}
