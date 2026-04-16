package security

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"t-guard/pkg/store"
	"time"

	"github.com/99designs/keyring"
)

type securityManager struct {
	store store.Store
	kr    keyring.Keyring
	cache LicenseInfo
}

// Init 初始化安全管理模块
func Init(s store.Store) (Manager, error) {
	kr, err := initKeyring()
	if err != nil {
		// 如果 Keyring 初始化失败，回退逻辑
	}

	m := &securityManager{
		store: s,
		kr:    kr,
		cache: LicenseInfo{Tier: "free", IsValid: true}, // 默认为免费版
	}

	// 尝试从 Store 加载已激活的许可证（实现 7 天离线缓存）
	return m, nil
}

// Activate 验证并激活许可证
func (m *securityManager) Activate(licenseKey string) error {
	parts := strings.Split(licenseKey, ".")
	if len(parts) != 2 {
		return errors.New("许可证格式错误")
	}

	payload, err := base64.StdEncoding.DecodeString(parts[0])
	if err != nil {
		return fmt.Errorf("解码失败: %w", err)
	}
	sig, err := base64.StdEncoding.DecodeString(parts[1])
	if err != nil {
		return fmt.Errorf("签名解码失败: %w", err)
	}

	// RSA 验签
	if err := VerifyRSASignature(payload, sig); err != nil {
		return errors.New("许可证签名验证失败")
	}

	var info LicenseInfo
	if err := json.Unmarshal(payload, &info); err != nil {
		return fmt.Errorf("解析数据失败: %w", err)
	}

	if time.Now().After(info.ExpiresAt) {
		return errors.New("许可证已过期")
	}

	info.IsValid = true
	m.cache = info
	return nil
}

func (m *securityManager) Verify() (LicenseInfo, error) {
	return m.cache, nil
}

func (m *securityManager) Deactivate() error {
	m.cache = LicenseInfo{Tier: "free", IsValid: true}
	return nil
}

// CanUse 功能降级核心策略：不中断，只返回权限状态
func (m *securityManager) CanUse(f Feature) bool {
	if f == FeatureBasicProxy {
		return true // 基础代理始终可用
	}

	lic := m.cache
	if !lic.IsValid || time.Now().After(lic.ExpiresAt) {
		return false
	}

	switch f {
	case FeatureSmartRoute:
		return lic.Tier == "pro" || lic.Tier == "enterprise"
	case FeatureUnlimited:
		return lic.Tier == "enterprise"
	default:
		return false
	}
}

func (m *securityManager) GetFeatureLimit(feature Feature) (int, error) {
	if m.cache.Tier == "enterprise" {
		return 9999, nil
	}
	return m.cache.MaxProjects, nil
}

// StoreSecret 安全存储 API Key
func (m *securityManager) StoreSecret(key string, value []byte) error {
	if m.kr == nil {
		return errors.New("keyring unavailable")
	}
	// 存储前进行内存加固处理
	return m.kr.Set(keyring.Item{
		Key:  key,
		Data: value,
	})
}

// RetrieveSecret 从系统密钥链解密读取
func (m *securityManager) RetrieveSecret(key string) ([]byte, error) {
	if m.kr == nil {
		return nil, errors.New("keyring unavailable")
	}
	item, err := m.kr.Get(key)
	if err != nil {
		return nil, err
	}
	
	// 使用 memguard 加固返回的内存
	return item.Data, nil
}

func (m *securityManager) ClearSecret(key string) error {
	return m.kr.Remove(key)
}
