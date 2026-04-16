package proxy

import (
	"context"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"t-guard/pkg/route"
	"time"
)

type Server interface {
	Start(ctx context.Context) error
	Addr() net.Addr
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

			decision, err := cfg.Router.Decide(req.Context(), route.Request{
				Model:   model,
				Project: project,
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
		},
		ModifyResponse: s.modifyResponse,
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			log.Printf("[proxy] proxy error: %v", err)
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

		// 1. 健康检查开放
		if r.URL.Path == "/health" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("ok"))
			return
		}

		// 2. 准入校验 (API Key 校验)
		if s.config.AuthKey != "" {
			authHeader := r.Header.Get("X-TGuard-Auth")
			if authHeader != s.config.AuthKey {
				log.Printf("[proxy] unauthorized access attempt from %s", r.RemoteAddr)
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte("Unauthorized: Missing or Invalid X-TGuard-Auth token"))
				return
			}
		}

		proxy.ServeHTTP(w, r)
	})

	return s
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
