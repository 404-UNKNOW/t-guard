package app

import (
	"fmt"
	"os"
	"t-guard/pkg/budget"

	"github.com/AlecAivazis/survey/v2"
	"gopkg.in/yaml.v3"
)

// RunWizard 开启交互式配置向导
func RunWizard() (*Config, error) {
	fmt.Println("🛡️ Welcome to T-Guard! Let's get you set up in 60 seconds.")
	fmt.Println("---------------------------------------------------------")

	var answers struct {
		Provider   string
		APIKey     string
		BudgetUSD  float64
		ListenPort string
		AuthToken  string
	}

	// 1. 定义交互问题
	var qs = []*survey.Question{
		{
			Name: "provider",
			Prompt: &survey.Select{
				Message: "Choose your primary AI provider:",
				Options: []string{"OpenAI", "Anthropic (Claude)", "Custom"},
				Default: "OpenAI",
			},
		},
		{
			Name: "apiKey",
			Prompt: &survey.Password{
				Message: "Enter your API Key (will be stored securely):",
			},
		},
		{
			Name: "budgetUSD",
			Prompt: &survey.Input{
				Message: "Daily budget limit (in USD):",
				Default: "10.0",
			},
		},
		{
			Name: "authToken",
			Prompt: &survey.Password{
				Message: "Set a T-Guard access token (your secret password for proxy access):",
			},
		},
	}

	// 2. 执行提问
	err := survey.Ask(qs, &answers)
	if err != nil {
		return nil, err
	}

	// 3. 构建配置对象
	upstreamURL := "https://api.openai.com"
	if answers.Provider == "Anthropic (Claude)" {
		upstreamURL = "https://api.anthropic.com"
	}

	cfg := &Config{
		DataDir: "./data",
		Listen:  "127.0.0.1:8080",
		Project: "default-project",
		AuthKey: answers.AuthToken,
		Upstreams: map[string]string{
			"default": upstreamURL,
		},
		Budget: []interface{}{
			budget.BudgetConfig{
				Project:   "default-project",
				HardLimit: int64(answers.BudgetUSD * 100000), // 转化为毫美分
				SoftLimit: 0.8,
			},
		},
	}

	// 4. 保存为 config.yaml
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.MkdirAll(cfg.DataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}

	if err := os.WriteFile("config.yaml", data, 0600); err != nil { // 使用更安全的 600 权限
		return nil, fmt.Errorf("failed to write config file: %w", err)
	}

	fmt.Println("\n✅ Configuration saved to config.yaml")
	fmt.Println("🔒 API Key has been received and will be utilized for this session.")
	
	return cfg, nil
}
