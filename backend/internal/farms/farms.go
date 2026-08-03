package farms

import (
	"net/url"
	"strings"

	"wikikeeper-backend/internal/models"
)

type domainRule struct {
	farm    models.WikiFarm
	domains []string
}

// domainRules classifies farm-hosted domains. Add new farms or legacy domains
// here; every newly created wiki is classified from this list.
var domainRules = []domainRule{
	{farm: models.WikiFarmFandom, domains: []string{"fandom.com", "wikia.com", "wikia.org", "gamepedia.com"}},
	{farm: models.WikiFarmMiraheze, domains: []string{"miraheze.org"}},
	{farm: models.WikiFarmShoutWiki, domains: []string{"shoutwiki.com"}},
}

// Detect returns the farm hosting rawURL, if its hostname is recognized.
func Detect(rawURL string) *models.WikiFarm {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return nil
	}
	host := strings.TrimSuffix(strings.ToLower(parsed.Hostname()), ".")
	for _, rule := range domainRules {
		for _, domain := range rule.domains {
			if host == domain || strings.HasSuffix(host, "."+domain) {
				farm := rule.farm
				return &farm
			}
		}
	}
	return nil
}
