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
	"wikikeeper-backend/internal/repository"
)

func TestRenderPartialLoadsPageTemplate(t *testing.T) {
	tests := []struct {
		name         string
		pageFile     string
		templateName string
		data         M
		want         string
		wantLink     string
		wantTarget   string
	}{
		{
			name:         "wiki list",
			pageFile:     "wiki_list.html",
			templateName: "wiki_list_content",
			data: M{
				"Total": 0, "Page": 2, "PageSize": 20, "Pages": 3,
				"Status": "", "Archive": "", "Search": "", "OrderBy": "updated_at DESC",
				"BaseURL": "/wikis", "ListTarget": "#wiki-list-content",
			},
			want:       "0 wikis found",
			wantLink:   "/wikis?page=1",
			wantTarget: `hx-target="#wiki-list-content"`,
		},
		{
			name:         "extension list",
			pageFile:     "extension_list.html",
			templateName: "ext_list_content",
			data: M{
				"Total": 0, "Page": 1, "PageSize": 50, "Pages": 2,
				"Search": "", "Offset": 0, "BaseURL": "/extensions", "ListTarget": "#ext-list-content",
			},
			want:       "0 extensions found",
			wantTarget: `hx-target="#ext-list-content"`,
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
			if tt.wantTarget != "" {
				require.Contains(t, rec.Body.String(), tt.wantTarget)
			}
			require.False(t, strings.Contains(rec.Body.String(), "<!doctype html>"))
		})
	}
}

func TestExtensionListHasSingleSearchParameterSource(t *testing.T) {
	p := &Pages{
		cfg:         &config.Config{LogLevel: "INFO"},
		templateDir: "../../web/templates",
	}
	p.baseTemplates = p.parseBaseTemplates()

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/extensions?search=Parser", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	require.NoError(t, p.render(c, "extension_list.html", M{
		"Search": "Parser",
	}))

	body := rec.Body.String()
	require.Equal(t, 1, strings.Count(body, `name="search"`))
	require.NotContains(t, body, "ext-search-form")
	require.NotContains(t, body, "hx-include")
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

func TestTimeFormattingAlwaysUsesUTC(t *testing.T) {
	paris := time.FixedZone("CEST", 2*60*60)
	value := time.Date(2026, time.July, 31, 14, 25, 57, 0, paris)

	require.Equal(t, "2026-07-31 12:25 UTC", formatDate(value))
	require.Equal(t, "2026-07-31T12:25:57Z", toa(value))
	require.Equal(t, "2026-07-31T12:25:57Z", toa(&value))
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

func TestDashboardFormatsArchivedSize(t *testing.T) {
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
		"Stats": map[string]any{"ArchivedSize": int64(3 * 1024 * 1024 * 1024 / 2)},
	}))

	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), "Archived Size")
	require.Contains(t, rec.Body.String(), "1.5 GiB")
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

func TestWikiListLabelsUnfilteredFarmOptionAsAllWikis(t *testing.T) {
	p := &Pages{
		cfg:         &config.Config{LogLevel: "INFO"},
		templateDir: "../../web/templates",
	}
	p.baseTemplates = p.parseBaseTemplates()

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/wikis", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	require.NoError(t, p.render(c, "wiki_list.html", M{
		"Status": "", "Archive": "", "Farm": "", "Search": "", "OrderBy": "updated_at DESC",
		"Total": int64(0), "Page": 1, "PageSize": 20, "Pages": 1, "BaseURL": "/wikis",
	}))

	require.Contains(t, rec.Body.String(), ">All Wikis</option>")
	require.NotContains(t, rec.Body.String(), ">All Farms</option>")
}

func TestExtensionDetailUsesSitenameWhenWikiNameIsMissing(t *testing.T) {
	wikiID := uuid.New()
	sitename := "MoeGirl London Bridge"
	p := &Pages{
		cfg:         &config.Config{LogLevel: "INFO"},
		templateDir: "../../web/templates",
	}
	p.baseTemplates = p.parseBaseTemplates()

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/extensions/MoegirlLondonBridge", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	require.NoError(t, p.renderPartial(c, "extension_detail.html", "ext_wiki_rows", M{
		"Wikis": []*repository.ExtensionWikiInfo{{
			WikiID:   wikiID,
			Sitename: &sitename,
			URL:      "https://en.moegirl.org.cn/",
		}},
		"WikiPage":  1,
		"WikiPages": 1,
	}))

	body := rec.Body.String()
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, body, `href="/wikis/`+wikiID.String()+`"`)
	require.Contains(t, body, ">MoeGirl London Bridge</a>")
}

func TestStatsChartPointsAreChronological(t *testing.T) {
	wikiID := uuid.New()
	newer := time.Date(2026, time.July, 31, 12, 0, 0, 0, time.UTC)
	older := newer.Add(-48 * time.Hour)

	points := statsChartPoints([]*models.WikiStats{
		{WikiID: wikiID, Time: newer, Pages: 20, Articles: 10, Edits: 30, Images: 4, Users: 50},
		{WikiID: wikiID, Time: older, Pages: 15, Articles: 8, Edits: 25, Images: 3, Users: 40},
	})

	require.Len(t, points, 2)
	require.Equal(t, older, points[0].Time)
	require.Equal(t, newer, points[1].Time)

	encoded, err := json.Marshal(points)
	require.NoError(t, err)
	require.JSONEq(t, `[
		{"time":"2026-07-29T12:00:00Z","pages":15,"articles":8,"edits":25,"images":3,"users":40},
		{"time":"2026-07-31T12:00:00Z","pages":20,"articles":10,"edits":30,"images":4,"users":50}
	]`, string(encoded))
}

func TestWikiDetailRendersStatsChartSeriesSelector(t *testing.T) {
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
		"Wiki": &models.Wiki{ID: uuid.New(), URL: "https://example.org", Status: models.WikiStatusOK},
		"StatsJSON": toJS([]statsChartPoint{{
			Time: time.Date(2026, time.July, 31, 12, 0, 0, 0, time.UTC),
		}}),
	}))

	body := rec.Body.String()
	require.Equal(t, http.StatusOK, rec.Code)
	for _, series := range []string{"pages", "articles", "edits", "images", "users"} {
		require.Contains(t, body, `data-chart-series="`+series+`"`)
	}
	require.Contains(t, body, `id="open-stats-embed"`)
	require.Contains(t, body, `/wikis/`)
	require.Contains(t, body, `/stats/embed`)
	require.Contains(t, body, `src="/static/stats-chart.js"`)
	require.Contains(t, body, `id="stats-chart-data"`)
}

func TestStatsEmbedRendersStandaloneChart(t *testing.T) {
	wikiID := uuid.New()
	p := &Pages{
		cfg:         &config.Config{LogLevel: "INFO"},
		templateDir: "../../web/templates",
	}
	p.baseTemplates = p.parseBaseTemplates()

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/wikis/"+wikiID.String()+"/stats/embed", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	require.NoError(t, p.renderPartial(c, "stats_embed.html", "stats_embed", M{
		"Wiki":      &models.Wiki{ID: wikiID, URL: "https://example.org"},
		"WikiLabel": "Example Wiki",
		"StatsJSON": toJS([]statsChartPoint{{
			Time: time.Date(2026, time.July, 31, 12, 0, 0, 0, time.UTC),
		}}),
	}))

	body := rec.Body.String()
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, body, "<!doctype html>")
	require.Contains(t, body, "Example Wiki Stats History - WikiKeeper")
	require.Contains(t, body, `href="/wikis/`+wikiID.String()+`"`)
	require.Contains(t, body, `data-fill-container`)
	require.Contains(t, body, `src="/static/stats-chart.js"`)
	require.NotContains(t, body, "<nav")
}

func TestWikiDetailRendersErrorsForPublicViewer(t *testing.T) {
	sitename := `Example & Test Wiki`
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
		"Title":   sitename + " - Wiki Detail",
		"IsAdmin": false,
		"Wiki": &models.Wiki{
			ID:                 uuid.New(),
			URL:                "https://example.fandom.com",
			Sitename:           &sitename,
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
	require.Contains(t, body, "<title>Example &amp; Test Wiki - Wiki Detail - WikiKeeper</title>")
	require.Contains(t, body, "Last Stats Error")
	require.Contains(t, body, "Last Archive Error")
	require.Contains(t, body, "Last Stats Check")
	require.Contains(t, body, "Last Stats Success")
	require.Contains(t, body, "Next Stats Check")
	require.Contains(t, body, "Last Archive Check")
	require.Contains(t, body, "collection: rate limited")
	require.Contains(t, body, `/api/wikis/`)
	require.Contains(t, body, `/thumbnail`)
	require.Contains(t, body, "HTTP 429")
	require.Contains(t, body, "archive lookup timed out")
	require.Contains(t, body, "2026-07-31 04:25 UTC")
	require.Contains(t, body, `datetime="2026-07-31T04:25:57Z"`)
	require.Contains(t, body, `&lt;script&gt;alert(&#34;not executable&#34;)&lt;/script&gt;`)
	require.NotContains(t, body, `<script>alert("not executable")</script>`)
}

func TestWikiDetailRendersExtensionSnapshotHistory(t *testing.T) {
	currentVersion := "MediaWiki 1.46.0"
	oldVersion := "MediaWiki 1.45.1"
	currentAt := time.Date(2026, time.July, 31, 12, 0, 0, 0, time.UTC)
	oldAt := currentAt.AddDate(0, -2, 0)
	oldUntil := currentAt
	current := &models.WikiExtensionsSnapshot{
		ID:               uuid.New(),
		SnapshotAt:       currentAt,
		MediaWikiVersion: &currentVersion,
		Items:            []models.WikiExtensionItem{{Name: "VisualEditor"}},
	}
	old := &models.WikiExtensionsSnapshot{
		ID:               uuid.New(),
		SnapshotAt:       oldAt,
		ValidUntil:       &oldUntil,
		MediaWikiVersion: &oldVersion,
		Items:            []models.WikiExtensionItem{{Name: "WikiEditor"}},
	}

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
		"Wiki":             &models.Wiki{ID: uuid.New(), URL: "https://example.org", Status: models.WikiStatusOK},
		"Extensions":       current,
		"ExtensionHistory": []*models.WikiExtensionsSnapshot{current, old},
	}))

	body := rec.Body.String()
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, body, "Extensions History")
	require.Contains(t, body, "Snapshot 2026-07-31 12:00 UTC")
	require.Contains(t, body, "MediaWiki 1.46.0")
	require.Contains(t, body, "MediaWiki 1.45.1")
	require.Contains(t, body, "Current")
	require.Contains(t, body, "Until 2026-07-31 12:00 UTC")
	require.Contains(t, body, "VisualEditor")
	require.Contains(t, body, "WikiEditor")
}
