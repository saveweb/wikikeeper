package services

import (
	"net/url"
	"strings"
)

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
