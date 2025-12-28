package config

import (
	"os"
	"testing"
)

func TestConfigLoad(t *testing.T) {
	// Reset config
	cfg = nil

	// Test default values
	c := Load()

	if c.AppName != "WikiKeeper" {
		t.Errorf("Expected AppName 'WikiKeeper', got '%s'", c.AppName)
	}

	if c.AppVersion != "0.2.0" {
		t.Errorf("Expected AppVersion '0.2.0', got '%s'", c.AppVersion)
	}

	if c.Host != "0.0.0.0" {
		t.Errorf("Expected Host '0.0.0.0', got '%s'", c.Host)
	}

	if c.Port != 8000 {
		t.Errorf("Expected Port 8000, got %d", c.Port)
	}

	if c.HTTPTimeout != 30.0 {
		t.Errorf("Expected HTTPTimeout 30.0, got %f", c.HTTPTimeout)
	}

	if c.Debug != false {
		t.Errorf("Expected Debug false, got %v", c.Debug)
	}
}

func TestConfigEnvOverride(t *testing.T) {
	// Reset config
	cfg = nil

	// Set environment variables
	os.Setenv("APP_NAME", "TestTracker")
	os.Setenv("PORT", "9000")
	os.Setenv("DEBUG", "true")
	os.Setenv("HTTP_TIMEOUT", "60.0")

	// Load config
	c := Load()

	// Check overrides
	if c.AppName != "TestTracker" {
		t.Errorf("Expected AppName 'TestTracker', got '%s'", c.AppName)
	}

	if c.Port != 9000 {
		t.Errorf("Expected Port 9000, got %d", c.Port)
	}

	if c.Debug != true {
		t.Errorf("Expected Debug true, got %v", c.Debug)
	}

	if c.HTTPTimeout != 60.0 {
		t.Errorf("Expected HTTPTimeout 60.0, got %f", c.HTTPTimeout)
	}

	// Cleanup
	os.Unsetenv("APP_NAME")
	os.Unsetenv("PORT")
	os.Unsetenv("DEBUG")
	os.Unsetenv("HTTP_TIMEOUT")
	cfg = nil
}

func TestConfigGet(t *testing.T) {
	// Reset config
	cfg = nil

	c1 := Get()
	c2 := Get()

	if c1 != c2 {
		t.Error("Expected Get() to return singleton instance")
	}
}

func TestGetEnvInt(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		value    string
		fallback int
		expected int
	}{
		{
			name:     "valid int",
			key:      "TEST_INT",
			value:    "12345",
			fallback: 0,
			expected: 12345,
		},
		{
			name:     "invalid int uses fallback",
			key:      "TEST_INT_INVALID",
			value:    "not_a_number",
			fallback: 42,
			expected: 42,
		},
		{
			name:     "missing env uses fallback",
			key:      "TEST_INT_MISSING",
			value:    "",
			fallback: 99,
			expected: 99,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.value != "" {
				os.Setenv(tt.key, tt.value)
				defer os.Unsetenv(tt.key)
			}

			result := getEnvInt(tt.key, tt.fallback)
			if result != tt.expected {
				t.Errorf("Expected %d, got %d", tt.expected, result)
			}
		})
	}
}

func TestGetEnvBool(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		value    string
		fallback bool
		expected bool
	}{
		{
			name:     "true string",
			key:      "TEST_BOOL",
			value:    "true",
			fallback: false,
			expected: true,
		},
		{
			name:     "false string",
			key:      "TEST_BOOL",
			value:    "false",
			fallback: true,
			expected: false,
		},
		{
			name:     "invalid bool uses fallback",
			key:      "TEST_BOOL_INVALID",
			value:    "not_a_bool",
			fallback: true,
			expected: true,
		},
		{
			name:     "number 1 is true",
			key:      "TEST_BOOL",
			value:    "1",
			fallback: false,
			expected: true,
		},
		{
			name:     "number 0 is false",
			key:      "TEST_BOOL",
			value:    "0",
			fallback: true,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.value != "" {
				os.Setenv(tt.key, tt.value)
				defer os.Unsetenv(tt.key)
			}

			result := getEnvBool(tt.key, tt.fallback)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestGetEnvFloat(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		value    string
		fallback float64
		expected float64
	}{
		{
			name:     "valid float",
			key:      "TEST_FLOAT",
			value:    "3.14",
			fallback: 0.0,
			expected: 3.14,
		},
		{
			name:     "integer string",
			key:      "TEST_FLOAT",
			value:    "42",
			fallback: 0.0,
			expected: 42.0,
		},
		{
			name:     "invalid float uses fallback",
			key:      "TEST_FLOAT_INVALID",
			value:    "not_a_float",
			fallback: 2.71,
			expected: 2.71,
		},
		{
			name:     "missing env uses fallback",
			key:      "TEST_FLOAT_MISSING",
			value:    "",
			fallback: 1.0,
			expected: 1.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.value != "" {
				os.Setenv(tt.key, tt.value)
				defer os.Unsetenv(tt.key)
			}

			result := getEnvFloat(tt.key, tt.fallback)
			if result != tt.expected {
				t.Errorf("Expected %f, got %f", tt.expected, result)
			}
		})
	}
}
