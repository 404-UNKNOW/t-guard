package process

import (
	"context"
	"errors"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

var (
	// DefaultWhitelist 默认命令白名单
	DefaultWhitelist = []string{"python", "node", "curl", "echo", "go"}
	
	// ErrCommandNotAllowed 命令不在白名单中
	ErrCommandNotAllowed = errors.New("command not allowed")
	
	// SensitiveKeywords 敏感环境变量关键词
	SensitiveKeywords = []string{"KEY", "TOKEN", "SECRET", "PASSWORD"}
)

// SecureExecutor 提供安全受限的命令执行
type SecureExecutor struct {
	Whitelist []string
	Timeout   time.Duration
}

// NewSecureExecutor 创建默认安全执行器
func NewSecureExecutor() *SecureExecutor {
	return NewSecureExecutorWithConfig(DefaultWhitelist, 30*time.Second)
}

// NewSecureExecutorWithConfig 创建带自定义配置的安全执行器
func NewSecureExecutorWithConfig(whitelist []string, timeout time.Duration) *SecureExecutor {
	if len(whitelist) == 0 {
		whitelist = DefaultWhitelist
	}
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	return &SecureExecutor{
		Whitelist: whitelist,
		Timeout:   timeout,
	}
}

// PrepareCommand 验证命令并准备 exec.Cmd，自动过滤敏感环境变量
func (e *SecureExecutor) PrepareCommand(ctx context.Context, cfg ChildConfig) (*exec.Cmd, error) {
	// 1. 命令白名单校验
	cmdBase := filepath.Base(cfg.Command)
	allowed := false
	for _, a := range e.Whitelist {
		if cmdBase == a {
			allowed = true
			break
		}
	}
	if !allowed {
		return nil, ErrCommandNotAllowed
	}

	// 2. 强制使用数组形式执行，禁止 sh -c 注入
	// 已经在 NewManager 中使用 exec.Command(cfg.Command, cfg.Args...) 实现

	// 3. 环境变量过滤
	fullEnv := prepareEnv(cfg)
	filteredEnv := make([]string, 0, len(fullEnv))
	for _, env := range fullEnv {
		if !e.isSensitive(env) {
			filteredEnv = append(filteredEnv, env)
		}
	}

	// 4. 应用超时 (使用带超时的 Context)
	runCtx, cancel := context.WithTimeout(ctx, e.Timeout)
	_ = cancel // 由调用方负责或在进程结束时自动释放

	cmd := exec.CommandContext(runCtx, cfg.Command, cfg.Args...)
	cmd.Env = filteredEnv
	cmd.Dir = cfg.WorkingDir
	
	return cmd, nil
}

// isSensitive 检查环境变量是否包含敏感关键词
func (e *SecureExecutor) isSensitive(env string) bool {
	parts := strings.SplitN(env, "=", 2)
	if len(parts) == 0 {
		return false
	}
	key := strings.ToUpper(parts[0])
	
	// 豁免 TGUARD_ 及其自身的配置
	if strings.HasPrefix(key, "TGUARD_") || strings.HasPrefix(key, "OPENAI_") || strings.HasPrefix(key, "ANTHROPIC_") {
		return false
	}

	for _, kw := range SensitiveKeywords {
		if strings.Contains(key, kw) {
			return true
		}
	}
	return false
}
