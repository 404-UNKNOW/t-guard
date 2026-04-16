package process

import (
	"context"
	"testing"
	"time"
)

// 验收标准：环境变量隔离与注入测试
func TestEnv_Prepare(t *testing.T) {
	cfg := ChildConfig{
		ProxyAddr: "localhost:9999",
		ExtraEnv:  map[string]string{"CUSTOM_VAR": "value"},
	}
	env := prepareEnv(cfg)
	
	foundProxy := false
	foundCustom := false
	foundActive := false
	for _, e := range env {
		if e == "OPENAI_BASE_URL=http://localhost:9999/v1" {
			foundProxy = true
		}
		if e == "CUSTOM_VAR=value" {
			foundCustom = true
		}
		if e == "TOKENFLOW_ACTIVE=1" {
			foundActive = true
		}
	}
	
	if !foundProxy { t.Error("Proxy env injection failed") }
	if !foundCustom { t.Error("Custom env injection failed") }
	if !foundActive { t.Error("Active flag injection failed") }
}

// 验收标准：基本的进程启动与生命周期测试
func TestManager_Run(t *testing.T) {
	m := NewManager()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// 使用 go version 作为一个轻量级的跨平台命令
	cfg := ChildConfig{
		Command:   "go",
		Args:      []string{"version"},
		ProxyAddr: "127.0.0.1:8080",
	}

	err := m.Run(ctx, cfg)
	if err != nil {
		t.Errorf("Process failed to run: %v", err)
	}
}
