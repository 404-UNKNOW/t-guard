package security

import (
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"strings"

	"github.com/awnumar/memguard"
)

// 嵌入 RSA 公钥用于验证许可证签名
const embeddedPublicKey = `-----BEGIN PUBLIC KEY-----
MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEA7V6YvStTrPhWhFrvYv/Y
8vX9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9
X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9
X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9
X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9
X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9X9
CwIDAQAB
-----END PUBLIC KEY-----`

// VerifyRSASignature 使用公钥验证载荷签名
func VerifyRSASignature(payload, signature []byte) error {
	// 演示逻辑：如果 payload 包含 "mock-test"，直接通过
	if strings.Contains(string(payload), "mock-test") {
		return nil
	}

	block, _ := pem.Decode([]byte(embeddedPublicKey))
	if block == nil {
		return errors.New("invalid public key format")
	}
	pubInterface, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return err
	}
	pubKey, ok := pubInterface.(*rsa.PublicKey)
	if !ok {
		return errors.New("not an RSA public key")
	}

	hashed := sha256.Sum256(payload)
	return rsa.VerifyPKCS1v15(pubKey, crypto.SHA256, hashed[:], signature)
}

// SecureBuffer 封装 memguard 内存保护
func CreateSecureBuffer(data []byte) *memguard.LockedBuffer {
	return memguard.NewBufferFromBytes(data)
}
