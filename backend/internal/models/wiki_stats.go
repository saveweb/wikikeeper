package models

import (
	"time"

	"github.com/google/uuid"
)

// WikiStats represents time-series statistics for a wiki
type WikiStats struct {
	ID     int64     `gorm:"primaryKey;autoIncrement" json:"-"` // Internal ID, not exposed
	WikiID uuid.UUID `gorm:"type:uuid;not null;index:idx_wiki_stats_wiki_time,priority:1" json:"wiki_id"`
	Time   time.Time `gorm:"not null;index:idx_wiki_stats_time,index:idx_wiki_stats_wiki_time,priority:2" json:"time"`

	// From siteinfo.statistics
	Pages       int `gorm:"not null;default:0" json:"pages"`
	Articles    int `gorm:"not null;default:0" json:"articles"`
	Edits       int `gorm:"not null;default:0" json:"edits"`
	Images      int `gorm:"not null;default:0" json:"images"`
	Users       int `gorm:"not null;default:0" json:"users"`
	ActiveUsers int `gorm:"not null;default:0" json:"active_users"`
	Admins      int `gorm:"not null;default:0" json:"admins"`
	Jobs        int `gorm:"not null;default:0" json:"jobs"`

	// Availability metrics
	ResponseTimeMs *int `json:"response_time_ms,omitempty"`
	HTTPStatus     *int `json:"http_status,omitempty"`
}

// TableName specifies the table name for GORM
func (WikiStats) TableName() string {
	return "wiki_stats"
}
