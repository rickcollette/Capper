package lb

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"sync/atomic"
	"time"
)

// HTTPProxy is a reverse-proxy load balancer for ModeHTTP load balancers.
type HTTPProxy struct {
	lb           LoadBalancer
	store        *Store
	certResolver CertResolver
	logPath      string
	srv          *http.Server
	cursor       uint64
	cancel       context.CancelFunc

	TotalRequests atomic.Uint64
	ActiveConns   atomic.Int64
}

func newHTTPProxy(lb LoadBalancer, store *Store, certRes CertResolver, logPath string) *HTTPProxy {
	return &HTTPProxy{lb: lb, store: store, certResolver: certRes, logPath: logPath}
}

func (p *HTTPProxy) Start(ctx context.Context) error {
	ctx, p.cancel = context.WithCancel(ctx)

	var logFile *os.File
	if p.logPath != "" {
		f, err := os.OpenFile(p.logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		if err == nil {
			logFile = f
			defer f.Close()
		}
	}

	rp := &httputil.ReverseProxy{
		Director: p.director,
		Transport: &http.Transport{
			DialContext: (&net.Dialer{Timeout: dialTimeout}).DialContext,
		},
		ModifyResponse: func(resp *http.Response) error {
			if logFile != nil {
				fmt.Fprintf(logFile, "%s %s %s %d\n",
					time.Now().UTC().Format(time.RFC3339),
					resp.Request.Method,
					resp.Request.URL.String(),
					resp.StatusCode,
				)
			}
			return nil
		},
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			if logFile != nil {
				fmt.Fprintf(logFile, "%s %s %s error: %v\n",
					time.Now().UTC().Format(time.RFC3339),
					r.Method, r.URL.String(), err)
			}
			http.Error(w, fmt.Sprintf("lb: no healthy backend: %v", err), http.StatusBadGateway)
		},
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p.TotalRequests.Add(1)
		p.ActiveConns.Add(1)
		defer p.ActiveConns.Add(-1)
		rp.ServeHTTP(w, r)
	})

	p.srv = &http.Server{
		Addr:        p.lb.ListenAddr,
		Handler:     handler,
		ReadTimeout: 30 * time.Second,
	}

	go func() {
		<-ctx.Done()
		_ = p.srv.Close()
	}()

	if p.lb.TLSCertName != "" && p.certResolver != nil {
		cert, key, err := p.certResolver(p.lb.TLSCertName)
		if err == nil {
			tlsCert, err := tls.X509KeyPair(cert, key)
			if err == nil {
				p.srv.TLSConfig = &tls.Config{Certificates: []tls.Certificate{tlsCert}}
				if err := p.srv.ListenAndServeTLS("", ""); err != nil && err != http.ErrServerClosed {
					return err
				}
				return nil
			}
		}
	}

	if err := p.srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

func (p *HTTPProxy) Stop() {
	if p.cancel != nil {
		p.cancel()
	}
}

func (p *HTTPProxy) director(req *http.Request) {
	backends, err := p.store.ListBackends(p.lb.ID)
	if err != nil || len(backends) == 0 {
		return
	}
	var addr string
	if p.lb.Algorithm == AlgoLeastConnections {
		// HTTP proxy doesn't track per-backend connection counts here;
		// fall back to round-robin as an approximation.
		addr = backends[0].Address
	} else {
		idx := atomic.AddUint64(&p.cursor, 1) - 1
		addr = backends[int(idx)%len(backends)].Address
	}
	target, err := url.Parse("http://" + addr)
	if err != nil {
		return
	}
	req.URL.Scheme = target.Scheme
	req.URL.Host = target.Host
	if req.Header.Get("X-Forwarded-For") == "" {
		if host, _, err := net.SplitHostPort(req.RemoteAddr); err == nil {
			req.Header.Set("X-Forwarded-For", host)
		}
	}
}
