package metadata

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"
)

const (
	metadataIP   = "169.254.169.254"
	apiPrefix    = "/capper/v1/"
	tokenTTL     = 6 * time.Hour
)

// InstanceLookup resolves the requesting instance by source IP.
type InstanceLookup func(srcIP string) (InstanceMetadata, bool)

// EventEmitter emits resource events.
type EventEmitter func(resourceType, id, action, project string, meta map[string]any)

// Server is the capmeta HTTP metadata service.
type Server struct {
	store   *Store
	tokens  *TokenManager
	signer  *Signer
	lookup  InstanceLookup
	emit    EventEmitter
	srv     *http.Server
}

// NewServer constructs the metadata server.
func NewServer(st *Store, tokens *TokenManager, signer *Signer, lookup InstanceLookup, emit EventEmitter) *Server {
	s := &Server{store: st, tokens: tokens, signer: signer, lookup: lookup, emit: emit}
	mux := http.NewServeMux()

	// /capper/v1/ — Capper-native API
	mux.HandleFunc("/capper/v1/", s.routeCapperV1)

	// /latest/ — EC2 IMDS compat
	mux.HandleFunc("/latest/meta-data/", s.handleLegacyMetaData)
	mux.HandleFunc("/latest/user-data", s.handleLegacyUserData)
	mux.HandleFunc("/latest/", s.handleLegacyRoot)

	// root
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprintln(w, "latest\ncapper/v1/")
	})

	s.srv = &http.Server{
		Addr:        metadataIP + ":80",
		Handler:     mux,
		ReadTimeout: 5 * time.Second,
	}
	return s
}

// ListenAndServe starts the HTTP server, blocking until ctx is cancelled.
func (s *Server) ListenAndServe(ctx context.Context) error {
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

// Close shuts down the server.
func (s *Server) Close() error { return s.srv.Close() }

// routeCapperV1 dispatches /capper/v1/* requests.
func (s *Server) routeCapperV1(w http.ResponseWriter, r *http.Request) {
	sub := strings.TrimPrefix(r.URL.Path, apiPrefix)
	sub = strings.TrimSuffix(sub, "/")

	srcIP := extractIP(r.RemoteAddr)
	meta, found := s.lookup(srcIP)

	if sub == "" {
		// Discovery endpoint — always public
		disc := map[string]string{
			"meta-data":    apiPrefix + "meta-data",
			"network-data": apiPrefix + "network-data",
			"user-data":    apiPrefix + "user-data",
			"signature":    apiPrefix + "signature",
		}
		if found && meta.AISessionID != "" {
			disc["identity"] = apiPrefix + "identity"
			disc["policy"] = apiPrefix + "policy"
			disc["secrets"] = apiPrefix + "secrets"
		}
		writeJSON(w, disc)
		return
	}

	// Public endpoints require only source IP resolution.
	isPublic := sub == "meta-data/instance-id" || sub == "meta-data/hostname"

	if !found {
		s.logAccess("", srcIP, sub, false, "no_instance")
		http.Error(w, "instance not found for source IP", http.StatusNotFound)
		return
	}

	// Token check for protected endpoints.
	if !isPublic {
		token := r.Header.Get("X-Capper-Token")
		if token == "" {
			s.logAccess(meta.InstanceID, srcIP, sub, false, "token_missing")
			s.emitEvent("metadata", meta.InstanceID, "metadata.fetch.denied", meta.Project,
				map[string]any{"endpoint": sub, "reason": "token_missing"})
			http.Error(w, "X-Capper-Token required", http.StatusUnauthorized)
			return
		}
		if !s.tokens.Validate(token, meta.InstanceID) {
			s.logAccess(meta.InstanceID, srcIP, sub, false, "token_invalid")
			s.emitEvent("metadata", meta.InstanceID, "metadata.fetch.denied", meta.Project,
				map[string]any{"endpoint": sub, "reason": "token_invalid"})
			http.Error(w, "invalid or expired token", http.StatusUnauthorized)
			return
		}
	}

	s.logAccess(meta.InstanceID, srcIP, sub, true, "token_ok")
	s.emitEvent("metadata", meta.InstanceID, "metadata.fetch.allowed", meta.Project,
		map[string]any{"endpoint": sub})

	switch {
	case sub == "meta-data" || sub == "meta-data/":
		writeText(w, "instance-id\nhostname\nproject\nlabels/\ninstance-type\n")
	case sub == "meta-data/instance-id":
		writeText(w, meta.InstanceID)
	case sub == "meta-data/hostname":
		writeText(w, meta.Hostname)
	case sub == "meta-data/project":
		writeText(w, meta.Project)
	case sub == "meta-data/instance-type":
		writeText(w, meta.InstanceType)
	case sub == "meta-data/labels" || sub == "meta-data/labels/":
		keys := make([]string, 0, len(meta.Labels))
		for k := range meta.Labels {
			keys = append(keys, k)
		}
		writeText(w, strings.Join(keys, "\n"))
	case strings.HasPrefix(sub, "meta-data/labels/"):
		key := strings.TrimPrefix(sub, "meta-data/labels/")
		if v, ok := meta.Labels[key]; ok {
			writeText(w, v)
		} else {
			http.NotFound(w, r)
		}
	case sub == "network-data":
		nd := NetworkData{
			DNS:     meta.DNS,
			Gateway: meta.Gateway,
			Interfaces: []InterfaceInfo{
				{Name: "eth0", IP: meta.NetworkIP, Gateway: meta.Gateway},
			},
		}
		writeJSON(w, nd)
	case sub == "user-data":
		if meta.UserData == "" {
			http.NotFound(w, r)
			return
		}
		writeText(w, meta.UserData)
	case sub == "signature":
		if s.signer == nil {
			http.Error(w, "signing not configured", http.StatusServiceUnavailable)
			return
		}
		metaDoc, _ := json.Marshal(meta)
		nd := NetworkData{DNS: meta.DNS, Gateway: meta.Gateway,
			Interfaces: []InterfaceInfo{{Name: "eth0", IP: meta.NetworkIP}}}
		netDoc, _ := json.Marshal(nd)
		bundle, err := s.signer.Sign(map[string][]byte{"meta-data": metaDoc, "network-data": netDoc})
		if err != nil {
			http.Error(w, "signing failed", http.StatusInternalServerError)
			return
		}
		writeJSON(w, bundle)
	case sub == "identity":
		if meta.AISessionID == "" {
			http.Error(w, "not an AI capsule", http.StatusNotFound)
			return
		}
		writeJSON(w, map[string]any{
			"sessionId":       meta.AISessionID,
			"agentId":         meta.AIAgentID,
			"model":           meta.AIModel,
			"assumedRole":     meta.AIAssumedRole,
			"mcpServers":      meta.AIMCPServers,
			"toolBroker":      meta.AIToolBroker,
			"modelGateway":    meta.AIModelGateway,
			"approvalEndpoint": meta.AIApprovalEndpoint,
		})
	case sub == "policy":
		if meta.AISessionID == "" {
			http.Error(w, "not an AI capsule", http.StatusNotFound)
			return
		}
		writeJSON(w, map[string]any{
			"allowedActions": meta.AllowedActions,
			"deniedActions":  meta.DeniedActions,
			"resourceLock":   meta.ResourceLock,
		})
	case sub == "secrets":
		if meta.AISessionID == "" {
			http.Error(w, "not an AI capsule", http.StatusNotFound)
			return
		}
		writeJSON(w, map[string]any{"secretRefs": meta.SecretRefs})
	default:
		http.NotFound(w, r)
	}
}

// handleLegacyMetaData serves EC2-IMDS compat at /latest/meta-data/.
func (s *Server) handleLegacyMetaData(w http.ResponseWriter, r *http.Request) {
	sub := strings.TrimPrefix(r.URL.Path, "/latest/meta-data/")
	sub = strings.TrimSuffix(sub, "/")
	srcIP := extractIP(r.RemoteAddr)
	meta, found := s.lookup(srcIP)
	if !found {
		http.Error(w, "instance not found", http.StatusNotFound)
		return
	}
	switch sub {
	case "", "/":
		writeText(w, "instance-id\nlocal-ipv4\nhostname\nlabels/\n")
	case "instance-id":
		writeText(w, meta.InstanceID)
	case "local-ipv4":
		writeText(w, meta.NetworkIP)
	case "hostname":
		writeText(w, meta.Hostname)
	case "labels", "labels/":
		keys := make([]string, 0, len(meta.Labels))
		for k := range meta.Labels {
			keys = append(keys, k)
		}
		writeText(w, strings.Join(keys, "\n"))
	default:
		if strings.HasPrefix(sub, "labels/") {
			key := strings.TrimPrefix(sub, "labels/")
			if v, ok := meta.Labels[key]; ok {
				writeText(w, v)
				return
			}
		}
		http.NotFound(w, r)
	}
}

func (s *Server) handleLegacyUserData(w http.ResponseWriter, r *http.Request) {
	srcIP := extractIP(r.RemoteAddr)
	meta, found := s.lookup(srcIP)
	if !found || meta.UserData == "" {
		http.NotFound(w, r)
		return
	}
	writeText(w, meta.UserData)
}

func (s *Server) handleLegacyRoot(w http.ResponseWriter, r *http.Request) {
	writeText(w, "meta-data/\nuser-data\n")
}

func (s *Server) logAccess(instanceID, srcIP, endpoint string, allowed bool, authStatus string) {
	_ = s.store.LogAccess(AccessLog{
		InstanceID: instanceID,
		SourceIP:   srcIP,
		Endpoint:   endpoint,
		Allowed:    allowed,
		AuthStatus: authStatus,
		CreatedAt:  time.Now().UTC().Format(time.RFC3339),
	})
}

func (s *Server) emitEvent(rtype, id, action, project string, meta map[string]any) {
	if s.emit != nil {
		s.emit(rtype, id, action, project, meta)
	}
}

func extractIP(remoteAddr string) string {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		return remoteAddr
	}
	return host
}

func writeText(w http.ResponseWriter, body string) {
	w.Header().Set("Content-Type", "text/plain")
	fmt.Fprint(w, body)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
