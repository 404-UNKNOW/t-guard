package proxy

import (
	"bufio"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"t-guard/pkg/budget"
	"t-guard/pkg/logger"
	"t-guard/pkg/pricing"
	"t-guard/pkg/route"
	"t-guard/pkg/store"
	"t-guard/pkg/token"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	logger.Init()
	os.Exit(m.Run())
}

type mockPricing struct{}
func (m *mockPricing) CalculateCost(model string, input, output int) int64 { 
	if output == 4000 { return 10 } // Pre-deduction estimate is small
	return int64(output * 20)      // Actual cost in SSE is large
}
func (m *mockPricing) UpdatePrices(prices map[string]pricing.ModelPrice) {}

type mockToken struct{}
func (m *mockToken) Calculate(ctx context.Context, req token.CalcRequest) (token.CalcResult, error) {
	return token.CalcResult{TokenCount: 1}, nil
}
func (m *mockToken) Warmup(model string) error { return nil }
func (m *mockToken) Close() error                 { return nil }

// 验收标准：流式拦截与中途熔断测试
func TestProxy_StreamingBreaker(t *testing.T) {
	// 1. 模拟上游服务器：持续发送数据直至被代理切断
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		
		flusher, ok := w.(http.Flusher)
		if !ok {
			return
		}

		for i := 0; i < 20; i++ {
			// 发送足够大的 chunk 触发预算限额
			_, _ = fmt.Fprintf(w, "data: {\"choices\":[{\"delta\":{\"content\":\"a\"}}]}\n\n")
			flusher.Flush()
			time.Sleep(20 * time.Millisecond)
		}
	}))
	defer upstream.Close()

	upstreamURL, _ := url.Parse(upstream.URL)

	// 2. 环境初始化
	dbPath := "test_proxy.db"
	s, _ := store.NewSQLiteStore(dbPath)
	defer s.Close()

	router := route.NewEngine()
	_ = router.LoadRules([]route.Rule{{
		ID: "r1", 
		Action: route.RouteAction{Target: "upstream-1"},
	}})
	
	// 设置硬限制：100 毫美分
	// 预扣减 10 (通过)，每行 SSE 增加 20。第 5 行左右应触发熔断。
	billing, _ := budget.NewController(s, []budget.BudgetConfig{{
		Project:   "p1",
		HardLimit: 100,
	}})

	cfg := Config{
		ListenAddr:    "127.0.0.1:0", // 动态端口
		Upstreams:     map[string]*url.URL{"upstream-1": upstreamURL},
		DefaultTarget: "upstream-1",
		Router:        router,
		Billing:       billing,
		Store:         s,
		Pricing:       &mockPricing{},
		Token:         &mockToken{},
		AuthKey:       "",
	}

	server := NewServer(cfg)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() { _ = server.Start(ctx) }()
	time.Sleep(200 * time.Millisecond) // 等待启动

	// 3. 客户端发起请求
	proxyAddr := server.Addr().String()
	client := &http.Client{}
	req, _ := http.NewRequest("GET", "http://"+proxyAddr, nil)
	req.Header.Set("X-Project-ID", "p1")
	req.Header.Set("X-Model-Target", "upstream-1")
	req.Header.Set("X-TGuard-Auth", "mock-auth") // 如果开启了认证

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Failed to call proxy: %v", err)
	}
	defer resp.Body.Close()

	// 4. 读取响应流并捕捉熔断信号
	scanner := bufio.NewScanner(resp.Body)
	hasExceeded := false
	for scanner.Scan() {
		line := scanner.Text()
		t.Logf("Line: %s", line)
		if strings.Contains(line, "insufficient_budget") {
			hasExceeded = true
			break
		}
	}

	if !hasExceeded {
		t.Errorf("Proxy failed to inject budget_exceeded signal into SSE stream")
	}
}
