package store

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestSqliteStore_DeductBudget_Concurrency(t *testing.T) {
	dbPath := "test_concurrency.db"
	os.Remove(dbPath)
	defer os.Remove(dbPath)

	s, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer s.Close()

	project := "test-project"
	limit := int64(100) // 总额度 100 毫美分
	cost := int64(10)   // 每次扣 10
	iterations := 15    // 总共尝试 15 次，预期成功 10 次，失败 5 次

	var wg sync.WaitGroup
	var successCount int64
	var failCount int64
	var mu sync.Mutex

	for i := 0; i < iterations; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			r := Record{
				ID:             uuid.New(),
				Project:        project,
				Model:          "gpt-4",
				CostMillicents: cost,
				Timestamp:      time.Now(),
			}

			err := s.DeductBudget(ctx, r, limit)
			mu.Lock()
			if err == nil {
				successCount++
			} else if err == ErrInsufficientBudget {
				failCount++
			} else {
				t.Errorf("Unexpected error: %v", err)
			}
			mu.Unlock()
		}(i)
	}

	wg.Wait()

	if successCount != 10 {
		t.Errorf("Expected 10 successes, got %d", successCount)
	}
	if failCount != 5 {
		t.Errorf("Expected 5 failures, got %d", failCount)
	}

	// 最终检查数据库中的余额
	stats, err := s.GetDailyStats(context.Background(), project, time.Now().UTC().Format("2006-01-02"))
	if err != nil {
		t.Fatalf("Failed to get stats: %v", err)
	}
	if stats.TotalMillicents != 100 {
		t.Errorf("Expected total cost 100, got %d", stats.TotalMillicents)
	}
}
