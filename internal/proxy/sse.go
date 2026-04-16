package proxy

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"t-guard/pkg/token"
)

// handleSSE 处理流式响应拦截，实现中途熔断
func (s *proxyServer) handleSSE(resp *http.Response) error {
	project := resp.Request.Header.Get("X-Project-ID")
	model := resp.Request.Header.Get("X-Model-Target")
	
	pr, pw := io.Pipe()
	originalBody := resp.Body
	resp.Body = pr

	go func() {
		defer originalBody.Close()
		defer pw.Close()

		sw := s.config.Billing.NewStreamWriter(pw, project)
		scanner := bufio.NewScanner(originalBody)
		
		var totalOutputTokens int

		// 1. 预估输入 Token (如果请求体能读到)
		// TODO: 在 Director 中预计算并存入 Context


		for scanner.Scan() {
			line := scanner.Text()
			if _, err := sw.Write([]byte(line + "\n")); err != nil {
				return
			}

			if strings.HasPrefix(line, "data: ") {
				data := strings.TrimPrefix(line, "data: ")
				if data == "[DONE]" {
					continue
				}

				// A. 尝试提取 content 内容 (OpenAI 格式)
				var chunk struct {
					Choices []struct {
						Delta struct {
							Content string `json:"content"`
						} `json:"delta"`
					} `json:"choices"`
				}
				
				if err := json.Unmarshal([]byte(data), &chunk); err == nil && len(chunk.Choices) > 0 {
					content := chunk.Choices[0].Delta.Content
					if content != "" {
						// B. 计算 Incremental Token
						res, _ := s.config.Token.Calculate(context.Background(), token.CalcRequest{
							Model:   model,
							Content: content,
						})
						
						incrementalTokens := res.TokenCount
						totalOutputTokens += incrementalTokens

						// C. 转化为毫美分成本
						incrementalCost := s.config.Pricing.CalculateCost(model, 0, incrementalTokens)

						// D. 调用 M3 实现中途熔断
						if err := sw.CheckBudget(resp.Request.Context(), incrementalCost); err != nil {
							_ = originalBody.Close()
							return
						}

						// E. 记录流水
						_ = s.config.Billing.Record(resp.Request.Context(), project, incrementalCost)
					}
				}
			}
		}
	}()

	return nil
}
