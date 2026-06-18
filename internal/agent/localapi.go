package agent

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"os"
)

// LocalAPI serves a Unix socket API for local tooling.
type LocalAPI struct {
	agent  *Agent
	socket string
}

// NewLocalAPI creates a LocalAPI that will listen on socketPath.
func NewLocalAPI(a *Agent, socketPath string) *LocalAPI {
	return &LocalAPI{agent: a, socket: socketPath}
}

// Serve starts the Unix socket server and blocks until ctx is cancelled.
func (l *LocalAPI) Serve(ctx context.Context) error {
	_ = os.Remove(l.socket)
	ln, err := net.Listen("unix", l.socket)
	if err != nil {
		return err
	}
	defer func() {
		ln.Close()
		os.Remove(l.socket)
	}()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /status", l.handleStatus)
	mux.HandleFunc("GET /services", l.handleServices)
	mux.HandleFunc("GET /inventory", l.handleInventory)
	mux.HandleFunc("POST /doctor", l.handleDoctor)
	// Host security (this node's host OS): the agent owns fail2ban/UFW on the
	// node it runs on, via the same exclusive workers used by the control daemon.
	mux.HandleFunc("GET /hostsec/fail2ban/status", l.handleF2BStatus)
	mux.HandleFunc("POST /hostsec/fail2ban/ban", l.handleF2BBan)
	mux.HandleFunc("POST /hostsec/fail2ban/unban", l.handleF2BUnban)
	mux.HandleFunc("GET /hostsec/ufw/status", l.handleUFWStatus)
	mux.HandleFunc("POST /hostsec/ufw/rules", l.handleUFWAddRule)
	mux.HandleFunc("DELETE /hostsec/ufw/rules/{num}", l.handleUFWDeleteRule)

	srv := &http.Server{Handler: mux}
	go func() {
		<-ctx.Done()
		_ = srv.Close()
	}()
	return srv.Serve(ln)
}

func (l *LocalAPI) handleStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(l.agent.Status())
}

func (l *LocalAPI) handleServices(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(l.agent.supervisor.ServiceStates())
}

func (l *LocalAPI) handleInventory(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(l.agent.supervisor.Inventory())
}

func (l *LocalAPI) handleDoctor(w http.ResponseWriter, r *http.Request) {
	status := l.agent.Status()
	status["ok"] = true
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(status)
}
