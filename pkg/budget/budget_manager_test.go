package budget

import (
	"context"
	"sync"
	"t-guard/pkg/store"
	"testing"
)

func TestController_PreDeduction_Concurrency(t *testing.T) {
	// 使用 MockStore 简化测试
	s := &mockStore{}
	ctx := context.Background()
	project := "concurrency-project"
	
	// 设置 1000 毫美分限制
	hardLimit := int64(1000)
	config := BudgetConfig{
		Project:   project,
		HardLimit: hardLimit,
	}

	ctrl, _ := NewController(s, []BudgetConfig{config})

	const concurrentRequests = 100
	const estimatedCost = 50 // 每次预扣 50
	const actualCost = 10    // 实际只用 10 (退回 40)

	var wg sync.WaitGroup
	var successCount int32
	var mu sync.Mutex
	
	// 启动 100 个请求
	// 预期：因为 100 * 50 = 5000 > 1000，必然有一部分被拒绝
	// 但结算后由于退回了金额，后续请求可能又能通过 (取决于执行顺序，此用例侧重于不超扣)
	for i := 0; i < concurrentRequests; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			
			decision, err := ctrl.Allow(ctx, project, estimatedCost)
			if err != nil {
				return
			}

			if decision.Allowed {
				mu.Lock()
				successCount++
				mu.Unlock()
				
				// 模拟请求耗时
				// 精准结算
				_ = ctrl.SettleBudget(ctx, project, estimatedCost, actualCost)
			}
		}()
	}

	wg.Wait()

	status, _ := ctrl.GetStatus(ctx, project)
	
	// 验证核心不变性：已用金额不能超过硬限制
	if status.Used > hardLimit {
		t.Errorf("CRITICAL SECURITY FAILURE: Used budget %d exceeded hard limit %d", status.Used, hardLimit)
	}

	// 验证最终状态正确性：已用金额应等于 成功次数 * 实际单次费用
	expectedUsed := int64(successCount) * actualCost
	if status.Used != expectedUsed {
		t.Errorf("Budget accounting mismatch: expected %d, got %d", expectedUsed, status.Used)
	}
	
	t.Logf("Requests: %d, Success: %d, Final Used: %d", concurrentRequests, successCount, status.Used)
}

type mockStore struct {
	store.Store
}
func (m *mockStore) GetDailyStats(ctx context.Context, project, date string) (store.DailyStats, error) {
	return store.DailyStats{}, nil
}
func (m *mockStore) Write(ctx context.Context, r store.Record) error { return nil }
