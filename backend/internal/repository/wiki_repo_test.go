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
			collection_status TEXT NOT NULL DEFAULT 'pending',
			last_error TEXT,
			last_error_at DATETIME,
			last_success_at DATETIME,
			next_check_at DATETIME,
			consecutive_failures INTEGER NOT NULL DEFAULT 0,
			archive_last_check_at DATETIME,
			archive_last_error TEXT,
			archive_last_error_at DATETIME,
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
		ID:           uuid.New(),
		URL:          "https://example.com",
		WikiName:     &wikiName,
		Sitename:     &sitename,
		Status:       models.WikiStatusPending,
		HasArchive:   false,
		APIAvailable: true,
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
		ID:     uuid.New(),
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
		ID:     uuid.New(),
		URL:    "https://example.com",
		APIURL: &apiURL,
		Status: models.WikiStatusOK,
	}
	require.NoError(t, repo.Create(ctx, wiki))

	found, err := repo.GetByAPIURL(ctx, apiURL)
	require.NoError(t, err)
	assert.Equal(t, apiURL, *found.APIURL)
}

func TestWikiRepositoryArchiveUpdatesOnlyOwnedFields(t *testing.T) {
	db := setupTestDB(t)
	repo := NewWikiRepository(db)
	ctx := context.Background()
	updatedAt := time.Date(2026, time.July, 1, 12, 0, 0, 0, time.UTC)
	lastCheck := updatedAt.Add(-time.Hour)
	lastSuccess := updatedAt.Add(-24 * time.Hour)
	nextCheck := updatedAt.Add(21 * 24 * time.Hour)
	lastErrorAt := updatedAt.Add(-2 * time.Hour)
	lastError := "HTTP 429"
	archiveError := "old archive error"
	wiki := &models.Wiki{
		ID:                  uuid.New(),
		URL:                 "https://archive-fields.example",
		Status:              models.WikiStatusOK,
		CollectionStatus:    models.CollectionStatusRateLimited,
		LastError:           &lastError,
		LastErrorAt:         &lastErrorAt,
		LastSuccessAt:       &lastSuccess,
		NextCheckAt:         &nextCheck,
		ConsecutiveFailures: 4,
		ArchiveLastError:    &archiveError,
		ArchiveLastErrorAt:  &lastErrorAt,
		LastCheckAt:         &lastCheck,
		UpdatedAt:           updatedAt,
		IsActive:            true,
	}
	require.NoError(t, repo.Create(ctx, wiki))

	archiveCheck := updatedAt.Add(48 * time.Hour)
	require.NoError(t, repo.UpdateArchiveStatus(ctx, wiki.ID, true, archiveCheck))

	updated, err := repo.GetByID(ctx, wiki.ID)
	require.NoError(t, err)
	require.True(t, updated.HasArchive)
	require.Equal(t, archiveCheck, *updated.ArchiveLastCheckAt)
	require.Nil(t, updated.ArchiveLastError)
	require.Nil(t, updated.ArchiveLastErrorAt)
	require.Equal(t, models.CollectionStatusRateLimited, updated.CollectionStatus)
	require.Equal(t, lastError, *updated.LastError)
	require.Equal(t, lastErrorAt, *updated.LastErrorAt)
	require.Equal(t, lastSuccess, *updated.LastSuccessAt)
	require.Equal(t, nextCheck, *updated.NextCheckAt)
	require.Equal(t, 4, updated.ConsecutiveFailures)
	require.Equal(t, updatedAt, updated.UpdatedAt)

	archiveFailureAt := archiveCheck.Add(time.Hour)
	require.NoError(t, repo.UpdateArchiveError(ctx, wiki.ID, "archive unavailable", archiveFailureAt))
	updated, err = repo.GetByID(ctx, wiki.ID)
	require.NoError(t, err)
	require.Equal(t, archiveFailureAt, *updated.ArchiveLastCheckAt)
	require.Equal(t, "archive unavailable", *updated.ArchiveLastError)
	require.Equal(t, archiveFailureAt, *updated.ArchiveLastErrorAt)
	require.Equal(t, models.CollectionStatusRateLimited, updated.CollectionStatus)
	require.Equal(t, lastError, *updated.LastError)
	require.Equal(t, nextCheck, *updated.NextCheckAt)
	require.Equal(t, updatedAt, updated.UpdatedAt)
}

func TestWikiRepository_List(t *testing.T) {
	db := setupTestDB(t)
	repo := NewWikiRepository(db)
	ctx := context.Background()

	// Create test wikis
	for i := 1; i <= 15; i++ {
		sitename := fmt.Sprintf("Wiki %d", i)
		wiki := &models.Wiki{
			ID:       uuid.New(),
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

func TestParseWikiOrder(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    WikiOrder
		wantErr error
	}{
		{name: "updated", in: "updated_at DESC", want: WikiOrderUpdatedDesc},
		{name: "created", in: "created_at DESC", want: WikiOrderCreatedDesc},
		{name: "sitename", in: "sitename ASC", want: WikiOrderSitenameAsc},
		{name: "unchecked", in: "last_check_at ASC NULLS FIRST", want: WikiOrderLastCheckAscNulls},
		{name: "empty", want: WikiOrderUpdatedDesc},
		{name: "unknown", in: "updated_at DESC; DROP TABLE wikis", wantErr: ErrInvalidWikiOrder},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseWikiOrder(tt.in)
			assert.ErrorIs(t, err, tt.wantErr)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestWikiRepository_List_RejectsUnknownOrder(t *testing.T) {
	db := setupTestDB(t)
	repo := NewWikiRepository(db)
	ctx := context.Background()

	require.NoError(t, repo.Create(ctx, &models.Wiki{
		ID:     uuid.New(),
		URL:    "https://example.com",
		Status: models.WikiStatusOK,
	}))

	_, _, err := repo.List(ctx, ListOptions{
		OrderBy: WikiOrder("not_a_column DESC"),
	})
	assert.ErrorIs(t, err, ErrInvalidWikiOrder)
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
		ID:     uuid.New(),
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

func TestWikiRepository_GetDueForUpdate(t *testing.T) {
	db := setupTestDB(t)
	repo := NewWikiRepository(db)
	ctx := context.Background()
	now := time.Date(2026, time.July, 31, 5, 0, 0, 0, time.UTC)
	past := now.Add(-time.Minute)
	future := now.Add(time.Hour)

	wikis := []*models.Wiki{
		{ID: uuid.New(), URL: "https://never.example", Status: models.WikiStatusPending, CollectionStatus: models.CollectionStatusPending, IsActive: true},
		{ID: uuid.New(), URL: "https://due.example", Status: models.WikiStatusOK, CollectionStatus: models.CollectionStatusOK, NextCheckAt: &past, IsActive: true},
		{ID: uuid.New(), URL: "https://later.example", Status: models.WikiStatusOK, CollectionStatus: models.CollectionStatusOK, NextCheckAt: &future, IsActive: true},
		{ID: uuid.New(), URL: "https://inactive.example", Status: models.WikiStatusPending, CollectionStatus: models.CollectionStatusPending, IsActive: false},
	}
	for _, wiki := range wikis {
		require.NoError(t, repo.Create(ctx, wiki))
	}
	require.NoError(t, db.Model(&models.Wiki{}).
		Where("id = ?", wikis[3].ID).
		Update("is_active", false).Error)

	due, err := repo.GetDueForUpdate(ctx, 10, now)
	require.NoError(t, err)
	require.Len(t, due, 2)
	require.Equal(t, "https://never.example", due[0].URL)
	require.Equal(t, "https://due.example", due[1].URL)
}

func TestWikiRepository_GetDueForUpdateFair(t *testing.T) {
	db := setupTestDB(t)
	repo := NewWikiRepository(db)
	ctx := context.Background()
	now := time.Date(2026, time.August, 1, 5, 0, 0, 0, time.UTC)
	veryOld := now.Add(-72 * time.Hour)
	old := now.Add(-48 * time.Hour)
	lastSuccess := now.Add(-24 * time.Hour)

	wikis := []*models.Wiki{
		{ID: uuid.New(), URL: "https://never-oldest.example", Status: models.WikiStatusPending, CollectionStatus: models.CollectionStatusPending, NextCheckAt: &veryOld, IsActive: true},
		{ID: uuid.New(), URL: "https://never-second.example", Status: models.WikiStatusPending, CollectionStatus: models.CollectionStatusPending, NextCheckAt: &old, IsActive: true},
		{ID: uuid.New(), URL: "https://healthy.example", Status: models.WikiStatusOK, CollectionStatus: models.CollectionStatusOK, LastSuccessAt: &lastSuccess, NextCheckAt: &old, IsActive: true},
		{ID: uuid.New(), URL: "https://failed.example", Status: models.WikiStatusOK, CollectionStatus: models.CollectionStatusError, LastSuccessAt: &lastSuccess, NextCheckAt: &old, IsActive: true},
	}
	for _, wiki := range wikis {
		require.NoError(t, repo.Create(ctx, wiki))
	}

	due, err := repo.GetDueForUpdateFair(ctx, 3, now)
	require.NoError(t, err)
	require.Len(t, due, 3)
	require.Equal(t, "https://healthy.example", due[0].URL)
	require.Equal(t, "https://never-oldest.example", due[1].URL)
	require.Equal(t, "https://failed.example", due[2].URL)

	due, err = repo.GetDueForUpdateFair(ctx, 4, now)
	require.NoError(t, err)
	require.Len(t, due, 4)
	require.Equal(t, "https://never-second.example", due[3].URL)
}

func TestWikiRepository_GetDueForArchiveCheck(t *testing.T) {
	db := setupTestDB(t)
	repo := NewWikiRepository(db)
	ctx := context.Background()
	now := time.Date(2026, time.August, 1, 5, 0, 0, 0, time.UTC)
	dueBefore := now.Add(-3 * 24 * time.Hour)
	oldCheck := dueBefore.Add(-time.Hour)
	recentCheck := dueBefore.Add(time.Hour)
	apiURL := "https://example.org/api.php"
	emptyAPIURL := ""

	wikis := []*models.Wiki{
		{ID: uuid.New(), URL: "https://no-api.example", Status: models.WikiStatusError, IsActive: true},
		{ID: uuid.New(), URL: "https://empty-api.example", APIURL: &emptyAPIURL, Status: models.WikiStatusError, IsActive: true},
		{ID: uuid.New(), URL: "https://never-checked.example", APIURL: &apiURL, Status: models.WikiStatusOK, IsActive: true},
		{ID: uuid.New(), URL: "https://due.example", APIURL: &apiURL, Status: models.WikiStatusOK, ArchiveLastCheckAt: &oldCheck, IsActive: true},
		{ID: uuid.New(), URL: "https://recent.example", APIURL: &apiURL, Status: models.WikiStatusOK, ArchiveLastCheckAt: &recentCheck, IsActive: true},
		{ID: uuid.New(), URL: "https://inactive.example", APIURL: &apiURL, Status: models.WikiStatusOK, IsActive: false},
	}
	for _, wiki := range wikis {
		require.NoError(t, repo.Create(ctx, wiki))
	}
	require.NoError(t, db.Model(&models.Wiki{}).
		Where("id = ?", wikis[5].ID).
		Update("is_active", false).Error)

	due, err := repo.GetDueForArchiveCheck(ctx, 10, dueBefore)
	require.NoError(t, err)
	require.Len(t, due, 2)
	require.Equal(t, "https://never-checked.example", due[0].URL)
	require.Equal(t, "https://due.example", due[1].URL)

	limited, err := repo.GetDueForArchiveCheck(ctx, 1, dueBefore)
	require.NoError(t, err)
	require.Len(t, limited, 1)
	require.Equal(t, "https://never-checked.example", limited[0].URL)
}

func TestWikiRepository_DeferCollectionChecksOnlyChangesSchedule(t *testing.T) {
	db := setupTestDB(t)
	repo := NewWikiRepository(db)
	ctx := context.Background()
	lastError := "previous error"
	lastCheck := time.Date(2026, time.July, 30, 5, 0, 0, 0, time.UTC)
	nextCheck := lastCheck.Add(2 * time.Hour)
	wiki := &models.Wiki{
		ID:                  uuid.New(),
		URL:                 "https://deferred.example",
		Status:              models.WikiStatusOK,
		CollectionStatus:    models.CollectionStatusError,
		LastError:           &lastError,
		LastCheckAt:         &lastCheck,
		ConsecutiveFailures: 2,
		IsActive:            true,
	}
	require.NoError(t, repo.Create(ctx, wiki))

	require.NoError(t, repo.DeferCollectionChecks(ctx, []uuid.UUID{wiki.ID}, nextCheck))

	updated, err := repo.GetByID(ctx, wiki.ID)
	require.NoError(t, err)
	require.Equal(t, models.CollectionStatusError, updated.CollectionStatus)
	require.Equal(t, 2, updated.ConsecutiveFailures)
	require.Equal(t, lastError, *updated.LastError)
	require.Equal(t, lastCheck, *updated.LastCheckAt)
	require.Equal(t, nextCheck, *updated.NextCheckAt)
}

func TestWikiRepository_Delete(t *testing.T) {
	db := setupTestDB(t)
	repo := NewWikiRepository(db)
	ctx := context.Background()

	wiki := &models.Wiki{
		ID:     uuid.New(),
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

	exists, err = repo.ExistsByURL(ctx, "https://example.com/")
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
