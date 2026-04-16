package route

import (
	"context"
	"fmt"
	"testing"
	"time"
)

// 验收标准 2：优先级测试
func TestEngine_PriorityAndMatch(t *testing.T) {
	engine := NewEngine()
	rules := []Rule{
		{
			ID:       "rule-low-priority",
			Priority: 200,
			Match:    MatchCond{Keywords: []string{"urgent"}},
			Action:   RouteAction{Target: "fast-lane"},
		},
		{
			ID:       "rule-high-priority",
			Priority: 10,
			Match:    MatchCond{Keywords: []string{"urgent"}},
			Action:   RouteAction{Target: "premium-lane"},
		},
	}
	_ = engine.LoadRules(rules)

	req := Request{
		Model:    "gpt-4",
		Messages: []Message{{Role: "user", Content: "this is an urgent request"}},
	}
	decision, _ := engine.Decide(context.Background(), req)

	if decision.Target != "premium-lane" {
		t.Errorf("Priority failed: Expected premium-lane, got %s", decision.Target)
	}
}

// 验收标准 3：预算拦截测试
func TestEngine_BudgetPreempt(t *testing.T) {
	engine := NewEngine()
	rules := []Rule{
		{
			ID:       "budget-rule",
			Priority: 1,
			Match:    MatchCond{MaxBudget: 1000}, // 1000 毫美分
			Action:   RouteAction{Target: "openai"},
		},
	}
	_ = engine.LoadRules(rules)

	// 情况 A：未超支
	reqA := Request{CurrentSpend: 500}
	decA, _ := engine.Decide(context.Background(), reqA)
	if decA.Preempted {
		t.Error("Budget false positive preemption")
	}

	// 情况 B：已超支
	reqB := Request{CurrentSpend: 1001}
	decB, _ := engine.Decide(context.Background(), reqB)
	if !decB.Preempted || decB.Target != "local" {
		t.Errorf("Budget preemption failed: %+v", decB)
	}
}

// 验收标准 1：匹配性能测试
func TestEngine_Performance(t *testing.T) {
	engine := NewEngine()
	var rules []Rule
	// 构造 100 条包含不同关键词的规则
	for i := 0; i < 100; i++ {
		rules = append(rules, Rule{
			ID:       fmt.Sprintf("r-%d", i),
			Match:    MatchCond{Keywords: []string{fmt.Sprintf("token-%d", i)}},
			Action:   RouteAction{Target: "ai-cluster"},
		})
	}
	_ = engine.LoadRules(rules)

	req := Request{
		Messages: []Message{{Role: "user", Content: "this message contains token-50 for testing"}},
	}

	// 预热并测试延迟
	start := time.Now()
	iterations := 10000
	for i := 0; i < iterations; i++ {
		_, _ = engine.Decide(context.Background(), req)
	}
	duration := time.Since(start)
	avg := duration / time.Duration(iterations)
	
	t.Logf("Average Decision Latency: %v", avg)
	if avg > 50*time.Microsecond {
		t.Errorf("Performance constraint violated: %v > 50us", avg)
	}
}

// 验收标准 4：热更新测试
func TestEngine_HotUpdate(t *testing.T) {
	engine := NewEngine()
	ctx := context.Background()
	req := Request{Model: "gpt-4"}

	// 初始状态：无规则
	dec1, _ := engine.Decide(ctx, req)
	if dec1.Target != "default" {
		t.Error("Initial state should be default")
	}

	// 更新规则
	_ = engine.LoadRules([]Rule{{ID: "new", Action: RouteAction{Target: "updated"}}})
	dec2, _ := engine.Decide(ctx, req)
	if dec2.Target != "updated" {
		t.Error("Hot update failed to reflect new rules")
	}
}
