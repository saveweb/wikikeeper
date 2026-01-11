package config

import (
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

// Config holds application configuration
type Config struct {
	AppName    string
	AppVersion string
	Host       string
	Port       int

	// Database (PostgreSQL)
	DBHost     string
	DBPort     string
	DBUser     string
	DBPassword string
	DBName     string

	// MongoDB (for migration read-only)
	MongoDBURI    string
	MongoDBDBName string

	// HTTP Client
	HTTPTimeout   float64
	HTTPUserAgent string

	// Siteinfo check settings
	SiteinfoCheckBatchSize int // Number of wikis to check per cycle

	// Archive.org check settings
	ArchiveCheckBatchSize int // Number of wikis to check per cycle

	// Authentication
	AdminToken string // Token for admin access

	// CORS
	AllowOrigins []string // CORS allowed origins

	// Logging
	LogLevel string
}

var cfg *Config

// Load loads configuration from environment variables
// It automatically loads .env file if present
func Load() *Config {
	if cfg != nil {
		return cfg
	}

	// Load .env file if exists (ignore error in production)
	godotenv.Load()

	cfg = &Config{
		AppName:                getEnv("APP_NAME", "WikiKeeper"),
		AppVersion:             getEnv("APP_VERSION", "0.2.0"),
		Host:                   getEnv("HOST", "0.0.0.0"),
		Port:                   getEnvInt("PORT", 8000),
		DBHost:                 getEnv("DB_HOST", "localhost"),
		DBPort:                 getEnv("DB_PORT", "5432"),
		DBUser:                 getEnv("DB_USER", "wikikeeper"),
		DBPassword:             getEnv("DB_PASSWORD", "wikikeeper123"),
		DBName:                 getEnv("DB_NAME", "wikikeeper"),
		MongoDBURI:             getEnv("MONGODB_URI", "mongodb://localhost:27017"),
		MongoDBDBName:          getEnv("MONGODB_DB_NAME", "wikikeeper"),
		HTTPTimeout:            getEnvFloat("HTTP_TIMEOUT", 30.0),
		HTTPUserAgent:          getEnv("HTTP_USER_AGENT", "WikiKeeper/0.3.0 (https://wikikeeper.saveweb.org/)"),
		SiteinfoCheckBatchSize: getEnvInt("SITEINFO_CHECK_BATCH_SIZE", 100), // Check 100 wikis per cycle
		ArchiveCheckBatchSize:  getEnvInt("ARCHIVE_CHECK_BATCH_SIZE", 100),  // Check 100 wikis per cycle
		AdminToken:             getEnv("ADMIN_TOKEN", ""),                   // Empty means no admin protection
		AllowOrigins:           getEnvStringSlice("ALLOW_ORIGINS", []string{"http://localhost:5173", "http://localhost:3000", "http://localhost:8000"}),
		LogLevel:               getEnv("LOG_LEVEL", "INFO"),
	}

	return cfg
}

// Get returns the loaded configuration
func Get() *Config {
	if cfg == nil {
		return Load()
	}
	return cfg
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return fallback
}

func getEnvBool(key string, fallback bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolVal, err := strconv.ParseBool(value); err == nil {
			return boolVal
		}
	}
	return fallback
}

func getEnvFloat(key string, fallback float64) float64 {
	if value := os.Getenv(key); value != "" {
		if floatVal, err := strconv.ParseFloat(value, 64); err == nil {
			return floatVal
		}
	}
	return fallback
}

func getEnvStringSlice(key string, fallback []string) []string {
	if value := os.Getenv(key); value != "" {
		// Split by comma
		parts := splitString(value)
		if len(parts) > 0 {
			return parts
		}
	}
	return fallback
}

func splitString(s string) []string {
	if s == "" {
		return []string{}
	}

	var result []string
	var current string
	inQuotes := false

	for _, r := range s {
		switch r {
		case ',':
			if inQuotes {
				current += string(r)
			} else {
				if current != "" {
					result = append(result, current)
				}
				current = ""
			}
		case '"', '\'':
			inQuotes = !inQuotes
		default:
			current += string(r)
		}
	}

	if current != "" {
		result = append(result, current)
	}

	return result
}
