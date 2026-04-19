package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/99designs/keyring"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	cfgFile string
	rootCmd = &cobra.Command{
		Use:   "tokenflow",
		Short: "TokenFlow: 一个高性能、毫美分精度的 AI 流量守卫网关",
		Long: `TokenFlow 集成了实时路由、计费熔断、跨平台进程管理及 TUI 监控。
支持 OpenAI、Claude 等多种模型的流式代理与成本控制。`,
	}
)

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "配置文件路径 (默认查找 ./.tokenflow.yml)")
	rootCmd.PersistentFlags().String("port", "8080", "代理监听端口")
	rootCmd.PersistentFlags().String("data-dir", ".", "数据存储目录")

	_ = viper.BindPFlag("listen", rootCmd.PersistentFlags().Lookup("port"))
	_ = viper.BindPFlag("data_dir", rootCmd.PersistentFlags().Lookup("data-dir"))
}

func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		// 优先级：./.tokenflow.yml -> $HOME/.tokenflow/config.yml
		home, _ := os.UserHomeDir()
		viper.AddConfigPath(".")
		viper.AddConfigPath(home + "/.tokenflow")
		viper.SetConfigName(".tokenflow")
		viper.SetConfigType("yml")
	}

	viper.SetEnvPrefix("TOKENFLOW")
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err == nil {
		// fmt.Println("Using config file:", viper.ConfigFileUsed())
	} else {
		// 配置文件不存在时，静默处理，由 run 命令决定是否开启向导
	}
}

var setKeyCmd = &cobra.Command{
	Use:   "set-key [provider] [key]",
	Short: "Securely store an API key in the system keyring",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		provider := strings.ToUpper(args[0])
		key := args[1]

		kr, err := keyring.Open(keyring.Config{
			ServiceName: "tokenflow",
		})
		if err != nil {
			return err
		}

		fmt.Printf("Storing key for %s in system keyring...\n", provider)
		return kr.Set(keyring.Item{
			Key:  provider + "_API_KEY",
			Data: []byte(key),
		})
	},
}

func init() {
	rootCmd.AddCommand(setKeyCmd)
}
