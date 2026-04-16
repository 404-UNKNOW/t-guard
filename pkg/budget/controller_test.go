package budget

import (
	"bytes"
	"context"
	"os"
	"t-guard/pkg/store"
	"testing"
)

// 验收标准：流式熔断与崩溃恢复测试
func TestController_StreamingAndRecovery(t *testing.T) {
	dbPath := "test_budget_ext.db"
	_ = os.Remove(dbPath)
	defer os.Remove(dbPath)

	s, err := store.NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer s.Close()

	ctx := context.Background()
	project := "stream-project"
	config := BudgetConfig{
		Project:   project,
		HardLimit: 100, // 100 毫美分
	}

	ctrl, err := NewController(s, []BudgetConfig{config})
	if err != nil {
		t.Fatalf("Failed to create controller: %v", err)
	}

	// 1. 流式熔断测试
	var buf bytes.Buffer
	sw := ctrl.NewStreamWriter(&buf, project)

	// 模拟已用 90
	_ = ctrl.Record(ctx, project, 90)

	// 写入一个刚好到达预算的块（允许）
	err = sw.CheckBudget(ctx, 10) 
	if err != nil {
		t.Errorf("Should allow exactly 100: %v", err)
	}
	_, _ = sw.Write([]byte("ok"))
	_ = ctrl.Record(ctx, project, 10) // 此时已用 100

	// 再次写入，触发熔断
	err = sw.CheckBudget(ctx, 1) // 100 + 1 > 100
	if err == nil {
		t.Error("Streaming breaker failed to trigger at 101")
	}

	if !bytes.Contains(buf.Bytes(), []byte("budget_exceeded")) {
		t.Errorf("Stream should contain error JSON, got: %s", buf.String())
	}

	// 2. 崩溃恢复测试
	// 记录一笔大额消费并确保其进入 Store (Record 会调用 store.Write)
	_ = ctrl.Record(ctx, project, 500)
	
	// 强制等待 Store 异步写入完成（测试环境下简单处理）
	_ = s.Close() 

	// 重新打开 Store 并创建新 Controller 模拟重启
	s2, _ := store.NewSQLiteStore(dbPath)
	defer s2.Close()

	ctrl2, _ := NewController(s2, []BudgetConfig{config})
	status, _ := ctrl2.GetStatus(ctx, project)
	
	// 重启后应加载出之前的 100 + 500 = 600
	if status.Used != 600 {
		t.Errorf("Recovery failed: expected 600, got %d", status.Used)
	}
}

// 验收标准：精度测试 (1000次 1毫美分)
func TestController_Precision(t *testing.T) {
	dbPath := "test_precision.db"
	_ = os.Remove(dbPath)
	defer os.Remove(dbPath)
	s, _ := store.NewSQLiteStore(dbPath)
	defer s.Close()

	ctrl, _ := NewController(s, []BudgetConfig{{Project: "prec", HardLimit: 5000}})
	ctx := context.Background()

	for i := 0; i < 1000; i++ {
		_ = ctrl.Record(ctx, "prec", 1)
	}

	status, _ := ctrl.GetStatus(ctx, "prec")
	if status.Used != 1000 {
		t.Errorf("Precision test failed: expected 1000, got %d", status.Used)
	}
}
