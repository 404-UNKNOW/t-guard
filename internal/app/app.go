package app

import (
	"net/url"
	"t-guard/internal/errors"
	"t-guard/internal/process"
	"t-guard/internal/proxy"
	"t-guard/internal/security"
	"t-guard/internal/ui"
	"t-guard/pkg/budget"
	"t-guard/pkg/route"
	"t-guard/pkg/store"
	"t-guard/pkg/token"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

type App struct {
	Config   *Config
	Store    store.Store
	Security security.Manager
	Token    token.Engine
	Router   route.Engine
	Billing  budget.Controller
	Proxy    proxy.Server
	UI       tea.Model
	Process  process.Manager
}

type Config struct {
	DataDir    string                `mapstructure:"data_dir"`
	ConfigFile string                `mapstructure:"config_file"`
	Listen     string                `mapstructure:"listen"`
	Budget     []budget.BudgetConfig `mapstructure:"budget"`
	Project    string                `mapstructure:"project"`
	Upstreams  map[string]string     `mapstructure:"upstreams"`
	PublicKey  string                `mapstructure:"public_key"` // 契约要求
	AuthKey    string                `mapstructure:"auth_key"`   // 代理准入令牌
	Rules      []route.Rule          `mapstructure:"rules"`      // 动态路由规则
}

func (c *Config) Validate() error {
	if c.DataDir == "" {
		return errors.New(errors.ErrCLI, "INVALID_CONFIG", "DataDir 不能为空", nil)
	}
	if c.Listen == "" {
		return errors.New(errors.ErrCLI, "INVALID_CONFIG", "Listen 地址不能为空", nil)
	}
	return nil
}

// InitializeApp 严格遵守契约：M1 -> M8 -> M0 -> M2 -> M3 -> M4 -> M5 -> M6
func InitializeApp(cfg *Config) (*App, func(), error) {
	// 0. 校验配置
	if err := cfg.Validate(); err != nil {
		return nil, nil, err
	}

	// 1. M1: Store 数据层
	s, err := store.NewSQLiteStore(cfg.DataDir + "/tokenflow.db")
	if err != nil {
		return nil, nil, errors.New(errors.ErrStore, "INIT_FAILED", "M1 init failed", err)
	}

	// 2. M8: Security 安全层
	sec, err := security.Init(s)
	if err != nil {
		_ = s.Close()
		return nil, nil, errors.New(errors.ErrSecurity, "INIT_FAILED", "M8 init failed", err)
	}

	// 3. M0: Tokenizer
	tokenizer := token.NewEngine()
// 4. M2: Router 路由
router := route.NewEngine()
// 优先加载配置文件中的规则，无规则时加载默认规则
rules := cfg.Rules
if len(rules) == 0 {
	rules = []route.Rule{{ID: "default", Action: route.RouteAction{Target: "openai"}}}
}
if err := router.LoadRules(rules); err != nil {
	_ = s.Close()
	return nil, nil, errors.New(errors.ErrRouter, "INIT_FAILED", "M2 init failed", err)
}

// 5. M3: Billing 计费
billing, err := budget.NewController(s, cfg.Budget)
if err != nil {
	_ = s.Close()
	return nil, nil, errors.New(errors.ErrBilling, "INIT_FAILED", "M3 init failed", err)
}

// 6. M4: Proxy 代理
upstreams := make(map[string]*url.URL)
for k, v := range cfg.Upstreams {
	u, _ := url.Parse(v)
	upstreams[k] = u
}
px := proxy.NewServer(proxy.Config{
	ListenAddr:    cfg.Listen,
	Upstreams:     upstreams,
	DefaultTarget: "openai",
	Router:        router,
	Billing:       billing,
	Store:         s,
	AuthKey:       cfg.AuthKey,
})


	// 7. M5: TUI 界面
	uim := ui.NewModel(ui.Config{
		Store:       s,
		Billing:     billing,
		ProxyAddr:   cfg.Listen,
		RefreshRate: 1 * time.Second,
	})

	// 8. M6: Process 进程管理
	pm := process.NewManager()

	cleanup := func() {
		// 契约要求：反向顺序清理
		_ = pm.Cleanup()
		// UI, Proxy 优雅关闭逻辑需各模块支持 Shutdown
		_ = s.Close()
		_ = tokenizer.Close()
	}

	return &App{
		Config:   cfg,
		Store:    s,
		Security: sec,
		Token:    tokenizer,
		Router:   router,
		Billing:  billing,
		Proxy:    px,
		UI:       uim,
		Process:  pm,
	}, cleanup, nil
}
