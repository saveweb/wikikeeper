package database

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestCollectionStateMigrationPostgres(t *testing.T) {
	dsn := os.Getenv("WIKIKEEPER_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("set WIKIKEEPER_TEST_POSTGRES_DSN to run PostgreSQL migration tests")
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	for version := 1; version <= 6; version++ {
		sql, err := readMigrationFile(version, "up")
		require.NoError(t, err)
		require.NoError(t, db.Exec(sql).Error, "migration %d", version)
	}

	const verifiedID = "00000000-0000-0000-0000-000000000001"
	const unverifiedID = "00000000-0000-0000-0000-000000000002"
	for _, fixture := range []struct {
		id  string
		url string
	}{
		{id: verifiedID, url: "https://verified.fandom.com"},
		{id: unverifiedID, url: "https://unverified.fandom.com"},
	} {
		require.NoError(t, db.Exec(`
			INSERT INTO wikis (id, url, status, api_available, last_error, last_error_at, last_check_at)
			VALUES (?, ?, 'error', false, 'fetch_siteinfo: HTTP 429', NOW(), NOW())
		`, fixture.id, fixture.url).Error)
	}
	require.NoError(t, db.Exec(`
		INSERT INTO wiki_stats (wiki_id, time, pages, articles, edits, images, users, active_users, admins, jobs)
		VALUES (?, NOW() - INTERVAL '1 hour', 10, 5, 20, 2, 3, 1, 1, 0)
	`, verifiedID).Error)

	up, err := readMigrationFile(7, "up")
	require.NoError(t, err)
	require.NoError(t, db.Exec(up).Error)

	type state struct {
		Status              string
		APIAvailable        bool
		CollectionStatus    string
		LastSuccessAt       *time.Time
		NextCheckAt         *time.Time
		ConsecutiveFailures int
	}
	readState := func(id string) state {
		t.Helper()
		var got state
		require.NoError(t, db.Raw(`
			SELECT status, api_available, collection_status, last_success_at,
			       next_check_at, consecutive_failures
			FROM wikis WHERE id = ?
		`, id).Scan(&got).Error)
		return got
	}

	verified := readState(verifiedID)
	require.Equal(t, "ok", verified.Status)
	require.True(t, verified.APIAvailable)
	require.Equal(t, "rate_limited", verified.CollectionStatus)
	require.NotNil(t, verified.LastSuccessAt)
	require.NotNil(t, verified.NextCheckAt)
	require.Equal(t, 1, verified.ConsecutiveFailures)

	unverified := readState(unverifiedID)
	require.Equal(t, "pending", unverified.Status)
	require.False(t, unverified.APIAvailable)
	require.Equal(t, "rate_limited", unverified.CollectionStatus)
	require.Nil(t, unverified.LastSuccessAt)

	up, err = readMigrationFile(8, "up")
	require.NoError(t, err)
	require.NoError(t, db.Exec(up).Error)
	require.NoError(t, db.Exec(`
		UPDATE wikis
		SET next_check_at = CASE
			WHEN id = ? THEN NOW() - INTERVAL '1 minute'
			ELSE NOW() + INTERVAL '1 hour'
		END
		WHERE id IN (?, ?)
	`, verifiedID, verifiedID, unverifiedID).Error)

	migrationStart := time.Now().Add(-time.Second)
	up, err = readMigrationFile(9, "up")
	require.NoError(t, err)
	require.NoError(t, db.Exec(up).Error)
	migrationEnd := time.Now().Add(time.Second)

	verified = readState(verifiedID)
	require.Equal(t, "pending", verified.CollectionStatus)
	require.Equal(t, 0, verified.ConsecutiveFailures)
	require.NotNil(t, verified.NextCheckAt)
	require.False(t, verified.NextCheckAt.Before(migrationStart))
	require.False(t, verified.NextCheckAt.After(migrationEnd.Add(7*24*time.Hour)))

	unverified = readState(unverifiedID)
	require.Equal(t, "rate_limited", unverified.CollectionStatus)
	require.Equal(t, 1, unverified.ConsecutiveFailures)
	require.NotNil(t, unverified.NextCheckAt)
	require.True(t, unverified.NextCheckAt.After(migrationStart))

	up, err = readMigrationFile(10, "up")
	require.NoError(t, err)
	require.NoError(t, db.Exec(up).Error)
	for _, table := range []string{"wiki_extension_sets", "wiki_extension_set_items", "extension_storage_state"} {
		var exists bool
		require.NoError(t, db.Raw(`SELECT to_regclass(?) IS NOT NULL`, table).Scan(&exists).Error)
		require.True(t, exists, table)
	}
	var legacyWrites bool
	require.NoError(t, db.Raw(`SELECT legacy_writes FROM extension_storage_state WHERE singleton`).Scan(&legacyWrites).Error)
	require.True(t, legacyWrites)
	up, err = readMigrationFile(11, "up")
	require.NoError(t, err)
	require.NoError(t, db.Exec(up).Error)
	var cursor int64
	require.NoError(t, db.Raw(`SELECT backfill_cursor FROM extension_storage_state WHERE singleton`).Scan(&cursor).Error)
	require.Zero(t, cursor)

	require.NoError(t, db.Exec(`SET TIME ZONE 'Europe/Paris'`).Error)
	require.NoError(t, db.Exec(`
		CREATE TABLE schema_migrations (
			version INT PRIMARY KEY,
			name VARCHAR(255) NOT NULL,
			applied_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
		)
	`).Error)
	require.NoError(t, db.Exec(`
		INSERT INTO schema_migrations (version, name, applied_at)
		VALUES (1, 'fixture', TIMESTAMPTZ '2026-01-02 03:04:05+00')
	`).Error)
	require.NoError(t, db.Exec(
		`UPDATE wikis SET created_at = TIMESTAMP '2026-01-02 03:04:05' WHERE id = ?`,
		verifiedID,
	).Error)
	up, err = readMigrationFile(12, "up")
	require.NoError(t, err)
	require.NoError(t, db.Exec(up).Error)

	var createdAt time.Time
	require.NoError(t, db.Raw(`SELECT created_at FROM wikis WHERE id = ?`, verifiedID).Scan(&createdAt).Error)
	require.Equal(t, time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC), createdAt.UTC())
	var appliedAt time.Time
	require.NoError(t, db.Raw(`SELECT applied_at FROM schema_migrations WHERE version = 1`).Scan(&appliedAt).Error)
	require.Equal(t, time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC), appliedAt.UTC())

	var nonUTCColumns int64
	require.NoError(t, db.Raw(`
		SELECT COUNT(*)
		FROM information_schema.columns
		WHERE table_schema = 'public'
		  AND data_type = 'timestamp without time zone'
	`).Scan(&nonUTCColumns).Error)
	require.Zero(t, nonUTCColumns)

	down, err := readMigrationFile(12, "down")
	require.NoError(t, err)
	require.NoError(t, db.Exec(`UPDATE extension_storage_state SET legacy_writes = FALSE`).Error)
	require.NoError(t, db.Exec(down).Error)
	require.NoError(t, db.Exec(up).Error)

	var viewDefinition string
	require.NoError(t, db.Raw(`SELECT pg_get_viewdef('mv_extension_stats'::regclass, true)`).Scan(&viewDefinition).Error)
	require.Contains(t, viewDefinition, "wiki_extension_set_items")
	require.NotContains(t, viewDefinition, "wiki_extension_items wei")
	require.NoError(t, db.Raw(`SELECT applied_at FROM schema_migrations WHERE version = 1`).Scan(&appliedAt).Error)
	require.Equal(t, time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC), appliedAt.UTC())

	require.NoError(t, db.Exec(`DROP SCHEMA public CASCADE; CREATE SCHEMA public`).Error)
	require.NoError(t, RunMigrations(db))
	var appliedCount int64
	require.NoError(t, db.Table("schema_migrations").Count(&appliedCount).Error)
	require.EqualValues(t, len(migrations), appliedCount)
	require.NoError(t, db.Raw(`
		SELECT COUNT(*)
		FROM information_schema.columns
		WHERE table_schema = 'public'
		  AND data_type = 'timestamp without time zone'
	`).Scan(&nonUTCColumns).Error)
	require.Zero(t, nonUTCColumns)
}
