package certmgr

import (
	"fmt"
	"net/http"
	"path"
	"sync"
)

// HTTPSolver implements the ACME http-01 challenge by storing token→keyAuth in memory.
type HTTPSolver struct {
	mu     sync.RWMutex
	tokens map[string]string
}

func NewHTTPSolver() *HTTPSolver {
	return &HTTPSolver{tokens: make(map[string]string)}
}

func (s *HTTPSolver) Present(domain, token, keyAuth string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tokens[token] = keyAuth
	return nil
}

func (s *HTTPSolver) CleanUp(domain, token, keyAuth string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.tokens, token)
	return nil
}

// ServeChallenge handles GET /.well-known/acme-challenge/{token}
func (s *HTTPSolver) ServeChallenge(w http.ResponseWriter, r *http.Request) {
	token := path.Base(r.URL.Path)
	s.mu.RLock()
	keyAuth, ok := s.tokens[token]
	s.mu.RUnlock()
	if !ok {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/plain")
	fmt.Fprint(w, keyAuth)
}
