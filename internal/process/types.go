package process

import (
	"context"
	"os"
	"time"
)

// ChildConfig 子进程启动配置
type ChildConfig struct {
	Command    string            // 可执行文件路径
	Args       []string          // 参数
	ExtraEnv   map[string]string // 需注入的环境变量
	WorkingDir string            // 工作目录
	ProxyAddr  string            // 代理地址
	AuthKey    string            // 准入令牌
}

// ProcessInfo 进程运行信息
type ProcessInfo struct {
	PID       int
	StartTime time.Time
	ExitCode  int
	ExitError error
}

// Manager 进程组管理接口
type Manager interface {
	// Run 启动子进程并阻塞直到退出
	Run(ctx context.Context, cfg ChildConfig) error
	
	// ForwardSignal 信号转发
	ForwardSignal(sig os.Signal) error
	
	// Cleanup 强行清理子进程
	Cleanup() error
}
