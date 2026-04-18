package ratelimit

import (
	"sync"

	"golang.org/x/time/rate"
)

// Config 定义限流参数
type Config struct {
	IPRate      float64 // 每秒请求数
	IPBurst     int
	UserRate    float64
	UserBurst   int
}

// Limiter 管理多维度令牌桶
type Limiter struct {
	config Config
	ips    sync.Map
	users  sync.Map
}

// NewLimiter 创建新的限流管理器
func NewLimiter(cfg Config) *Limiter {
	if cfg.IPRate == 0 {
		cfg.IPRate = 10
		cfg.IPBurst = 20
	}
	if cfg.UserRate == 0 {
		cfg.UserRate = 30
		cfg.UserBurst = 50
	}
	return &Limiter{config: cfg}
}

// Allow 检查指定 IP 或用户是否触发限流
func (l *Limiter) Allow(ip string, userID string) bool {
	// 1. 用户级限流优先
	if userID != "" {
		uLimiter, _ := l.users.LoadOrStore(userID, rate.NewLimiter(rate.Limit(l.config.UserRate), l.config.UserBurst))
		if !uLimiter.(*rate.Limiter).Allow() {
			return false
		}
	}

	// 2. IP 级限流
	if ip != "" {
		iLimiter, _ := l.ips.LoadOrStore(ip, rate.NewLimiter(rate.Limit(l.config.IPRate), l.config.IPBurst))
		if !iLimiter.(*rate.Limiter).Allow() {
			return false
		}
	}

	return true
}
