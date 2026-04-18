package security

import (
	"time"
)

type Feature string

const (
	FeatureBasicProxy Feature = "basic_proxy"   // 免费版功能
	FeatureSmartRoute Feature = "smart_routing" // Pro版功能
	FeatureUnlimited  Feature = "unlimited"     // 企业版功能
	FeatureTeamSync   Feature = "team_sync"
)

// LicenseInfo 描述许可证状态
type LicenseInfo struct {
	Tier        string    `json:"tier"` // "free", "pro", "enterprise"
	IssuedAt    time.Time `json:"issued_at"`
	ExpiresAt   time.Time `json:"expires_at"`
	Nonce       string    `json:"nonce"`
	Features    []Feature `json:"features"`
	MaxProjects int       `json:"max_projects"`
	IsValid     bool      `json:"is_valid"`
	GracePeriod bool      `json:"grace_period"` // 离线宽限期
}

// Manager 核心安全接口
type Manager interface {
	// License 管理
	Activate(licenseKey string) error
	Verify() (LicenseInfo, error)
	Deactivate() error

	// 功能降级检查
	CanUse(feature Feature) bool
	GetFeatureLimit(feature Feature) (int, error)

	// 密钥安全管理 (使用 memguard + keyring)
	StoreSecret(key string, value []byte) error
	RetrieveSecret(key string) ([]byte, error)
	ClearSecret(key string) error
}
