package process

import (
	"context"
	"testing"
)

func TestSecureExecutor_Whitelist(t *testing.T) {
	executor := NewSecureExecutor()
	ctx := context.Background()

	t.Run("AllowedCommand", func(t *testing.T) {
		cfg := ChildConfig{Command: "python"}
		_, err := executor.PrepareCommand(ctx, cfg)
		if err != nil {
			t.Errorf("Expected python to be allowed, got error: %v", err)
		}
	})

	t.Run("DisallowedCommand", func(t *testing.T) {
		cfg := ChildConfig{Command: "rm"}
		_, err := executor.PrepareCommand(ctx, cfg)
		if err != ErrCommandNotAllowed {
			t.Errorf("Expected rm to be disallowed, got error: %v", err)
		}
	})

	t.Run("PathBypassAttempt", func(t *testing.T) {
		cfg := ChildConfig{Command: "/usr/bin/python"}
		_, err := executor.PrepareCommand(ctx, cfg)
		if err != nil {
			t.Errorf("Expected path bypass attempt with python to be allowed (via base), got error: %v", err)
		}
	})
}

func TestSecureExecutor_EnvFiltering(t *testing.T) {
	executor := NewSecureExecutor()
	ctx := context.Background()

	t.Run("FilterSensitiveEnv", func(t *testing.T) {
		cfg := ChildConfig{
			Command: "python",
			ExtraEnv: map[string]string{
				"MY_SECRET_KEY": "12345",
				"SAFE_VAR":      "hello",
				"API_TOKEN":     "abc",
			},
		}
		cmd, err := executor.PrepareCommand(ctx, cfg)
		if err != nil {
			t.Fatalf("PrepareCommand failed: %v", err)
		}

		foundSecret := false
		foundToken := false
		foundSafe := false

		for _, env := range cmd.Env {
			if contains(env, "MY_SECRET_KEY") {
				foundSecret = true
			}
			if contains(env, "API_TOKEN") {
				foundToken = true
			}
			if contains(env, "SAFE_VAR") {
				foundSafe = true
			}
		}

		if foundSecret {
			t.Error("MY_SECRET_KEY should have been filtered out")
		}
		if foundToken {
			t.Error("API_TOKEN should have been filtered out")
		}
		if !foundSafe {
			t.Error("SAFE_VAR should have been preserved")
		}
	})

	t.Run("AllowTGuardEnv", func(t *testing.T) {
		cfg := ChildConfig{
			Command: "python",
			AuthKey: "secret-auth",
		}
		cmd, err := executor.PrepareCommand(ctx, cfg)
		if err != nil {
			t.Fatalf("PrepareCommand failed: %v", err)
		}

		foundAuth := false
		for _, env := range cmd.Env {
			if contains(env, "TGUARD_AUTH_TOKEN") {
				foundAuth = true
			}
		}
		if !foundAuth {
			t.Error("TGUARD_AUTH_TOKEN should be preserved even if it contains 'TOKEN'")
		}
	})
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || (len(substr) > 0 && (s[:len(substr)] == substr || contains(s[1:], substr))))
}
