package services

import (
	"net/url"
	"strings"
)

// NormalizeExplicitAPIURL parses a user-provided MediaWiki API endpoint while
// preserving the path's case, which may be significant on the remote server.
func NormalizeExplicitAPIURL(rawURL string) (wikiURL, apiURL, indexURL string, ok bool) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return "", "", "", false
	}
	if !strings.HasPrefix(rawURL, "http://") && !strings.HasPrefix(rawURL, "https://") {
		rawURL = "https://" + rawURL
	}

	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Hostname() == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return "", "", "", false
	}

	path := strings.TrimSuffix(parsed.Path, "/")
	separator := strings.LastIndex(path, "/")
	if separator < 0 || !strings.EqualFold(path[separator+1:], "api.php") {
		return "", "", "", false
	}

	parsed.Path = path
	parsed.RawPath = ""
	parsed.RawQuery = ""
	parsed.Fragment = ""
	apiURL = parsed.String()

	parsed.Path = path[:separator]
	wikiURL = strings.TrimSuffix(parsed.String(), "/")

	parsed.Path = path[:separator+1] + "index.php"
	indexURL = parsed.String()
	return wikiURL, apiURL, indexURL, true
}

// NormalizeURL reduces API, index, and article URLs to the MediaWiki base path.
func NormalizeURL(rawURL string) string {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return ""
	}
	if !strings.HasPrefix(rawURL, "http://") && !strings.HasPrefix(rawURL, "https://") {
		rawURL = "https://" + rawURL
	}

	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Hostname() == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return ""
	}
	parsed.RawQuery = ""
	parsed.Fragment = ""

	segments := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(segments) == 1 && segments[0] == "" {
		segments = nil
	}

	for i, segment := range segments {
		if strings.EqualFold(segment, "wiki") {
			segments = segments[:i]
			break
		}
	}
	if len(segments) > 0 {
		last := segments[len(segments)-1]
		if strings.EqualFold(last, "api.php") || strings.EqualFold(last, "index.php") {
			segments = segments[:len(segments)-1]
		} else if strings.EqualFold(last, "w") || strings.EqualFold(last, "wiki") {
			segments = segments[:len(segments)-1]
		}
	}

	parsed.Path = ""
	if len(segments) > 0 {
		parsed.Path = "/" + strings.Join(segments, "/")
	}
	parsed.RawPath = ""

	return strings.TrimSuffix(parsed.String(), "/")
}
