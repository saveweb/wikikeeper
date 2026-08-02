package pages

import (
	"math"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"

	"wikikeeper-backend/internal/repository"
)

func (p *Pages) ExtensionList(c echo.Context) error {
	data := p.baseData(c, "Extensions")
	ctx := c.Request().Context()

	page, _ := strconv.Atoi(c.QueryParam("page"))
	if page < 1 {
		page = 1
	}
	limit := 50
	search := c.QueryParam("search")

	extRepo := repository.NewExtensionsRepository(p.db)
	extensions, total, err := extRepo.GetAllExtensionsStats(ctx, repository.GetAllExtensionsStatsOptions{
		Page:   page,
		Limit:  limit,
		Search: search,
	})
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	data["Extensions"] = extensions
	data["Total"] = total
	data["Page"] = page
	data["PageSize"] = limit
	data["Pages"] = int(math.Ceil(float64(total) / float64(limit)))
	data["Offset"] = (page - 1) * limit
	data["Search"] = search
	data["BaseURL"] = "/extensions"
	data["ListTarget"] = "#ext-list-content"

	if p.isHTMX(c) {
		return p.renderPartial(c, "extension_list.html", "ext_list_content", data)
	}
	return p.render(c, "extension_list.html", data)
}

type versionInfo struct {
	Version string
	Count   int64
	Percent float64
}

func (p *Pages) ExtensionDetail(c echo.Context) error {
	name := c.Param("name")
	data := p.baseData(c, name)
	ctx := c.Request().Context()

	wikiPage, _ := strconv.Atoi(c.QueryParam("wiki_page"))
	if wikiPage < 1 {
		wikiPage = 1
	}
	wikiLimit := 20

	extRepo := repository.NewExtensionsRepository(p.db)
	data["Name"] = name

	versions, totalWikis, err := extRepo.GetExtensionVersionDistribution(ctx, name)
	if err == nil {
		var vi []versionInfo
		for _, v := range versions {
			pct := 0.0
			if totalWikis > 0 {
				pct = float64(v.Count) / float64(totalWikis) * 100
			}
			vi = append(vi, versionInfo{
				Version: v.Version,
				Count:   v.Count,
				Percent: math.Round(pct*10) / 10,
			})
		}
		data["Versions"] = vi
		data["TotalWikis"] = totalWikis
	}

	wikis, wikiTotal, err := extRepo.GetWikisUsingExtension(ctx, name, repository.ExtensionWikisListOptions{
		Page:  wikiPage,
		Limit: wikiLimit,
	})
	if err == nil {
		data["Wikis"] = wikis
		data["WikiTotal"] = wikiTotal
		data["WikiPage"] = wikiPage
		data["WikiPages"] = int(math.Ceil(float64(wikiTotal) / float64(wikiLimit)))
	}

	return p.render(c, "extension_detail.html", data)
}
