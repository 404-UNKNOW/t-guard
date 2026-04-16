package token

import (
	"context"
)

// ImagePart 描述图像输入
type ImagePart struct {
	Base64 string
	Format string // "jpeg", "png", "webp", "gif"
	Detail string // "low", "high", "auto"
}

// CalcRequest 定义输入结构
type CalcRequest struct {
	Model   string      // 如 "gpt-4", "claude-3-opus"
	Content string      // 待计算的文本
	Images  []ImagePart // 图像输入
	Type    string      // "prompt" 或 "completion"
}

// CalcResult 定义输出结构
type CalcResult struct {
	TokenCount int
	Method     string  // "tiktoken_exact", "claude_tokenizer", "estimate"
	Confidence float64 // 0.0-1.0，估算时为0.8
}

// Engine 核心引擎接口
type Engine interface {
	Calculate(ctx context.Context, req CalcRequest) (CalcResult, error)
	Warmup(model string) error // 预加载encoder到内存
	Close() error
}

// 常量定义
const (
	MethodTiktokenExact   = "tiktoken_exact"
	MethodClaudeTokenizer = "claude_tokenizer"
	MethodEstimate        = "estimate"
	DefaultConfidence     = 1.0
	EstimateConfidence    = 0.8
	MaxTokenLimit         = 1000000 // 1M tokens limit
)
