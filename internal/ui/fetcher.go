package ui

import (
	"context"
	tea "github.com/charmbracelet/bubbletea"
	"t-guard/pkg/budget"
	"time"
)

// ListenBudget 实时监听预算状态变更
func ListenBudget(c budget.Controller, project string) tea.Cmd {
	return func() tea.Msg {
		sub := c.Subscribe(project)
		status := <-sub
		return StatusMsg(status)
	}
}

// FetchSnapshot 抓取当前数据的快照
func FetchSnapshot(ctx context.Context, cfg Config) tea.Cmd {
	return func() tea.Msg {
		project := "test-project"
		dateStr := time.Now().UTC().Format("2006-01-02")
		
		stats, _ := cfg.Store.GetDailyStats(ctx, project, dateStr)
		logs, _ := cfg.Store.GetRecentRequests(ctx, project, 30)
		bStatus, _ := cfg.Billing.GetStatus(ctx, project)

		return DataMsg{
			Stats:        stats,
			RecentLogs:   logs,
			BudgetStatus: bStatus,
			Timestamp:    time.Now(),
		}
	}
}

func Tick(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(t time.Time) tea.Msg {
		return TickMsg(t)
	})
}
