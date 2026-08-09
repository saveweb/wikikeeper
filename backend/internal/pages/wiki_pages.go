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
	Images   int       `json:"images"`
	Users    int       `json:"users"`
}

type extensionSnapshotComparison struct {
	From             *models.WikiExtensionsSnapshot
	To               *models.WikiExtensionsSnapshot
	Diff             *services.ExtensionsDiff
	MediaWikiChanged bool
}

type extensionHistoryEntry struct {
	Snapshot   *models.WikiExtensionsSnapshot
	Comparison *extensionSnapshotComparison
}

func compareExtensionSnapshots(from, to *models.WikiExtensionsSnapshot) *extensionSnapshotComparison {
	return &extensionSnapshotComparison{
		From:             from,
		To:               to,
		Diff:             services.CompareExtensionSnapshots(from, to),
		MediaWikiChanged: stringPointerValue(from.MediaWikiVersion) != stringPointerValue(to.MediaWikiVersion),
	}
}

func extensionHistoryEntries(snapshots []*models.WikiExtensionsSnapshot) []extensionHistoryEntry {
	entries := make([]extensionHistoryEntry, 0, len(snapshots))
	for i, snapshot := range snapshots {
		entry := extensionHistoryEntry{Snapshot: snapshot}
		if i+1 < len(snapshots) {
			entry.Comparison = compareExtensionSnapshots(snapshots[i+1], snapshot)
		}
		entries = append(entries, entry)
	}
	return entries
}

func stringPointerValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func statsChartPoints(stats []*models.WikiStats) []statsChartPoint {
	points := make([]statsChartPoint, 0, len(stats))
	for _, s := range stats {
		points = append(points, statsChartPoint{
			Time:     s.Time.UTC(),
			Pages:    s.Pages,
			Articles: s.Articles,
			Edits:    s.Edits,
			Images:   s.Images,
			Users:    s.Users,
		})
	}
	slices.SortFunc(points, func(a, b statsChartPoint) int {
		return a.Time.Compare(b.Time)
	})
	return points
}

func (p *Pages) getArchiveDumpDates(
	ctx context.Context,
	wikis []*models.Wiki,
) (map[uuid.UUID]repository.ArchiveDumpDates, error) {
	wikiIDs := make([]uuid.UUID, 0, len(wikis))
	for _, wiki := range wikis {
		wikiIDs = append(wikiIDs, wiki.ID)
	}
	return repository.NewArchiveRepository(p.db).GetLatestDumpDatesByWikiIDs(ctx, wikiIDs)
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
	active := c.QueryParam("is_active")
	archive := c.QueryParam("has_archive")
	farm := c.QueryParam("farm")
	search := c.QueryParam("search")
	orderBy, err := repository.ParseWikiOrder(c.QueryParam("order_by"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid order_by value")
	}

	statusFilter, err := repository.ParseWikiStatusFilter(status)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid status value")
	}
	activeFilter, err := repository.ParseWikiActiveFilter(active)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid is_active value")
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
		IsActive:   activeFilter,
		HasArchive: archiveFilter,
		Farm:       farm,
		Search:     search,
		OrderBy:    orderBy,
	})
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	archiveDumpDates, err := p.getArchiveDumpDates(ctx, wikis)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	data["Wikis"] = wikis
	data["ArchiveDumpDates"] = archiveDumpDates
	data["Total"] = total
	data["Page"] = page
	data["PageSize"] = pageSize
	data["Pages"] = totalPages(total, pageSize)
	data["Status"] = status
	data["Active"] = active
	data["Archive"] = archive
	data["Farm"] = farm
	data["Search"] = search
	farms, err := wikiRepo.ListFarms(ctx)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	data["Farms"] = farms
	data["OrderBy"] = string(orderBy)
	data["BaseURL"] = "/wikis"
	data["ListTarget"] = "#wiki-list-content"

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
	data["Title"] = wikiLabel(wiki.Sitename, wiki.WikiName, wiki.URL) + " - Wiki Detail"

	statsRepo := repository.NewStatsRepository(p.db)
	latestStats, err := statsRepo.GetLatestByWikiID(ctx, id)
	if err == nil && latestStats != nil {
		data["LatestStats"] = latestStats
	}

	// A zero-day window returns every stored sample for the detail chart.
	statsHistory, err := statsRepo.GetByWikiID(ctx, id, 0)
	if err == nil && len(statsHistory) > 1 {
		data["StatsJSON"] = toJS(statsChartPoints(statsHistory))
	}

	extRepo := repository.NewExtensionsRepository(p.db)
	extSnapshot, err := extRepo.GetLatestSnapshot(ctx, id)
	if err == nil && extSnapshot != nil {
		data["Extensions"] = extSnapshot
	}
	extensionHistory, err := extRepo.GetAllSnapshots(ctx, id)
	if err == nil && len(extensionHistory) > 1 {
		data["ExtensionSnapshots"] = extensionHistory
		data["ExtensionHistory"] = extensionHistoryEntries(extensionHistory)
		data["ExtensionComparison"] = compareExtensionSnapshots(extensionHistory[1], extensionHistory[0])
	}

	archiveRepo := repository.NewArchiveRepository(p.db)
	archives, err := archiveRepo.GetByWikiID(ctx, id)
	if err == nil {
		data["Archives"] = archives
	}

	return p.render(c, "wiki_detail.html", data)
}

func (p *Pages) AdminSetWikiActive(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid wiki ID")
	}
	active, err := repository.ParseWikiActiveFilter(c.FormValue("is_active"))
	if err != nil || active == nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid is_active value")
	}

	if err := repository.NewWikiRepository(p.db).SetActive(c.Request().Context(), id, *active); err != nil {
		if err == gorm.ErrRecordNotFound {
			return echo.NewHTTPError(http.StatusNotFound, "Wiki not found")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	c.Response().Header().Set("HX-Redirect", "/wikis/"+id.String())
	return c.NoContent(http.StatusNoContent)
}

func (p *Pages) WikiExtensionsCompare(c echo.Context) error {
	wikiID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid wiki ID")
	}
	fromID, err := uuid.Parse(c.QueryParam("from"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid from snapshot ID")
	}
	toID, err := uuid.Parse(c.QueryParam("to"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid to snapshot ID")
	}

	repo := repository.NewExtensionsRepository(p.db)
	ctx := c.Request().Context()
	from, err := repo.GetSnapshotByID(ctx, wikiID, fromID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return echo.NewHTTPError(http.StatusNotFound, "From snapshot not found")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	to, err := repo.GetSnapshotByID(ctx, wikiID, toID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return echo.NewHTTPError(http.StatusNotFound, "To snapshot not found")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return p.renderPartial(c, "wiki_detail.html", "extension_comparison", M{
		"ExtensionComparison": compareExtensionSnapshots(from, to),
		"ExpandComparison":    true,
	})
}

func (p *Pages) WikiStatsEmbed(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid wiki ID")
	}

	ctx := c.Request().Context()
	wikiRepo := repository.NewWikiRepository(p.db)
	wiki, err := wikiRepo.GetByID(ctx, id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return echo.NewHTTPError(http.StatusNotFound, "Wiki not found")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	statsRepo := repository.NewStatsRepository(p.db)
	statsHistory, err := statsRepo.GetByWikiID(ctx, id, 0)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	if len(statsHistory) < 2 {
		return echo.NewHTTPError(http.StatusNotFound, "Stats history is not available")
	}

	c.Response().Header().Set("Content-Security-Policy", "frame-ancestors *")
	c.Response().Header().Del("X-Frame-Options")
	return p.renderPartial(c, "stats_embed.html", "stats_embed", M{
		"Wiki":      wiki,
		"WikiLabel": wikiLabel(wiki.Sitename, wiki.WikiName, wiki.URL),
		"StatsJSON": toJS(statsChartPoints(statsHistory)),
	})
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
