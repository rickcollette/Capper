// Package capinit implements a lightweight IMDS-compatible metadata service
// that listens on 169.254.169.254:80 and serves per-instance data to capsule
// instances. Requests are identified by source IP → network lease lookup.
package capinit

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"

	"capper/internal/network"
	"capper/internal/store"
	"capper/internal/types"
)

const MetadataIP = "169.254.169.254"

// Server is the instance metadata service.
type Server struct {
	st      *store.Store
	gateway string
	srv     *http.Server
}

// NewServer creates a metadata server backed by the given store.
// It binds to MetadataIP:80 and adds a loopback alias for that address.
func NewServer(st *store.Store) *Server {
	return NewServerWithAddr(st, MetadataIP+":80")
}

// NewServerWithAddr creates a metadata server bound to the given addr (host:port).
// Used for per-network gateway binding (one server per network bridge).
func NewServerWithAddr(st *store.Store, addr string) *Server {
	host, _, _ := net.SplitHostPort(addr)
	s := &Server{st: st, gateway: host}
	mux := http.NewServeMux()
	mux.HandleFunc("/capper/v1/", s.handleCapperV1)
	mux.HandleFunc("/latest/meta-data/", s.handleMetaData)
	mux.HandleFunc("/latest/user-data", s.handleUserData)
	mux.HandleFunc("/latest/", s.handleLatest)
	mux.HandleFunc("/", s.handleRoot)
	s.srv = &http.Server{
		Addr:    addr,
		Handler: mux,
	}
	return s
}

// ListenAndServe configures the loopback alias (only for 169.254.169.254 bindings)
// and starts serving. It blocks until the context is cancelled.
func (s *Server) ListenAndServe(ctx context.Context) error {
	host, _, _ := net.SplitHostPort(s.srv.Addr)
	if host == MetadataIP {
		if err := ensureLoopbackAlias(); err != nil {
			return fmt.Errorf("capinit: add loopback alias: %w", err)
		}
	}
	errCh := make(chan error, 1)
	go func() { errCh <- s.srv.ListenAndServe() }()
	select {
	case <-ctx.Done():
		_ = s.srv.Shutdown(context.Background())
		return nil
	case err := <-errCh:
		if err == http.ErrServerClosed {
			return nil
		}
		return err
	}
}

// Close shuts down the HTTP server.
func (s *Server) Close() error { return s.srv.Close() }

// ensureLoopbackAlias adds 169.254.169.254/32 to the loopback device if absent.
func ensureLoopbackAlias() error {
	lo, err := net.InterfaceByName("lo")
	if err != nil {
		return err
	}
	existing, err := lo.Addrs()
	if err != nil {
		return err
	}
	for _, a := range existing {
		if strings.HasPrefix(a.String(), MetadataIP) {
			return nil
		}
	}
	out, err := exec.Command("ip", "addr", "add", MetadataIP+"/32", "dev", "lo").CombinedOutput()
	if err != nil && !strings.Contains(string(out), "exists") {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// handleRoot lists available API versions.
func (s *Server) handleRoot(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	fmt.Fprintln(w, "latest")
}

// handleLatest lists top-level categories.
func (s *Server) handleLatest(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	fmt.Fprintln(w, "meta-data/")
	fmt.Fprintln(w, "user-data")
}

// handleMetaData serves EC2-IMDS-compatible metadata for the requesting instance.
func (s *Server) handleMetaData(w http.ResponseWriter, r *http.Request) {
	sub := strings.TrimPrefix(r.URL.Path, "/latest/meta-data/")
	sub = strings.TrimSuffix(sub, "/")

	if sub == "" {
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprintln(w, "instance-id")
		fmt.Fprintln(w, "local-ipv4")
		fmt.Fprintln(w, "hostname")
		fmt.Fprintln(w, "labels/")
		return
	}

	inst := s.instanceForRequest(r)
	if inst == nil {
		http.Error(w, "instance not found for source IP", http.StatusNotFound)
		return
	}

	switch {
	case sub == "instance-id":
		plainText(w, inst.ID)
	case sub == "local-ipv4":
		plainText(w, inst.NetworkIP)
	case sub == "hostname":
		plainText(w, inst.Name)
	case sub == "labels" || sub == "labels/":
		w.Header().Set("Content-Type", "text/plain")
		for k := range inst.Labels {
			fmt.Fprintln(w, k)
		}
	case strings.HasPrefix(sub, "labels/"):
		key := strings.TrimPrefix(sub, "labels/")
		if v, ok := inst.Labels[key]; ok {
			plainText(w, v)
		} else {
			http.NotFound(w, r)
		}
	default:
		http.NotFound(w, r)
	}
}

// handleUserData serves stored user-data (CapInit content) for the instance.
func (s *Server) handleUserData(w http.ResponseWriter, r *http.Request) {
	inst := s.instanceForRequest(r)
	if inst == nil {
		http.Error(w, "instance not found", http.StatusNotFound)
		return
	}
	meta, ok := s.loadInstanceMeta(inst.ID)
	if !ok {
		http.NotFound(w, r)
		return
	}
	userData, _ := meta["userData"].(string)
	if userData == "" {
		http.NotFound(w, r)
		return
	}
	plainText(w, userData)
}

// instanceForRequest resolves the requesting instance by matching the source IP.
func (s *Server) instanceForRequest(r *http.Request) *types.Instance {
	srcIP, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		srcIP = r.RemoteAddr
	}
	if srcIP == "" {
		return nil
	}
	instances, err := s.st.ListInstances()
	if err != nil {
		return nil
	}
	// Fast path: instance has NetworkIP recorded directly.
	for i := range instances {
		if instances[i].NetworkIP == srcIP {
			return &instances[i]
		}
	}
	// Slower path: cross-reference via lease table for cases where NetworkIP is stale.
	nets, err := network.NewManager(s.st.Networks).List("")
	if err != nil {
		return nil
	}
	for _, n := range nets {
		_, leases, err := network.NewManager(s.st.Networks).Inspect(n.ID, "")
		if err != nil {
			continue
		}
		for _, l := range leases {
			if l.IP != srcIP {
				continue
			}
			for i := range instances {
				if instances[i].ID == l.InstanceID {
					instances[i].NetworkIP = l.IP
					return &instances[i]
				}
			}
		}
	}
	return nil
}

func (s *Server) loadInstanceMeta(instanceID string) (map[string]any, bool) {
	path := s.st.Paths.Root + "/capinit/instance-metadata/" + instanceID + ".json"
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, false
	}
	return m, true
}

// handleCapperV1 serves the Capper-native /capper/v1/ API consumed by the
// in-instance capinit binary (cmd/capinit/main.go).
func (s *Server) handleCapperV1(w http.ResponseWriter, r *http.Request) {
	sub := strings.TrimPrefix(r.URL.Path, "/capper/v1/")
	sub = strings.TrimSuffix(sub, "/")

	switch sub {
	case "":
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, `{"meta-data":"/capper/v1/meta-data","network-data":"/capper/v1/network-data"}`)
	case "meta-data":
		inst := s.instanceForRequest(r)
		if inst == nil {
			http.Error(w, "instance not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"instanceId": inst.ID,
			"hostname":   inst.Name,
			"networkIp":  inst.NetworkIP,
		})
	case "network-data":
		inst := s.instanceForRequest(r)
		ip := ""
		if inst != nil {
			ip = inst.NetworkIP
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"dns":     s.gateway,
			"gateway": s.gateway,
			"interfaces": []map[string]any{
				{"name": "eth0", "ip": ip, "gateway": s.gateway},
			},
		})
	default:
		http.NotFound(w, r)
	}
}

func plainText(w http.ResponseWriter, v string) {
	w.Header().Set("Content-Type", "text/plain")
	fmt.Fprint(w, v)
}
