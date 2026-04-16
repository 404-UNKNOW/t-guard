//go:build windows

package process

import (
	"context"
	"os"
	"os/exec"
	"unsafe"

	"golang.org/x/sys/windows"
)

type windowsManager struct {
	cmd *exec.Cmd
	job windows.Handle
}

func NewManager() Manager {
	return &windowsManager{}
}

func (m *windowsManager) Run(ctx context.Context, cfg ChildConfig) error {
	m.cmd = exec.CommandContext(ctx, cfg.Command, cfg.Args...)
	m.cmd.Env = prepareEnv(cfg)
	m.cmd.Dir = cfg.WorkingDir
	m.cmd.Stdout = os.Stdout
	m.cmd.Stderr = os.Stderr
	m.cmd.Stdin = os.Stdin

	// 1. 创建 Job Object
	handle, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return err
	}
	m.job = handle

	// 2. 设置 Kill-on-Close 限制
	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{
		BasicLimitInformation: windows.JOBOBJECT_BASIC_LIMIT_INFORMATION{
			LimitFlags: windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE,
		},
	}
	
	_, err = windows.SetInformationJobObject(
		handle,
		windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&info)),
		uint32(unsafe.Sizeof(info)),
	)
	if err != nil {
		windows.CloseHandle(handle)
		return err
	}

	// 3. 启动进程
	if err := m.cmd.Start(); err != nil {
		windows.CloseHandle(handle)
		return err
	}

	// 4. 将启动的进程分配给 Job (需要获取进程句柄)
	hProcess, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE, false, uint32(m.cmd.Process.Pid))
	if err == nil {
		defer windows.CloseHandle(hProcess)
		_ = windows.AssignProcessToJobObject(handle, hProcess)
	}

	return m.cmd.Wait()
}

func (m *windowsManager) ForwardSignal(sig os.Signal) error {
	if m.cmd == nil || m.cmd.Process == nil {
		return nil
	}
	return m.cmd.Process.Signal(sig)
}

func (m *windowsManager) Cleanup() error {
	if m.job != 0 {
		_ = windows.TerminateJobObject(m.job, 1)
		windows.CloseHandle(m.job)
		m.job = 0
	}
	if m.cmd != nil && m.cmd.Process != nil {
		return m.cmd.Process.Kill()
	}
	return nil
}
