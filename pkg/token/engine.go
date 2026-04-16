package token

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/pkoukk/tiktoken-go"
)

type tokenEngine struct {
	encoders atomic.Value // 存储 map[string]*tiktoken.Tiktoken
	metrics  *Metrics
	mu       sync.Mutex
}

// NewEngine 创建并初始化引擎
func NewEngine() Engine {
	e := &tokenEngine{
		metrics: &Metrics{},
	}
	e.encoders.Store(make(map[string]*tiktoken.Tiktoken))
	return e
}

// Warmup 预热 encoder，确保 Calculate 中无初始化逻辑
func (e *tokenEngine) Warmup(model string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	encoders := e.encoders.Load().(map[string]*tiktoken.Tiktoken)
	if _, ok := encoders[model]; ok {
		return nil
	}

	tke, err := tiktoken.EncodingForModel(model)
	if err != nil {
		// 容错：尝试获取通用的 cl100k_base
		tke, err = tiktoken.GetEncoding("cl100k_base")
		if err != nil {
			return fmt.Errorf("failed to load encoder for %s: %w", model, err)
		}
	}

	// 复制 map 并更新（Copy-on-Write 策略确保原子性）
	newEncoders := make(map[string]*tiktoken.Tiktoken)
	for k, v := range encoders {
		newEncoders[k] = v
	}
	newEncoders[model] = tke
	e.encoders.Store(newEncoders)
	return nil
}

// Calculate 实现双轨计算：Tiktoken -> 估算
func (e *tokenEngine) Calculate(ctx context.Context, req CalcRequest) (CalcResult, error) {
	e.metrics.IncCalculations()

	// 输入校验
	if req.Content == "" {
		return CalcResult{TokenCount: 0, Method: MethodEstimate, Confidence: DefaultConfidence}, nil
	}

	// 轨迹 1：尝试精确计算
	encoders := e.encoders.Load().(map[string]*tiktoken.Tiktoken)
	if tke, ok := encoders[req.Model]; ok {
		e.metrics.IncCacheHit()
		tokens := tke.Encode(req.Content, nil, nil)
		count := len(tokens)
		if count > MaxTokenLimit {
			count = MaxTokenLimit
		}
		return CalcResult{
			TokenCount: count,
			Method:     MethodTiktokenExact,
			Confidence: DefaultConfidence,
		}, nil
	}

	// 轨迹 2：回退至估算
	e.metrics.IncFallback()
	count := EstimateFallback(req.Content)
	return CalcResult{
		TokenCount: count,
		Method:     MethodEstimate,
		Confidence: EstimateConfidence,
	}, nil
}

func (e *tokenEngine) Close() error {
	return nil
}
