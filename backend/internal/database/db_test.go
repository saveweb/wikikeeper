package database

import (
	"testing"
	"wikikeeper-backend/internal/config"
)

func TestConnect(t *testing.T) {
	// Skip if not in integration environment
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	// This test requires a running PostgreSQL instance
	// For CI/CD, use testcontainers or Docker

	db, err := Connect()
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}

	if db == nil {
		t.Fatal("Expected non-nil DB")
	}

	// Test GetDB
	db2 := GetDB()
	if db2 != db {
		t.Error("Expected GetDB() to return same instance")
	}

	// Close connection
	if err := Close(); err != nil {
		t.Errorf("Failed to close: %v", err)
	}
}

func TestConnectInvalidURL(t *testing.T) {
	// Temporarily modify config
	cfg := config.Get()
	originalURL := cfg.DatabaseURL
	defer func() { cfg.DatabaseURL = originalURL }()

	cfg.DatabaseURL = "postgres://invalid:invalid@localhost:9999/bogus?sslmode=disable"

	db, err := Connect()
	if err == nil {
		t.Error("Expected error for invalid connection string")
		Close()
	}

	if db != nil {
		t.Error("Expected nil DB for invalid connection")
	}
}
