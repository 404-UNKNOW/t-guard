package budget

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	budgetUsedMillicents = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "tguard_budget_used_millicents",
		Help: "The total amount of budget used in millicents.",
	}, []string{"project"})

	budgetLimitMillicents = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "tguard_budget_limit_millicents",
		Help: "The hard limit of the budget in millicents.",
	}, []string{"project"})

	requestTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "tguard_requests_total",
		Help: "The total number of requests handled.",
	}, []string{"project", "status"})
)
