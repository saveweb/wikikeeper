package models

import (
	"time"

	"github.com/google/uuid"
)

// WikiExtensionsSnapshot represents a snapshot of wiki extensions/skins at a point in time
type WikiExtensionsSnapshot struct {
	ID               uuid.UUID           `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	WikiID           uuid.UUID           `gorm:"type:uuid;not null;index:idx_wiki_extensions_wiki_id,priority:1" json:"wiki_id"`
	SnapshotAt       time.Time           `gorm:"not null;index:idx_wiki_extensions_snapshot_at;index:idx_wiki_extensions_wiki_time,priority:2" json:"snapshot_at"`
	ValidUntil       *time.Time          `gorm:"index:idx_wiki_extensions_valid_until" json:"valid_until,omitempty"`
	MediaWikiVersion *string             `gorm:"column:mediawiki_version;type:varchar(255);index:idx_wiki_extensions_mw_version" json:"mediawiki_version,omitempty"`
	ExtensionSetID   *int64              `gorm:"column:extension_set_id" json:"-"`
	Items            []WikiExtensionItem `gorm:"-" json:"items"`
}

// WikiExtensionItem represents a single extension or skin
type WikiExtensionItem struct {
	ID          int64     `gorm:"primaryKey;autoIncrement" json:"-"`
	SnapshotID  uuid.UUID `gorm:"type:uuid;not null;index:idx_wiki_extension_items_snapshot_id" json:"snapshot_id"`
	ExtType     string    `gorm:"type:varchar(50);not null;index:idx_wiki_extension_items_type" json:"ext_type"` // 'extension' or 'skin'
	Name        string    `gorm:"type:varchar(255);not null;index:idx_wiki_extension_items_name" json:"name"`
	URL         *string   `gorm:"type:varchar(2048)" json:"url,omitempty"`
	Version     *string   `gorm:"type:varchar(255)" json:"version,omitempty"`
	LicenseName *string   `gorm:"type:varchar(255)" json:"license_name,omitempty"`
	CreatedAt   time.Time `gorm:"not null;default:now()" json:"created_at"`
}

// WikiExtensionSetItem is a canonical member of a content-addressed extension set.
// Set members deliberately have no surrogate ID or per-row timestamp.
type WikiExtensionSetItem struct {
	SetID       int64   `gorm:"column:set_id;primaryKey"`
	ExtType     string  `gorm:"column:ext_type;primaryKey"`
	Name        string  `gorm:"column:name;primaryKey"`
	URL         *string `gorm:"column:url"`
	Version     *string `gorm:"column:version"`
	LicenseName *string `gorm:"column:license_name"`
}

// TableName specifies the table name for WikiExtensionsSnapshot
func (WikiExtensionsSnapshot) TableName() string {
	return "wiki_extensions_snapshots"
}

// TableName specifies the table name for WikiExtensionItem
func (WikiExtensionItem) TableName() string {
	return "wiki_extension_items"
}

func (WikiExtensionSetItem) TableName() string {
	return "wiki_extension_set_items"
}
