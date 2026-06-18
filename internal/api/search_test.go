package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"capper/internal/controller"
	"capper/internal/store"
)

func openTestServer(t *testing.T) *Server {
	t.Helper()
	tmp := t.TempDir()
	paths := store.NewPaths(tmp)
	st, err := store.Open(paths)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { st.DB.Close() })
	ctrl := controller.New(st, false, "none")
	return NewServer(ctrl, Options{})
}

// TestSearchEmptyReturnsEmptyList verifies that a fresh store returns an empty results list.
func TestSearchEmptyReturnsEmptyList(t *testing.T) {
	srv := openTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/search?q=nothing", nil)
	rr := httptest.NewRecorder()
	srv.mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d — body: %s", rr.Code, rr.Body)
	}
	var resp struct {
		Results []any `json:"results"`
		Count   int   `json:"count"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Count != 0 {
		t.Errorf("expected 0 results, got %d", resp.Count)
	}
}

// TestSearchTypeFilter verifies that type=instances limits results to instances.
func TestSearchTypeFilter(t *testing.T) {
	srv := openTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/search?type=instances", nil)
	rr := httptest.NewRecorder()
	srv.mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var resp map[string]any
	json.NewDecoder(rr.Body).Decode(&resp)

	results, ok := resp["results"].([]any)
	if !ok {
		t.Fatalf("results field missing or wrong type: %T", resp["results"])
	}
	for _, r := range results {
		m := r.(map[string]any)
		if m["type"].(string) != "instance" {
			t.Errorf("type filter broken: got result of type %q", m["type"])
		}
	}
}

// TestSearchAllTypesInResponse verifies the response has results and count fields.
func TestSearchAllTypesInResponse(t *testing.T) {
	srv := openTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/search", nil)
	rr := httptest.NewRecorder()
	srv.mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var resp map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if _, ok := resp["results"]; !ok {
		t.Error("response missing 'results' field")
	}
	if _, ok := resp["count"]; !ok {
		t.Error("response missing 'count' field")
	}
}
