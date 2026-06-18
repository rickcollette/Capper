package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"capper/internal/version"
)

const (
	nodeIDFile    = "/etc/capper/node-id"
	agentTokenFile = "/etc/capper/agent-token"
	socketPath    = "/run/capper-agent.sock"
)

// Agent is the Capper node agent daemon.
type Agent struct {
	cfg        Config
	nodeID     string
	token      string
	httpClient *http.Client
	supervisor *Supervisor
	localAPI   *LocalAPI
}

// New creates a new Agent from the given config.
func New(cfg Config) *Agent {
	a := &Agent{
		cfg:        cfg,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
	a.supervisor = NewSupervisor()
	a.localAPI = NewLocalAPI(a, socketPath)
	return a
}

// Run starts the agent: joins the control plane, then runs heartbeat and
// local API loops until ctx is cancelled.
func (a *Agent) Run(ctx context.Context) error {
	if err := a.loadOrJoin(ctx); err != nil {
		return err
	}

	errCh := make(chan error, 3)
	go func() { errCh <- a.localAPI.Serve(ctx) }()
	go func() { errCh <- a.heartbeatLoop(ctx) }()
	go func() { errCh <- a.metricsLoop(ctx) }()

	select {
	case <-ctx.Done():
		return nil
	case err := <-errCh:
		return err
	}
}

// loadOrJoin loads a persisted node ID / token, or registers with the control plane.
func (a *Agent) loadOrJoin(ctx context.Context) error {
	id, err := os.ReadFile(nodeIDFile)
	tok, terr := os.ReadFile(agentTokenFile)
	if err == nil && terr == nil {
		a.nodeID = string(id)
		a.token = string(tok)
		return nil
	}
	return a.join(ctx)
}

func (a *Agent) join(ctx context.Context) error {
	body := map[string]any{
		"name":          a.cfg.Node.Name,
		"realm":         a.cfg.Node.Realm,
		"region":        a.cfg.Node.Region,
		"zone":          a.cfg.Node.Zone,
		"roles":         a.cfg.Node.Roles,
		"failureDomain": a.cfg.Node.FailureDomain,
		"agentVersion":  version.Version,
	}
	data, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		a.cfg.ControlPlane.URL+"/api/v1/nodes/join", bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := a.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("agent: join: %w", err)
	}
	defer resp.Body.Close()

	// If the zone doesn't exist yet (404 or 400), bootstrap topology and retry once.
	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusBadRequest {
		if bErr := a.bootstrapTopology(ctx); bErr != nil {
			return fmt.Errorf("agent: bootstrap topology: %w", bErr)
		}
		return a.join(ctx)
	}
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("agent: join: server returned %d", resp.StatusCode)
	}

	var result struct {
		Data struct {
			ID    string `json:"id"`
			Token string `json:"token"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("agent: join: decode response: %w", err)
	}
	a.nodeID = result.Data.ID
	a.token = result.Data.Token

	// Persist for reconnects.
	if err := os.MkdirAll(filepath.Dir(nodeIDFile), 0o755); err == nil {
		_ = os.WriteFile(nodeIDFile, []byte(a.nodeID), 0o600)
		_ = os.WriteFile(agentTokenFile, []byte(a.token), 0o600)
	}
	return nil
}

func (a *Agent) bootstrapTopology(ctx context.Context) error {
	type topo struct {
		path string
		body map[string]any
	}
	steps := []topo{
		{"/api/v1/topology/realms", map[string]any{"name": "Local", "slug": "local"}},
		{"/api/v1/topology/regions", map[string]any{"name": "Local", "slug": "local", "realmId": "realm_local"}},
		{"/api/v1/topology/zones", map[string]any{"name": "Local A", "slug": "local-a", "regionId": "region_local"}},
	}
	for _, s := range steps {
		data, _ := json.Marshal(s.body)
		req, _ := http.NewRequestWithContext(ctx, http.MethodPost,
			a.cfg.ControlPlane.URL+s.path, bytes.NewReader(data))
		req.Header.Set("Content-Type", "application/json")
		resp, err := a.httpClient.Do(req)
		if err != nil {
			return err
		}
		resp.Body.Close()
	}
	return nil
}

func (a *Agent) heartbeatLoop(ctx context.Context) error {
	ticker := time.NewTicker(a.cfg.ControlPlane.HeartbeatInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			_ = a.sendHeartbeat(ctx)
		}
	}
}

func (a *Agent) sendHeartbeat(ctx context.Context) error {
	if a.nodeID == "" {
		return nil
	}
	body := map[string]any{
		"status":    "ready",
		"inventory": a.supervisor.Inventory(),
		"version":   version.Version,
	}
	data, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		a.cfg.ControlPlane.URL+"/api/v1/nodes/"+a.nodeID+"/heartbeat",
		bytes.NewReader(data))
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

// Status returns the agent's current status summary.
func (a *Agent) Status() map[string]any {
	return map[string]any{
		"nodeID":    a.nodeID,
		"name":      a.cfg.Node.Name,
		"realm":     a.cfg.Node.Realm,
		"region":    a.cfg.Node.Region,
		"zone":      a.cfg.Node.Zone,
		"roles":     a.cfg.Node.Roles,
		"services":  a.supervisor.ServiceStates(),
	}
}
