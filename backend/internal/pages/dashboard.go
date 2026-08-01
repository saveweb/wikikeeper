package pages

import (
	"github.com/labstack/echo/v4"

	"wikikeeper-backend/internal/repository"
)

func (p *Pages) Dashboard(c echo.Context) error {
	data := p.baseData(c, "Dashboard")
	ctx := c.Request().Context()

	wikiRepo := repository.NewWikiRepository(p.db)
	stats, err := wikiRepo.GetSummaryStats(ctx, false)
	if err == nil && stats != nil {
		data["Stats"] = map[string]any{
			"TotalWikis":       stats["total_wikis"],
			"ArchivedWikis":    stats["archived_wikis"],
			"ArchivedSize":     stats["archived_size"],
			"StatusOkWikis":    stats["status_ok_wikis"],
			"StatusErrorWikis": stats["status_error_wikis"],
			"ActiveWikis":      stats["active_wikis"],
			"TotalPages":       stats["total_pages"],
			"TotalEdits":       stats["total_edits"],
		}
	}

	wikis, _, err := wikiRepo.List(ctx, repository.ListOptions{
		Page:     1,
		PageSize: 10,
		OrderBy:  "updated_at DESC",
	})
	if err == nil {
		data["RecentWikis"] = wikis
	}

	return p.render(c, "dashboard.html", data)
}
