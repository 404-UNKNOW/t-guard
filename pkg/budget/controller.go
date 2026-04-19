package budget

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	"t-guard/pkg/store"
	"time"

	"github.com/google/uuid"
)

type projectBudget struct {
	project   string
	used      atomic.Int64
	frozen    atomic.Int64 // 预扣减冻结金额
	limit     int64
	softMark  int64
	lastState atomic.Value // 存储 State
}

type budgetController struct {
	store       store.Store
	budgets     sync.Map // map[string]*projectBudget
	subscribers sync.Map // map[string][]chan Status
	mu          sync.Mutex
	stopChan    chan struct{}
}

// NewController 创建计费控制器并集成持久化同步
func NewController(s store.Store, configs []BudgetConfig) (Controller, error) {
	c := &budgetController{
		store:    s,
		stopChan: make(chan struct{}),
	}

	for _, cfg := range configs {
		pb := &projectBudget{
			project:  cfg.Project,
			limit:    cfg.HardLimit,
			softMark: int64(float64(cfg.HardLimit) * cfg.SoftLimit),
		}
		pb.lastState.Store(StateNormal)

		// 启动加载：读取今日已用额度实现崩溃恢复
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		stats, _ := s.GetDailyStats(ctx, cfg.Project, time.Now().UTC().Format("2006-01-02"))
		cancel()
		pb.used.Store(stats.TotalMillicents)
		
		c.budgets.Store(cfg.Project, pb)
	}

	return c, nil
}

func (c *budgetController) Allow(ctx context.Context, project string, estimatedCost int64) (Decision, error) {
	val, ok := c.budgets.Load(project)
	if !ok {
		return Decision{Allowed: true}, nil
	}
	pb := val.(*projectBudget)
	
	// 并发安全地检查：已用 + 冻结 + 当前预估
	used := pb.used.Load()
	frozen := pb.frozen.Load()
	
	if pb.limit > 0 && used+frozen+estimatedCost > pb.limit {
		return Decision{Allowed: false, Remaining: pb.limit - used - frozen, Warning: "Hard limit reached (including frozen)"}, nil
	}

	// 原子冻结：预扣减
	pb.frozen.Add(estimatedCost)

	warning := ""
	if pb.softMark > 0 && used+frozen+estimatedCost > pb.softMark {
		warning = "Soft limit threshold exceeded"
	}

	return Decision{
		Allowed:   true,
		Remaining: pb.limit - used - frozen - estimatedCost,
		Warning:   warning,
	}, nil
}

func (c *budgetController) SettleBudget(ctx context.Context, project string, frozenAmount int64, actualCost int64) error {
	val, ok := c.budgets.Load(project)
	if !ok {
		return nil
	}
	pb := val.(*projectBudget)

	// 原子解冻并增加实际消耗
	pb.frozen.Add(-frozenAmount)
	return c.Record(ctx, project, actualCost)
}

func (c *budgetController) Record(ctx context.Context, project string, actualCost int64) error {
	val, ok := c.budgets.Load(project)
	if !ok {
		return nil
	}
	pb := val.(*projectBudget)
	
	newUsed := pb.used.Add(actualCost)

	// 更新 Prometheus 指标
	budgetUsedMillicents.WithLabelValues(project).Set(float64(newUsed))
	budgetLimitMillicents.WithLabelValues(project).Set(float64(pb.limit))
	requestTotal.WithLabelValues(project, "success").Inc()

	// 1. 原子持久化：写入 Store 异步队列
	_ = c.store.Write(ctx, store.Record{
		ID:             uuid.New(),
		Project:        project,
		CostMillicents: actualCost,
		Timestamp:      time.Now().UTC(),
	})

	// 2. 状态跃迁检测与事件通知
	newState := c.calculateState(pb, newUsed)
	oldState := pb.lastState.Swap(newState).(State)
	if oldState != newState {
		go c.notify(project, newUsed, pb.limit, newState)
	}

	return nil
}

func (c *budgetController) calculateState(pb *projectBudget, used int64) State {
	if pb.limit <= 0 {
		return StateNormal
	}
	if used >= pb.limit {
		return StateExhausted
	}
	if float64(used)/float64(pb.limit) >= 0.9 {
		return StateCritical
	}
	if pb.softMark > 0 && used >= pb.softMark {
		return StateWarning
	}
	return StateNormal
}

func (c *budgetController) GetStatus(ctx context.Context, project string) (Status, error) {
	val, ok := c.budgets.Load(project)
	if !ok {
		return Status{Project: project, State: StateNormal}, nil
	}
	pb := val.(*projectBudget)
	used := pb.used.Load()
	
	percentage := 0.0
	if pb.limit > 0 {
		percentage = float64(used) / float64(pb.limit)
	}

	return Status{
		Project:    project,
		Used:       used,
		Limit:      pb.limit,
		Percentage: percentage,
		State:      c.calculateState(pb, used),
	}, nil
}

func (c *budgetController) NewStreamWriter(w io.Writer, project string) *StreamWriter {
	return &StreamWriter{
		w:       w,
		project: project,
		ctrl:    c,
	}
}

func (c *budgetController) Subscribe(project string) <-chan Status {
	ch := make(chan Status, 10) // 缓冲容量 10
	c.mu.Lock()
	defer c.mu.Unlock()
	
	var subs []chan Status
	if val, ok := c.subscribers.Load(project); ok {
		subs = val.([]chan Status)
	}
	c.subscribers.Store(project, append(subs, ch))
	return ch
}

func (c *budgetController) notify(project string, used, limit int64, state State) {
	val, ok := c.subscribers.Load(project)
	if !ok {
		return
	}
	
	status := Status{
		Project:    project,
		Used:       used,
		Limit:      limit,
		Percentage: float64(used) / float64(limit),
		State:      state,
	}
	
	subs := val.([]chan Status)
	for _, ch := range subs {
		select {
		case ch <- status:
		default:
			// 避免慢消费者阻塞
		}
	}
}

func (c *budgetController) Close() error {
	close(c.stopChan)
	return nil
}

// StreamWriter 实现流式熔断
type StreamWriter struct {
	w       io.Writer
	project string
	ctrl    Controller
	mu      sync.Mutex
	closed  bool
}

func (sw *StreamWriter) Write(p []byte) (int, error) {
	sw.mu.Lock()
	defer sw.mu.Unlock()
	if sw.closed {
		return 0, io.EOF
	}
	return sw.w.Write(p)
}

// CheckBudget 检查预算并支持中途截断
func (sw *StreamWriter) CheckBudget(ctx context.Context, additionalCost int64) error {
	sw.mu.Lock()
	defer sw.mu.Unlock()
	if sw.closed {
		return io.EOF
	}

	status, _ := sw.ctrl.GetStatus(ctx, sw.project)
	// 按照需求：超预算时发送特殊 JSON 并关闭
	if status.Limit > 0 && status.Used+additionalCost > status.Limit {
		resp := struct {
			Error string `json:"error"`
			Cost  int64  `json:"cost"`
		}{
			Error: "budget_exceeded",
			Cost:  status.Used + additionalCost,
		}
		data, _ := json.Marshal(resp)
		_, _ = sw.w.Write([]byte("\n"))
		_, _ = sw.w.Write(data)
		sw.closed = true
		return fmt.Errorf("budget exceeded")
	}
	return nil
}
