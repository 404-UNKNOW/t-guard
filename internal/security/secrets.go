package security

import (
	"crypto/subtle"
	"os"
	"strings"

	"github.com/99designs/keyring"
)

// SecretProvider 提供敏感凭证的统一获取入口
type SecretProvider struct {
	kr keyring.Keyring
}

// NewSecretProvider 初始化凭证提供者
func NewSecretProvider() *SecretProvider {
	kr, _ := initKeyring()
	return &SecretProvider{kr: kr}
}

// GetAPIKey 获取 API Key，优先级：Keyring > 环境变量 > 配置文件占位符
func (p *SecretProvider) GetAPIKey(provider string, configValue string) string {
	provider = strings.ToUpper(provider)
	
	// 1. 尝试从 Keyring 获取
	if p.kr != nil {
		item, err := p.kr.Get(provider + "_API_KEY")
		if err == nil {
			return string(item.Data)
		}
	}

	// 2. 尝试从环境变量获取 (例如 TG_GUARD_OPENAI_API_KEY)
	envKey := "TG_GUARD_" + provider + "_API_KEY"
	if val := os.Getenv(envKey); val != "" {
		return val
	}

	// 3. 回退到配置文件
	return configValue
}

// SecureCompare 使用恒定时间比较防止时序攻击
func SecureCompare(provided, expected string) bool {
	return subtle.ConstantTimeCompare([]byte(provided), []byte(expected)) == 1
}
