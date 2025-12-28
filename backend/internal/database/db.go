package database

import (
	"fmt"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	applogger "wikikeeper-backend/internal/logger"
	"wikikeeper-backend/internal/config"
)

// buildDSN constructs PostgreSQL DSN from config
func buildDSN(cfg *config.Config) string {
	return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		cfg.DBHost, cfg.DBPort, cfg.DBUser, cfg.DBPassword, cfg.DBName)
}

var DB *gorm.DB

// Connect connects to PostgreSQL database using GORM
func Connect() (*gorm.DB, error) {
	cfg := config.Get()

	// Configure GORM
	logLevel := logger.Silent
	switch cfg.LogLevel {
	case "INFO":
		logLevel = logger.Info
	case "WARN":
		logLevel = logger.Warn
	case "ERROR":
		logLevel = logger.Error
	}

	db, err := gorm.Open(postgres.Open(buildDSN(cfg)), &gorm.Config{
		Logger: logger.Default.LogMode(logLevel),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Get underlying SQL DB to configure connection pool
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get database instance: %w", err)
	}

	// Configure connection pool
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)

	// Test connection
	if err := sqlDB.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	applogger.Log.Info("Database connection successful")
	DB = db
	return db, nil
}

// GetDB returns the database connection
func GetDB() *gorm.DB {
	return DB
}

// Close closes the database connection
func Close() error {
	if DB == nil {
		return nil
	}

	sqlDB, err := DB.DB()
	if err != nil {
		return err
	}

	return sqlDB.Close()
}
