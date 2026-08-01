package database

import (
	"testing"

	"github.com/stretchr/testify/require"

	"wikikeeper-backend/internal/config"
)

func TestBuildDSNForcesUTC(t *testing.T) {
	dsn := buildDSN(&config.Config{
		DBHost: "postgres",
		DBPort: "5432",
		DBUser: "wikikeeper",
		DBName: "wikikeeper",
	})

	require.Contains(t, dsn, "TimeZone=UTC")
}
