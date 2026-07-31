package pages

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"

	"wikikeeper-backend/internal/config"
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
