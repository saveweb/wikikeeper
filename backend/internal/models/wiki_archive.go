package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// WikiArchive represents Archive.org backup information
type WikiArchive struct {
	ID           uuid.UUID  `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	WikiID       uuid.UUID  `gorm:"type:uuid;not null;index:idx_wiki_archives_wiki_id;uniqueIndex:idx_wiki_archive_unique,priority:1" json:"wiki_id"`
	IAIdentifier string     `gorm:"type:varchar(255);not null;uniqueIndex:idx_wiki_archive_unique,priority:2;index" json:"ia_identifier"`

	// Archive metadata
	AddedDate    *time.Time `gorm:"index:idx_wiki_archives_dump_date" json:"added_date"`
	DumpDate     *time.Time `gorm:"index:idx_wiki_archives_dump_date" json:"dump_date"`
	ItemSize     *int64     `json:"item_size"`
	Uploader     *string    `gorm:"type:varchar(255)" json:"uploader"`
	Scanner      *string    `gorm:"type:varchar(255)" json:"scanner"`
	UploadState  *string    `gorm:"type:varchar(50)" json:"upload_state"`

	// Dump content flags
	HasXMLCurrent     bool `gorm:"not null;default:false" json:"has_xml_current"`
	HasXMLHistory     bool `gorm:"not null;default:false" json:"has_xml_history"`
	HasImagesDump     bool `gorm:"not null;default:false" json:"has_images_dump"`
	HasTitlesList     bool `gorm:"not null;default:false" json:"has_titles_list"`
	HasImagesList     bool `gorm:"not null;default:false" json:"has_images_list"`
	HasLegacyWikidump bool `gorm:"not null;default:false" json:"has_legacy_wikidump"`

	// Timestamps
	CreatedAt time.Time `gorm:"not null;default:now()" json:"created_at"`
	UpdatedAt time.Time `gorm:"not null;default:now()" json:"updated_at"`
}

// BeforeUpdate hook to set UpdatedAt
func (wa *WikiArchive) BeforeUpdate(tx *gorm.DB) error {
	wa.UpdatedAt = time.Now()
	return nil
}

// TableName specifies the table name for GORM
func (WikiArchive) TableName() string {
	return "wiki_archives"
}
