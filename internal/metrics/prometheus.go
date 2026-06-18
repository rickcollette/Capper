package metrics

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"capper/internal/dns"
	"capper/internal/lb"
	"capper/internal/types"
)

// InstanceLister is satisfied by store.Store.ListInstances.
type InstanceLister func() ([]types.Instance, error)

// LBStatsLister is satisfied by lb.Manager.RunningStats.
type LBStatsLister func() []lb.LBStat

// DNSStatsFunc is satisfied by dns.Resolver.Stats.
type DNSStatsFunc func() dns.ResolverStats

// NewPrometheusServer returns an HTTP server that exposes instance cgroup
// and LB metrics at /metrics in Prometheus text format.
func NewPrometheusServer(addr string, listInstances InstanceLister) *http.Server {
	return NewPrometheusServerFull(addr, listInstances, nil, nil)
}

// NewPrometheusServerWithLB is like NewPrometheusServer but also exposes LB metrics.
func NewPrometheusServerWithLB(addr string, listInstances InstanceLister, lbStats LBStatsLister) *http.Server {
	return NewPrometheusServerFull(addr, listInstances, lbStats, nil)
}

// NewPrometheusServerFull exposes instance, LB, and DNS metrics.
func NewPrometheusServerFull(addr string, listInstances InstanceLister, lbStats LBStatsLister, dnsStats DNSStatsFunc) *http.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		writeMetrics(w, listInstances, lbStats, dnsStats)
	})
	return &http.Server{
		Addr:        addr,
		Handler:     mux,
		ReadTimeout: 5 * time.Second,
	}
}

func writeMetrics(w http.ResponseWriter, listInstances InstanceLister, lbStats LBStatsLister, dnsStats DNSStatsFunc) {
	instances, err := listInstances()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var sb strings.Builder
	sb.WriteString("# HELP capper_instance_cpu_usage_micros Cumulative CPU usage in microseconds\n")
	sb.WriteString("# TYPE capper_instance_cpu_usage_micros counter\n")
	for _, inst := range instances {
		if inst.Status != types.StatusRunning {
			continue
		}
		m := ReadInstance(inst.ID, inst.Name)
		lbl := fmt.Sprintf(`instance_id=%q,instance_name=%q`, inst.ID, inst.Name)
		sb.WriteString(fmt.Sprintf("capper_instance_cpu_usage_micros{%s} %d\n", lbl, m.CPUUsageUs))
	}

	sb.WriteString("# HELP capper_instance_memory_bytes Current memory usage in bytes\n")
	sb.WriteString("# TYPE capper_instance_memory_bytes gauge\n")
	for _, inst := range instances {
		if inst.Status != types.StatusRunning {
			continue
		}
		m := ReadInstance(inst.ID, inst.Name)
		lbl := fmt.Sprintf(`instance_id=%q,instance_name=%q`, inst.ID, inst.Name)
		sb.WriteString(fmt.Sprintf("capper_instance_memory_bytes{%s} %d\n", lbl, m.MemoryBytes))
	}

	sb.WriteString("# HELP capper_instance_pid_count Current PID count\n")
	sb.WriteString("# TYPE capper_instance_pid_count gauge\n")
	for _, inst := range instances {
		if inst.Status != types.StatusRunning {
			continue
		}
		m := ReadInstance(inst.ID, inst.Name)
		lbl := fmt.Sprintf(`instance_id=%q,instance_name=%q`, inst.ID, inst.Name)
		sb.WriteString(fmt.Sprintf("capper_instance_pid_count{%s} %d\n", lbl, m.PIDCount))
	}

	// LB metrics
	if lbStats != nil {
		stats := lbStats()
		sb.WriteString("# HELP capper_lb_requests_total Total requests handled by this load balancer\n")
		sb.WriteString("# TYPE capper_lb_requests_total counter\n")
		for _, s := range stats {
			lbl := fmt.Sprintf(`lb_id=%q,lb_name=%q`, s.LBID, s.LBName)
			sb.WriteString(fmt.Sprintf("capper_lb_requests_total{%s} %d\n", lbl, s.TotalRequests))
		}
		sb.WriteString("# HELP capper_lb_active_connections Active connections on this load balancer\n")
		sb.WriteString("# TYPE capper_lb_active_connections gauge\n")
		for _, s := range stats {
			lbl := fmt.Sprintf(`lb_id=%q,lb_name=%q`, s.LBID, s.LBName)
			sb.WriteString(fmt.Sprintf("capper_lb_active_connections{%s} %d\n", lbl, s.ActiveConns))
		}
	}

	if dnsStats != nil {
		s := dnsStats()
		sb.WriteString("# HELP capper_dns_queries_total Total DNS queries received\n")
		sb.WriteString("# TYPE capper_dns_queries_total counter\n")
		sb.WriteString(fmt.Sprintf("capper_dns_queries_total %d\n", s.Total))
		sb.WriteString("# HELP capper_dns_queries_hit_total DNS queries answered from local store\n")
		sb.WriteString("# TYPE capper_dns_queries_hit_total counter\n")
		sb.WriteString(fmt.Sprintf("capper_dns_queries_hit_total %d\n", s.Hit))
		sb.WriteString("# HELP capper_dns_queries_forwarded_total DNS queries forwarded to upstream\n")
		sb.WriteString("# TYPE capper_dns_queries_forwarded_total counter\n")
		sb.WriteString(fmt.Sprintf("capper_dns_queries_forwarded_total %d\n", s.Forward))
		sb.WriteString("# HELP capper_dns_queries_nxdomain_total DNS queries that returned NXDOMAIN\n")
		sb.WriteString("# TYPE capper_dns_queries_nxdomain_total counter\n")
		sb.WriteString(fmt.Sprintf("capper_dns_queries_nxdomain_total %d\n", s.NXDomain))
	}

	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	fmt.Fprint(w, sb.String())
}
