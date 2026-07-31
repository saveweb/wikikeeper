package pages

import (
	"context"
	"net/http"
	"slices"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"gorm.io/gorm"

	"wikikeeper-backend/internal/models"
	"wikikeeper-backend/internal/repository"
	"wikikeeper-backend/internal/services"
)

type statsChartPoint struct {
	Time     time.Time `json:"time"`
	Pages    int       `json:"pages"`
	Articles int       `json:"articles"`
	Edits    int       `json:"edits"`
}

func statsChartPoints(stats []*models.WikiStats) []statsChartPoint {
	points := make([]statsChartPoint, 0, len(stats))
	for _, s := range stats {
		points = append(points, statsChartPoint{
			Time:     s.Time,
			Pages:    s.Pages,
			Articles: s.Articles,
			Edits:    s.Edits,
		})
	}
	slices.SortFunc(points, func(a, b statsChartPoint) int {
		return a.Time.Compare(b.Time)
	})
	return points
}

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
		Page:       page,
		PageSize:   pageSize,
		Status:     statusFilter,
		HasArchive: archiveFilter,
		Search:     search,
		OrderBy:    orderBy,
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
		return p.renderPartial(c, "wiki_list.html", "wiki_list_content", data)
	}
	return p.render(c, "wiki_list.html", data)
}

func (p *Pages) WikiAdd(c echo.Context) error {
	data := p.baseData(c, "Add Wiki")
	return p.render(c, "wiki_add.html", data)
}

func (p *Pages) WikiAddSubmit(c echo.Context) error {
	rawURL := c.FormValue("url")
	if rawURL == "" {
		return c.HTML(http.StatusBadRequest, `<span class="text-red-600 text-sm">URL is required</span>`)
	}
	wikiURL := services.NormalizeURL(rawURL)
	if wikiURL == "" {
		return c.HTML(http.StatusBadRequest, `<span class="text-red-600 text-sm">Invalid wiki URL</span>`)
	}

	ctx := c.Request().Context()
	wikiRepo := repository.NewWikiRepository(p.db)

	exists, _ := wikiRepo.ExistsByURL(ctx, wikiURL)
	if exists {
		return c.HTML(http.StatusConflict, `<span class="text-red-600 text-sm">Wiki already exists</span>`)
	}

	wikiName := c.FormValue("wiki_name")
	wiki := &models.Wiki{
		URL:              wikiURL,
		Status:           models.WikiStatusPending,
		CollectionStatus: models.CollectionStatusPending,
		IsActive:         true,
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
		data["StatsJSON"] = toJS(statsChartPoints(statsHistory))
	}

	extRepo := repository.NewExtensionsRepository(p.db)
	extSnapshot, err := extRepo.GetLatestSnapshot(ctx, id)
	if err == nil && extSnapshot != nil {
		data["Extensions"] = extSnapshot
	}
	now := time.Now()
	extensionHistory, err := extRepo.GetSnapshotsInTimeRange(ctx, id, now.AddDate(-1, 0, 0), now)
	if err == nil && len(extensionHistory) > 1 {
		data["ExtensionHistory"] = extensionHistory
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
	if retryAt, limited := p.collectorService.ProviderCooldown(ctx, wiki.URL); limited {
		retryAfter := time.Until(retryAt).Round(time.Second)
		return c.HTML(http.StatusTooManyRequests, `<span class="text-yellow-600 text-sm">Provider rate limited; retry in `+retryAfter.String()+`</span>`)
	}

	go func() {
		bgCtx := context.Background()
		_ = p.collectorService.CollectSingleWiki(bgCtx, id)
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
