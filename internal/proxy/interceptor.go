package proxy

import (
	"net/http"
	"net/url"
	"t-guard/pkg/budget"
	"t-guard/pkg/pricing"
	"t-guard/pkg/ratelimit"
	"t-guard/pkg/route"
	"t-guard/pkg/store"
	"t-guard/pkg/token"
	"time"
)

type Config struct {
	ListenAddr    string
	Upstreams     map[string]*url.URL // target_name -> URL
	DefaultTarget string
	Router        route.Engine
	Billing       budget.Controller
	Store         store.Store
	Token         token.Engine
	Pricing       pricing.Engine
	AuthKey       string // 代理准入令牌
	RateLimit     *ratelimit.Limiter
}

type StreamStats struct {
	RequestID    string
	InputTokens  int
	OutputTokens int
	Duration     time.Duration
	Rerouted     bool
}

type Interceptor interface {
	OnRequest(req *http.Request) (*http.Request, error)
	OnResponse(resp *http.Response) error
	OnStreamChunk(chunk []byte) ([]byte, error)
	OnComplete(stats StreamStats)
}
