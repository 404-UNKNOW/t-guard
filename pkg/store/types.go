package store

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Record 定义写入结构
type Record struct {
	ID             uuid.UUID `json:"id"`
	TraceID        string    `json:"trace_id"`
	Project        string    `json:"project"`
	Model          string    `json:"model"`
	InputTokens    int       `json:"input_tokens"`
	OutputTokens   int       `json:"output_tokens"`
	CostMillicents int64     `json:"cost_millicents"` // 毫美分
	RouteTarget    string    `json:"route_target"`
	DurationMs     int64     `json:"duration_ms"`
	Timestamp      time.Time `json:"timestamp"`
}

// DailyStats 定义查询统计结果
type DailyStats struct {
	Date            string         `json:"date"`
	Project         string         `json:"project"`
	TotalMillicents int64          `json:"total_millicents"`
	RequestCount    int            `json:"request_count"`
	ModelBreakdown  map[string]int `json:"model_breakdown"` // 模型使用次数统计
}

// Store 核心数据接口
type Store interface {
	// 写入接口（异步）
	Write(ctx context.Context, r Record) error

	// 查询接口（同步）
	GetDailyStats(ctx context.Context, project string, date string) (DailyStats, error)
	GetRecentRequests(ctx context.Context, project string, limit int) ([]Record, error)
	QueryProjects(ctx context.Context) ([]string, error)

	// 管理接口
	Archive(ctx context.Context, beforeDate string) (int, error) // 返回归档条数
	Close() error                                               // 优雅关闭，排空队列
}
