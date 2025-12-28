package services

import (
	"net/url"
	"strings"
)

// NormalizeURL normalizes a wiki URL by removing common paths and ensuring scheme
func NormalizeURL(rawURL string) string {
	// Trim whitespace
	rawURL = strings.TrimSpace(rawURL)

	// Remove trailing slash
	rawURL = strings.TrimSuffix(rawURL, "/")

	// Remove common wiki paths
	rawURL = strings.TrimSuffix(rawURL, "/wiki")
	rawURL = strings.TrimSuffix(rawURL, "/w")
	rawURL = strings.TrimSuffix(rawURL, "/index.php")

	// Ensure scheme
	if !strings.HasPrefix(rawURL, "http://") && !strings.HasPrefix(rawURL, "https://") {
		rawURL = "https://" + rawURL
	}

	// Validate URL format
	if _, err := url.Parse(rawURL); err != nil {
		return ""
	}

	return rawURL
}

// isValidURL checks if a URL string is valid
func isValidURL(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	return u.Scheme != "" && u.Host != ""
}
