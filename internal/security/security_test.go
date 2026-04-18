package security

import (
	"testing"
	"time"
)

// 验收标准：功能降级策略测试
func TestSecurity_CanUse(t *testing.T) {
	m, _ := Init(nil)

	// 1. 验证默认（免费版）
	if !m.CanUse(FeatureBasicProxy) {
		t.Error("免费版应能使用基础代理")
	}
	if m.CanUse(FeatureSmartRoute) {
		t.Error("免费版不应能使用智能路由")
	}

	// 2. 模拟激活 Pro 版
	sm := m.(*securityManager)
	sm.cache = LicenseInfo{
		Tier:      "pro",
		ExpiresAt: time.Now().Add(100 * time.Hour),
		IsValid:   true,
	}

	if !m.CanUse(FeatureSmartRoute) {
		t.Error("Pro 版应能使用智能路由")
	}
}

