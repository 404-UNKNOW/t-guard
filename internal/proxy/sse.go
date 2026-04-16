package proxy

import (
	"bufio"
	"io"
	"net/http"
	"strings"
)

// handleSSE 处理流式响应拦截，实现中途熔断
func (s *proxyServer) handleSSE(resp *http.Response) error {
	project := resp.Request.Header.Get("X-Project-ID")
	
	// 使用 io.Pipe 进行响应流重定向
	pr, pw := io.Pipe()
	originalBody := resp.Body
	resp.Body = pr

	go func() {
		defer originalBody.Close()
		defer pw.Close()

		// 获取预算流式写入器
		sw := s.config.Billing.NewStreamWriter(pw, project)
		scanner := bufio.NewScanner(originalBody)

		for scanner.Scan() {
			line := scanner.Text()
			
			// A. 透传原始 SSE 数据块
			if _, err := sw.Write([]byte(line + "\n")); err != nil {
				return
			}

			// B. 拦截 data 块进行预算检查
			if strings.HasPrefix(line, "data: ") {
				content := strings.TrimPrefix(line, "data: ")
				if content == "[DONE]" {
					continue
				}

				// 策略：每收到一个 chunk 进行一次预算校验
				// 此处暂以字节长度作为预估成本（实际应使用 M0 Tokenizer）
				incrementalCost := int64(len(content)) 

				// C. 调用 M3 实现流式中途熔断
				if err := sw.CheckBudget(resp.Request.Context(), incrementalCost); err != nil {
					// 已在 CheckBudget 中写入错误 JSON，此处直接关闭上游响应
					_ = originalBody.Close()
					return
				}

				// D. 记录实际扣费
				_ = s.config.Billing.Record(resp.Request.Context(), project, incrementalCost)
			}

			// 处理空行（SSE 块分隔符）
			if line == "" {
				// 发送空行
			}
		}
		
		if err := scanner.Err(); err != nil {
			// 处理读取错误
		}
	}()

	return nil
}
