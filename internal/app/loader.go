package app

import (
	"fmt"
	"os"
	"reflect"
	"strings"

	"github.com/spf13/viper"
)

type ProcessConfig struct {
	Whitelist []string      `mapstructure:"whitelist"`
	Timeout   string        `mapstructure:"timeout"`
}

type Config struct {
	DataDir    string                `mapstructure:"data_dir"`
	ConfigFile string                `mapstructure:"config_file"`
	Listen     string                `mapstructure:"listen"`
	Process    ProcessConfig         `mapstructure:"process"`
	Budget     []interface{}         `mapstructure:"budget"` // 保持兼容，内部会解析
	Project    string                `mapstructure:"project"`
	Upstreams  map[string]string     `mapstructure:"upstreams"`
	PublicKey  string                `mapstructure:"public_key"`
	AuthKey    string                `mapstructure:"auth_key" secure:"true"`
	UpstreamToken string             `mapstructure:"upstream_token" secure:"true"`
	Rules      []interface{}         `mapstructure:"rules"`
	Pricing    map[string]interface{} `mapstructure:"pricing"`
}

// LoadConfig 加载并验证配置
func LoadConfig(path string, isProduction bool) (*Config, error) {
	if isProduction {
		if err := checkFilePermissions(path); err != nil {
			return nil, err
		}
	}

	v := viper.New()
	v.SetConfigFile(path)
	v.AutomaticEnv()
	v.SetEnvPrefix("TGUARD")

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// 处理敏感字段
	cfg.loadSensitives(v)

	return &cfg, nil
}

// loadSensitives 处理带有 secure:"true" 标签的字段
func (c *Config) loadSensitives(v *viper.Viper) {
	val := reflect.ValueOf(c).Elem()
	typ := val.Type()

	for i := 0; i < val.NumField(); i++ {
		field := typ.Field(i)
		if field.Tag.Get("secure") == "true" {
			key := field.Tag.Get("mapstructure")
			if key == "" {
				key = strings.ToLower(field.Name)
			}

			// 优先级 1: 环境变量 (TGUARD_UPPER_CASE)
			envKey := "TGUARD_" + strings.ToUpper(key)
			if envVal := os.Getenv(envKey); envVal != "" {
				val.Field(i).SetString(envVal)
				continue
			}

			// 优先级 2: 系统密钥链 (此处调用已有的 security 模块逻辑，暂时 mock)
			// TODO: 集成 security.RetrieveSecret(key)

			// 优先级 3: 配置文件 (保持原有值)
		}
	}
}

// checkFilePermissions 检查文件权限，防止世界可读 (0644 在生产环境被视为风险)
func checkFilePermissions(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	mode := info.Mode().Perm()
	// 如果是 "其他用户可读" (Mode & 0004 != 0)
	if mode&0004 != 0 {
		return fmt.Errorf("config file %s has insecure permissions (%o), must not be world-readable in production", path, mode)
	}
	return nil
}

// MaskConfig 返回脱敏后的配置字符串用于日志记录
func (c *Config) MaskConfig() string {
	val := reflect.ValueOf(*c)
	typ := val.Type()
	var parts []string

	for i := 0; i < val.NumField(); i++ {
		field := typ.Field(i)
		fieldName := field.Name
		fieldVal := val.Field(i).Interface()

		if field.Tag.Get("secure") == "true" {
			fieldVal = "***"
		}
		parts = append(parts, fmt.Sprintf("%s: %v", fieldName, fieldVal))
	}
	return strings.Join(parts, ", ")
}
