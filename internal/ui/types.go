package ui

import (
	"t-guard/pkg/budget"
	"t-guard/pkg/store"
	"time"
)

const (
	ColorPrimary = "#7D56F4"
	ColorSuccess = "#04B575"
	ColorWarning = "#F59E0B"
	ColorDanger  = "#EF4444"
)

type Config struct {
	Store       store.Store
	Billing     budget.Controller
	ProxyAddr   string
	RefreshRate time.Duration
}

type DataMsg struct {
	Stats        store.DailyStats
	RecentLogs   []store.Record
	BudgetStatus budget.Status
	Timestamp    time.Time
}

type StatusMsg budget.Status
type TickMsg time.Time
type ErrorMsg string
