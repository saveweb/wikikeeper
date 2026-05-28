package pages

import (
	"context"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"wikikeeper-backend/internal/repository"
	"wikikeeper-backend/internal/services"
)

func (p *Pages) AdminDeleteWiki(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.HTML(http.StatusBadRequest, `<span class="text-red-600">Invalid ID</span>`)
	}

	wikiRepo := repository.NewWikiRepository(p.db)
	if err := wikiRepo.Delete(c.Request().Context(), id); err != nil {
		return c.HTML(http.StatusInternalServerError, `<span class="text-red-600">Delete failed</span>`)
	}

	c.Response().Header().Set("HX-Redirect", "/wikis")
	return c.HTML(http.StatusOK, "")
}

func (p *Pages) AdminCollectAll(c echo.Context) error {
	go func() {
		bgCtx := context.Background()
		mwService := services.NewMediaWikiService(
			time.Duration(p.cfg.HTTPTimeout)*time.Second,
			p.cfg.HTTPUserAgent,
		)
		collector := services.NewCollectorService(p.db, mwService, p.cfg)
		_, _ = collector.CollectBatch(bgCtx, 10000, 1*time.Second)
	}()

	return c.HTML(http.StatusAccepted, `<span class="text-green-600 text-sm">Collection started for all wikis</span>`)
}

func (p *Pages) AdminCheckAllArchives(c echo.Context) error {
	go func() {
		bgCtx := context.Background()
		wikiRepo := repository.NewWikiRepository(p.db)
		wikis, _, _ := wikiRepo.List(bgCtx, repository.ListOptions{PageSize: 10000})
		for _, w := range wikis {
			if w.APIURL != nil && *w.APIURL != "" {
				apiURL := *w.APIURL
				indexURL := ""
				if w.IndexURL != nil {
					indexURL = *w.IndexURL
				}
				_, _, _, _ = p.archiveService.CollectArchives(bgCtx, p.db, w.ID, apiURL, indexURL)
			}
		}
	}()

	return c.HTML(http.StatusAccepted, `<span class="text-green-600 text-sm">Archive check started for all wikis</span>`)
}
