package farms

import (
	"testing"

	"github.com/stretchr/testify/require"
	"wikikeeper-backend/internal/models"
)

func TestDetect(t *testing.T) {
	tests := []struct {
		url  string
		farm *models.WikiFarm
	}{
		{url: "https://starwars.fandom.com", farm: farmPtr(models.WikiFarmFandom)},
		{url: "https://community.wikia.org/api.php", farm: farmPtr(models.WikiFarmFandom)},
		{url: "https://example.miraheze.org", farm: farmPtr(models.WikiFarmMiraheze)},
		{url: "https://example.shoutwiki.com/wiki/Main_Page", farm: farmPtr(models.WikiFarmShoutWiki)},
		{url: "https://example.wikioasis.org/wiki/Main_Page", farm: farmPtr(models.WikiFarmWikiOasis)},
		{url: "https://wikioasis.org", farm: farmPtr(models.WikiFarmWikiOasis)},
		{url: "https://minecraft.wiki.gg/wiki/Main_Page", farm: farmPtr(models.WikiFarmWikiGG)},
		{url: "https://wiki.gg", farm: farmPtr(models.WikiFarmWikiGG)},
		{url: "https://notfandom.com", farm: nil},
		{url: "https://notwikioasis.org", farm: nil},
		{url: "https://notwiki.gg", farm: nil},
		{url: "https://example.org", farm: nil},
	}
	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			require.Equal(t, tt.farm, Detect(tt.url))
		})
	}
}

func farmPtr(farm models.WikiFarm) *models.WikiFarm { return &farm }
