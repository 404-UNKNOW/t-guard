package app

import (
	"os"
	"testing"
)

func TestConfig_Security(t *testing.T) {
	// 准备一个测试配置文件
	configContent := `
data_dir: "./data"
listen: "127.0.0.1:8080"
auth_key: "plain-text-key"
`
	configFile := "test_config.yaml"
	if err := os.WriteFile(configFile, []byte(configContent), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	defer func() {
		_ = os.Remove(configFile)
	}()

	t.Run("EnvPriority", func(t *testing.T) {
		if err := os.Setenv("TGUARD_AUTH_KEY", "env-secret-key"); err != nil {
			t.Fatalf("Setenv failed: %v", err)
		}
		defer func() {
			_ = os.Unsetenv("TGUARD_AUTH_KEY")
		}()

		cfg, err := LoadConfig(configFile, false)
		if err != nil {
			t.Fatalf("LoadConfig failed: %v", err)
		}

		if cfg.AuthKey != "env-secret-key" {
			t.Errorf("Expected AuthKey to be 'env-secret-key', got '%s'", cfg.AuthKey)
		}
	})

	t.Run("InsecurePermissionsInProduction", func(t *testing.T) {
		// 0644 是世界可读，在生产模式下应该报错
		if err := os.Chmod(configFile, 0644); err != nil {
			t.Fatalf("Chmod failed: %v", err)
		}
		_, err := LoadConfig(configFile, true)
		if err == nil {
			t.Error("Should have failed due to insecure permissions, but succeeded")
		} else {
			t.Logf("Correctly failed with error: %v", err)
		}
	})

	t.Run("SecurePermissionsInProduction", func(t *testing.T) {
		// 0600 只有所有者可读写
		if err := os.Chmod(configFile, 0600); err != nil {
			t.Fatalf("Chmod failed: %v", err)
		}
		_, err := LoadConfig(configFile, true)
		if err != nil {
			t.Errorf("Should have succeeded with 0600 permissions, but failed: %v", err)
		}
	})

	t.Run("Masking", func(t *testing.T) {
		cfg := &Config{
			AuthKey: "secret",
			Project: "my-project",
		}
		masked := cfg.MaskConfig()
		if !contains(masked, "AuthKey: ***") {
			t.Errorf("MaskConfig did not mask AuthKey: %s", masked)
		}
		if !contains(masked, "Project: my-project") {
			t.Errorf("MaskConfig masked non-secure field: %s", masked)
		}
	})
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || (len(substr) > 0 && (s[:len(substr)] == substr || contains(s[1:], substr))))
}
