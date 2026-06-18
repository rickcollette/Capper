package health_test

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"capper/internal/health"
)

func TestCheckTCP_Success(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	host, portStr, _ := net.SplitHostPort(ln.Addr().String())
	port := 0
	fmt.Sscanf(portStr, "%d", &port)

	result := health.CheckTCP("inst-1", host, port, 2)
	if result.Status != "healthy" {
		t.Errorf("expected healthy, got %q: %s", result.Status, result.Message)
	}
	if result.InstanceID != "inst-1" {
		t.Errorf("InstanceID: got %q", result.InstanceID)
	}
	if result.CheckedAt == "" {
		t.Error("CheckedAt must be set")
	}
}

func TestCheckTCP_Failure(t *testing.T) {
	// Port 1 is almost certainly not open on loopback in test environments.
	result := health.CheckTCP("inst-2", "127.0.0.1", 1, 1)
	if result.Status != "unhealthy" {
		t.Errorf("expected unhealthy for closed port, got %q", result.Status)
	}
	if result.Message == "" {
		t.Error("unhealthy result must include a message")
	}
}

func TestCheckHTTP_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	host, portStr, _ := net.SplitHostPort(ts.Listener.Addr().String())
	port := 0
	fmt.Sscanf(portStr, "%d", &port)

	result := health.CheckHTTP("inst-3", host, "/", port, 2)
	if result.Status != "healthy" {
		t.Errorf("expected healthy, got %q: %s", result.Status, result.Message)
	}
}

func TestCheckHTTP_ServerError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	host, portStr, _ := net.SplitHostPort(ts.Listener.Addr().String())
	port := 0
	fmt.Sscanf(portStr, "%d", &port)

	result := health.CheckHTTP("inst-4", host, "/health", port, 2)
	if result.Status != "unhealthy" {
		t.Errorf("expected unhealthy for 500, got %q", result.Status)
	}
}

func TestCheckHTTP_ConnectionRefused(t *testing.T) {
	result := health.CheckHTTP("inst-5", "127.0.0.1", "/", 1, 1)
	if result.Status != "unhealthy" {
		t.Errorf("expected unhealthy for refused connection, got %q", result.Status)
	}
}

func TestCheckHTTP_NotFound(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	host, portStr, _ := net.SplitHostPort(ts.Listener.Addr().String())
	port := 0
	fmt.Sscanf(portStr, "%d", &port)

	result := health.CheckHTTP("inst-6", host, "/missing", port, 2)
	if result.Status != "unhealthy" {
		t.Errorf("expected unhealthy for 404, got %q", result.Status)
	}
}
