package lb

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"capper/internal/types"
)

// proxyRunner is the common interface for TCP and HTTP proxies.
type proxyRunner interface {
	Start(ctx context.Context) error
	Stop()
}

// InstanceLister supplies the current instance list for selector-mode backend sync.
type InstanceLister func() ([]types.Instance, error)

// NodeHealthChecker returns true if the node hosting an instance is healthy.
// Injected from topology to avoid circular imports.
type NodeHealthChecker func(nodeID string) bool

// Manager manages LB records and coordinates running proxies.
// Call Reconcile from the daemon's reconciler loop to start/stop proxies.
type Manager struct {
	store       *Store
	mu          sync.Mutex
	running     map[string]proxyRunner // keyed by lb ID
	certRes     CertResolver
	logDir      string
	instList    InstanceLister
	nodeHealthy NodeHealthChecker
}

func NewManager(s *Store) *Manager {
	return &Manager{store: s, running: make(map[string]proxyRunner)}
}

// SetCertResolver configures the TLS cert resolver used by TLS-terminated proxies.
func (m *Manager) SetCertResolver(fn CertResolver) { m.certRes = fn }

// SetLogDir sets the directory where LB request logs are written.
func (m *Manager) SetLogDir(dir string) { m.logDir = dir }

// SetInstanceLister configures the function used to resolve selector-mode backends.
func (m *Manager) SetInstanceLister(fn InstanceLister) { m.instList = fn }

// SetNodeHealthChecker installs a checker that filters selector-mode backends
// to only healthy nodes. Pass nil to disable health filtering.
func (m *Manager) SetNodeHealthChecker(fn NodeHealthChecker) { m.nodeHealthy = fn }

func (m *Manager) Store() *Store { return m.store }

// DNSAliasCreator creates a CNAME record so that alias resolves to target.
// Injected to avoid circular imports with capper/internal/dns.
type DNSAliasCreator func(zoneID, alias, target string) error

// SetAlias attaches a DNS CNAME alias to an existing load balancer.
// alias is a bare hostname (e.g. "myapp"); target is the LB's existing DNS name.
// The caller must supply a DNSAliasCreator that writes the CNAME record.
func (m *Manager) SetAlias(nameOrID, project, zoneID, alias, target string, createCNAME DNSAliasCreator) error {
	lb, err := m.store.Get(nameOrID, project)
	if err != nil {
		return fmt.Errorf("lb: alias: %w", err)
	}
	if err := m.store.SetServiceAlias(lb.ID, alias); err != nil {
		return fmt.Errorf("lb: alias: store: %w", err)
	}
	if createCNAME != nil && zoneID != "" && alias != "" && target != "" {
		if err := createCNAME(zoneID, alias, target); err != nil {
			return fmt.Errorf("lb: alias: dns: %w", err)
		}
	}
	return nil
}

// Create stores a new load balancer record.
func (m *Manager) Create(name, project, networkID, listenAddr string, mode LBMode) (LoadBalancer, error) {
	if name == "" {
		return LoadBalancer{}, fmt.Errorf("lb: name is required")
	}
	lb := LoadBalancer{
		ID:         newID(),
		Name:       name,
		Project:    project,
		NetworkID:  networkID,
		Mode:       mode,
		ListenAddr: listenAddr,
		Status:     StatusActive,
		CreatedAt:  time.Now().UTC().Format(time.RFC3339),
	}
	if err := m.store.Insert(lb); err != nil {
		return LoadBalancer{}, fmt.Errorf("lb: store: %w", err)
	}
	return lb, nil
}

// Get returns a load balancer by name or ID.
func (m *Manager) Get(nameOrID, project string) (LoadBalancer, error) {
	return m.store.Get(nameOrID, project)
}

// List returns all load balancers for the project.
func (m *Manager) List(project string) ([]LoadBalancer, error) {
	return m.store.List(project)
}

// Delete removes an LB and stops its proxy if running.
func (m *Manager) Delete(nameOrID, project string) error {
	lb, err := m.store.Get(nameOrID, project)
	if err != nil {
		return err
	}
	m.stopProxy(lb.ID)
	return m.store.Delete(nameOrID, project)
}

// Publish updates the listen address of an LB and restarts its proxy.
func (m *Manager) Publish(nameOrID, project, addr string) error {
	lb, err := m.store.Get(nameOrID, project)
	if err != nil {
		return err
	}
	if err := m.store.UpdateListenAddr(lb.ID, addr); err != nil {
		return err
	}
	// Restart proxy with new address.
	m.stopProxy(lb.ID)
	return nil
}

// AddBackend adds a backend to the LB and signals the proxy to reload.
func (m *Manager) AddBackend(nameOrID, project, address string) (Backend, error) {
	lb, err := m.store.Get(nameOrID, project)
	if err != nil {
		return Backend{}, err
	}
	b, err := m.store.AddBackend(lb.ID, address)
	if err != nil {
		return Backend{}, fmt.Errorf("lb: add backend: %w", err)
	}
	// Signal running proxy to reload its backend list on next tick.
	return b, nil
}

// RemoveBackend removes a backend from the LB.
func (m *Manager) RemoveBackend(nameOrID, project, address string) error {
	lb, err := m.store.Get(nameOrID, project)
	if err != nil {
		return err
	}
	return m.store.RemoveBackend(lb.ID, address)
}

// ListBackends returns all backends for the named LB.
func (m *Manager) ListBackends(nameOrID, project string) ([]Backend, error) {
	lb, err := m.store.Get(nameOrID, project)
	if err != nil {
		return nil, err
	}
	return m.store.ListBackends(lb.ID)
}

// Reconcile starts proxies for active LBs that are not yet running, syncs
// selector-mode backends, and stops proxies for deleted/stopped LBs.
func (m *Manager) Reconcile(ctx context.Context) error {
	active, err := m.store.ListActive()
	if err != nil {
		return err
	}

	// Sync selector-mode backends before starting proxies.
	if m.instList != nil {
		instances, ierr := m.instList()
		if ierr == nil {
			for _, lb := range active {
				if lb.Selector != "" {
					m.syncSelectorBackends(lb, instances)
				}
			}
		}
	}

	activeIDs := make(map[string]LoadBalancer, len(active))
	for _, lb := range active {
		activeIDs[lb.ID] = lb
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	for id, p := range m.running {
		if _, ok := activeIDs[id]; !ok {
			p.Stop()
			delete(m.running, id)
		}
	}

	for id, l := range activeIDs {
		if _, ok := m.running[id]; ok {
			continue
		}
		logPath := ""
		if m.logDir != "" {
			logPath = m.logDir + "/" + l.ID + ".log"
		}
		var p proxyRunner
		if l.Mode == ModeHTTP {
			p = newHTTPProxy(l, m.store, m.certRes, logPath)
		} else {
			p = newProxy(l, m.store, m.certRes, logPath)
		}
		m.running[id] = p
		go func(p proxyRunner) { _ = p.Start(ctx) }(p)
	}
	return nil
}

// syncSelectorBackends adds/removes backends for a selector-mode LB based on
// the current instance list. Selector format: "key=value".
func (m *Manager) syncSelectorBackends(lb LoadBalancer, instances []types.Instance) {
	k, v, ok := strings.Cut(lb.Selector, "=")
	if !ok {
		return
	}
	existing, _ := m.store.ListBackends(lb.ID)
	have := make(map[string]bool, len(existing))
	for _, b := range existing {
		have[b.Address] = true
	}
	for _, inst := range instances {
		if inst.Labels[k] != v || inst.NetworkIP == "" {
			continue
		}
		// Skip instances on unhealthy nodes when a health checker is wired in.
		if m.nodeHealthy != nil && inst.NodeID != "" && !m.nodeHealthy(inst.NodeID) {
			continue
		}
		addr := inst.NetworkIP + ":80"
		if !have[addr] {
			_, _ = m.store.AddBackend(lb.ID, addr)
		}
	}
}

// LBStat is a point-in-time stats snapshot for one load balancer.
type LBStat struct {
	LBID          string
	LBName        string
	TotalRequests uint64
	ActiveConns   int64
}

// RunningStats returns stats for all currently-running proxies.
func (m *Manager) RunningStats() []LBStat {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]LBStat, 0, len(m.running))
	for id, p := range m.running {
		stat := LBStat{LBID: id}
		switch pr := p.(type) {
		case *Proxy:
			stat.TotalRequests = pr.TotalRequests.Load()
			stat.ActiveConns = pr.ActiveConns.Load()
		case *HTTPProxy:
			stat.TotalRequests = pr.TotalRequests.Load()
			stat.ActiveConns = pr.ActiveConns.Load()
		}
		out = append(out, stat)
	}
	return out
}

func (m *Manager) stopProxy(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if p, ok := m.running[id]; ok {
		p.Stop()
		delete(m.running, id)
	}
}
