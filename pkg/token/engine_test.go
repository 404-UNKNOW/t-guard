package token

import (
	"context"
	"sync"
	"testing"
	"time"
)

// 验收标准 1：精确性测试
func TestEngine_Accuracy(t *testing.T) {
	engine := NewEngine()
	model := "gpt-3.5-turbo"
	err := engine.Warmup(model)
	if err != nil {
		t.Fatalf("Warmup failed: %v", err)
	}

	req := CalcRequest{
		Model:   model,
		Content: "Hello world", // OpenAI cl100k_base 中 "Hello" + " world" = 2 tokens
	}

	result, err := engine.Calculate(context.Background(), req)
	if err != nil {
		t.Fatalf("Calculate failed: %v", err)
	}

	if result.TokenCount <= 0 {
		t.Errorf("Expected positive token count, got %d", result.TokenCount)
	}
	if result.Method != MethodTiktokenExact {
		t.Errorf("Expected exact method, got %s", result.Method)
	}
}

// 验收标准 2：并发测试
func TestEngine_Concurrency(t *testing.T) {
	engine := NewEngine()
	model := "gpt-4"
	_ = engine.Warmup(model)

	start := time.Now()
	var wg sync.WaitGroup
	workers := 1000
	wg.Add(workers)

	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			_, _ = engine.Calculate(context.Background(), CalcRequest{
				Model:   model,
				Content: "Concurrency test content for safety validation",
			})
		}()
	}

	wg.Wait()
	duration := time.Since(start)
	if duration > 500*time.Millisecond {
		t.Errorf("Concurrency performance check failed: %v", duration)
	}
}

// 验收标准 3：回退测试
func TestEngine_Fallback(t *testing.T) {
	engine := NewEngine()
	// 不预热未知模型，强制触发回退
	req := CalcRequest{
		Model:   "unknown-model",
		Content: "Testing the fallback logic for unknown models",
	}

	start := time.Now()
	result, err := engine.Calculate(context.Background(), req)
	duration := time.Since(start)

	if err != nil {
		t.Errorf("Fallback should not return error: %v", err)
	}
	if duration > 5*time.Millisecond {
		t.Errorf("Fallback latency too high: %v", duration)
	}
	if result.Method != MethodEstimate || result.Confidence != 0.8 {
		t.Errorf("Invalid fallback result: %+v", result)
	}
}
