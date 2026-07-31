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
