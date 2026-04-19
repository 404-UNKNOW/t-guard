package app

import (
	"fmt"
	"net/http"
	"strings"
	"t-guard/pkg/logger"
	"time"

	"go.uber.org/zap"
)

// WaitForHealth 同步等待服务可用
func WaitForHealth(addr string, timeout time.Duration) error {
	if !strings.HasPrefix(addr, "http://") && !strings.HasPrefix(addr, "https://") {
		addr = "http://" + addr
	}
	addr = strings.TrimSuffix(addr, "/") + "/healthz" // 使用最新的健康检查路径

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp, err := http.Get(addr)
		if err == nil {
			if resp.StatusCode == http.StatusOK {
				if cerr := resp.Body.Close(); cerr != nil {
					logger.Log.Warn("failed to close health check body", zap.Error(cerr))
				}
				return nil
			}
			_ = resp.Body.Close()
		}
		time.Sleep(200 * time.Millisecond)
	}
	return fmt.Errorf("服务健康检查超时: %s", addr)
}
