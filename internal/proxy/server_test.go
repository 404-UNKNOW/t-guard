package proxy

import (
	"bufio"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"t-guard/pkg/budget"
	"t-guard/pkg/route"
	"t-guard/pkg/store"
	"testing"
	"time"
)

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
			_, _ = fmt.Fprintf(w, "data: long-content-chunk-%d\n\n", i)
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
	// 设置极低硬限制：50 毫美分
	billing, _ := budget.NewController(s, []budget.BudgetConfig{{
		Project:   "p1",
		HardLimit: 50,
	}})

	cfg := Config{
		ListenAddr:    "127.0.0.1:0", // 动态端口
		Upstreams:     map[string]*url.URL{"default": upstreamURL},
		DefaultTarget: "default",
		Router:        router,
		Billing:       billing,
		Store:         s,
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
	req.Header.Set("X-Model-Target", "gpt-4")

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
		if strings.Contains(line, "budget_exceeded") {
			hasExceeded = true
			break
		}
	}

	if !hasExceeded {
		t.Errorf("Proxy failed to inject budget_exceeded signal into SSE stream")
	}
}
