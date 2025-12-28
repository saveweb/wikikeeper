package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"wikikeeper-backend/internal/config"
	applogger "wikikeeper-backend/internal/logger"
	"wikikeeper-backend/internal/models"
)

// MongoDB document structures (for reading only)
type MongoWiki struct {
	ID               primitive.ObjectID `bson:"_id"`
	URL              string             `bson:"url"`
	APIURL           *string            `bson:"api_url"`
	IndexURL         *string            `bson:"index_url"`
	WikiName         *string            `bson:"wiki_name"`
	Sitename         *string            `bson:"sitename"`
	Lang             *string            `bson:"lang"`
	DBType           *string            `bson:"dbtype"`
	DBVersion        *string            `bson:"dbversion"`
	MediaWikiVersion *string            `bson:"mediawiki_version"`
	MaxPageID        *int               `bson:"max_page_id"`
	Status           string             `bson:"status"`
	HasArchive       bool               `bson:"has_archive"`
	APIAvailable     bool               `bson:"api_available"`
	LastError        *string            `bson:"last_error"`
	LastErrorAt      *time.Time         `bson:"last_error_at"`
	CreatedAt        time.Time          `bson:"created_at"`
	UpdatedAt        time.Time          `bson:"updated_at"`
	LastCheckAt      *time.Time         `bson:"last_check_at"`
	IsActive         bool               `bson:"is_active"`
}

type MongoWikiStats struct {
	ID             primitive.ObjectID `bson:"_id"`
	WikiID         string             `bson:"wiki_id"`
	Time           time.Time          `bson:"time"`
	Pages          int                `bson:"pages"`
	Articles       int                `bson:"articles"`
	Edits          int                `bson:"edits"`
	Images         int                `bson:"images"`
	Users          int                `bson:"users"`
	ActiveUsers    int                `bson:"active_users"`
	Admins         int                `bson:"admins"`
	Jobs           int                `bson:"jobs"`
	ResponseTimeMs *int               `bson:"response_time_ms"`
	HTTPStatus     *int               `bson:"http_status"`
}

type MongoWikiArchive struct {
	ID                primitive.ObjectID `bson:"_id"`
	WikiID            string             `bson:"wiki_id"`
	IAIdentifier      string             `bson:"ia_identifier"`
	AddedDate         *time.Time         `bson:"added_date"`
	DumpDate          *time.Time         `bson:"dump_date"`
	ItemSize          *int64             `bson:"item_size"`
	Uploader          *string            `bson:"uploader"`
	Scanner           *string            `bson:"scanner"`
	UploadState       *string            `bson:"upload_state"`
	HasXMLCurrent     bool               `bson:"has_xml_current"`
	HasXMLHistory     bool               `bson:"has_xml_history"`
	HasImagesDump     bool               `bson:"has_images_dump"`
	HasTitlesList     bool               `bson:"has_titles_list"`
	HasImagesList     bool               `bson:"has_images_list"`
	HasLegacyWikidump bool               `bson:"has_legacy_wikidump"`
	CreatedAt         time.Time          `bson:"created_at"`
	UpdatedAt         time.Time          `bson:"updatedAt_at"`
}

var (
	mongoClient *mongo.Client
	gormDB      *gorm.DB
	batchSize   = 100
	// Mapping from MongoDB ObjectId Hex to PostgreSQL UUID
	idMapping = make(map[string]uuid.UUID)
)

func main() {
	// Load .env
	if err := godotenv.Load(); err != nil {
		applogger.Log.Info("Warning: .env file not found: %v", err)
	}

	cfg := config.Get()

	fmt.Println("=== WikiKeeper Data Migration ===")
	fmt.Println("MongoDB → PostgreSQL")
	fmt.Printf("MongoDB: %s @ %s\n", cfg.MongoDBDBName, cfg.MongoDBURI)
	fmt.Printf("PostgreSQL: %s\n", cfg.DatabaseURL)
	fmt.Println()

	// Connect to MongoDB
	ctx := context.Background()
	var err error
	mongoClient, err = mongo.Connect(ctx, options.Client().ApplyURI(cfg.MongoDBURI))
	if err != nil {
		log.Fatalf("Failed to connect to MongoDB: %v", err)
	}
	defer mongoClient.Disconnect(ctx)

	// Ping MongoDB
	if err := mongoClient.Ping(ctx, nil); err != nil {
		log.Fatalf("Failed to ping MongoDB: %v", err)
	}
	fmt.Println("✓ Connected to MongoDB")

	// Connect to PostgreSQL
	gormDB, err = gorm.Open(postgres.Open(cfg.DatabaseURL), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})
	if err != nil {
		log.Fatalf("Failed to connect to PostgreSQL: %v", err)
	}

	// Ping PostgreSQL
	sqlDB, _ := gormDB.DB()
	if err := sqlDB.Ping(); err != nil {
		log.Fatalf("Failed to ping PostgreSQL: %v", err)
	}
	fmt.Println("✓ Connected to PostgreSQL")
	fmt.Println()

	// Check if PostgreSQL schema exists
	if !gormDB.Migrator().HasTable(&models.Wiki{}) {
		log.Fatal("PostgreSQL schema not found. Please run migrations first: make migrate-up")
	}

	// Ask user for confirmation
	fmt.Print("This will migrate data from MongoDB to PostgreSQL. Continue? (y/N): ")
	var confirm string
	fmt.Scanln(&confirm)
	if confirm != "y" && confirm != "Y" {
		fmt.Println("Migration cancelled.")
		os.Exit(0)
	}
	fmt.Println()

	// Start migration
	startTime := time.Now()

	// Step 1: Migrate wikis
	fmt.Println("=== Step 1: Migrating Wikis ===")
	if err := migrateWikis(ctx, cfg.MongoDBDBName); err != nil {
		log.Fatalf("Failed to migrate wikis: %v", err)
	}

	// Step 2: Migrate wiki_stats
	fmt.Println("\n=== Step 2: Migrating Wiki Stats ===")
	if err := migrateWikiStats(ctx, cfg.MongoDBDBName); err != nil {
		log.Fatalf("Failed to migrate wiki stats: %v", err)
	}

	// Step 3: Migrate wiki_archives
	fmt.Println("\n=== Step 3: Migrating Wiki Archives ===")
	if err := migrateWikiArchives(ctx, cfg.MongoDBDBName); err != nil {
		log.Fatalf("Failed to migrate wiki archives: %v", err)
	}

	elapsed := time.Since(startTime)
	fmt.Printf("\n=== Migration Complete ===\n")
	fmt.Printf("Total time: %s\n", elapsed.Round(time.Second))
}

func migrateWikis(ctx context.Context, dbName string) error {
	collection := mongoClient.Database(dbName).Collection("wikis")

	// Get total count
	total, err := collection.CountDocuments(ctx, bson.M{})
	if err != nil {
		return err
	}
	fmt.Printf("Total wikis in MongoDB: %d\n", total)

	if total == 0 {
		fmt.Println("No wikis to migrate.")
		return nil
	}

	// Check existing in PostgreSQL
	var existingCount int64
	gormDB.Model(&models.Wiki{}).Count(&existingCount)
	if existingCount > 0 {
		fmt.Printf("PostgreSQL already has %d wikis. Skipping migration.\n", existingCount)
		return nil
	}

	// Fetch all wikis with cursor
	cursor, err := collection.Find(ctx, bson.M{})
	if err != nil {
		return err
	}
	defer cursor.Close(ctx)

	// Batch processing
	batch := make([]*models.Wiki, 0, batchSize)
	count := 0
	migrated := 0
	skipped := 0

	for cursor.Next(ctx) {
		var mongoWiki MongoWiki
		if err := cursor.Decode(&mongoWiki); err != nil {
			applogger.Log.Info("Error decoding wiki: %v", err)
			continue
		}

		// Generate new UUID for this wiki
		wikiID := uuid.New()

		// Store mapping from old MongoDB ID to new UUID
		idMapping[mongoWiki.ID.Hex()] = wikiID

		// Convert to GORM model
		wiki := &models.Wiki{
			ID:               wikiID,
			URL:              mongoWiki.URL,
			APIURL:           mongoWiki.APIURL,
			IndexURL:         mongoWiki.IndexURL,
			WikiName:         mongoWiki.WikiName,
			Sitename:         mongoWiki.Sitename,
			Lang:             mongoWiki.Lang,
			DBType:           mongoWiki.DBType,
			DBVersion:        mongoWiki.DBVersion,
			MediaWikiVersion: mongoWiki.MediaWikiVersion,
			MaxPageID:        mongoWiki.MaxPageID,
			Status:           models.WikiStatus(mongoWiki.Status),
			HasArchive:       mongoWiki.HasArchive,
			APIAvailable:     mongoWiki.APIAvailable,
			LastError:        mongoWiki.LastError,
			LastErrorAt:      mongoWiki.LastErrorAt,
			CreatedAt:        mongoWiki.CreatedAt,
			UpdatedAt:        mongoWiki.UpdatedAt,
			LastCheckAt:      mongoWiki.LastCheckAt,
			IsActive:         mongoWiki.IsActive,
		}

		batch = append(batch, wiki)
		count++

		// Batch insert
		if len(batch) >= batchSize {
			inserted, err := insertWikiBatch(batch)
			if err != nil {
				applogger.Log.Info("Error inserting batch: %v", err)
			} else {
				migrated += inserted
			}
			fmt.Printf("Progress: %d/%d migrated, %d skipped\n", count, total, skipped)
			batch = batch[:0] // Clear batch
		}
	}

	// Insert remaining
	if len(batch) > 0 {
		inserted, err := insertWikiBatch(batch)
		if err != nil {
			applogger.Log.Info("Error inserting final batch: %v", err)
		} else {
			migrated += inserted
		}
	}

	if err := cursor.Err(); err != nil {
		return err
	}

	fmt.Printf("✓ Migrated %d wikis (skipped %d)\n", migrated, skipped)
	return nil
}

func insertWikiBatch(batch []*models.Wiki) (int, error) {
	if err := gormDB.Create(&batch).Error; err != nil {
		return 0, err
	}
	return len(batch), nil
}

func migrateWikiStats(ctx context.Context, dbName string) error {
	collection := mongoClient.Database(dbName).Collection("wiki_stats")

	// Get total count
	total, err := collection.CountDocuments(ctx, bson.M{})
	if err != nil {
		return err
	}
	fmt.Printf("Total wiki_stats in MongoDB: %d\n", total)

	if total == 0 {
		fmt.Println("No wiki stats to migrate.")
		return nil
	}

	// Check existing in PostgreSQL
	var existingCount int64
	gormDB.Model(&models.WikiStats{}).Count(&existingCount)
	if existingCount > 0 {
		fmt.Printf("PostgreSQL already has %d stats. Skipping migration.\n", existingCount)
		return nil
	}

	cursor, err := collection.Find(ctx, bson.M{})
	if err != nil {
		return err
	}
	defer cursor.Close(ctx)

	batch := make([]*models.WikiStats, 0, batchSize)
	count := 0
	migrated := 0

	for cursor.Next(ctx) {
		var mongoStats MongoWikiStats
		if err := cursor.Decode(&mongoStats); err != nil {
			applogger.Log.Info("Error decoding wiki stats: %v", err)
			continue
		}

		// Look up new UUID from mapping
		wikiID, ok := idMapping[mongoStats.WikiID]
		if !ok {
			applogger.Log.Info("Warning: wiki_id %s not found in mapping, skipping stats", mongoStats.WikiID)
			continue
		}

		stats := &models.WikiStats{
			WikiID:         wikiID,
			Time:           mongoStats.Time,
			Pages:          mongoStats.Pages,
			Articles:       mongoStats.Articles,
			Edits:          mongoStats.Edits,
			Images:         mongoStats.Images,
			Users:          mongoStats.Users,
			ActiveUsers:    mongoStats.ActiveUsers,
			Admins:         mongoStats.Admins,
			Jobs:           mongoStats.Jobs,
			ResponseTimeMs: mongoStats.ResponseTimeMs,
			HTTPStatus:     mongoStats.HTTPStatus,
		}

		batch = append(batch, stats)
		count++

		if len(batch) >= batchSize {
			inserted, err := insertStatsBatch(batch)
			if err != nil {
				applogger.Log.Info("Error inserting batch: %v", err)
			} else {
				migrated += inserted
			}
			fmt.Printf("Progress: %d/%d\n", count, total)
			batch = batch[:0]
		}
	}

	if len(batch) > 0 {
		inserted, err := insertStatsBatch(batch)
		if err != nil {
			applogger.Log.Info("Error inserting final batch: %v", err)
		} else {
			migrated += inserted
		}
	}

	fmt.Printf("✓ Migrated %d wiki stats\n", migrated)
	return cursor.Err()
}

func insertStatsBatch(batch []*models.WikiStats) (int, error) {
	if err := gormDB.Create(&batch).Error; err != nil {
		return 0, err
	}
	return len(batch), nil
}

func migrateWikiArchives(ctx context.Context, dbName string) error {
	collection := mongoClient.Database(dbName).Collection("wiki_archives")

	// Get total count
	total, err := collection.CountDocuments(ctx, bson.M{})
	if err != nil {
		return err
	}
	fmt.Printf("Total wiki_archives in MongoDB: %d\n", total)

	if total == 0 {
		fmt.Println("No wiki archives to migrate.")
		return nil
	}

	// Check existing in PostgreSQL
	var existingCount int64
	gormDB.Model(&models.WikiArchive{}).Count(&existingCount)
	if existingCount > 0 {
		fmt.Printf("PostgreSQL already has %d archives. Skipping migration.\n", existingCount)
		return nil
	}

	cursor, err := collection.Find(ctx, bson.M{})
	if err != nil {
		return err
	}
	defer cursor.Close(ctx)

	batch := make([]*models.WikiArchive, 0, batchSize)
	count := 0
	migrated := 0

	for cursor.Next(ctx) {
		var mongoArchive MongoWikiArchive
		if err := cursor.Decode(&mongoArchive); err != nil {
			applogger.Log.Info("Error decoding wiki archive: %v", err)
			continue
		}

		// Look up new UUID from mapping
		wikiID, ok := idMapping[mongoArchive.WikiID]
		if !ok {
			applogger.Log.Info("Warning: wiki_id %s not found in mapping, skipping archive", mongoArchive.WikiID)
			continue
		}

		// Generate UUID for archive
		archiveID := uuid.New()

		archive := &models.WikiArchive{
			ID:                archiveID,
			WikiID:            wikiID,
			IAIdentifier:      mongoArchive.IAIdentifier,
			AddedDate:         mongoArchive.AddedDate,
			DumpDate:          mongoArchive.DumpDate,
			ItemSize:          mongoArchive.ItemSize,
			Uploader:          mongoArchive.Uploader,
			Scanner:           mongoArchive.Scanner,
			UploadState:       mongoArchive.UploadState,
			HasXMLCurrent:     mongoArchive.HasXMLCurrent,
			HasXMLHistory:     mongoArchive.HasXMLHistory,
			HasImagesDump:     mongoArchive.HasImagesDump,
			HasTitlesList:     mongoArchive.HasTitlesList,
			HasImagesList:     mongoArchive.HasImagesList,
			HasLegacyWikidump: mongoArchive.HasLegacyWikidump,
			CreatedAt:         mongoArchive.CreatedAt,
			UpdatedAt:         mongoArchive.UpdatedAt,
		}

		batch = append(batch, archive)
		count++

		if len(batch) >= batchSize {
			inserted, err := insertArchiveBatch(batch)
			if err != nil {
				applogger.Log.Info("Error inserting batch: %v", err)
			} else {
				migrated += inserted
			}
			fmt.Printf("Progress: %d/%d\n", count, total)
			batch = batch[:0]
		}
	}

	if len(batch) > 0 {
		inserted, err := insertArchiveBatch(batch)
		if err != nil {
			applogger.Log.Info("Error inserting final batch: %v", err)
		} else {
			migrated += inserted
		}
	}

	fmt.Printf("✓ Migrated %d wiki archives\n", migrated)
	return cursor.Err()
}

func insertArchiveBatch(batch []*models.WikiArchive) (int, error) {
	if err := gormDB.Create(&batch).Error; err != nil {
		return 0, err
	}
	return len(batch), nil
}
