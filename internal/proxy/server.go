package proxy

import (
	"context"
	"encoding/json"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"strings"
	"t-guard/pkg/logger"
	"t-guard/pkg/route"
	"time"

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
}

// NewServer 创建高性能反向代理服务器
func NewServer(cfg Config) Server {
	s := &proxyServer{config: cfg}

	// 配置复用连接池
	transport := &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 20,
		IdleConnTimeout:     90 * time.Second,
	}

	proxy := &httputil.ReverseProxy{
		Transport: transport,
		Director: func(req *http.Request) {
			// A. 获取路由决策
			model := req.Header.Get("X-Model-Target")
			project := req.Header.Get("X-Project-ID")

			// 提取 IP 和 Headers
			clientIP, _, _ := net.SplitHostPort(req.RemoteAddr)
			headers := make(map[string]string)
			for k, v := range req.Header {
				if len(v) > 0 {
					headers[k] = v[0]
				}
			}

			decision, err := cfg.Router.Decide(req.Context(), route.Request{
				Model:   model,
				Project: project,
				IP:      clientIP,
				Headers: headers,
			})
			if err != nil {
				log.Printf("[proxy] routing decision failed: %v", err)
				// 通过 Context 传递路由错误
				return
			}

			// B. 目标重定向
			targetURL, ok := cfg.Upstreams[decision.Target]
			if !ok {
				targetURL = cfg.Upstreams[cfg.DefaultTarget]
			}

			if targetURL == nil {
				log.Printf("[proxy] no upstream found for target: %s", decision.Target)
				return
			}

			req.URL.Scheme = targetURL.Scheme
			req.URL.Host = targetURL.Host
			req.Host = targetURL.Host

			// C. 注入 Header
			for k, v := range decision.Headers {
				req.Header.Set(k, v)
			}
			req.Header.Set("X-TGuard-Rule", decision.RuleID)
			
			// 将决策存入 Context 以便 ErrorHandler 使用
			*req = *req.WithContext(context.WithValue(req.Context(), "decision", decision))
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
		// Panic 恢复
		defer func() {
			if err := recover(); err != nil {
				log.Printf("[proxy] panic recovered: %v", err)
				http.Error(w, "Internal Gateway Error", http.StatusInternalServerError)
			}
		}()

		// 1. 公共端点 (健康检查与监控)
		if r.URL.Path == "/health/live" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("live"))
			return
		}

		if r.URL.Path == "/health/ready" {
			// 检查数据库
			if _, err := s.config.Store.QueryProjects(r.Context()); err != nil {
				w.WriteHeader(http.StatusServiceUnavailable)
				_, _ = w.Write([]byte("db_not_ready"))
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

		// 1b. Admin API (仅限本地或带 Token 访问)
		if strings.HasPrefix(r.URL.Path, "/admin/") {
			s.handleAdmin(w, r)
			return
		}

		// 2. 准入校验 (API Key 校验)
		if s.config.AuthKey != "" {
			authHeader := r.Header.Get("X-TGuard-Auth")
			if authHeader != s.config.AuthKey {
				logger.Audit("unauthorized_access", 
					zap.String("remote_addr", r.RemoteAddr),
					logger.Mask("provided_key", authHeader),
				)
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte("Unauthorized"))
				return
			}
		}

		// 3. 限流校验
		if s.config.RateLimit != nil {
			// 提取 IP (支持 X-Forwarded-For)
			clientIP := r.Header.Get("X-Forwarded-For")
			if clientIP == "" {
				clientIP, _, _ = net.SplitHostPort(r.RemoteAddr)
			} else {
				// 获取第一个 IP
				clientIP = strings.Split(clientIP, ",")[0]
			}
			userID := r.Header.Get("X-User-ID")

			if !s.config.RateLimit.Allow(clientIP, userID) {
				w.Header().Set("Retry-After", "3")
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusTooManyRequests)
				_, _ = w.Write([]byte(`{"error": "rate_limit_exceeded", "retry_after": 3}`))
				return
			}
		}

		if s.proxy == nil {
			w.WriteHeader(http.StatusNotImplemented)
			_, _ = w.Write([]byte("Proxy not initialized"))
			return
		}

		s.proxy.ServeHTTP(w, r)
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
	s.ln, err = net.Listen("tcp", s.config.ListenAddr)
	if err != nil {
		return err
	}

	s.srv = &http.Server{
		Handler:      s.handler,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 0, // 流式响应不设写超时
		IdleTimeout:  120 * time.Second,
	}

	// 优雅关闭协程
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = s.srv.Shutdown(shutdownCtx)
	}()

	return s.srv.Serve(s.ln)
}

func (s *proxyServer) Addr() net.Addr {
	if s.ln == nil {
		return nil
	}
	return s.ln.Addr()
}

func (s *proxyServer) UpdateAuthKey(key string) {
	s.config.AuthKey = key
}
