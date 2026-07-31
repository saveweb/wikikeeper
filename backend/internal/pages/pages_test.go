package pages

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"

	"wikikeeper-backend/internal/config"
	"wikikeeper-backend/internal/models"
)

func TestRenderPartialLoadsPageTemplate(t *testing.T) {
	tests := []struct {
		name         string
		pageFile     string
		templateName string
		data         M
		want         string
		wantLink     string
	}{
		{
			name:         "wiki list",
			pageFile:     "wiki_list.html",
			templateName: "wiki_list_content",
			data: M{
				"Total": 0, "Page": 2, "PageSize": 20, "Pages": 3,
				"Status": "", "Archive": "", "Search": "", "OrderBy": "updated_at DESC",
				"BaseURL": "/wikis",
			},
			want:     "0 wikis found",
			wantLink: "/wikis?page=1",
		},
		{
			name:         "extension list",
			pageFile:     "extension_list.html",
			templateName: "ext_list_content",
			data: M{
				"Total": 0, "Page": 1, "PageSize": 50, "Pages": 0,
				"Search": "", "Offset": 0, "BaseURL": "/extensions",
			},
			want: "0 extensions found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &Pages{
				cfg:         &config.Config{LogLevel: "INFO"},
				templateDir: "../../web/templates",
			}
			p.baseTemplates = p.parseBaseTemplates()

			e := echo.New()
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)

			require.NoError(t, p.renderPartial(c, tt.pageFile, tt.templateName, tt.data))
			require.Equal(t, http.StatusOK, rec.Code)
			require.Contains(t, rec.Body.String(), tt.want)
			if tt.wantLink != "" {
				require.Contains(t, rec.Body.String(), tt.wantLink)
			}
			require.False(t, strings.Contains(rec.Body.String(), "<!doctype html>"))
		})
	}
}

func TestWikiLabelFallsBackToHostname(t *testing.T) {
	sitename := " Akasa Universe Wiki "
	wikiName := "Custom label"
	empty := "   "

	require.Equal(t, "Akasa Universe Wiki", wikiLabel(&sitename, &wikiName, "https://akasauniverse.miraheze.org/w/"))
	require.Equal(t, "Custom label", wikiLabel(nil, &wikiName, "https://akasauniverse.miraheze.org/w/"))
	require.Equal(t, "alacity.miraheze.org", wikiLabel(&empty, nil, "https://Alacity.Miraheze.org/w/"))
	require.Equal(t, "alacity.miraheze.org", wikiLabel(nil, nil, "alacity.miraheze.org/w/"))
}

func TestDashboardMissingWikiNameUsesLinkedHostname(t *testing.T) {
	wikiID := uuid.New()
	p := &Pages{
		cfg:         &config.Config{LogLevel: "INFO"},
		templateDir: "../../web/templates",
	}
	p.baseTemplates = p.parseBaseTemplates()

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	require.NoError(t, p.render(c, "dashboard.html", M{
		"RecentWikis": []*models.Wiki{{
			ID:        wikiID,
			URL:       "https://alacity.miraheze.org/w/",
			Status:    models.WikiStatusError,
			UpdatedAt: time.Now(),
		}},
	}))

	body := rec.Body.String()
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, body, `href="/wikis/`+wikiID.String()+`"`)
	require.Contains(t, body, ">alacity.miraheze.org</span>")
	require.Contains(t, body, `src="/api/wikis/`+wikiID.String()+`/thumbnail"`)
}

func TestWikiListRendersThumbnail(t *testing.T) {
	wikiID := uuid.New()
	p := &Pages{
		cfg:         &config.Config{LogLevel: "INFO"},
		templateDir: "../../web/templates",
	}
	p.baseTemplates = p.parseBaseTemplates()

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/wikis", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	require.NoError(t, p.renderPartial(c, "wiki_list.html", "wiki_list_content", M{
		"Wikis":    []*models.Wiki{{ID: wikiID, URL: "https://example.org", Status: models.WikiStatusOK}},
		"Total":    int64(1),
		"Page":     1,
		"PageSize": 20,
		"Pages":    1,
		"BaseURL":  "/wikis",
	}))

	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), `src="/api/wikis/`+wikiID.String()+`/thumbnail"`)
}

func TestStatsChartPointsAreChronological(t *testing.T) {
	wikiID := uuid.New()
	newer := time.Date(2026, time.July, 31, 12, 0, 0, 0, time.UTC)
	older := newer.Add(-48 * time.Hour)

	points := statsChartPoints([]*models.WikiStats{
		{WikiID: wikiID, Time: newer, Pages: 20, Articles: 10, Edits: 30},
		{WikiID: wikiID, Time: older, Pages: 15, Articles: 8, Edits: 25},
	})

	require.Len(t, points, 2)
	require.Equal(t, older, points[0].Time)
	require.Equal(t, newer, points[1].Time)

	encoded, err := json.Marshal(points)
	require.NoError(t, err)
	require.JSONEq(t, `[
		{"time":"2026-07-29T12:00:00Z","pages":15,"articles":8,"edits":25},
		{"time":"2026-07-31T12:00:00Z","pages":20,"articles":10,"edits":30}
	]`, string(encoded))
}

func TestWikiDetailRendersErrorsForPublicViewer(t *testing.T) {
	statsError := `HTTP 429: <script>alert("not executable")</script>`
	archiveError := "archive lookup timed out"
	statsErrorAt := time.Date(2026, time.July, 31, 4, 25, 57, 0, time.UTC)
	archiveErrorAt := statsErrorAt.Add(-time.Hour)

	p := &Pages{
		cfg:         &config.Config{LogLevel: "INFO"},
		templateDir: "../../web/templates",
	}
	p.baseTemplates = p.parseBaseTemplates()

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/wikis/test", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	require.NoError(t, p.render(c, "wiki_detail.html", M{
		"Title":   "Wiki Detail",
		"IsAdmin": false,
		"Wiki": &models.Wiki{
			ID:                 uuid.New(),
			URL:                "https://example.fandom.com",
			Status:             models.WikiStatusError,
			CollectionStatus:   models.CollectionStatusRateLimited,
			LastError:          &statsError,
			LastErrorAt:        &statsErrorAt,
			ArchiveLastError:   &archiveError,
			ArchiveLastErrorAt: &archiveErrorAt,
		},
	}))

	body := rec.Body.String()
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, body, "Last Stats Error")
	require.Contains(t, body, "Last Archive Error")
	require.Contains(t, body, "collection: rate limited")
	require.Contains(t, body, `/api/wikis/`)
	require.Contains(t, body, `/thumbnail`)
	require.Contains(t, body, "HTTP 429")
	require.Contains(t, body, "archive lookup timed out")
	require.Contains(t, body, "2026-07-31 04:25")
	require.Contains(t, body, `&lt;script&gt;alert(&#34;not executable&#34;)&lt;/script&gt;`)
	require.NotContains(t, body, `<script>alert("not executable")</script>`)
}
