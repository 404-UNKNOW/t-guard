//go:build linux

package process

import "syscall"

func (m *unixManager) applyPlatformSysProcAttr() {
	if m.cmd.SysProcAttr != nil {
		m.cmd.SysProcAttr.Pdeathsig = syscall.SIGKILL
	}
}
