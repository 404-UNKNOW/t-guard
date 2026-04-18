package security

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"os"
	"testing"
	"time"
)

func TestSecurity_Fixes(t *testing.T) {
	// 生成测试用的 RSA 密钥对
	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("Failed to generate private key: %v", err)
	}
	pubKey := &privKey.PublicKey
	pubKeyBytes, _ := x509.MarshalPKIXPublicKey(pubKey)
	pubKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: pubKeyBytes,
	})

	// 将公钥写入临时文件并设置环境变量
	tmpKeyFile, _ := os.CreateTemp("", "pubkey-*.pem")
	defer os.Remove(tmpKeyFile.Name())
	tmpKeyFile.Write(pubKeyPEM)
	tmpKeyFile.Close()
	os.Setenv("TGUARD_PUBLIC_KEY_PATH", tmpKeyFile.Name())
	defer os.Unsetenv("TGUARD_PUBLIC_KEY_PATH")

	m, _ := Init(nil)

	// 辅助函数：生成签名后的许可证字符串
	generateLicense := func(info LicenseInfo) string {
		payload, _ := json.Marshal(info)
		hashed := sha256.Sum256(payload)
		sig, _ := rsa.SignPKCS1v15(rand.Reader, privKey, crypto.SHA256, hashed[:])
		return base64.StdEncoding.EncodeToString(payload) + "." + base64.StdEncoding.EncodeToString(sig)
	}

	// 1. 正常签名通过
	t.Run("NormalSignaturePass", func(t *testing.T) {
		info := LicenseInfo{
			Tier:      "pro",
			IssuedAt:  time.Now().Add(-1 * time.Hour),
			ExpiresAt: time.Now().Add(24 * time.Hour),
			Nonce:     "nonce-123",
		}
		licenseKey := generateLicense(info)
		err := m.Activate(licenseKey)
		if err != nil {
			t.Errorf("Should pass, but got error: %v", err)
		}
	})

	// 2. 过期许可证拒绝
	t.Run("ExpiredLicenseRejected", func(t *testing.T) {
		info := LicenseInfo{
			Tier:      "pro",
			IssuedAt:  time.Now().Add(-10 * time.Hour),
			ExpiresAt: time.Now().Add(-1 * time.Hour),
			Nonce:     "nonce-456",
		}
		licenseKey := generateLicense(info)
		err := m.Activate(licenseKey)
		if err == nil || err.Error() != "invalid signature" {
			t.Errorf("Should be rejected as invalid signature, but got: %v", err)
		}
	})

	// 3. 重放攻击拒绝
	t.Run("ReplayAttackRejected", func(t *testing.T) {
		info := LicenseInfo{
			Tier:      "pro",
			IssuedAt:  time.Now().Add(-1 * time.Hour),
			ExpiresAt: time.Now().Add(24 * time.Hour),
			Nonce:     "nonce-repeat",
		}
		licenseKey := generateLicense(info)
		
		// 第一次激活成功
		err := m.Activate(licenseKey)
		if err != nil {
			t.Fatalf("First activation failed: %v", err)
		}

		// 第二次激活（重放）失败
		err = m.Activate(licenseKey)
		if err == nil || err.Error() != "invalid signature" {
			t.Errorf("Replay should be rejected as invalid signature, but got: %v", err)
		}
	})
}

func TestSecurity_BackdoorRemoved(t *testing.T) {
	m, _ := Init(nil)
	// 构造以前能绕过验证的 payload
	payload := []byte(`{"tier":"enterprise","mock":"mock-test"}`)
	sig := []byte("fake-signature")
	licenseKey := base64.StdEncoding.EncodeToString(payload) + "." + base64.StdEncoding.EncodeToString(sig)

	err := m.Activate(licenseKey)
	if err == nil {
		t.Error("Backdoor still exists! Mock-test payload was accepted.")
	}
}
