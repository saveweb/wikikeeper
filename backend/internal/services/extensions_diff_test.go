package services

import (
	"testing"

	"github.com/stretchr/testify/require"

	"wikikeeper-backend/internal/models"
)

func TestExtensionSnapshotVersionChanged(t *testing.T) {
	oldVersion := "MediaWiki 1.45.1"
	snapshot := &models.WikiExtensionsSnapshot{MediaWikiVersion: &oldVersion}

	require.False(t, extensionSnapshotVersionChanged(nil, "MediaWiki 1.46.0"))
	require.False(t, extensionSnapshotVersionChanged(snapshot, oldVersion))
	require.True(t, extensionSnapshotVersionChanged(snapshot, "MediaWiki 1.46.0"))
	require.True(t, extensionSnapshotVersionChanged(&models.WikiExtensionsSnapshot{}, oldVersion))
}

func TestUnchangedExtensionsStillDetectMediaWikiUpgrade(t *testing.T) {
	oldVersion := "MediaWiki 1.45.1"
	extensionVersion := "1.0"
	snapshot := &models.WikiExtensionsSnapshot{
		MediaWikiVersion: &oldVersion,
		Items: []models.WikiExtensionItem{{
			ExtType: "parserhook",
			Name:    "Example",
			Version: &extensionVersion,
		}},
	}
	current := &SiteInfoExtensions{Extensions: []ExtensionInfo{{
		Type:    "parserhook",
		Name:    "Example",
		Version: &extensionVersion,
	}}}

	diff := CompareExtensionsFromSnapshot(snapshot, current)
	require.False(t, diff.HasChanges)
	require.False(t, extensionSnapshotNeedsUpdate(snapshot, oldVersion, diff))
	require.True(t, extensionSnapshotNeedsUpdate(snapshot, "MediaWiki 1.46.0", diff))
	require.True(t, extensionSnapshotNeedsUpdate(nil, "MediaWiki 1.46.0", diff))
}

func TestCompareExtensionSnapshots(t *testing.T) {
	oldVersion := "1.0"
	newVersion := "2.0"
	oldURL := "https://old.example"
	newURL := "https://new.example"
	old := &models.WikiExtensionsSnapshot{Items: []models.WikiExtensionItem{
		{ExtType: "extension", Name: "Removed", Version: &oldVersion},
		{ExtType: "extension", Name: "Changed", Version: &oldVersion, URL: &oldURL},
	}}
	new := &models.WikiExtensionsSnapshot{Items: []models.WikiExtensionItem{
		{ExtType: "skin", Name: "Added", Version: &newVersion},
		{ExtType: "extension", Name: "Changed", Version: &newVersion, URL: &newURL},
	}}

	diff := CompareExtensionSnapshots(old, new)
	require.True(t, diff.HasChanges)
	require.Equal(t, "Added", diff.Added[0].Name)
	require.Equal(t, "Removed", diff.Removed[0].Name)
	require.Equal(t, "Changed", diff.Modified[0].Name)
	require.Equal(t, &oldVersion, diff.Modified[0].Old.Version)
	require.Equal(t, &newVersion, diff.Modified[0].New.Version)
	require.True(t, diff.Modified[0].VersionChanged)
	require.True(t, diff.Modified[0].URLChanged)
	require.False(t, diff.Modified[0].LicenseChanged)
}
