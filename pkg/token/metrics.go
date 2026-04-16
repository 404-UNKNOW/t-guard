package token

import "sync/atomic"

type Metrics struct {
	TotalCalculations int64
	CacheHitCount     int64
	FallbackCount     int64
}

func (m *Metrics) IncCalculations() {
	atomic.AddInt64(&m.TotalCalculations, 1)
}

func (m *Metrics) IncCacheHit() {
	atomic.AddInt64(&m.CacheHitCount, 1)
}

func (m *Metrics) IncFallback() {
	atomic.AddInt64(&m.FallbackCount, 1)
}

func (m *Metrics) Snapshot() (int64, int64, int64) {
	return atomic.LoadInt64(&m.TotalCalculations),
		atomic.LoadInt64(&m.CacheHitCount),
		atomic.LoadInt64(&m.FallbackCount)
}
