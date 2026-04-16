package token

import (
	"unicode/utf8"
)

// EstimateFallback 采用 1 token ≈ 4 字符的策略
func EstimateFallback(content string) int {
	charCount := utf8.RuneCountInString(content)
	if charCount == 0 {
		return 0
	}
	// 4 字符 1 token 的近似值
	tokens := (charCount + 3) / 4
	if tokens > MaxTokenLimit {
		return MaxTokenLimit
	}
	return tokens
}
