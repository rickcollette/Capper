package metrics_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"capper/internal/metrics"
	"capper/internal/types"
)

func TestPrometheusEndpoint_EmptyInstances(t *testing.T) {
	srv := metrics.NewPrometheusServer(":0", func() ([]types.Instance, error) {
		return nil, nil
	})
	ts := httptest.NewServer(srv.Handler)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/metrics")
	if err != nil {
		t.Fatalf("GET /metrics: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status: %d", resp.StatusCode)
	}
	ct := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "text/plain") {
		t.Errorf("content-type: %q", ct)
	}
}

func TestPrometheusEndpoint_RunningInstances(t *testing.T) {
	instances := []types.Instance{
		{ID: "i1", Name: "web01", Status: types.StatusRunning},
		{ID: "i2", Name: "api01", Status: types.StatusStopped},
	}
	srv := metrics.NewPrometheusServer(":0", func() ([]types.Instance, error) {
		return instances, nil
	})
	ts := httptest.NewServer(srv.Handler)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/metrics")
	if err != nil {
		t.Fatalf("GET /metrics: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	text := string(body)
	// Must contain at least one Prometheus metric line.
	if !strings.Contains(text, "capper_") {
		t.Errorf("expected capper_ metrics in output, got:\n%s", text)
	}
}

func TestPrometheusEndpoint_UnknownPath(t *testing.T) {
	srv := metrics.NewPrometheusServer(":0", func() ([]types.Instance, error) {
		return nil, nil
	})
	ts := httptest.NewServer(srv.Handler)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/does-not-exist")
	if err != nil {
		t.Fatalf("GET /does-not-exist: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404 for unknown path, got %d", resp.StatusCode)
	}
}
