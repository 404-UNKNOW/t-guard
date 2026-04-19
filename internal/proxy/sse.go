package proxy

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"t-guard/pkg/logger"
	"t-guard/pkg/token"

	"go.uber.org/zap"
)

// handleSSE 处理流式响应拦截，实现健壮的行解析与并发安全扣费
func (s *proxyServer) handleSSE(resp *http.Response) error {
	project := resp.Request.Header.Get("X-Project-ID")
	model := resp.Request.Header.Get("X-Model-Target")
	
	// 从 Context 获取预估冻结金额 (在 Director 中设置)
	frozenAmount, _ := resp.Request.Context().Value("frozen_amount").(int64)

	pr, pw := io.Pipe()
	originalBody := resp.Body
	resp.Body = pr

	go func() {
		// 确保所有出口都能关闭上游 Body，防止连接泄露
		defer func() {
			if err := originalBody.Close(); err != nil {
				logger.Log.Error("failed to close original response body", zap.Error(err))
			}
		}()
		defer func() {
			if err := pw.Close(); err != nil {
				logger.Log.Error("failed to close pipe writer", zap.Error(err))
			}
		}()

		sw := s.config.Billing.NewStreamWriter(pw, project)
		reader := bufio.NewReader(originalBody)
		
		var totalCost int64
		defer func() {
			// 最终结算：解冻并扣除实际费用
			_ = s.config.Billing.SettleBudget(context.Background(), project, frozenAmount, totalCost)
		}()

		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				if err != io.EOF {
					// 记录异常日志
				}
				return
			}

			// 透传原始行到客户端
			if _, err := sw.Write([]byte(line)); err != nil {
				return
			}

			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}

			if strings.HasPrefix(line, "data: ") {
				data := strings.TrimPrefix(line, "data: ")
				if data == "[DONE]" {
					return // 正常结束
				}

				// 健壮性：处理可能的半包或非 JSON 数据
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
						// 计算 Incremental Token
						res, _ := s.config.Token.Calculate(resp.Request.Context(), token.CalcRequest{
							Model:   model,
							Content: content,
						})
						
						incrementalCost := s.config.Pricing.CalculateCost(model, 0, res.TokenCount)
						totalCost += incrementalCost

						// 中途熔断检查：基于当前累积实际消耗
						if err := sw.CheckBudget(resp.Request.Context(), incrementalCost); err != nil {
							return
						}
					}
				}
			}
		}
	}()

	return nil
}
