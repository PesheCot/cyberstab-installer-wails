package system

import (
	"fmt"
	"net"
	"time"
)

func isLocalTCPPortOpen(port int, timeout time.Duration) bool {
	if port <= 0 || port > 65535 {
		return false
	}
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	conn, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}
