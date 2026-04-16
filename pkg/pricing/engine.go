package pricing

import (
	"sync"
)

// ModelPrice 定义每百万 Token 的价格（单位：毫美分）
type ModelPrice struct {
	InputPrice  int64 `mapstructure:"input_price"`  // 每百万输入 tokens 的毫美分
	OutputPrice int64 `mapstructure:"output_price"` // 每百万输出 tokens 的毫美分
}

type Engine interface {
	CalculateCost(model string, inputTokens, outputTokens int) int64
	UpdatePrices(prices map[string]ModelPrice)
}

type pricingEngine struct {
	mu     sync.RWMutex
	prices map[string]ModelPrice
}

func NewEngine(initialPrices map[string]ModelPrice) Engine {
	if initialPrices == nil {
		initialPrices = make(map[string]ModelPrice)
	}
	// 默认兜底价格 (GPT-3.5 级别)
	if _, ok := initialPrices["default"]; !ok {
		initialPrices["default"] = ModelPrice{InputPrice: 50, OutputPrice: 150}
	}
	return &pricingEngine{
		prices: initialPrices,
	}
}

func (e *pricingEngine) CalculateCost(model string, inputTokens, outputTokens int) int64 {
	e.mu.RLock()
	defer e.mu.RUnlock()

	price, ok := e.prices[model]
	if !ok {
		price = e.prices["default"]
	}

	// 费用 = (输入Tokens * 输入单价 / 1,000,000) + (输出Tokens * 输出单价 / 1,000,000)
	// 为避免浮点误差，使用毫美分进行整型运算
	inputCost := (int64(inputTokens) * price.InputPrice) / 1000
	outputCost := (int64(outputTokens) * price.OutputPrice) / 1000
	
	return inputCost + outputCost
}

func (e *pricingEngine) UpdatePrices(prices map[string]ModelPrice) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.prices = prices
}
