package services

import (
	"reflect"
	"slices"
	"strings"

	applogger "wikikeeper-backend/internal/logger"
	"wikikeeper-backend/internal/models"
)

var extensionsDiffLog = applogger.With("component", "extensions_diff")

// ExtensionsDiff represents the difference between two extensions snapshots
type ExtensionsDiff struct {
	HasChanges bool
	Added      []models.WikiExtensionItem
	Removed    []models.WikiExtensionItem
	Modified   []ExtensionChange
}

// ExtensionChange represents a modified extension
type ExtensionChange struct {
	Name           string
	Type           string
	Old            *models.WikiExtensionItem
	New            *models.WikiExtensionItem
	VersionChanged bool
	URLChanged     bool
	LicenseChanged bool
}

// CompareExtensions compares old and new extensions data and returns the diff
func CompareExtensions(old *SiteInfoExtensions, new *SiteInfoExtensions) *ExtensionsDiff {
	return compareExtensionItems(flattenExtensions(old), flattenExtensions(new))
}

// CompareExtensionSnapshots compares two stored extension snapshots.
func CompareExtensionSnapshots(old, new *models.WikiExtensionsSnapshot) *ExtensionsDiff {
	var oldItems, newItems []models.WikiExtensionItem
	if old != nil {
		oldItems = old.Items
	}
	if new != nil {
		newItems = new.Items
	}
	return compareExtensionItems(oldItems, newItems)
}

func compareExtensionItems(oldItems, newItems []models.WikiExtensionItem) *ExtensionsDiff {
	diff := &ExtensionsDiff{
		Added:    []models.WikiExtensionItem{},
		Removed:  []models.WikiExtensionItem{},
		Modified: []ExtensionChange{},
	}

	// Create maps for comparison
	oldMap := make(map[string]*models.WikiExtensionItem)
	for i := range oldItems {
		key := getExtensionKey(&oldItems[i])
		oldMap[key] = &oldItems[i]
	}

	newMap := make(map[string]*models.WikiExtensionItem)
	for i := range newItems {
		key := getExtensionKey(&newItems[i])
		newMap[key] = &newItems[i]
	}

	// Detect additions and modifications
	for key, newItem := range newMap {
		if oldItem, exists := oldMap[key]; exists {
			// Extension exists in both, check if modified
			if !extensionEqual(oldItem, newItem) {
				diff.Modified = append(diff.Modified, ExtensionChange{
					Name:           newItem.Name,
					Type:           newItem.ExtType,
					Old:            oldItem,
					New:            newItem,
					VersionChanged: !reflect.DeepEqual(oldItem.Version, newItem.Version),
					URLChanged:     !reflect.DeepEqual(oldItem.URL, newItem.URL),
					LicenseChanged: !reflect.DeepEqual(oldItem.LicenseName, newItem.LicenseName),
				})
				diff.HasChanges = true
			}
		} else {
			// New extension added
			diff.Added = append(diff.Added, *newItem)
			diff.HasChanges = true
		}
	}

	// Detect removals
	for key, oldItem := range oldMap {
		if _, exists := newMap[key]; !exists {
			diff.Removed = append(diff.Removed, *oldItem)
			diff.HasChanges = true
		}
	}

	slices.SortFunc(diff.Added, compareExtensionItemsByTypeAndName)
	slices.SortFunc(diff.Removed, compareExtensionItemsByTypeAndName)
	slices.SortFunc(diff.Modified, func(a, b ExtensionChange) int {
		if a.Type != b.Type {
			return strings.Compare(a.Type, b.Type)
		}
		return strings.Compare(a.Name, b.Name)
	})

	return diff
}

func compareExtensionItemsByTypeAndName(a, b models.WikiExtensionItem) int {
	if a.ExtType != b.ExtType {
		return strings.Compare(a.ExtType, b.ExtType)
	}
	return strings.Compare(a.Name, b.Name)
}

// flattenExtensions converts SiteInfoExtensions to a WikiExtensionItem list
// Deduplicates items by (ext_type, name) in case MediaWiki API returns duplicates
func flattenExtensions(ext *SiteInfoExtensions) []models.WikiExtensionItem {
	if ext == nil {
		return []models.WikiExtensionItem{}
	}

	items := make([]models.WikiExtensionItem, 0, len(ext.Extensions)+len(ext.Skins))
	seen := make(map[string]bool) // Track seen (ext_type:name) to avoid duplicates
	duplicates := 0

	// Add extensions
	for _, e := range ext.Extensions {
		key := e.Type + ":" + e.Name
		if !seen[key] {
			seen[key] = true
			items = append(items, models.WikiExtensionItem{
				ExtType:     e.Type,
				Name:        e.Name,
				URL:         e.URL,
				Version:     e.Version,
				LicenseName: e.LicenseName,
			})
		} else {
			duplicates++
		}
	}

	// Add skins
	for _, s := range ext.Skins {
		key := s.Type + ":" + s.Name
		if !seen[key] {
			seen[key] = true
			items = append(items, models.WikiExtensionItem{
				ExtType:     s.Type,
				Name:        s.Name,
				URL:         s.URL,
				Version:     s.Version,
				LicenseName: s.LicenseName,
			})
		} else {
			duplicates++
		}
	}

	if duplicates > 0 {
		extensionsDiffLog.Info("Removed duplicate extensions", "duplicates", duplicates, "unique_items", len(items))
	}

	return items
}

// getExtensionKey generates a unique key for an extension
func getExtensionKey(item *models.WikiExtensionItem) string {
	return item.ExtType + ":" + item.Name
}

// extensionEqual compares two extensions for equality
func extensionEqual(a, b *models.WikiExtensionItem) bool {
	// Compare URL, Version, and LicenseName using deep equality
	return reflect.DeepEqual(a.URL, b.URL) &&
		reflect.DeepEqual(a.Version, b.Version) &&
		reflect.DeepEqual(a.LicenseName, b.LicenseName)
}

// CompareExtensionsFromSnapshot compares extensions from a snapshot with new extensions
func CompareExtensionsFromSnapshot(oldSnapshot *models.WikiExtensionsSnapshot, new *SiteInfoExtensions) *ExtensionsDiff {
	if oldSnapshot == nil {
		// No old snapshot, all new items are considered added
		diff := &ExtensionsDiff{
			HasChanges: true,
			Added:      flattenExtensions(new),
			Removed:    []models.WikiExtensionItem{},
			Modified:   []ExtensionChange{},
		}
		return diff
	}

	// Convert snapshot items to SiteInfoExtensions for comparison
	oldExtensions := &SiteInfoExtensions{
		Extensions: []ExtensionInfo{},
		Skins:      []ExtensionInfo{},
	}

	for _, item := range oldSnapshot.Items {
		info := ExtensionInfo{
			Type:        item.ExtType,
			Name:        item.Name,
			URL:         item.URL,
			Version:     item.Version,
			LicenseName: item.LicenseName,
		}

		if item.ExtType == "skin" {
			oldExtensions.Skins = append(oldExtensions.Skins, info)
		} else {
			oldExtensions.Extensions = append(oldExtensions.Extensions, info)
		}
	}

	return CompareExtensions(oldExtensions, new)
}

func extensionSnapshotVersionChanged(snapshot *models.WikiExtensionsSnapshot, version string) bool {
	if snapshot == nil {
		return false
	}
	return snapshot.MediaWikiVersion == nil || *snapshot.MediaWikiVersion != version
}

func extensionSnapshotNeedsUpdate(snapshot *models.WikiExtensionsSnapshot, version string, diff *ExtensionsDiff) bool {
	return snapshot == nil || diff.HasChanges || extensionSnapshotVersionChanged(snapshot, version)
}
