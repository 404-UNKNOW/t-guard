package ui

import (
	"fmt"
	"github.com/charmbracelet/lipgloss"
)

var (
	baseStyle = lipgloss.NewStyle().Margin(1, 2)
	
	headerStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FAFAFA")).
		Background(lipgloss.Color(ColorPrimary)).
		Padding(0, 1)

	warnStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FAFAFA")).
		Background(lipgloss.Color(ColorDanger)).
		Padding(0, 1)

	panelStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(ColorPrimary)).
		Padding(1, 2).
		MarginRight(2)
)

func (m *model) View() string {
	if m.width == 0 {
		return "Initializing..."
	}
	
	if m.width < 40 {
		return fmt.Sprintf("Used: %d\nLimit: %d\n[q] Quit", m.data.BudgetStatus.Used, m.data.BudgetStatus.Limit)
	}

	// 1. Header
	header := headerStyle.Render(" T-GUARD | REAL-TIME ")
	if m.errorMsg != "" {
		header = warnStyle.Render(" ! ERROR: " + m.errorMsg + " ")
	}
	headerView := lipgloss.JoinHorizontal(lipgloss.Center, header, fmt.Sprintf("  Last Sync: %s", m.lastUpdated))

	// 2. Budget Panel
	status := string(m.data.BudgetStatus.State)
	if status == "" { status = "Normal" }
	
	budgetInfo := fmt.Sprintf("Project: %s\nUsed: %d m$\nLimit: %d m$\nState: %s",
		m.data.BudgetStatus.Project,
		m.data.BudgetStatus.Used,
		m.data.BudgetStatus.Limit,
		status,
	)
	budgetPanel := panelStyle.Render(budgetInfo)

	// 3. Stats Panel
	statsInfo := fmt.Sprintf("Total Req: %d\nProxy: %s\nMode: Stable", 
		m.data.Stats.RequestCount, 
		m.config.ProxyAddr,
	)
	statsPanel := panelStyle.Render(statsInfo)

	// 4. Dashboard (Horizontal or Vertical)
	var dashboard string
	if m.width > 80 {
		dashboard = lipgloss.JoinHorizontal(lipgloss.Top, budgetPanel, statsPanel)
	} else {
		dashboard = lipgloss.JoinVertical(lipgloss.Left, budgetPanel, statsPanel)
	}

	// 5. Progress Bar
	progressBar := ""
	if m.data.BudgetStatus.Limit > 0 {
		// 根据百分比动态调整颜色逻辑由 progress.Model 处理
		progressBar = "\nBudget Utilization:\n" + m.progress.ViewAs(m.data.BudgetStatus.Percentage) + "\n"
	}

	// 6. Log Table
	tableView := "\nRecent Requests (Sorted by " + m.sorting + "):\n" + m.table.View()

	footer := lipgloss.NewStyle().Foreground(lipgloss.Color("#626262")).Render("\n[q] Quit | [r] Refresh | [s] Sort | [↑/↓] Logs")

	return baseStyle.Render(lipgloss.JoinVertical(lipgloss.Left,
		headerView,
		dashboard,
		progressBar,
		tableView,
		footer,
	))
}
