package services

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func requireMediaWikiIntegration(t *testing.T) {
	t.Helper()
	if testing.Short() || os.Getenv("WIKIKEEPER_INTEGRATION_TESTS") != "1" {
		t.Skip("set WIKIKEEPER_INTEGRATION_TESTS=1 to run external MediaWiki API tests")
	}
}

// TestMediaWikiService_Initialize_RealAPI tests API detection with real Wikipedia
func TestMediaWikiService_Initialize_RealAPI(t *testing.T) {
	requireMediaWikiIntegration(t)

	service := NewMediaWikiService(30*time.Second, "WikiKeeper-Test/1.0")
	ctx := context.Background()

	t.Run("Test Wikipedia", func(t *testing.T) {
		client, err := service.Initialize(ctx, "https://test.wikipedia.org/")

		require.NoError(t, err)
		require.NotNil(t, client)
		require.NotNil(t, client.APIURL)
		assert.Contains(t, *client.APIURL, "api.php")
		assert.Contains(t, *client.IndexURL, "index.php")
	})

	t.Run("English Wikipedia", func(t *testing.T) {
		client, err := service.Initialize(ctx, "https://en.wikipedia.org/")

		require.NoError(t, err)
		require.NotNil(t, client)
		assert.NotNil(t, client.APIURL)
	})
}

func TestDetectSchemeUpgradeUsesHTTPRedirect(t *testing.T) {
	tests := []struct {
		name      string
		status    int
		location  string
		wantHTTPS bool
		wantHost  string
	}{
		{
			name:      "redirect to HTTPS despite challenged destination",
			status:    http.StatusFound,
			location:  "https://canonical.example/challenge",
			wantHTTPS: true,
			wantHost:  "canonical.example",
		},
		{
			name:      "direct forbidden response does not prove HTTPS redirect",
			status:    http.StatusForbidden,
			wantHTTPS: false,
		},
		{
			name:      "redirect remaining on HTTP is ignored",
			status:    http.StatusMovedPermanently,
			location:  "http://canonical.example/",
			wantHTTPS: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				require.Equal(t, http.MethodHead, r.Method)
				if tt.location != "" {
					w.Header().Set("Location", tt.location)
				}
				w.WriteHeader(tt.status)
			}))
			defer server.Close()

			service := NewMediaWikiService(time.Second, "WikiKeeper-Test/1.0")
			input := server.URL + "/base"
			got, upgraded := service.detectSchemeUpgrade(context.Background(), input)
			require.Equal(t, tt.wantHTTPS, upgraded)
			if tt.wantHTTPS {
				require.Equal(t, "https://"+tt.wantHost+"/base", got)
			} else {
				require.Equal(t, input, got)
			}
		})
	}
}

func TestDetectSchemeUpgradeSkipsHTTPSInput(t *testing.T) {
	service := NewMediaWikiService(time.Second, "WikiKeeper-Test/1.0")
	input := "https://example.com"
	got, upgraded := service.detectSchemeUpgrade(context.Background(), input)
	require.False(t, upgraded)
	require.True(t, strings.HasPrefix(got, "https://"))
}

func TestInitializePreservesRateLimitFromAPIDetection(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Retry-After", "90")
		http.Error(w, "rate limited", http.StatusTooManyRequests)
	}))
	defer server.Close()

	service := NewMediaWikiService(time.Second, "WikiKeeper-Test/1.0")
	_, err := service.Initialize(context.Background(), server.URL)
	require.Error(t, err)

	statusErr, rateLimited := asRateLimitError(err)
	require.True(t, rateLimited)
	require.Equal(t, http.StatusTooManyRequests, statusErr.StatusCode)
	require.Equal(t, "90", statusErr.RetryAfter)
}

// TestMediaWikiService_FetchSiteinfo_RealAPI tests fetching real siteinfo
func TestMediaWikiService_FetchSiteinfo_RealAPI(t *testing.T) {
	requireMediaWikiIntegration(t)

	service := NewMediaWikiService(30*time.Second, "WikiKeeper-Test/1.0")
	ctx := context.Background()

	client, err := service.Initialize(ctx, "https://test.wikipedia.org/")
	require.NoError(t, err)

	siteinfo, err := service.FetchSiteinfo(ctx, client)
	require.NoError(t, err)

	// Verify general info
	assert.NotEmpty(t, siteinfo.General.Sitename)
	assert.NotEmpty(t, siteinfo.General.Lang)
	assert.NotEmpty(t, siteinfo.General.Generator)
	assert.Contains(t, siteinfo.General.Generator, "MediaWiki")

	// Verify statistics
	assert.Greater(t, siteinfo.Statistics.Pages, 0)
	assert.Greater(t, siteinfo.Statistics.Articles, 0)
	assert.Greater(t, siteinfo.Statistics.Edits, 0)
	assert.Greater(t, siteinfo.Statistics.Users, 0)

	// Verify response metrics
	assert.Greater(t, siteinfo.ResponseTime, 0)
	assert.Equal(t, 200, siteinfo.HTTPStatus)

	t.Logf("Site: %s (%s)", siteinfo.General.Sitename, siteinfo.General.Lang)
	t.Logf("Pages: %d, Articles: %d, Edits: %d",
		siteinfo.Statistics.Pages, siteinfo.Statistics.Articles, siteinfo.Statistics.Edits)
	t.Logf("Response time: %dms", siteinfo.ResponseTime)
}

// TestMediaWikiService_InvalidURL tests error handling for invalid URLs
func TestMediaWikiService_InvalidURL(t *testing.T) {
	requireMediaWikiIntegration(t)

	service := NewMediaWikiService(10*time.Second, "WikiKeeper-Test/1.0")
	ctx := context.Background()

	testCases := []struct {
		name    string
		url     string
		wantErr error
	}{
		{
			name:    "Not a wiki",
			url:     "https://example.com/",
			wantErr: ErrMediaWikiNotFound,
		},
		{
			name:    "Invalid domain",
			url:     "https://this-domain-does-not-exist-12345.com/",
			wantErr: ErrMediaWikiUnavailable, // Or connection error
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := service.Initialize(ctx, tc.url)

			// For connection errors, we may get different errors
			assert.Error(t, err)
		})
	}
}

// TestMediaWikiService_RedirectDetection tests redirect detection
func TestMediaWikiService_RedirectDetection(t *testing.T) {
	requireMediaWikiIntegration(t)

	service := NewMediaWikiService(10*time.Second, "WikiKeeper-Test/1.0")
	ctx := context.Background()

	// Test a URL that might redirect
	client, err := service.Initialize(ctx, "http://test.wikipedia.org/") // Note: http instead of https

	require.NoError(t, err)
	assert.NotNil(t, client)

	// Most sites redirect http to https
	if client.WasRedirected {
		t.Logf("Redirect detected: original URL was redirected")
	}
}

// TestNormalizeURL tests URL normalization
func TestNormalizeURL(t *testing.T) {
	testCases := []struct {
		input    string
		expected string
	}{
		{"https://en.wikipedia.org/", "https://en.wikipedia.org"},
		{"https://en.wikipedia.org/wiki", "https://en.wikipedia.org"},
		{"https://en.wikipedia.org/w", "https://en.wikipedia.org"},
		{"en.wikipedia.org", "https://en.wikipedia.org"},
		{"  https://en.wikipedia.org/  ", "https://en.wikipedia.org"},
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			result := NormalizeURL(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
}

// TestMediaWikiService_Timeout tests timeout handling
func TestMediaWikiService_Timeout(t *testing.T) {
	requireMediaWikiIntegration(t)

	// Use a very short timeout
	service := NewMediaWikiService(1*time.Millisecond, "WikiKeeper-Test/1.0")
	ctx := context.Background()

	// This should timeout due to slow response
	client, err := service.Initialize(ctx, "https://test.wikipedia.org/")

	// We expect an error (timeout)
	assert.Error(t, err)
	if client == nil {
		t.Logf("Timeout working correctly: %v", err)
	}
}
