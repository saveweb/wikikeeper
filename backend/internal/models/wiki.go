package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// WikiStatus represents the status of a wiki
type WikiStatus string

const (
	WikiStatusPending WikiStatus = "pending"
	WikiStatusOK      WikiStatus = "ok"
	WikiStatusError   WikiStatus = "error"
	WikiStatusOffline WikiStatus = "offline"
)

// Wiki represents a wiki site being tracked
type Wiki struct {
	ID       uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	URL      string    `gorm:"type:varchar(2048);not null;uniqueIndex" json:"url"`
	APIURL   *string   `gorm:"type:varchar(2048);index" json:"api_url"`
	IndexURL *string   `gorm:"type:varchar(2048)" json:"index_url,omitempty"`
	WikiName *string   `gorm:"type:varchar(255)" json:"wiki_name,omitempty"`

	// Metadata from siteinfo.general
	Sitename         *string `gorm:"type:varchar(255);index" json:"sitename"`
	Lang             *string `gorm:"type:varchar(10)" json:"lang"`
	DBType           *string `gorm:"type:varchar(50)" json:"dbtype,omitempty"`
	DBVersion        *string `gorm:"type:varchar(50)" json:"dbversion,omitempty"`
	MediaWikiVersion *string `gorm:"type:varchar(50)" json:"mediawiki_version,omitempty"`
	MaxPageID        *int    `json:"max_page_id,omitempty"`

	// Status and tracking
	Status       WikiStatus `gorm:"type:varchar(20);not null;default:'pending';index" json:"status"`
	HasArchive   bool       `gorm:"not null;default:false;index" json:"has_archive"`
	APIAvailable bool       `gorm:"not null;default:true" json:"api_available"`

	// Error tracking (for siteinfo checks)
	LastError   *string    `gorm:"type:text" json:"last_error"`
	LastErrorAt *time.Time `json:"last_error_at,omitempty"`

	// Archive check status
	ArchiveLastCheckAt *time.Time `gorm:"index" json:"archive_last_check_at,omitempty"`
	ArchiveLastError   *string    `gorm:"type:text" json:"archive_last_error,omitempty"`
	ArchiveLastErrorAt *time.Time `json:"archive_last_error_at,omitempty"`

	// Timestamps
	CreatedAt   time.Time  `gorm:"not null;default:now();index" json:"created_at"`
	UpdatedAt   time.Time  `gorm:"not null;default:now()" json:"updated_at"`
	LastCheckAt *time.Time `gorm:"index" json:"last_check_at,omitempty"`

	// Settings
	IsActive bool `gorm:"not null;default:true" json:"is_active,omitempty"`

	// Relations
	Stats    []WikiStats   `gorm:"foreignKey:WikiID;constraint:OnDelete:CASCADE" json:"-"`
	Archives []WikiArchive `gorm:"foreignKey:WikiID;constraint:OnDelete:CASCADE" json:"-"`
}

// BeforeUpdate hook to set UpdatedAt
func (w *Wiki) BeforeUpdate(tx *gorm.DB) error {
	w.UpdatedAt = time.Now()
	return nil
}

// TableName specifies the table name for GORM
func (Wiki) TableName() string {
	return "wikis"
}
