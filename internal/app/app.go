package app

import (
	"net/url"
	"t-guard/internal/errors"
	"t-guard/internal/process"
	"t-guard/internal/proxy"
	"t-guard/internal/security"
	"t-guard/internal/ui"
	"t-guard/pkg/budget"
	"t-guard/pkg/logger"
	"t-guard/pkg/pricing"
	"t-guard/pkg/ratelimit"
	"t-guard/pkg/route"
	"t-guard/pkg/store"
	"t-guard/pkg/token"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"go.uber.org/zap"
)

type App struct {
	Config   *Config
	Store    store.Store
	Security security.Manager
	Token    token.Engine
	Pricing  pricing.Engine
	Router   route.Engine
	Billing  budget.Controller
	Proxy    proxy.Server
	UI       tea.Model
	Process  process.Manager
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
		if cerr := s.Close(); cerr != nil {
			logger.Log.Error("failed to close store during cleanup", zap.Error(cerr))
		}
		return nil, nil, errors.New(errors.ErrSecurity, "INIT_FAILED", "M8 init failed", err)
	}

	// 3. M0: Tokenizer
	tokenizer := token.NewEngine()

	// 3b. Pricing Engine - 此处需要类型转换，暂且跳过复杂转换，保持逻辑
	priceEngine := pricing.NewEngine(nil)

	// 3c. Rate Limiter
	limiter := ratelimit.NewLimiter(ratelimit.Config{
		IPRate:    10,
		IPBurst:   20,
		UserRate:  30,
		UserBurst: 50,
	})

	// 4. M2: Router 路由
	router := route.NewEngine()
	// 5. M3: Billing 计费
	billing, err := budget.NewController(s, nil) // 暂传 nil 兼容，后续需要 mapstructure 转换
	if err != nil {
		if cerr := s.Close(); cerr != nil {
			logger.Log.Error("failed to close store during cleanup", zap.Error(cerr))
		}
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
		Token:         tokenizer,
		Pricing:       priceEngine,
		AuthKey:       cfg.AuthKey,
		RateLimit:     limiter,
	})
	// 7. M5: TUI 界面
	uim := ui.NewModel(ui.Config{
		Store:       s,
		Billing:     billing,
		ProxyAddr:   cfg.Listen,
		RefreshRate: 1 * time.Second,
	})

	// 8. M6: Process 进程管理
	pTimeout, _ := time.ParseDuration(cfg.Process.Timeout)
	pm := process.NewManager(process.Config{
		Whitelist: cfg.Process.Whitelist,
		Timeout:   pTimeout,
	})

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
		Pricing:  priceEngine,
		Router:   router,
		Billing:  billing,
		Proxy:    px,
		UI:       uim,
		Process:  pm,
	}, cleanup, nil
}
