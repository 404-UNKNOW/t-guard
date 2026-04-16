package route

import (
	"context"
	"sort"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/cloudflare/ahocorasick"
)

type matcher struct {
	rules []Rule
	ac    *ahocorasick.Matcher
	// kwToRuleIndices 映射关键词到匹配它的规则索引列表
	kwToRuleIndices map[int][]int
}

type routeEngine struct {
	m    atomic.Value
	pool sync.Pool
}

func NewEngine() Engine {
	e := &routeEngine{
		pool: sync.Pool{
			New: func() interface{} { return &Decision{} },
		},
	}
	e.m.Store(&matcher{rules: []Rule{}})
	return e
}

func (e *routeEngine) LoadRules(rules []Rule) error {
	sortedRules := make([]Rule, len(rules))
	copy(sortedRules, rules)
	sort.Slice(sortedRules, func(i, j int) bool {
		if sortedRules[i].Priority == sortedRules[j].Priority {
			return sortedRules[i].ID < sortedRules[j].ID
		}
		return sortedRules[i].Priority < sortedRules[j].Priority
	})

	// 关键词去重映射：同一关键词可能对应多个规则
	kwMap := make(map[string][]int)
	for ruleIdx, rule := range sortedRules {
		for _, kw := range rule.Match.Keywords {
			if kw == "" {
				continue
			}
			kwMap[kw] = append(kwMap[kw], ruleIdx)
		}
	}

	var keywords [][]byte
	kwToRuleIndices := make(map[int][]int)
	idx := 0
	for kw, ruleIndices := range kwMap {
		keywords = append(keywords, []byte(kw))
		kwToRuleIndices[idx] = ruleIndices
		idx++
	}

	var ac *ahocorasick.Matcher
	if len(keywords) > 0 {
		ac = ahocorasick.NewMatcher(keywords)
	}

	e.m.Store(&matcher{
		rules:           sortedRules,
		ac:              ac,
		kwToRuleIndices: kwToRuleIndices,
	})
	return nil
}

func (e *routeEngine) Decide(ctx context.Context, req Request) (Decision, error) {
	m := e.m.Load().(*matcher)
	rules := m.rules

	var content string
	if len(req.Messages) == 1 {
		content = req.Messages[0].Content
	} else if len(req.Messages) > 1 {
		var builder strings.Builder
		for i, msg := range req.Messages {
			if i > 0 {
				builder.WriteString(" ")
			}
			builder.WriteString(msg.Content)
		}
		content = builder.String()
	}

	kwMatchedRules := make(map[int]bool)
	if m.ac != nil && content != "" {
		hits := m.ac.Match([]byte(content))
		for _, hitIdx := range hits {
			for _, ruleIdx := range m.kwToRuleIndices[hitIdx] {
				kwMatchedRules[ruleIdx] = true
			}
		}
	}

	for i, rule := range rules {
		// 模型匹配
		if len(rule.Match.Models) > 0 {
			match := false
			for _, mPrefix := range rule.Match.Models {
				if strings.HasPrefix(req.Model, mPrefix) {
					match = true
					break
				}
			}
			if !match {
				continue
			}
		}

		// IP 匹配
		if len(rule.Match.SourceIPs) > 0 {
			match := false
			for _, ip := range rule.Match.SourceIPs {
				if req.IP == ip { // 简单实现，暂不支持 CIDR
					match = true
					break
				}
			}
			if !match {
				continue
			}
		}

		// Header 匹配
		if len(rule.Match.Headers) > 0 {
			match := true
			for k, v := range rule.Match.Headers {
				if req.Headers[k] != v {
					match = false
					break
				}
			}
			if !match {
				continue
			}
		}

		// 关键词匹配
		if len(rule.Match.Keywords) > 0 && !kwMatchedRules[i] {
			continue
		}

		// Token 检查
		if req.EstTokens < rule.Match.MinTokens {
			continue
		}

		// 预算检查
		if rule.Match.MaxBudget > 0 && req.CurrentSpend >= rule.Match.MaxBudget {
			return Decision{
				Target:     "local",
				FinalModel: req.Model,
				RuleID:     rule.ID,
				Preempted:  true,
				Reason:     "Budget limit exceeded",
			}, nil
		}

		finalModel := req.Model
		if mapped, ok := rule.Action.ModelMap[req.Model]; ok {
			finalModel = mapped
		}

		return Decision{
			Target:         rule.Action.Target,
			FallbackTarget: rule.Action.FallbackTarget,
			FinalModel:     finalModel,
			Headers:        rule.Action.Headers,
			RuleID:         rule.ID,
			Reason:         "Rule matched: " + rule.ID,
		}, nil
	}

	return Decision{
		Target:     "default",
		FinalModel: req.Model,
		Reason:     "No matching rules",
	}, nil
}
