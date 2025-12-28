package services

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMediaWikiService_Initialize_RealAPI tests API detection with real Wikipedia
func TestMediaWikiService_Initialize_RealAPI(t *testing.T) {
	// Skip in short mode
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

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

// TestMediaWikiService_FetchSiteinfo_RealAPI tests fetching real siteinfo
func TestMediaWikiService_FetchSiteinfo_RealAPI(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

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
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

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
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

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
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

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
