package store

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestStore_Concurrency(t *testing.T) {
	dbPath := "test_concurrency.db"
	_ = os.Remove(dbPath)
	_ = os.Remove(dbPath + "-wal")
	_ = os.Remove(dbPath + "-shm")
	defer os.Remove(dbPath)
	defer os.Remove(dbPath + "-wal")
	defer os.Remove(dbPath + "-shm")

	s, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}

	ctx := context.Background()
	workers := 20
	reqsPerWorker := 50
	var wg sync.WaitGroup
	wg.Add(workers)

	now := time.Now().UTC()
	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < reqsPerWorker; j++ {
				_ = s.Write(ctx, Record{
					ID:             uuid.New(),
					Project:        "test-project",
					Model:          "gpt-4",
					CostMillicents: 100,
					Timestamp:      now,
				})
			}
		}()
	}
	wg.Wait()
	
	_ = s.Close()
	
	s2, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to reopen store: %v", err)
	}
	defer s2.Close()

	// 明确使用 UTC 日期
	dateStr := now.Format("2006-01-02")
	stats, err := s2.GetDailyStats(ctx, "test-project", dateStr)
	if err != nil {
		t.Fatalf("GetDailyStats failed: %v", err)
	}
	
	if stats.RequestCount != workers*reqsPerWorker {
		t.Errorf("Expected %d requests, got %d for date %s", workers*reqsPerWorker, stats.RequestCount, dateStr)
	}
}

func TestStore_Archive(t *testing.T) {
	dbPath := "test_archive.db"
	_ = os.Remove(dbPath)
	defer os.Remove(dbPath)
	
	s, _ := NewSQLiteStore(dbPath)
	ctx := context.Background()

	oldDate := time.Now().UTC().AddDate(0, 0, -10)
	for i := 0; i < 100; i++ {
		_ = s.Write(ctx, Record{
			ID:             uuid.New(),
			Project:        "old-project",
			Timestamp:      oldDate,
			CostMillicents: 10,
		})
	}
	
	_ = s.Close()
	
	s2, _ := NewSQLiteStore(dbPath)
	defer s2.Close()

	archiveDate := time.Now().UTC().AddDate(0, 0, -7).Format("2006-01-02")
	count, err := s2.Archive(ctx, archiveDate)
	if err != nil {
		t.Fatalf("Archive failed: %v", err)
	}
	
	if count != 100 {
		t.Errorf("Expected 100 archived records, got %d", count)
	}

	stats, _ := s2.GetDailyStats(ctx, "old-project", oldDate.Format("2006-01-02"))
	if stats.RequestCount != 100 {
		t.Errorf("Aggregated stats should be preserved, got %d", stats.RequestCount)
	}
}
