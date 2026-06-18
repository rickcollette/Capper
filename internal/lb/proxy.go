package lb

import (
	"context"
	"crypto/tls"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

const (
	healthInterval = 10 * time.Second
	healthTimeout  = 2 * time.Second
	dialTimeout    = 3 * time.Second
)

// backendState tracks live health and connection count for a backend address.
type backendState struct {
	address string
	healthy atomic.Bool
	conns   atomic.Int64 // active connections (for least-connections)
}

// Proxy is a running TCP load balancer for a single LoadBalancer record.
type Proxy struct {
	lb           LoadBalancer
	store        *Store
	certResolver CertResolver
	logPath      string
	backends     []*backendState
	mu           sync.RWMutex
	cursor       uint64

	// metrics
	TotalRequests  atomic.Uint64
	ActiveConns    atomic.Int64

	cancel context.CancelFunc
}

// CertResolver returns (certPEM, keyPEM) for a named cert.
type CertResolver func(name string) (certPEM, keyPEM []byte, err error)

func newProxy(lb LoadBalancer, store *Store, certRes CertResolver, logPath string) *Proxy {
	return &Proxy{lb: lb, store: store, certResolver: certRes, logPath: logPath}
}

// Start begins the TCP listener and health-check loop. Blocks until ctx done.
func (p *Proxy) Start(ctx context.Context) error {
	ctx, p.cancel = context.WithCancel(ctx)

	if err := p.reloadBackends(); err != nil {
		return err
	}

	var ln net.Listener
	var err error

	if p.lb.TLSCertName != "" && p.certResolver != nil {
		cert, key, cerr := p.certResolver(p.lb.TLSCertName)
		if cerr == nil {
			tlsCert, cerr := tls.X509KeyPair(cert, key)
			if cerr == nil {
				cfg := &tls.Config{Certificates: []tls.Certificate{tlsCert}}
				ln, err = tls.Listen("tcp", p.lb.ListenAddr, cfg)
			} else {
				err = cerr
			}
		} else {
			err = cerr
		}
	}
	if ln == nil && err == nil {
		ln, err = net.Listen("tcp", p.lb.ListenAddr)
	}
	if err != nil {
		return err
	}

	go func() {
		ticker := time.NewTicker(healthInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				p.reloadBackends() //nolint
				p.checkHealth()
			}
		}
	}()

	go func() {
		<-ctx.Done()
		ln.Close()
	}()

	for {
		conn, err := ln.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
				return err
			}
		}
		p.TotalRequests.Add(1)
		p.ActiveConns.Add(1)
		go func() {
			defer p.ActiveConns.Add(-1)
			p.handle(conn)
		}()
	}
}

func (p *Proxy) Stop() {
	if p.cancel != nil {
		p.cancel()
	}
}

func (p *Proxy) handle(client net.Conn) {
	defer client.Close()
	backend := p.nextBackend()
	if backend == nil {
		return
	}
	backend.conns.Add(1)
	defer backend.conns.Add(-1)

	upstream, err := net.DialTimeout("tcp", backend.address, dialTimeout)
	if err != nil {
		backend.healthy.Store(false)
		return
	}
	defer upstream.Close()
	done := make(chan struct{}, 2)
	go func() { io.Copy(upstream, client); done <- struct{}{} }() //nolint
	go func() { io.Copy(client, upstream); done <- struct{}{} }() //nolint
	<-done
}

func (p *Proxy) nextBackend() *backendState {
	p.mu.RLock()
	bs := p.backends
	p.mu.RUnlock()
	if len(bs) == 0 {
		return nil
	}

	if p.lb.Algorithm == AlgoLeastConnections {
		return p.leastConnections(bs)
	}
	return p.roundRobin(bs)
}

func (p *Proxy) roundRobin(bs []*backendState) *backendState {
	start := atomic.AddUint64(&p.cursor, 1) - 1
	for i := range bs {
		b := bs[(int(start)+i)%len(bs)]
		if b.healthy.Load() {
			return b
		}
	}
	return nil
}

func (p *Proxy) leastConnections(bs []*backendState) *backendState {
	var best *backendState
	for _, b := range bs {
		if !b.healthy.Load() {
			continue
		}
		if best == nil || b.conns.Load() < best.conns.Load() {
			best = b
		}
	}
	return best
}

// nextHealthy returns the next healthy backend address (used by HTTP proxy director).
func (p *Proxy) nextHealthy() string {
	b := p.nextBackend()
	if b == nil {
		return ""
	}
	return b.address
}

func (p *Proxy) reloadBackends() error {
	stored, err := p.store.ListBackends(p.lb.ID)
	if err != nil {
		return err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	existing := make(map[string]*backendState, len(p.backends))
	for _, b := range p.backends {
		existing[b.address] = b
	}
	next := make([]*backendState, 0, len(stored))
	for _, s := range stored {
		if prev, ok := existing[s.Address]; ok {
			next = append(next, prev)
		} else {
			bs := &backendState{address: s.Address}
			bs.healthy.Store(true)
			next = append(next, bs)
		}
	}
	p.backends = next
	return nil
}

func (p *Proxy) checkHealth() {
	p.mu.RLock()
	bs := p.backends
	p.mu.RUnlock()
	var wg sync.WaitGroup
	for _, b := range bs {
		b := b
		wg.Add(1)
		go func() {
			defer wg.Done()
			conn, err := net.DialTimeout("tcp", b.address, healthTimeout)
			if err != nil {
				b.healthy.Store(false)
				return
			}
			conn.Close()
			b.healthy.Store(true)
		}()
	}
	wg.Wait()
}

// BackendStats returns a snapshot of per-backend connection counts and health.
func (p *Proxy) BackendStats() []BackendStat {
	p.mu.RLock()
	bs := p.backends
	p.mu.RUnlock()
	out := make([]BackendStat, len(bs))
	for i, b := range bs {
		out[i] = BackendStat{
			Address:     b.address,
			Healthy:     b.healthy.Load(),
			ActiveConns: b.conns.Load(),
		}
	}
	return out
}

// BackendStat is a point-in-time snapshot for one backend.
type BackendStat struct {
	Address     string
	Healthy     bool
	ActiveConns int64
}
