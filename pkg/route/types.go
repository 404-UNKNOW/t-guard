package route

import (
	"context"
)

// Message 消息结构
type Message struct {
	Role    string
	Content string
}

// Request 决策请求
type Request struct {
	Project      string
	Model        string
	Messages     []Message
	EstTokens    int
	CurrentSpend int64 // 毫美分
}

// Decision 决策结果
type Decision struct {
	Target      string
	FinalModel  string
	Headers     map[string]string
	RuleID      string
	Preempted   bool
	Reason      string
}

// MatchCond 匹配条件
type MatchCond struct {
	Keywords  []string // 提示词包含任意关键词
	Models    []string // 模型名前缀匹配
	MinTokens int      // 最小预估 token 数触发
	MaxBudget int64    // 该规则的最大预算限制
}

// RouteAction 路由动作
type RouteAction struct {
	Target    string
	ModelMap  map[string]string
	Headers   map[string]string
	Transform string
}

// Rule 路由规则
type Rule struct {
	ID       string
	Priority int // 越小越优先，默认 100
	Match    MatchCond
	Action   RouteAction
}

// Engine 核心接口
type Engine interface {
	LoadRules(rules []Rule) error // 原子性更新规则
	Decide(ctx context.Context, req Request) (Decision, error)
}
