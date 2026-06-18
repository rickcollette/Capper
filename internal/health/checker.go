package health

import (
	"fmt"
	"net"
	"net/http"
	"strconv"
	"time"
)

// Result is the outcome of a single health check.
type Result struct {
	InstanceID string
	Status     string // "healthy", "unhealthy"
	Message    string
	CheckedAt  string
}

// CheckTCP dials ip:port with timeout, returns Result.
func CheckTCP(instanceID, ip string, port, timeoutSecs int) Result {
	addr := net.JoinHostPort(ip, strconv.Itoa(port))
	to := time.Duration(timeoutSecs) * time.Second
	conn, err := net.DialTimeout("tcp", addr, to)
	if err != nil {
		return Result{
			InstanceID: instanceID,
			Status:     "unhealthy",
			Message:    err.Error(),
			CheckedAt:  time.Now().UTC().Format(time.RFC3339),
		}
	}
	conn.Close()
	return Result{
		InstanceID: instanceID,
		Status:     "healthy",
		Message:    "tcp connect ok",
		CheckedAt:  time.Now().UTC().Format(time.RFC3339),
	}
}

// CheckHTTP does GET http://ip:port/path, returns healthy if 200-399.
func CheckHTTP(instanceID, ip, path string, port, timeoutSecs int) Result {
	url := fmt.Sprintf("http://%s:%d%s", ip, port, path)
	client := &http.Client{Timeout: time.Duration(timeoutSecs) * time.Second}
	resp, err := client.Get(url)
	now := time.Now().UTC().Format(time.RFC3339)
	if err != nil {
		return Result{InstanceID: instanceID, Status: "unhealthy", Message: err.Error(), CheckedAt: now}
	}
	resp.Body.Close()
	if resp.StatusCode >= 200 && resp.StatusCode < 400 {
		return Result{InstanceID: instanceID, Status: "healthy", Message: fmt.Sprintf("HTTP %d", resp.StatusCode), CheckedAt: now}
	}
	return Result{InstanceID: instanceID, Status: "unhealthy", Message: fmt.Sprintf("HTTP %d", resp.StatusCode), CheckedAt: now}
}
