package database

import (
	"embed"
	"fmt"
	"slices"
	"sort"

	"gorm.io/gorm"

	applogger "wikikeeper-backend/internal/logger"
)

//go:embed migrations/*.sql
var migrationFS embed.FS

type Migration struct {
	Version int
	Name    string
	Up      string
	Down    string
}

var migrations = []Migration{
	{
		Version: 1,
		Name:    "init_schema",
	},
	{
		Version: 2,
		Name:    "add_extensions",
	},
	{
		Version: 3,
		Name:    "add_mw_version_to_extensions",
	},
	{
		Version: 4,
		Name:    "remove_redundant_fields",
	},
	{
		Version: 5,
		Name:    "optimize_extensions_performance",
	},
	{
		Version: 6,
		Name:    "create_extensions_stats_materialized_view",
	},
	{
		Version: 7,
		Name:    "add_collection_state",
	},
	{
		Version: 8,
		Name:    "add_provider_rate_limits",
	},
	{
		Version: 9,
		Name:    "reset_stale_rate_limits",
	},
	{
		Version: 10,
		Name:    "content_address_extension_sets",
	},
	{
		Version: 11,
		Name:    "extension_backfill_cursor",
	},
}

// RunMigrations executes all pending migrations
func RunMigrations(db *gorm.DB) error {
	// Create migrations tracking table if it doesn't exist
	if err := createMigrationsTable(db); err != nil {
		return fmt.Errorf("failed to create migrations table: %w", err)
	}

	// Get applied migrations
	appliedVersions, err := getAppliedMigrations(db)
	if err != nil {
		return fmt.Errorf("failed to get applied migrations: %w", err)
	}

	// Sort migrations by version
	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].Version < migrations[j].Version
	})

	// Run pending migrations
	for _, migration := range migrations {
		if slices.Contains(appliedVersions, migration.Version) {
			applogger.Log.Info("migration already applied", "version", migration.Version, "name", migration.Name)
			continue
		}

		applogger.Log.Info("running migration", "version", migration.Version, "name", migration.Name)

		// Read up migration file
		upSQL, err := readMigrationFile(migration.Version, "up")
		if err != nil {
			return fmt.Errorf("failed to read migration up file: %w", err)
		}

		// Execute migration
		if err := db.Exec(upSQL).Error; err != nil {
			return fmt.Errorf("failed to execute migration %d: %w", migration.Version, err)
		}

		// Record migration
		if err := recordMigration(db, migration.Version, migration.Name); err != nil {
			return fmt.Errorf("failed to record migration: %w", err)
		}

		applogger.Log.Info("migration completed", "version", migration.Version, "name", migration.Name)
	}

	applogger.Log.Info("all migrations completed successfully")
	return nil
}

func createMigrationsTable(db *gorm.DB) error {
	sql := `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version INT PRIMARY KEY,
			name VARCHAR(255) NOT NULL,
			applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);
	`
	return db.Exec(sql).Error
}

func getAppliedMigrations(db *gorm.DB) ([]int, error) {
	var versions []int
	if err := db.Table("schema_migrations").Pluck("version", &versions).Error; err != nil {
		return nil, err
	}
	return versions, nil
}

func recordMigration(db *gorm.DB, version int, name string) error {
	return db.Table("schema_migrations").Create(map[string]interface{}{
		"version": version,
		"name":    name,
	}).Error
}

func readMigrationFile(version int, direction string) (string, error) {
	filename := fmt.Sprintf("migrations/%03d_%s.%s.sql", version, getMigrationName(version), direction)
	content, err := migrationFS.ReadFile(filename)
	if err != nil {
		return "", err
	}
	return string(content), nil
}

func getMigrationName(version int) string {
	for _, m := range migrations {
		if m.Version == version {
			return m.Name
		}
	}
	return "unknown"
}
