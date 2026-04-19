package proxy

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"strings"
	"sync"
	"t-guard/internal/security"
	"t-guard/pkg/logger"
	"t-guard/pkg/route"
	"time"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
)

type Server interface {
	Start(ctx context.Context) error
	Addr() net.Addr
	UpdateAuthKey(key string)
}

type proxyServer struct {
	config  Config
	proxy   *httputil.ReverseProxy
	handler http.Handler
	ln      net.Listener
	srv     *http.Server
	mu      sync.RWMutex // 保护 ln 和 srv
}

// NewServer 创建高性能反向代理服务器
func NewServer(cfg Config) Server {
	s := &proxyServer{config: cfg}

	// 配置复用连接池
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment, // 支持系统代理
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: false,            // 强制 TLS 校验
			MinVersion:         tls.VersionTLS12, // 安全最低版本
		},
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 20,
		IdleConnTimeout:     90 * time.Second,
	}

	proxy := &httputil.ReverseProxy{
		Transport: transport,
		Director: func(req *http.Request) {
			// A. 从 Context 获取预判决策
			decision, ok := req.Context().Value("decision").(route.Decision)
			if !ok {
				// Fallback: 如果中间件没预判，则此处实时判
				model := req.Header.Get("X-Model-Target")
				project := req.Header.Get("X-Project-ID")
				clientIP, _, _ := net.SplitHostPort(req.RemoteAddr)
				decision, _ = cfg.Router.Decide(req.Context(), route.Request{
					Model: model, Project: project, IP: clientIP,
				})
			}

			// B. 目标重定向
			targetURL, ok := cfg.Upstreams[decision.Target]
			if !ok {
				targetURL = cfg.Upstreams[cfg.DefaultTarget]
			}

			if targetURL != nil {
				req.URL.Scheme = targetURL.Scheme
				req.URL.Host = targetURL.Host
				req.Host = targetURL.Host
			}

			// C. 注入 Header
			for k, v := range decision.Headers {
				req.Header.Set(k, v)
			}
			req.Header.Set("X-TGuard-Rule", decision.RuleID)
		},
		ModifyResponse: s.modifyResponse,
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			log.Printf("[proxy] proxy error: %v", err)
			
			// 尝试 Fallback 逻辑
			if decision, ok := r.Context().Value("decision").(route.Decision); ok && decision.FallbackTarget != "" {
				log.Printf("[proxy] attempting fallback to: %s", decision.FallbackTarget)
				targetURL, ok := cfg.Upstreams[decision.FallbackTarget]
				if ok {
					// 简单的重试逻辑：由于 Director 已经运行过，我们需要重新发起请求
					// 在生产环境下，更推荐使用自定义 Transport 实现
					r.URL.Scheme = targetURL.Scheme
					r.URL.Host = targetURL.Host
					r.Host = targetURL.Host
					// 清除 Context 避免无限循环
					*r = *r.WithContext(context.WithValue(r.Context(), "decision", route.Decision{}))
					s.proxy.ServeHTTP(w, r)
					return
				}
			}

			w.WriteHeader(http.StatusBadGateway)
			_, _ = w.Write([]byte("Gateway Error: Routing or Upstream Failure"))
		},
	}
	s.proxy = proxy

	// 包装处理器：添加鉴权、健康检查与恢复机制
	s.handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := r.Header.Get("X-Request-ID")
		if requestID == "" {
			requestID = uuid.New().String()
		}
		
		start := time.Now()
		
		// 结构化日志基础上下文
		loggerWithContext := logger.Log.With(
			zap.String("request_id", requestID),
			zap.String("method", r.Method),
			zap.String("path", r.URL.Path),
			zap.String("remote_addr", r.RemoteAddr),
		)

		// Panic 恢复
		defer func() {
			if err := recover(); err != nil {
				loggerWithContext.Error("panic_recovered", zap.Any("error", err))
				http.Error(w, "Internal Gateway Error", http.StatusInternalServerError)
			}
		}()

		// 1. 公共端点 (健康检查与监控)
		if r.URL.Path == "/healthz" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("ok"))
			return
		}

		if r.URL.Path == "/readyz" {
			// 检查上游与数据库
			if _, err := s.config.Store.QueryProjects(r.Context()); err != nil {
				w.WriteHeader(http.StatusServiceUnavailable)
				_, _ = w.Write([]byte("not_ready"))
				return
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("ready"))
			return
		}

		if r.URL.Path == "/metrics" {
			promhttp.Handler().ServeHTTP(w, r)
			return
		}

		// 1b. Admin API
		if strings.HasPrefix(r.URL.Path, "/admin/") {
			s.handleAdmin(w, r)
			return
		}

		// 2. 准入校验
		if s.config.AuthKey != "" {
			authHeader := r.Header.Get("X-TGuard-Auth")
			if !security.SecureCompare(authHeader, s.config.AuthKey) {
				logger.Audit("unauthorized_access", 
					zap.String("request_id", requestID),
					logger.Mask("provided_key", authHeader),
				)
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
		}

		// 3. 限流
		if s.config.RateLimit != nil {
			clientIP, _, _ := net.SplitHostPort(r.RemoteAddr)
			if !s.config.RateLimit.Allow(clientIP, r.Header.Get("X-User-ID")) {
				RequestTotal.WithLabelValues("unknown", "rate_limited").Inc()
				w.WriteHeader(http.StatusTooManyRequests)
				return
			}
		}

		if s.proxy == nil {
			w.WriteHeader(http.StatusNotImplemented)
			_, _ = w.Write([]byte("Proxy not initialized"))
			return
		}

		// 1c. 预判路由与预算
		model := r.Header.Get("X-Model-Target")
		project := r.Header.Get("X-Project-ID")
		clientIP, _, _ := net.SplitHostPort(r.RemoteAddr)

		decision, _ := s.config.Router.Decide(r.Context(), route.Request{
			Model: model, Project: project, IP: clientIP,
		})

		estimatedMaxCost := s.config.Pricing.CalculateCost(decision.Target, 0, 4000)
		budgetDecision, _ := s.config.Billing.Allow(r.Context(), project, estimatedMaxCost)
		if !budgetDecision.Allowed {
			loggerWithContext.Warn("budget_exceeded", zap.String("project", project))
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusPaymentRequired)
			_, _ = w.Write([]byte(`{"error": "insufficient_budget", "message": "Daily budget limit reached"}`))
			return
		}

		// 更新状态指标
		status, _ := s.config.Billing.GetStatus(r.Context(), project)
		BudgetRemaining.WithLabelValues(project).Set(float64(status.Limit - status.Used))

		ctx := context.WithValue(r.Context(), "request_id", requestID)
		ctx = context.WithValue(ctx, "frozen_amount", estimatedMaxCost)
		ctx = context.WithValue(ctx, "decision", decision)
		r = r.WithContext(ctx)

		// 执行代理
		s.proxy.ServeHTTP(w, r)
		
		// 记录请求成功指标
		duration := time.Since(start)
		RequestTotal.WithLabelValues(decision.Target, "success").Inc()
		RequestDuration.WithLabelValues(decision.Target).Observe(duration.Seconds())
		
		loggerWithContext.Info("request_completed", 
			zap.String("model", decision.Target),
			zap.Duration("duration", duration),
			zap.Int64("frozen_cost", estimatedMaxCost),
		)
	})



	return s
}

func (s *proxyServer) handleAdmin(w http.ResponseWriter, r *http.Request) {
	// 简单的安全检查
	if s.config.AuthKey != "" && r.Header.Get("X-TGuard-Auth") != s.config.AuthKey {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	switch r.URL.Path {
	case "/admin/rules":
		if r.Method == http.MethodPost {
			log.Printf("[audit] [admin] updating routing rules, user-agent: %s, remote: %s", r.UserAgent(), r.RemoteAddr)
			var rules []route.Rule
			if err := json.NewDecoder(r.Body).Decode(&rules); err != nil {
				log.Printf("[audit] [admin] rule update failed: %v", err)
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			if err := s.config.Router.LoadRules(rules); err != nil {
				log.Printf("[audit] [admin] rule loading failed: %v", err)
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			log.Printf("[audit] [admin] successfully updated %d rules", len(rules))
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"rules updated"}`))
			return
		}
	case "/admin/pricing":
		// TODO: 实现定价动态更新
	}
	http.Error(w, "Not Found", http.StatusNotFound)
}

func (s *proxyServer) modifyResponse(resp *http.Response) error {
	// SSE 流式拦截入口
	if resp.Header.Get("Content-Type") == "text/event-stream" {
		return s.handleSSE(resp)
	}
	return nil
}

func (s *proxyServer) Start(ctx context.Context) error {
	var err error
	ln, err := net.Listen("tcp", s.config.ListenAddr)
	if err != nil {
		return err
	}

	s.mu.Lock()
	s.ln = ln
	s.srv = &http.Server{
		Handler:      s.handler,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 0, // 流式响应不设写超时
		IdleTimeout:  120 * time.Second,
	}
	s.mu.Unlock()

	// 优雅关闭协程
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		s.mu.RLock()
		if s.srv != nil {
			_ = s.srv.Shutdown(shutdownCtx)
		}
		s.mu.RUnlock()
	}()

	return s.srv.Serve(ln)
}

func (s *proxyServer) Addr() net.Addr {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.ln == nil {
		return nil
	}
	return s.ln.Addr()
}

func (s *proxyServer) UpdateAuthKey(key string) {
	s.config.AuthKey = key
}
