//go:build darwin

package process

func (m *unixManager) applyPlatformSysProcAttr() {
	// Darwin (macOS) 不直接支持 SysProcAttr 中的 Pdeathsig。
	// 依靠进程组管理与 Cleanup() 逻辑实现资源清理。
}
