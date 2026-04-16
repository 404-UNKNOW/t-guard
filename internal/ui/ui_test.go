package ui

import (
	tea "github.com/charmbracelet/bubbletea"
	"t-guard/pkg/budget"
	"t-guard/pkg/store"
	"testing"
)

// 验收标准：验证 MVU 架构的消息处理逻辑
func TestUI_Update(t *testing.T) {
	cfg := Config{
		ProxyAddr:   "localhost:8080",
		RefreshRate: 1,
	}
	m := NewModel(cfg).(*model)

	// 模拟 DataMsg 异步到达
	data := DataMsg{
		BudgetStatus: budget.Status{
			Project:    "test-project",
			Used:       500,
			Limit:      1000,
			Percentage: 0.5,
			State:      budget.StateNormal,
		},
		RecentLogs: []store.Record{
			{Model: "gpt-4", CostMillicents: 100},
		},
	}

	updatedModel, _ := m.Update(data)
	newModel := updatedModel.(*model)

	// 1. 验证内存数据已更新 (双缓冲)
	if newModel.data.BudgetStatus.Used != 500 {
		t.Errorf("Model update failed: expected 500, got %d", newModel.data.BudgetStatus.Used)
	}

	// 2. 验证表格行已同步
	if len(newModel.table.Rows()) != 1 {
		t.Errorf("Table row sync failed: expected 1, got %d", len(newModel.table.Rows()))
	}

	// 3. 验证退出逻辑 (响应式)
	_, cmd := newModel.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	if cmd == nil {
		t.Error("Quit signal processing failed")
	}
}
