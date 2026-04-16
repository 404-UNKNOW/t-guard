package token

import (
	"encoding/base64"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"strings"
)

// CalculateImageTokens 参考 OpenAI 的计费规则
// Low detail: 固定 85 tokens
// High detail: 按 512x512 切块，每块 170 tokens + 85 tokens 基础
func CalculateImageTokens(img ImagePart) int {
	if img.Detail == "low" {
		return 85
	}

	// 尝试解析 Base64 以获取图像尺寸
	reader := base64.NewDecoder(base64.StdEncoding, strings.NewReader(img.Base64))
	config, _, err := image.DecodeConfig(reader)
	if err != nil {
		// 解析失败则返回默认高精度估算值
		return 1105 // 默认 2x2 块的估算值
	}

	width, height := config.Width, config.Height
	
	// OpenAI 规则：长边缩放到 2048px，短边缩放到 768px
	// 计算 512x512 的块数
	tilesW := (width + 511) / 512
	tilesH := (height + 511) / 512
	
	return tilesW*tilesH*170 + 85
}
