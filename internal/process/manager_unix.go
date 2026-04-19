//go:build !windows

package process

import (
	"context"
	"os"
	"os/exec"
	"syscall"
	"time"
)

type unixManager struct {
	cmd      *exec.Cmd
	executor *SecureExecutor
}

func NewManager(cfg Config) Manager {
	return &unixManager{
		executor: NewSecureExecutorWithConfig(cfg.Whitelist, cfg.Timeout),
	}
}

func (m *unixManager) Run(ctx context.Context, cfg ChildConfig) error {
	cmd, err := m.executor.PrepareCommand(ctx, cfg)
	if err != nil {
		return err
	}
	m.cmd = cmd
	m.cmd.Stdout = os.Stdout
	m.cmd.Stderr = os.Stderr
	m.cmd.Stdin = os.Stdin

	// 1. 设置 PGID (Process Group ID) 以便统一管理整个进程树
	m.cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}
	m.applyPlatformSysProcAttr()

	// 2. 启动子进程
	if err := m.cmd.Start(); err != nil {
		return err
	}

	// 3. 阻塞等待子进程退出，确保资源回收 (避免僵尸进程)
	// 使用 chan 配合 Wait 确保即使 context 取消也能正确处理 Wait 逻辑
	waitDone := make(chan error, 1)
	go func() {
		waitDone <- m.cmd.Wait()
	}()

	select {
	case err := <-waitDone:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (m *unixManager) ForwardSignal(sig os.Signal) error {
	if m.cmd == nil || m.cmd.Process == nil {
		return nil
	}

	// 向整个进程组发送信号 (负 PID 表示 PGID)
	pgid, err := syscall.Getpgid(m.cmd.Process.Pid)
	if err != nil {
		return m.cmd.Process.Signal(sig)
	}

	return syscall.Kill(-pgid, sig.(syscall.Signal))
}

func (m *unixManager) Cleanup() error {
	if m.cmd == nil || m.cmd.Process == nil {
		return nil
	}

	// 1. 尝试优雅终止 (SIGTERM)
	_ = m.ForwardSignal(syscall.SIGTERM)
	
	// 2. 给予 5 秒宽限期
	done := make(chan struct{})
	go func() {
		_ = m.cmd.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-time.After(5 * time.Second):
		// 3. 超时强制杀掉整个进程组 (SIGKILL)
		pgid, _ := syscall.Getpgid(m.cmd.Process.Pid)
		return syscall.Kill(-pgid, syscall.SIGKILL)
	}
}
