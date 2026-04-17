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
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var runCmd = &cobra.Command{
	Use:   "run [command...]",
	Short: "启动代理并运行子命令或监控 UI",
	Example: `  tokenflow run --project my-app
  tokenflow run claude-code
  tokenflow run --port 9000 -- openai-chat`,
	Run: func(cmd *cobra.Command, args []string) {
		// 1. 尝试加载配置
		var appCfg app.Config
		err := viper.Unmarshal(&appCfg)
		
		// 如果 DataDir 为空或 Upstreams 为空，说明可能没有加载到配置文件
		if err != nil || appCfg.DataDir == "" || len(appCfg.Upstreams) == 0 {
			fmt.Println("No configuration found.")
			wizardCfg, err := app.RunWizard()
			if err != nil {
				log.Fatalf("Wizard failed: %v", err)
			}
			appCfg = *wizardCfg
		}

		// 补全默认值逻辑
		if appCfg.Project == "" {
			appCfg.Project = "default-project"
		}
		if len(appCfg.Upstreams) == 0 {
			// 默认回退到 OpenAI
			appCfg.Upstreams = map[string]string{"openai": "https://api.openai.com"}
		}

		// 2. 初始化应用 (DI)
		application, cleanup, err := app.InitializeApp(&appCfg)
		if err != nil {
			log.Fatalf("启动前检查失败: %v", err)
		}
		defer cleanup()

		// 1b. 安全加固：尝试从 Keyring 获取 AuthKey (覆盖配置)
		if application.Security != nil {
			if secret, err := application.Security.RetrieveSecret("tguard_auth_key"); err == nil && len(secret) > 0 {
				application.Proxy.UpdateAuthKey(string(secret))
			}
		}

		// 3. 核心准入检查
		if info, err := application.Security.Verify(); err != nil || !info.IsValid {
			log.Fatalf("安全性校验未通过: %v (有效性: %v)", err, info.IsValid)
		}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// 4. 启动代理 (支持优雅降级)
		errSrv := make(chan error, 1)
		go func() {
			if err := application.Proxy.Start(ctx); err != nil && err != http.ErrServerClosed {
				errSrv <- err
			}
		}()

		// 同步等待健康检查
		select {
		case err := <-errSrv:
			log.Fatalf("代理启动失败: %v", err)
		case <-time.After(100 * time.Millisecond):
			if err := app.WaitForHealth(appCfg.Listen, 5*time.Second); err != nil {
				log.Fatalf("代理健康检查超时: %v", err)
			}
		}

		// 5. 信号处理
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

		// 6. 执行逻辑 (根据是否有参数切换模式)
		if len(args) > 0 {
			// 子进程代理模式 (Headless)
			go func() {
				childCfg := process.ChildConfig{
					Command:   args[0],
					Args:      args[1:],
					ProxyAddr: appCfg.Listen,
					AuthKey:   appCfg.AuthKey,
				}
				if err := application.Process.Run(ctx, childCfg); err != nil {
					log.Printf("子进程运行异常: %v", err)
				}
				sigChan <- syscall.SIGTERM
			}()
		} else {
			// TUI 监控交互模式
			p := tea.NewProgram(application.UI, tea.WithAltScreen())
			go func() {
				if _, err := p.Run(); err != nil {
					log.Printf("UI 终端运行异常: %v", err)
				}
				sigChan <- syscall.SIGTERM
			}()
		}

		// 7. 优雅关闭序列
		sig := <-sigChan
		fmt.Printf("\n收到信号 %v，正在安全退出...\n", sig)
		cancel()
		
		// 给予子进程和清理工作时间
		_ = application.Process.Cleanup()
		time.Sleep(500 * time.Millisecond)
		fmt.Println("T-Guard 已关闭。")
	},
}

func init() {
	rootCmd.AddCommand(runCmd)
}
