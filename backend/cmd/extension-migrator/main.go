package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"wikikeeper-backend/internal/database"
	applogger "wikikeeper-backend/internal/logger"
	"wikikeeper-backend/internal/repository"
)

func main() {
	time.Local = time.UTC

	batchSize := flag.Int("batch-size", 1000, "snapshots migrated per transaction")
	pause := flag.Duration("pause", 0, "pause between batches")
	finalize := flag.Bool("finalize", false, "rebuild statistics and truncate legacy items after backfill")
	flag.Parse()

	applogger.Init("INFO")
	db, err := database.Connect()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer database.Close()
	if err := database.RunMigrations(db); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	repo := repository.NewExtensionsRepository(db)
	var migrated, migratedItems int64
	for {
		result, err := repo.BackfillExtensionSets(ctx, *batchSize)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		if result.Snapshots == 0 && result.Items == 0 {
			break
		}
		migrated += int64(result.Snapshots)
		migratedItems += int64(result.Items)
		fmt.Printf("migrated_snapshots=%d migrated_items=%d\n", migrated, migratedItems)
		if *pause > 0 {
			select {
			case <-ctx.Done():
				fmt.Fprintln(os.Stderr, context.Cause(ctx))
				os.Exit(1)
			case <-time.After(*pause):
			}
		}
	}

	remaining, err := repo.RemainingLegacyExtensionSnapshots(ctx)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("backfill_complete=true remaining=%d migrated_snapshots=%d migrated_items=%d\n", remaining, migrated, migratedItems)
	if *finalize {
		legacyWrites, err := repo.LegacyExtensionWritesEnabled(ctx)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		if !legacyWrites {
			fmt.Println("already_finalized=true")
			return
		}
		if err := repo.FinalizeExtensionSetMigration(ctx); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Println("finalized=true")
	}
}
