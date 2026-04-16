package process

import (
	"fmt"
	"os"
	"strings"
)

// prepareEnv 准备子进程的环境变量
func prepareEnv(cfg ChildConfig) []string {
	env := os.Environ()
	
	// 1. 注入核心代理配置
	overrides := map[string]string{
		"OPENAI_BASE_URL":    "http://" + cfg.ProxyAddr + "/v1",
		"ANTHROPIC_BASE_URL": "http://" + cfg.ProxyAddr + "/v1",
		"TOKENFLOW_ACTIVE":   "1",
	}

	// 注入鉴权头信息 (供部分 SDK 自动识别或脚本使用)
	if cfg.AuthKey != "" {
		overrides["TGUARD_AUTH_TOKEN"] = cfg.AuthKey
	}

	// 2. 合并额外的 ExtraEnv
	for k, v := range cfg.ExtraEnv {
		overrides[k] = v
	}

	// 3. 合并逻辑：如果用户当前 Shell 中已经有了这些变量且不等于默认值，优先保留用户设置
	var finalEnv []string
	seen := make(map[string]bool)
	
	// 首先处理系统环境变量
	for _, kv := range env {
		parts := strings.SplitN(kv, "=", 2)
		if len(parts) == 2 {
			if _, ok := overrides[parts[0]]; ok {
				seen[parts[0]] = true
			}
			finalEnv = append(finalEnv, kv)
		}
	}

	// 将缺失的注入变量补全
	for k, v := range overrides {
		if !seen[k] {
			finalEnv = append(finalEnv, fmt.Sprintf("%s=%s", k, v))
		}
	}

	return finalEnv
}
