//go:build !windows

package process

import (
	"context"
	"os"
	"os/exec"
	"syscall"
)

type unixManager struct {
	cmd *exec.Cmd
}

func NewManager() Manager {
	return &unixManager{}
}

func (m *unixManager) Run(ctx context.Context, cfg ChildConfig) error {
	m.cmd = exec.CommandContext(ctx, cfg.Command, cfg.Args...)
	m.cmd.Env = prepareEnv(cfg)
	m.cmd.Dir = cfg.WorkingDir
	m.cmd.Stdout = os.Stdout
	m.cmd.Stderr = os.Stderr
	m.cmd.Stdin = os.Stdin

	// 通用 Unix 进程组管理
	m.cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
		Pgid:    0,
	}

	// 调用平台特定的 SysProcAttr 补丁（由 platform_unix.go 提供）
	m.applyPlatformSysProcAttr()

	return m.cmd.Run()
}

func (m *unixManager) ForwardSignal(sig os.Signal) error {
	if m.cmd == nil || m.cmd.Process == nil {
		return nil
	}
	pid := m.cmd.Process.Pid
	if s, ok := sig.(syscall.Signal); ok {
		return syscall.Kill(-pid, s)
	}
	return m.cmd.Process.Signal(sig)
}

func (m *unixManager) Cleanup() error {
	if m.cmd == nil || m.cmd.Process == nil {
		return nil
	}
	pid := m.cmd.Process.Pid
	_ = syscall.Kill(-pid, syscall.SIGTERM)
	return syscall.Kill(-pid, syscall.SIGKILL)
}
