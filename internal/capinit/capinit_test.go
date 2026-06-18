package capinit_test

import (
	"context"
	"io"
	"net"
	"net/http"
	"testing"
	"time"

	"capper/internal/capinit"
	"capper/internal/store"
)

func openStore(t *testing.T) *store.Store {
	t.Helper()
	root := t.TempDir()
	paths := store.NewPaths(root)
	st, err := store.Open(paths)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	return st
}

// freePort returns a free TCP port on loopback.
func freePort(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("freePort: %v", err)
	}
	addr := l.Addr().String()
	l.Close()
	return addr
}

func startServer(t *testing.T) string {
	t.Helper()
	st := openStore(t)
	addr := freePort(t)
	srv := capinit.NewServerWithAddr(st, addr)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	ready := make(chan struct{})
	go func() {
		close(ready)
		_ = srv.ListenAndServe(ctx)
	}()
	<-ready
	// Allow a moment for the server to bind.
	time.Sleep(20 * time.Millisecond)
	return "http://" + addr
}

func get(t *testing.T, baseURL, path string) (int, string) {
	t.Helper()
	resp, err := http.Get(baseURL + path)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(body)
}

// ---- Route: / ---------------------------------------------------------------

func TestHandleRoot(t *testing.T) {
	base := startServer(t)
	code, body := get(t, base, "/")
	if code != http.StatusOK {
		t.Errorf("status: %d", code)
	}
	if body != "latest\n" {
		t.Errorf("body: %q", body)
	}
}

// ---- Route: /latest/ --------------------------------------------------------

func TestHandleLatest(t *testing.T) {
	base := startServer(t)
	code, body := get(t, base, "/latest/")
	if code != http.StatusOK {
		t.Errorf("status: %d", code)
	}
	if body == "" {
		t.Error("expected non-empty body")
	}
}

// ---- Route: /latest/meta-data/ (index) --------------------------------------

func TestHandleMetaData_Index(t *testing.T) {
	base := startServer(t)
	code, body := get(t, base, "/latest/meta-data/")
	if code != http.StatusOK {
		t.Errorf("status: %d", code)
	}
	// Should list available keys like instance-id, local-ipv4, hostname.
	if body == "" {
		t.Error("expected metadata key listing")
	}
}

// ---- Route: /capper/v1/ (index) --------------------------------------------

func TestHandleCapperV1_Index(t *testing.T) {
	base := startServer(t)
	code, body := get(t, base, "/capper/v1/")
	if code != http.StatusOK {
		t.Errorf("status: %d", code)
	}
	if body == "" {
		t.Error("expected non-empty JSON response")
	}
}

// ---- Route: /capper/v1/network-data ----------------------------------------

func TestHandleCapperV1_NetworkData(t *testing.T) {
	base := startServer(t)
	code, body := get(t, base, "/capper/v1/network-data")
	if code != http.StatusOK {
		t.Errorf("status: %d", code)
	}
	if body == "" {
		t.Error("expected JSON network data")
	}
}

// ---- Route: /capper/v1/unknown ----------------------------------------------

func TestHandleCapperV1_Unknown(t *testing.T) {
	base := startServer(t)
	// /capper/v1/ prefix is matched; unknown sub-path yields 404.
	code, _ := get(t, base, "/capper/v1/no-such-endpoint")
	if code != http.StatusNotFound {
		t.Errorf("expected 404 for unknown capper/v1 path, got %d", code)
	}
}
