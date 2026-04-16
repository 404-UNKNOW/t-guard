package security

import (
	"encoding/base64"
	"encoding/json"
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

// 验收标准：RSA 演示逻辑激活测试
func TestSecurity_Activate(t *testing.T) {
	m, _ := Init(nil)

	// 构造包含 "mock-test" 的合法 JSON payload
	info := LicenseInfo{
		Tier:      "enterprise",
		ExpiresAt: time.Now().Add(24 * time.Hour),
		Mock:      "mock-test",
	}
	payload, _ := json.Marshal(info)
	
	payloadBase64 := base64.StdEncoding.EncodeToString(payload)
	sigBase64 := base64.StdEncoding.EncodeToString([]byte("dummy-signature"))
	
	licenseKey := payloadBase64 + "." + sigBase64
	
	err := m.Activate(licenseKey)
	if err != nil {
		t.Errorf("演示模式激活失败: %v", err)
	}

	verified, _ := m.Verify()
	if verified.Tier != "enterprise" || !verified.IsValid {
		t.Error("激活后状态未正确更新")
	}
}
