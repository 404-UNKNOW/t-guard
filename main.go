package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"t-guard/internal/app"
	"t-guard/internal/process"
	"t-guard/pkg/budget"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	// 1. 模拟配置加载 (Stable 模式建议使用统一 DI)
	cfg := &app.Config{
		DataDir: ".",
		Listen:  "127.0.0.1:8080",
		Project: "test-project",
		Upstreams: map[string]string{
			"openai": "https://api.openai.com",
		},
		Budget: []budget.BudgetConfig{
			{
				Project:   "test-project",
				HardLimit: 1000000,
				SoftLimit: 0.8,
			},
		},
	}

	// 2. 初始化全量应用
	application, cleanup, err := app.InitializeApp(cfg)
	if err != nil {
		log.Fatalf("Initialization failed: %v", err)
	}
	defer cleanup()

	// 3. 安全性校验
	if _, err := application.Security.Verify(); err != nil {
		log.Fatalf("Security check failed: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 4. 启动代理
	go func() {
		if err := application.Proxy.Start(ctx); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Proxy failed: %v", err)
		}
	}()

	// 同步：健康检查
	if err := app.WaitForHealth(cfg.Listen, 5*time.Second); err != nil {
		log.Fatalf("Health check failed: %v", err)
	}

	// 5. 运行模式选择
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	if len(os.Args) > 1 {
		go func() {
			childCfg := process.ChildConfig{
				Command:   os.Args[1],
				Args:      os.Args[2:],
				ProxyAddr: cfg.Listen,
				AuthKey:   cfg.AuthKey,
			}
			if err := application.Process.Run(ctx, childCfg); err != nil {
				log.Printf("Process error: %v", err)
			}
			sigChan <- syscall.SIGTERM
		}()
	} else {
		p := tea.NewProgram(application.UI, tea.WithAltScreen())
		go func() {
			if _, err := p.Run(); err != nil {
				log.Printf("UI error: %v", err)
			}
			sigChan <- syscall.SIGTERM
		}()
	}

	// 6. 优雅关闭
	sig := <-sigChan
	fmt.Printf("\n[t-guard] Shutting down (Signal: %v)...\n", sig)
	cancel()
	_ = application.Process.Cleanup()
	time.Sleep(500 * time.Millisecond)
	fmt.Println("[t-guard] Goodbye.")
}
