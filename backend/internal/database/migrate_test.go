package database

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCollectionStateMigrationIsRegistered(t *testing.T) {
	require.Equal(t, "add_collection_state", getMigrationName(7))

	up, err := readMigrationFile(7, "up")
	require.NoError(t, err)
	for _, fragment := range []string{
		"collection_status",
		"last_success_at",
		"next_check_at",
		"consecutive_failures",
		"last_error ILIKE '%HTTP 429%'",
		"WHEN last_success_at IS NULL THEN 'pending' ELSE 'ok'",
	} {
		require.True(t, strings.Contains(up, fragment), "migration must contain %q", fragment)
	}

	down, err := readMigrationFile(7, "down")
	require.NoError(t, err)
	require.Contains(t, down, "DROP COLUMN IF EXISTS collection_status")
}

func TestResetStaleRateLimitsMigrationIsRegistered(t *testing.T) {
	require.Equal(t, "reset_stale_rate_limits", getMigrationName(9))

	up, err := readMigrationFile(9, "up")
	require.NoError(t, err)
	for _, fragment := range []string{
		"collection_status = 'pending'",
		"consecutive_failures = 0",
		"HASHTEXT(id::text)",
		"604800",
		"next_check_at <= NOW()",
	} {
		require.Contains(t, up, fragment)
	}

	down, err := readMigrationFile(9, "down")
	require.NoError(t, err)
	require.Contains(t, down, "SELECT 1")
}
