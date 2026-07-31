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
