package repository

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"wikikeeper-backend/internal/models"
)

func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	// Create tables manually (SQLite doesn't support PostgreSQL's gen_random_uuid())
	db.Exec(`
		CREATE TABLE wikis (
			id TEXT PRIMARY KEY,
			url TEXT NOT NULL UNIQUE,
			api_url TEXT,
			index_url TEXT,
			wiki_name TEXT,
			sitename TEXT,
			lang TEXT,
			db_type TEXT,
			db_version TEXT,
			media_wiki_version TEXT,
			max_page_id INTEGER,
			status TEXT NOT NULL DEFAULT 'pending',
			has_archive INTEGER NOT NULL DEFAULT 0,
			api_available INTEGER NOT NULL DEFAULT 1,
			last_error TEXT,
			last_error_at DATETIME,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			last_check_at DATETIME,
			is_active INTEGER NOT NULL DEFAULT 1
		)
	`)

	db.Exec(`
		CREATE TABLE wiki_stats (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			wiki_id TEXT NOT NULL,
			time DATETIME NOT NULL,
			pages INTEGER NOT NULL DEFAULT 0,
			articles INTEGER NOT NULL DEFAULT 0,
			edits INTEGER NOT NULL DEFAULT 0,
			images INTEGER NOT NULL DEFAULT 0,
			users INTEGER NOT NULL DEFAULT 0,
			active_users INTEGER NOT NULL DEFAULT 0,
			admins INTEGER NOT NULL DEFAULT 0,
			jobs INTEGER NOT NULL DEFAULT 0,
			response_time_ms INTEGER,
			http_status INTEGER,
			FOREIGN KEY (wiki_id) REFERENCES wikis(id) ON DELETE CASCADE
		)
	`)

	db.Exec(`
		CREATE TABLE wiki_archives (
			id TEXT PRIMARY KEY,
			wiki_id TEXT NOT NULL,
			ia_identifier TEXT NOT NULL,
			added_date DATETIME,
			dump_date DATETIME,
			item_size INTEGER,
			uploader TEXT,
			scanner TEXT,
			upload_state TEXT,
			has_xml_current INTEGER NOT NULL DEFAULT 0,
			has_xml_history INTEGER NOT NULL DEFAULT 0,
			has_images_dump INTEGER NOT NULL DEFAULT 0,
			has_titles_list INTEGER NOT NULL DEFAULT 0,
			has_images_list INTEGER NOT NULL DEFAULT 0,
			has_legacy_wikidump INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(wiki_id, ia_identifier),
			FOREIGN KEY (wiki_id) REFERENCES wikis(id) ON DELETE CASCADE
		)
	`)

	return db
}

func TestWikiRepository_Create(t *testing.T) {
	db := setupTestDB(t)
	repo := NewWikiRepository(db)
	ctx := context.Background()

	wikiName := "Test Wiki"
	sitename := "Test Site"
	wiki := &models.Wiki{
		ID:              uuid.New(),
		URL:             "https://example.com",
		WikiName:        &wikiName,
		Sitename:        &sitename,
		Status:          models.WikiStatusPending,
		HasArchive:      false,
		APIAvailable:    true,
	}

	err := repo.Create(ctx, wiki)
	require.NoError(t, err)
	assert.NotEqual(t, uuid.UUID{}, wiki.ID)
}

func TestWikiRepository_GetByID(t *testing.T) {
	db := setupTestDB(t)
	repo := NewWikiRepository(db)
	ctx := context.Background()

	// Create test wiki
	wikiName := "Test Wiki"
	sitename := "Test Site"
	wiki := &models.Wiki{
		ID:       uuid.New(),
		URL:      "https://example.com",
		WikiName: &wikiName,
		Sitename: &sitename,
		Status:   models.WikiStatusOK,
	}
	require.NoError(t, repo.Create(ctx, wiki))

	// Get by ID
	found, err := repo.GetByID(ctx, wiki.ID)
	require.NoError(t, err)
	assert.Equal(t, wiki.ID, found.ID)
	assert.Equal(t, wiki.URL, found.URL)
	assert.Equal(t, "Test Wiki", *found.WikiName)
}

func TestWikiRepository_GetByID_NotFound(t *testing.T) {
	db := setupTestDB(t)
	repo := NewWikiRepository(db)
	ctx := context.Background()

	found, err := repo.GetByID(ctx, uuid.New())
	assert.Error(t, err)
	assert.Nil(t, found)
}

func TestWikiRepository_GetByURL(t *testing.T) {
	db := setupTestDB(t)
	repo := NewWikiRepository(db)
	ctx := context.Background()

	wiki := &models.Wiki{
		ID: uuid.New(),
		URL:    "https://example.com",
		Status: models.WikiStatusOK,
	}
	require.NoError(t, repo.Create(ctx, wiki))

	found, err := repo.GetByURL(ctx, "https://example.com")
	require.NoError(t, err)
	assert.Equal(t, wiki.URL, found.URL)
}

func TestWikiRepository_GetByAPIURL(t *testing.T) {
	db := setupTestDB(t)
	repo := NewWikiRepository(db)
	ctx := context.Background()

	apiURL := "https://example.com/api.php"
	wiki := &models.Wiki{
		ID: uuid.New(),
		URL:     "https://example.com",
		APIURL:  &apiURL,
		Status:  models.WikiStatusOK,
	}
	require.NoError(t, repo.Create(ctx, wiki))

	found, err := repo.GetByAPIURL(ctx, apiURL)
	require.NoError(t, err)
	assert.Equal(t, apiURL, *found.APIURL)
}

func TestWikiRepository_List(t *testing.T) {
	db := setupTestDB(t)
	repo := NewWikiRepository(db)
	ctx := context.Background()

	// Create test wikis
	for i := 1; i <= 15; i++ {
		sitename := fmt.Sprintf("Wiki %d", i)
		wiki := &models.Wiki{
		ID: uuid.New(),
			URL:      fmt.Sprintf("https://wiki%d.com", i),
			Sitename: &sitename,
			Status:   models.WikiStatusOK,
		}
		require.NoError(t, repo.Create(ctx, wiki))
	}

	// Test pagination
	wikis, total, err := repo.List(ctx, ListOptions{
		Page:     1,
		PageSize: 10,
	})
	require.NoError(t, err)
	assert.Equal(t, int64(15), total)
	assert.Len(t, wikis, 10)

	// Test second page
	wikis, total, err = repo.List(ctx, ListOptions{
		Page:     2,
		PageSize: 10,
	})
	require.NoError(t, err)
	assert.Equal(t, int64(15), total)
	assert.Len(t, wikis, 5)
}

func TestWikiRepository_List_FilterByStatus(t *testing.T) {
	db := setupTestDB(t)
	repo := NewWikiRepository(db)
	ctx := context.Background()

	// Create wikis with different statuses
	sitename1 := "OK Wiki"
	sitename2 := "Error Wiki"
	repo.Create(ctx, &models.Wiki{URL: "https://ok.com", Sitename: &sitename1, Status: models.WikiStatusOK})
	repo.Create(ctx, &models.Wiki{URL: "https://error.com", Sitename: &sitename2, Status: models.WikiStatusError})

	status := models.WikiStatusOK
	wikis, total, err := repo.List(ctx, ListOptions{
		Status: &status,
	})
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Len(t, wikis, 1)
	assert.Equal(t, models.WikiStatusOK, wikis[0].Status)
}

func TestWikiRepository_List_FilterByHasArchive(t *testing.T) {
	db := setupTestDB(t)
	repo := NewWikiRepository(db)
	ctx := context.Background()

	// Create wikis
	repo.Create(ctx, &models.Wiki{URL: "https://has-archive.com", Status: models.WikiStatusOK, HasArchive: true})
	repo.Create(ctx, &models.Wiki{URL: "https://no-archive.com", Status: models.WikiStatusOK, HasArchive: false})

	hasArchive := true
	wikis, total, err := repo.List(ctx, ListOptions{
		HasArchive: &hasArchive,
	})
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.True(t, wikis[0].HasArchive)
}

func TestWikiRepository_List_Search(t *testing.T) {
	t.Skip("ILIKE is PostgreSQL-specific, not supported in SQLite test database")

	db := setupTestDB(t)
	repo := NewWikiRepository(db)
	ctx := context.Background()

	// Create wikis
	sitename1 := "English Wikipedia"
	sitename2 := "French Wikipedia"
	sitename3 := "WikiFur"
	repo.Create(ctx, &models.Wiki{URL: "https://en.com", Sitename: &sitename1, Status: models.WikiStatusOK})
	repo.Create(ctx, &models.Wiki{URL: "https://fr.com", Sitename: &sitename2, Status: models.WikiStatusOK})
	repo.Create(ctx, &models.Wiki{URL: "https://fur.com", Sitename: &sitename3, Status: models.WikiStatusOK})

	wikis, total, err := repo.List(ctx, ListOptions{
		Search: "Wikipedia",
	})
	require.NoError(t, err)
	assert.Equal(t, int64(2), total)
	assert.Len(t, wikis, 2)
}

func TestWikiRepository_Update(t *testing.T) {
	db := setupTestDB(t)
	repo := NewWikiRepository(db)
	ctx := context.Background()

	wiki := &models.Wiki{
		ID: uuid.New(),
		URL:    "https://example.com",
		Status: models.WikiStatusPending,
	}
	require.NoError(t, repo.Create(ctx, wiki))

	// Update status
	wiki.Status = models.WikiStatusOK
	err := repo.Update(ctx, wiki)
	require.NoError(t, err)

	// Verify update
	found, err := repo.GetByID(ctx, wiki.ID)
	require.NoError(t, err)
	assert.Equal(t, models.WikiStatusOK, found.Status)
}

func TestWikiRepository_Delete(t *testing.T) {
	db := setupTestDB(t)
	repo := NewWikiRepository(db)
	ctx := context.Background()

	wiki := &models.Wiki{
		ID: uuid.New(),
		URL:    "https://example.com",
		Status: models.WikiStatusOK,
	}
	require.NoError(t, repo.Create(ctx, wiki))

	// Delete
	err := repo.Delete(ctx, wiki.ID)
	require.NoError(t, err)

	// Verify deletion
	_, err = repo.GetByID(ctx, wiki.ID)
	assert.Error(t, err)
}

func TestWikiRepository_ExistsByURL(t *testing.T) {
	db := setupTestDB(t)
	repo := NewWikiRepository(db)
	ctx := context.Background()

	repo.Create(ctx, &models.Wiki{URL: "https://example.com", Status: models.WikiStatusOK})

	exists, err := repo.ExistsByURL(ctx, "https://example.com")
	require.NoError(t, err)
	assert.True(t, exists)

	exists, err = repo.ExistsByURL(ctx, "https://notfound.com")
	require.NoError(t, err)
	assert.False(t, exists)
}

func TestWikiRepository_GetPendingForUpdate(t *testing.T) {
	db := setupTestDB(t)
	repo := NewWikiRepository(db)
	ctx := context.Background()

	// Create wikis with different last_check_at times
	now := time.Now()
	oldTime := now.Add(-24 * time.Hour)

	sitename := "Wiki1"
	repo.Create(ctx, &models.Wiki{
		URL:         "https://wiki1.com",
		Sitename:    &sitename,
		LastCheckAt: &oldTime,
		IsActive:    true,
	})

	sitename2 := "Wiki2"
	repo.Create(ctx, &models.Wiki{
		URL:         "https://wiki2.com",
		Sitename:    &sitename2,
		LastCheckAt: &now,
		IsActive:    true,
	})

	sitename3 := "Wiki3"
	repo.Create(ctx, &models.Wiki{
		URL:      "https://wiki3.com",
		Sitename: &sitename3,
		IsActive: false,
	})

	wikis, err := repo.GetPendingForUpdate(ctx, 10)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(wikis), 1)
}
