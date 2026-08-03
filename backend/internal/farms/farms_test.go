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
		{url: "https://notfandom.com", farm: nil},
		{url: "https://example.org", farm: nil},
	}
	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			require.Equal(t, tt.farm, Detect(tt.url))
		})
	}
}

func farmPtr(farm models.WikiFarm) *models.WikiFarm { return &farm }
