package models

import "time"

type ProviderRateLimit struct {
	Provider              string    `gorm:"primaryKey;size:255" json:"provider"`
	RetryAt               time.Time `gorm:"not null;index" json:"retry_at"`
	ConsecutiveRateLimits int       `gorm:"not null;default:1" json:"consecutive_rate_limits"`
	UpdatedAt             time.Time `gorm:"not null" json:"updated_at"`
}
