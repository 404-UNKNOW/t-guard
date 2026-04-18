package proxy

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"t-guard/pkg/ratelimit"
	"testing"
)

func TestProxy_RateLimit(t *testing.T) {
	// 1. 初始化限流器 (低阈值用于测试)
	limiter := ratelimit.NewLimiter(ratelimit.Config{
		IPRate:    1,
		IPBurst:   5,
		UserRate:  1,
		UserBurst: 5,
	})

	cfg := Config{
		RateLimit: limiter,
	}

	// 我们只需要测试 s.handler 的限流逻辑，不需要完整的代理转发
	s := NewServer(cfg).(*proxyServer)

	// 2. 发起并发请求
	totalRequests := 100
	var wg sync.WaitGroup
	results := make(chan int, totalRequests)

	for i := 0; i < totalRequests; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req := httptest.NewRequest("GET", "http://localhost/test", nil)
			req.Header.Set("X-Forwarded-For", "1.1.1.1")
			w := httptest.NewRecorder()
			s.handler.ServeHTTP(w, req)
			results <- w.Code
		}()
	}

	wg.Wait()
	close(results)

	// 3. 统计结果
	passed := 0
	rejected := 0
	for code := range results {
		if code == http.StatusOK || code == http.StatusNotFound { // NotFound 是因为我们没配置后端，但能走到那说明过了限流
			passed++
		} else if code == http.StatusTooManyRequests {
			rejected++
		}
	}

	t.Logf("Total: %d, Passed: %d, Rejected: %d", totalRequests, passed, rejected)

	if rejected == 0 {
		t.Error("Rate limiter did not reject any requests")
	}
	if passed > 6 { // burst(5) + 1s(1) 左右
		t.Errorf("Rate limiter allowed too many requests: %d", passed)
	}
}
