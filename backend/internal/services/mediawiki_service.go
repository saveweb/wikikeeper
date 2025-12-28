package services

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	applogger "wikikeeper-backend/internal/logger"
)

// MediaWikiService handles MediaWiki API interactions
type MediaWikiService struct {
	timeout  time.Duration
	userAgent string
}

// NewMediaWikiService creates a new MediaWiki service instance
func NewMediaWikiService(timeout time.Duration, userAgent string) *MediaWikiService {
	if userAgent == "" {
		userAgent = "WikiKeeper/1.0"
	}
	return &MediaWikiService{
		timeout:   timeout,
		userAgent: userAgent,
	}
}

// MediaWikiClient represents a detected MediaWiki installation
type MediaWikiClient struct {
	URL            string  // Original URL
	APIURL         *string // Detected API URL
	IndexURL       *string // Detected index URL
	WasRedirected  bool    // Whether URL was permanently redirected
}

// SiteInfo contains site information and statistics
type SiteInfo struct {
	General      SiteInfoGeneral
	Statistics   SiteInfoStatistics
	ResponseTime int   // Response time in milliseconds
	HTTPStatus   int   // HTTP status code
}

// SiteInfoGeneral contains general site information from siteinfo
type SiteInfoGeneral struct {
	Sitename      string  `json:"sitename"`
	Lang          string  `json:"lang"`
	DBType        string  `json:"dbtype"`
	DBVersion     string  `json:"dbversion"`
	Generator     string  `json:"generator"`
	BaseURL       string  `json:"baseurl"`
	MainPage      string  `json:"mainpage"`
	MaxPageID     *int    `json:"maxpageid,omitempty"`
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

// API response structures
type mediawikiResponse struct {
	Query struct {
		General     map[string]interface{} `json:"general"`
		Statistics  map[string]interface{} `json:"statistics"`
	} `json:"query"`
	Error *struct {
		Code    string `json:"code"`
		Info    string `json:"info"`
	} `json:"error"`
}

// Initialize detects and validates the MediaWiki API for a given URL
func (s *MediaWikiService) Initialize(ctx context.Context, wikiURL string) (*MediaWikiClient, error) {
	applogger.Log.Info("[MediaWiki] Initializing: %s", wikiURL)

	// Try to detect if the base URL needs scheme upgrade (http -> https)
	normalizedURL, wasRedirected := s.detectSchemeUpgrade(ctx, wikiURL)

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

	applogger.Log.Info("[MediaWiki] API found: %s (redirected: %v)", apiURL, wasRedirected)
	return client, nil
}

// CreateClientWithURL creates a MediaWikiClient with pre-known API and Index URLs
func (s *MediaWikiService) CreateClientWithURL(wikiURL, apiURL, indexURL string) *MediaWikiClient {
	applogger.Log.Info("[MediaWiki] Creating client with known API: %s", apiURL)

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

	// Build API request URL with both general and statistics
	apiURL := *client.APIURL
	reqURL := fmt.Sprintf("%s?action=query&meta=siteinfo&siprop=general|statistics&format=json", apiURL)

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

	siteinfo := &SiteInfo{
		General:      *general,
		Statistics:   *stats,
		ResponseTime: int(elapsed.Milliseconds()),
		HTTPStatus:   resp.StatusCode,
	}

	applogger.Log.Info("[MediaWiki] Fetched siteinfo: %s (pages=%d, edits=%d, %dms)",
		general.Sitename, stats.Pages, stats.Edits, siteinfo.ResponseTime)

	return siteinfo, nil
}

// detectAPIURL tries common MediaWiki API paths
// It intelligently follows scheme/host redirects but ignores path redirects
func (s *MediaWikiService) detectAPIURL(ctx context.Context, baseURL string) (apiURL, indexURL string, err error) {
	// Remove trailing slash for consistent path joining
	baseURL = strings.TrimSuffix(baseURL, "/")

	// Common API paths to try
	candidates := []struct {
		apiURL   string
		indexURL string
	}{
		{baseURL + "/w/api.php", baseURL + "/w/index.php"},
		{baseURL + "/api.php", baseURL + "/index.php"},
		{baseURL + "/wiki/api.php", baseURL + "/wiki/index.php"},
	}

	// Track last error details for better error reporting
	var lastErr error
	var lastHTTPStatus int
	var lastRespBody string

	for _, candidate := range candidates {
		// Check for permanent redirects on the API URL
		redirectedAPI, hasRedirect, checkErr := s.checkRedirect(ctx, candidate.apiURL)
		if checkErr == nil && hasRedirect {
			// Check if this is a scheme/host-only redirect (path unchanged)
			if isSchemeOrHostRedirect(candidate.apiURL, redirectedAPI) {
				applogger.Log.Info("[MediaWiki] Testing redirect for API: %s -> %s", candidate.apiURL, redirectedAPI)

				// Test if the redirected URL actually works as a MediaWiki API
				testURL := redirectedAPI + "?action=query&meta=siteinfo&format=json"
				resp, testErr := s.makeRequest(ctx, testURL)
				if testErr == nil {
					defer resp.Body.Close()

					// Check if response is valid MediaWiki API
					var result map[string]interface{}
					body, _ := io.ReadAll(resp.Body)
					if json.Unmarshal(body, &result) == nil {
						if _, ok := result["query"]; ok {
							// Redirected URL works! Use it
							applogger.Log.Info("[MediaWiki] Using redirected API: %s", redirectedAPI)
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
				applogger.Log.Info("[MediaWiki] Redirected URL doesn't work, trying original: %s", candidate.apiURL)
			} else {
				// Path changed - skip this candidate entirely
				applogger.Log.Info("[MediaWiki] Skipping candidate due to path redirect: %s -> %s", candidate.apiURL, redirectedAPI)
				continue
			}
		}

		// Test API URL (either original or if redirect didn't work)
		testURL := candidate.apiURL + "?action=query&meta=siteinfo&format=json"
		resp, err := s.makeRequest(ctx, testURL)
		if err != nil {
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

	return "", "", NewMediaWikiError("detect_api", baseURL, fmt.Errorf(errMsg))
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

	resp, err := client.Do(req)
	if err != nil {
		return "", false, err
	}
	defer resp.Body.Close()

	// Check for permanent redirect
	if resp.StatusCode == 301 || resp.StatusCode == 308 {
		location := resp.Header.Get("Location")
		if location != "" {
			applogger.Log.Info("[MediaWiki] Permanent redirect: %s -> %s", url, location)
			return location, true, nil
		}
	}

	return url, false, nil
}

// detectSchemeUpgrade checks if the URL should be upgraded from http to https
// Returns the normalized URL and whether a redirect occurred
func (s *MediaWikiService) detectSchemeUpgrade(ctx context.Context, url string) (string, bool) {
	// Only check http URLs
	if !strings.HasPrefix(url, "http://") {
		return url, false
	}

	// Try the https version directly
	httpsURL := strings.Replace(url, "http://", "https://", 1)

	// Test if https version is accessible
	// Use a quick HEAD request to the root path
	testURL := strings.TrimSuffix(httpsURL, "/") + "/"
	req, err := http.NewRequestWithContext(ctx, "HEAD", testURL, nil)
	if err != nil {
		return url, false
	}

	req.Header.Set("User-Agent", s.userAgent)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		// HTTPS not available, stick with HTTP
		return url, false
	}
	defer resp.Body.Close()

	// If HTTPS responds successfully (even with redirect), upgrade
	if resp.StatusCode >= 200 && resp.StatusCode < 400 {
		applogger.Log.Info("[MediaWiki] Scheme upgrade: %s -> %s", url, httpsURL)
		return httpsURL, true
	}

	// HTTPS not available, stick with HTTP
	return url, false
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
	resp, err := client.Do(req)
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
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, bodyStr)
	}

	return resp, nil
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
