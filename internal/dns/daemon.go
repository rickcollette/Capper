package dns

import (
	"context"
	"fmt"
	"sync"

	mdns "github.com/miekg/dns"
)

// Daemon runs UDP and TCP DNS listeners backed by a Resolver.
type Daemon struct {
	addr    string
	udp     *mdns.Server
	tcp     *mdns.Server
	handler mdns.Handler
	mu      sync.Mutex
}

// NewDaemon creates a Daemon that will listen on addr (e.g. "127.0.0.1:1053")
// using the provided handler for all queries.
func NewDaemon(addr string, handler mdns.Handler) *Daemon {
	return &Daemon{
		addr:    addr,
		handler: handler,
	}
}

// Start launches the UDP and TCP listeners. Blocks until both are serving.
func (d *Daemon) Start() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.udp != nil {
		return fmt.Errorf("dns daemon already running on %s", d.addr)
	}

	readyCh := make(chan struct{}, 2)

	d.udp = &mdns.Server{
		Addr:              d.addr,
		Net:               "udp",
		Handler:           d.handler,
		NotifyStartedFunc: func() { readyCh <- struct{}{} },
	}
	d.tcp = &mdns.Server{
		Addr:              d.addr,
		Net:               "tcp",
		Handler:           d.handler,
		NotifyStartedFunc: func() { readyCh <- struct{}{} },
	}

	var wg sync.WaitGroup
	errCh := make(chan error, 2)

	wg.Add(2)
	go func() {
		defer wg.Done()
		if err := d.udp.ListenAndServe(); err != nil {
			errCh <- fmt.Errorf("dns udp: %w", err)
		}
	}()
	go func() {
		defer wg.Done()
		if err := d.tcp.ListenAndServe(); err != nil {
			errCh <- fmt.Errorf("dns tcp: %w", err)
		}
	}()

	// Wait for both servers to signal ready or one to fail.
	ready := 0
	for ready < 2 {
		select {
		case <-readyCh:
			ready++
		case err := <-errCh:
			return err
		}
	}
	return nil
}

// Stop shuts down both listeners.
func (d *Daemon) Stop() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	var firstErr error
	if d.udp != nil {
		if err := d.udp.ShutdownContext(context.Background()); err != nil && firstErr == nil {
			firstErr = err
		}
		d.udp = nil
	}
	if d.tcp != nil {
		if err := d.tcp.ShutdownContext(context.Background()); err != nil && firstErr == nil {
			firstErr = err
		}
		d.tcp = nil
	}
	return firstErr
}

// Addr returns the listen address.
func (d *Daemon) Addr() string { return d.addr }
