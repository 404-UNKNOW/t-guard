package ui

import (
	"context"
	"fmt"
	"sort"
	"t-guard/pkg/budget"
	"t-guard/pkg/store"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type model struct {
	config      Config
	ctx         context.Context
	data        DataMsg
	table       table.Model
	progress    progress.Model
	width, height int
	sorting     string // "time", "cost"
	lastUpdated string
	errorMsg    string
}

func NewModel(cfg Config) tea.Model {
	columns := []table.Column{
		{Title: "Time", Width: 10},
		{Title: "Model", Width: 15},
		{Title: "Tokens", Width: 10},
		{Title: "Cost(m$)", Width: 10},
	}
	t := table.New(
		table.WithColumns(columns),
		table.WithFocused(true),
		table.WithHeight(10),
	)

	s := table.DefaultStyles()
	s.Header = s.Header.Bold(true).Foreground(lipgloss.Color(ColorPrimary))
	t.SetStyles(s)

	return &model{
		config:   cfg,
		ctx:      context.Background(),
		table:    t,
		progress: progress.New(progress.WithDefaultGradient()),
		sorting:  "time",
	}
}

func (m *model) Init() tea.Cmd {
	return tea.Batch(
		FetchSnapshot(m.ctx, m.config),
		ListenBudget(m.config.Billing, "test-project"),
		Tick(m.config.RefreshRate),
	)
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.table.SetWidth(m.width - 4)
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "r":
			return m, FetchSnapshot(m.ctx, m.config)
		case "s":
			if m.sorting == "time" {
				m.sorting = "cost"
			} else {
				m.sorting = "time"
			}
			m.updateTable()
		}
	case DataMsg:
		m.data = msg
		m.lastUpdated = msg.Timestamp.Format("15:04:05")
		m.updateTable()
	case StatusMsg:
		m.data.BudgetStatus = budget.Status(msg)
	case TickMsg:
		return m, tea.Batch(FetchSnapshot(m.ctx, m.config), Tick(m.config.RefreshRate))
	}

	var cmd tea.Cmd
	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

func (m *model) updateTable() {
	logs := make([]store.Record, len(m.data.RecentLogs))
	copy(logs, m.data.RecentLogs)
	
	if m.sorting == "cost" {
		sort.Slice(logs, func(i, j int) bool {
			return logs[i].CostMillicents > logs[j].CostMillicents
		})
	} else {
		sort.Slice(logs, func(i, j int) bool {
			return logs[i].Timestamp.After(logs[j].Timestamp)
		})
	}

	var rows []table.Row
	for _, r := range logs {
		rows = append(rows, table.Row{
			r.Timestamp.Format("15:04:05"),
			r.Model,
			fmt.Sprintf("%d", r.InputTokens+r.OutputTokens),
			fmt.Sprintf("%d", r.CostMillicents),
		})
	}
	m.table.SetRows(rows)
}
