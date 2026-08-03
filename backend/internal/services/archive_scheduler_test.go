package services

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"wikikeeper-backend/internal/config"
)

func TestArchiveCheckInterval(t *testing.T) {
	require.Equal(t, 30*24*time.Hour, archiveCheckInterval)
}

func TestArchiveSchedulerDisabledReturnsOnCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	scheduler := NewArchiveScheduler(nil, nil, &config.Config{ArchiveCheckBatchSize: 0})
	scheduler.Start(ctx)

	done := make(chan struct{})
	go func() {
		scheduler.Stop()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("disabled archive scheduler did not stop after cancellation")
	}
	require.False(t, scheduler.running)
}
