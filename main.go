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
	"t-guard/pkg/logger"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	// 初始化日志
	logger.Init()

	// 1. 尝试加载配置，若不存在则运行向导
	cfg := &app.Config{
		DataDir: ".",
		Listen:  "127.0.0.1:8080",
		Project: "test-project",
	}

	if _, err := os.Stat("config.yaml"); os.IsNotExist(err) {
		fmt.Println("Config file not found.")
		wizardCfg, err := app.RunWizard()
		if err != nil {
			log.Fatalf("Wizard failed: %v", err)
		}
		cfg = wizardCfg
	} else {
		// 正常逻辑：生产模式加载配置并进行权限校验
		loadedCfg, err := app.LoadConfig("config.yaml", true)
		if err != nil {
			log.Fatalf("Failed to load config: %v", err)
		}
		cfg = loadedCfg
		log.Printf("[t-guard] Configuration loaded: %s", cfg.MaskConfig())
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
	
	// 设置优雅关闭超时
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	cancel() // 通知所有 context 退出
	
	// 执行清理逻辑
	_ = application.Process.Cleanup()
	
	// 等待清理完成或超时
	select {
	case <-shutdownCtx.Done():
		fmt.Println("[t-guard] Shutdown timed out.")
	case <-time.After(500 * time.Millisecond):
		fmt.Println("[t-guard] Goodbye.")
	}
}
