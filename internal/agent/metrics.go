package agent

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// metricSample mirrors the control-plane resourcemon.MetricSample ingest shape.
type metricSample struct {
	ResourceType string  `json:"resourceType"`
	ResourceID   string  `json:"resourceId"`
	MetricName   string  `json:"metricName"`
	Value        float64 `json:"value"`
	Unit         string  `json:"unit"`
}

// metricsLoop periodically collects host metrics and pushes them to the control
// plane's metrics ingest endpoint, tagged against this node.
func (a *Agent) metricsLoop(ctx context.Context) error {
	interval := a.cfg.ControlPlane.HeartbeatInterval
	if interval <= 0 {
		interval = 60 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			_ = a.reportMetrics(ctx)
		}
	}
}

func (a *Agent) reportMetrics(ctx context.Context) error {
	if a.nodeID == "" {
		return nil
	}
	samples := a.collectNodeMetrics()
	if len(samples) == 0 {
		return nil
	}
	body, _ := json.Marshal(map[string]any{"samples": samples})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		a.cfg.ControlPlane.URL+"/api/v1/metrics/ingest", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if a.token != "" {
		req.Header.Set("Authorization", "Bearer "+a.token)
	}
	resp, err := a.httpClient.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

// collectNodeMetrics reads CPU, memory, disk, and load metrics from /proc and
// the filesystem. It returns an empty slice on platforms without /proc.
func (a *Agent) collectNodeMetrics() []metricSample {
	var out []metricSample
	add := func(name string, value float64, unit string) {
		out = append(out, metricSample{
			ResourceType: "node", ResourceID: a.nodeID, MetricName: name, Value: value, Unit: unit,
		})
	}

	if pct, ok := cpuUsagePercent(); ok {
		add("cpu.percent", pct, "percent")
	}
	if usedPct, usedBytes, ok := memoryUsage(); ok {
		add("memory.used_percent", usedPct, "percent")
		add("memory.used_bytes", usedBytes, "bytes")
	}
	if usedPct, usedBytes, ok := diskUsage("/"); ok {
		add("disk.used_percent", usedPct, "percent")
		add("disk.used_bytes", usedBytes, "bytes")
	}
	if load, ok := loadAvg1(); ok {
		add("load.1m", load, "")
	}
	return out
}

// cpuUsagePercent samples /proc/stat twice over a short interval and returns the
// busy fraction as a 0-100 percentage.
func cpuUsagePercent() (float64, bool) {
	idle1, total1, ok := readCPUStat()
	if !ok {
		return 0, false
	}
	time.Sleep(200 * time.Millisecond)
	idle2, total2, ok := readCPUStat()
	if !ok {
		return 0, false
	}
	dTotal := total2 - total1
	dIdle := idle2 - idle1
	if dTotal <= 0 {
		return 0, false
	}
	return (1.0 - float64(dIdle)/float64(dTotal)) * 100.0, true
}

func readCPUStat() (idle, total uint64, ok bool) {
	f, err := os.Open("/proc/stat")
	if err != nil {
		return 0, 0, false
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) < 5 || fields[0] != "cpu" {
			continue
		}
		var sum uint64
		for i := 1; i < len(fields); i++ {
			v, err := strconv.ParseUint(fields[i], 10, 64)
			if err != nil {
				continue
			}
			sum += v
			if i == 4 { // idle is the 4th value
				idle = v
			}
		}
		return idle, sum, true
	}
	return 0, 0, false
}

func memoryUsage() (usedPercent, usedBytes float64, ok bool) {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0, 0, false
	}
	defer f.Close()
	var total, available float64
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) < 2 {
			continue
		}
		v, _ := strconv.ParseFloat(fields[1], 64)
		v *= 1024 // kB → bytes
		switch fields[0] {
		case "MemTotal:":
			total = v
		case "MemAvailable:":
			available = v
		}
	}
	if total <= 0 {
		return 0, 0, false
	}
	used := total - available
	return used / total * 100.0, used, true
}

func diskUsage(path string) (usedPercent, usedBytes float64, ok bool) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return 0, 0, false
	}
	total := float64(st.Blocks) * float64(st.Bsize)
	free := float64(st.Bavail) * float64(st.Bsize)
	if total <= 0 {
		return 0, 0, false
	}
	used := total - free
	return used / total * 100.0, used, true
}

func loadAvg1() (float64, bool) {
	b, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return 0, false
	}
	fields := strings.Fields(string(b))
	if len(fields) < 1 {
		return 0, false
	}
	v, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return 0, false
	}
	return v, true
}
