package security

import (
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"log"
	"os"
	"sync"
	"time"

	"github.com/awnumar/memguard"
)

// 嵌入 RSA 公钥用于验证许可证签名（作为备选）
const embeddedPublicKey = `-----BEGIN PUBLIC KEY-----
MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEA7V6YvStTrPhWhFrvYv/Y
8vX9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9
X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9
X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9
X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9
X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9
CwIDAQAB
-----END PUBLIC KEY-----`

var (
	usedNonces sync.Map // 用于防重放攻击
	ErrInvalidSignature = errors.New("invalid signature")
)

// VerifyRSASignature 使用公钥验证载荷签名
func VerifyRSASignature(payload, signature []byte) error {
	pubKeyData := []byte(embeddedPublicKey)

	// 优先从环境变量加载公钥
	if path := os.Getenv("TGUARD_PUBLIC_KEY_PATH"); path != "" {
		data, err := os.ReadFile(path)
		if err == nil {
			pubKeyData = data
		} else {
			log.Printf("[SECURITY] Failed to read public key from %s: %v", path, err)
		}
	}

	block, _ := pem.Decode(pubKeyData)
	if block == nil {
		log.Printf("[SECURITY] Invalid public key format")
		return ErrInvalidSignature
	}
	pubInterface, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		log.Printf("[SECURITY] ParsePKIXPublicKey failed: %v", err)
		return ErrInvalidSignature
	}
	pubKey, ok := pubInterface.(*rsa.PublicKey)
	if !ok {
		log.Printf("[SECURITY] Not an RSA public key")
		return ErrInvalidSignature
	}

	hashed := sha256.Sum256(payload)
	if err := rsa.VerifyPKCS1v15(pubKey, crypto.SHA256, hashed[:], signature); err != nil {
		log.Printf("[SECURITY] RSA verification failed: %v", err)
		return ErrInvalidSignature
	}

	return nil
}

// VerifyLicense 检查时间有效性和 Nonce
func VerifyLicense(info LicenseInfo) error {
	now := time.Now()

	// 1. 检查有效时间范围
	if now.Before(info.IssuedAt) || now.After(info.ExpiresAt) {
		log.Printf("[SECURITY] License expired or not yet valid: IssuedAt=%v, ExpiresAt=%v, Now=%v", info.IssuedAt, info.ExpiresAt, now)
		return ErrInvalidSignature
	}

	// 2. 防重放攻击检查
	if info.Nonce == "" {
		log.Printf("[SECURITY] Missing nonce in license")
		return ErrInvalidSignature
	}
	if _, loaded := usedNonces.LoadOrStore(info.Nonce, struct{}{}); loaded {
		log.Printf("[SECURITY] Replay attack detected: Nonce %s already used", info.Nonce)
		return ErrInvalidSignature
	}

	return nil
}

// CreateSecureBuffer 封装 memguard 内存保护
func CreateSecureBuffer(data []byte) *memguard.LockedBuffer {
	return memguard.NewBufferFromBytes(data)
}
