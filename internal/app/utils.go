package app

import (
	"fmt"
	"net/http"
	"strings"
	"time"
)

// WaitForHealth 同步等待服务可用
func WaitForHealth(addr string, timeout time.Duration) error {
	if !strings.HasPrefix(addr, "http://") && !strings.HasPrefix(addr, "https://") {
		addr = "http://" + addr
	}
	addr = strings.TrimSuffix(addr, "/") + "/health"

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp, err := http.Get(addr)
		if err == nil && resp.StatusCode == http.StatusOK {
			resp.Body.Close()
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	return fmt.Errorf("服务健康检查超时: %s", addr)
}
