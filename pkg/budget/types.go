package budget

import (
	"context"
	"io"
)

// BudgetConfig 预算配置
type BudgetConfig struct {
	HardLimit int64   // 硬限制（毫美分），0表示无限制
	SoftLimit float64 // 软限制比例（如0.8表示80%）
	Project   string
	ResetCron string // 重置周期（默认 "0 0 * * *" 每天）
}

// Status 预算状态
type Status struct {
	Project    string
	Used       int64
	Limit      int64
	Percentage float64
	State      State // "normal", "warning", "critical", "exhausted"
}

type State string

const (
	StateNormal    State = "normal"
	StateWarning   State = "warning"
	StateCritical  State = "critical"
	StateExhausted State = "exhausted"
)

// Decision 决策结果
type Decision struct {
	Allowed   bool
	Remaining int64  // 剩余额度（毫美分）
	Warning   string // 软限制警告信息
}

// Controller 核心控制器接口
type Controller interface {
	Allow(ctx context.Context, project string, estimatedCost int64) (Decision, error)
	Record(ctx context.Context, project string, actualCost int64) error
	GetStatus(ctx context.Context, project string) (Status, error)
	
	// NewStreamWriter 创建流式熔断包装器
	NewStreamWriter(w io.Writer, project string) *StreamWriter
	
	// Subscribe 订阅预算状态变更（带缓冲 10）
	Subscribe(project string) <-chan Status
	
	// Close 优雅关闭并持久化
	Close() error
}
