package api

// certRoutes registers all Certificate Manager API routes.
// Called from routes() in server.go.
func (s *Server) certRoutes() {
	// ACME accounts
	s.mux.HandleFunc("GET /api/v1/certificates/acme/accounts", s.handleListACMEAccounts)
	s.mux.HandleFunc("POST /api/v1/certificates/acme/accounts", s.handleCreateACMEAccount)
	s.mux.HandleFunc("GET /api/v1/certificates/acme/accounts/{acmeAccount}", s.handleGetACMEAccount)
	s.mux.HandleFunc("DELETE /api/v1/certificates/acme/accounts/{acmeAccount}", s.handleDeleteACMEAccount)

	// Certificates
	s.mux.HandleFunc("GET /api/v1/certificates", s.handleListCertificates)
	s.mux.HandleFunc("POST /api/v1/certificates", s.handleCreateCertificate)
	s.mux.HandleFunc("GET /api/v1/certificates/{cert}", s.handleGetCertificate)
	s.mux.HandleFunc("DELETE /api/v1/certificates/{cert}", s.handleDeleteCertificate)
	s.mux.HandleFunc("POST /api/v1/certificates/{cert}/renew", s.handleRenewCertificate)
	s.mux.HandleFunc("POST /api/v1/certificates/{cert}/reissue", s.handleReissueCertificate)
	s.mux.HandleFunc("POST /api/v1/certificates/{cert}/revoke", s.handleRevokeCertificate)

	// Certificate bindings
	s.mux.HandleFunc("GET /api/v1/certificates/{cert}/bindings", s.handleListCertBindings)
	s.mux.HandleFunc("POST /api/v1/certificates/{cert}/bindings", s.handleCreateCertBinding)
	s.mux.HandleFunc("DELETE /api/v1/certificates/{cert}/bindings/{binding}", s.handleDeleteCertBinding)

	// LB certificate attachment
	s.mux.HandleFunc("POST /api/v1/lb/{lb}/certificates", s.handleAttachCertToLB)
	s.mux.HandleFunc("DELETE /api/v1/lb/{lb}/certificates/{cert}", s.handleDetachCertFromLB)
}
