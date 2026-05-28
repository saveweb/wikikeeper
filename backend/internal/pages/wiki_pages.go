package pages

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"gorm.io/gorm"

	"wikikeeper-backend/internal/models"
	"wikikeeper-backend/internal/repository"
	"wikikeeper-backend/internal/services"
)

func (p *Pages) WikiList(c echo.Context) error {
	data := p.baseData(c, "Wikis")
	ctx := c.Request().Context()

	page, _ := strconv.Atoi(c.QueryParam("page"))
	if page < 1 {
		page = 1
	}
	pageSize := 20
	status := c.QueryParam("status")
	archive := c.QueryParam("has_archive")
	search := c.QueryParam("search")
	orderBy := c.QueryParam("order_by")
	if orderBy == "" {
		orderBy = "updated_at DESC"
	}

	var statusFilter *models.WikiStatus
	if status != "" {
		s := models.WikiStatus(status)
		statusFilter = &s
	}
	var archiveFilter *bool
	if archive == "true" {
		t := true
		archiveFilter = &t
	} else if archive == "false" {
		f := false
		archiveFilter = &f
	}

	wikiRepo := repository.NewWikiRepository(p.db)
	wikis, total, err := wikiRepo.List(ctx, repository.ListOptions{
		Page:      page,
		PageSize:  pageSize,
		Status:    statusFilter,
		HasArchive: archiveFilter,
		Search:    search,
		OrderBy:   orderBy,
	})
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	data["Wikis"] = wikis
	data["Total"] = total
	data["Page"] = page
	data["PageSize"] = pageSize
	data["Pages"] = totalPages(total, pageSize)
	data["Status"] = status
	data["Archive"] = archive
	data["Search"] = search
	data["OrderBy"] = orderBy
	data["BaseURL"] = "/wikis"

	if p.isHTMX(c) {
		return p.renderPartial(c, "wiki_list_content", data)
	}
	return p.render(c, "wiki_list.html", data)
}

func (p *Pages) WikiAdd(c echo.Context) error {
	data := p.baseData(c, "Add Wiki")
	return p.render(c, "wiki_add.html", data)
}

func (p *Pages) WikiAddSubmit(c echo.Context) error {
	url := c.FormValue("url")
	if url == "" {
		return c.HTML(http.StatusBadRequest, `<span class="text-red-600 text-sm">URL is required</span>`)
	}

	ctx := c.Request().Context()
	wikiRepo := repository.NewWikiRepository(p.db)

	exists, _ := wikiRepo.ExistsByURL(ctx, url)
	if exists {
		return c.HTML(http.StatusConflict, `<span class="text-red-600 text-sm">Wiki already exists</span>`)
	}

	wikiName := c.FormValue("wiki_name")
	wiki := &models.Wiki{
		URL:      url,
		Status:   models.WikiStatusPending,
		IsActive: true,
	}
	if wikiName != "" {
		wiki.WikiName = &wikiName
	}

	if err := wikiRepo.Create(ctx, wiki); err != nil {
		return c.HTML(http.StatusInternalServerError, `<span class="text-red-600 text-sm">Failed to create wiki</span>`)
	}

	c.Response().Header().Set("HX-Redirect", "/wikis/"+wiki.ID.String())
	return c.HTML(http.StatusCreated, "")
}

func (p *Pages) WikiDetail(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid wiki ID")
	}

	data := p.baseData(c, "Wiki Detail")
	ctx := c.Request().Context()

	wikiRepo := repository.NewWikiRepository(p.db)
	wiki, err := wikiRepo.GetByID(ctx, id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return echo.NewHTTPError(http.StatusNotFound, "Wiki not found")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	data["Wiki"] = wiki

	statsRepo := repository.NewStatsRepository(p.db)
	latestStats, err := statsRepo.GetLatestByWikiID(ctx, id)
	if err == nil && latestStats != nil {
		data["LatestStats"] = latestStats
	}

	statsHistory, err := statsRepo.GetByWikiID(ctx, id, 365)
	if err == nil && len(statsHistory) > 1 {
		type chartPoint struct {
			Time     time.Time `json:"time"`
			Pages    int       `json:"Pages"`
			Articles int       `json:"Articles"`
			Edits    int       `json:"Edits"`
		}
		var points []chartPoint
		for _, s := range statsHistory {
			points = append(points, chartPoint{
				Time:     s.Time,
				Pages:    s.Pages,
				Articles: s.Articles,
				Edits:    s.Edits,
			})
		}
		data["StatsJSON"] = toJS(points)
	}

	extRepo := repository.NewExtensionsRepository(p.db)
	extSnapshot, err := extRepo.GetLatestSnapshot(ctx, id)
	if err == nil && extSnapshot != nil {
		data["Extensions"] = extSnapshot
	}

	archiveRepo := repository.NewArchiveRepository(p.db)
	archives, err := archiveRepo.GetByWikiID(ctx, id)
	if err == nil {
		data["Archives"] = archives
	}

	return p.render(c, "wiki_detail.html", data)
}

func (p *Pages) TriggerCheck(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.HTML(http.StatusBadRequest, `<span class="text-red-600 text-sm">Invalid wiki ID</span>`)
	}

	ctx := c.Request().Context()
	wikiRepo := repository.NewWikiRepository(p.db)
	wiki, err := wikiRepo.GetByID(ctx, id)
	if err != nil {
		return c.HTML(http.StatusNotFound, `<span class="text-red-600 text-sm">Wiki not found</span>`)
	}

	if !p.isAdmin(c) {
		if wiki.LastCheckAt != nil && time.Since(*wiki.LastCheckAt) < time.Hour {
			return c.HTML(http.StatusTooManyRequests, `<span class="text-yellow-600 text-sm">Rate limited, try again later</span>`)
		}
	}

	go func() {
		bgCtx := context.Background()
		mwService := services.NewMediaWikiService(
			time.Duration(p.cfg.HTTPTimeout)*time.Second,
			p.cfg.HTTPUserAgent,
		)
		collector := services.NewCollectorService(p.db, mwService, p.cfg)
		_ = collector.CollectSingleWiki(bgCtx, id)
	}()

	return c.HTML(http.StatusAccepted, `<span class="text-green-600 text-sm">Check started. <a href="/wikis/`+id.String()+`" class="underline">Refresh</a> in ~30s.</span>`)
}

func (p *Pages) TriggerArchiveCheck(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.HTML(http.StatusBadRequest, `<span class="text-red-600 text-sm">Invalid wiki ID</span>`)
	}

	ctx := c.Request().Context()
	wikiRepo := repository.NewWikiRepository(p.db)
	wiki, err := wikiRepo.GetByID(ctx, id)
	if err != nil {
		return c.HTML(http.StatusNotFound, `<span class="text-red-600 text-sm">Wiki not found</span>`)
	}

	if wiki.APIURL == nil || *wiki.APIURL == "" {
		return c.HTML(http.StatusBadRequest, `<span class="text-yellow-600 text-sm">No API URL. Run stats check first.</span>`)
	}

	if !p.isAdmin(c) {
		if wiki.ArchiveLastCheckAt != nil && time.Since(*wiki.ArchiveLastCheckAt) < time.Hour {
			return c.HTML(http.StatusTooManyRequests, `<span class="text-yellow-600 text-sm">Rate limited, try again later</span>`)
		}
	}

	go func() {
		bgCtx := context.Background()
		apiURL := ""
		if wiki.APIURL != nil {
			apiURL = *wiki.APIURL
		}
		indexURL := ""
		if wiki.IndexURL != nil {
			indexURL = *wiki.IndexURL
		}
		_, _, _, _ = p.archiveService.CollectArchives(bgCtx, p.db, id, apiURL, indexURL)
	}()

	return c.HTML(http.StatusAccepted, `<span class="text-green-600 text-sm">Archive check started. <a href="/wikis/`+id.String()+`" class="underline">Refresh</a> in ~30s.</span>`)
}
