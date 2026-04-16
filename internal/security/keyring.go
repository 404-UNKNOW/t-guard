package security

import (
	"github.com/99designs/keyring"
)

type keyringManager struct {
	kr keyring.Keyring
}

// initKeyring 按照优先级开启系统密钥链
func initKeyring() (keyring.Keyring, error) {
	return keyring.Open(keyring.Config{
		ServiceName: "tokenflow",
		AllowedBackends: []keyring.BackendType{
			keyring.KeychainBackend,      // macOS
			keyring.WinCredBackend,       // Windows
			keyring.SecretServiceBackend,  // Linux
			keyring.FileBackend,           // 回退方案
		},
		// 文件回退时的加密引导（此处在生产环境应根据机器指纹生成）
		FileDir:          "~/.tokenflow/secrets",
		FilePasswordFunc: func(s string) (string, error) { return "tf-default-pass", nil },
	})
}
