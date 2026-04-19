package proxy

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	RequestTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "t_guard_request_total",
		Help: "Total number of requests processed by T-Guard",
	}, []string{"model", "status"})

	RequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "t_guard_request_duration_seconds",
		Help:    "Request duration in seconds",
		Buckets: prometheus.DefBuckets,
	}, []string{"model"})

	TokensUsed = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "t_guard_tokens_used",
		Help: "Number of tokens consumed",
	}, []string{"model", "type"}) // type: prompt, completion

	BudgetRemaining = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "t_guard_budget_remaining",
		Help: "Remaining budget for a project in millicents",
	}, []string{"project"})
)
