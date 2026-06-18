package s3server

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"

	"capper/internal/audit"

	"github.com/gin-gonic/gin"
)

// Config holds the S3 server configuration.
type Config struct {
	// ListenAddr is the address:port to listen on (default: 127.0.0.1:9000).
	ListenAddr string
	// StorageDir is the root directory where bucket data lives.
	// Typically ~/.local/share/capper/storage/objects
	StorageDir string
	// Buckets provides bucket lifecycle backed by Capper storage.
	Buckets BucketProvider
	// Credentials provides SigV4 credential lookup.
	// If nil and ProductionMode is false, authentication is skipped (dev only).
	// If nil and ProductionMode is true, every request returns 503.
	Credentials CredentialProvider
	// ObjectAuth enforces IAM authorization on object operations.
	// If nil and ProductionMode is false, IAM checks are skipped (dev only).
	// If nil and ProductionMode is true, every request returns 503.
	ObjectAuth ObjectAuthorizer
	// ProductionMode enables strict enforcement: nil Credentials or ObjectAuth
	// causes server startup to fail and unauthenticated requests to return 503.
	ProductionMode bool
	// AuditStore records an audit event after every S3 operation. Optional.
	AuditStore *audit.Store
	// TLSCertFile and TLSKeyFile enable HTTPS when both are set.
	TLSCertFile string
	TLSKeyFile  string
}

// Server is the S3-compatible HTTP server.
type Server struct {
	cfg    Config
	engine *gin.Engine
}

// New creates a configured Server ready to Start.
// In ProductionMode, Credentials and ObjectAuth must not be nil.
func New(cfg Config) *Server {
	if cfg.ListenAddr == "" {
		cfg.ListenAddr = "127.0.0.1:9000"
	}
	if cfg.ProductionMode {
		if cfg.Credentials == nil {
			panic("s3server: ProductionMode requires a non-nil Credentials provider")
		}
		if cfg.ObjectAuth == nil {
			panic("s3server: ProductionMode requires a non-nil ObjectAuth provider")
		}
	}
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())
	if cfg.AuditStore != nil {
		r.Use(S3AuditMiddleware(cfg.AuditStore))
	}

	objSvc := NewObjectService(cfg.StorageDir)
	h := newHandler(objSvc, cfg.Buckets)
	if cfg.ObjectAuth != nil {
		h.WithObjectAuth(cfg.ObjectAuth)
	}
	auth := SigV4Auth(cfg.Credentials, cfg.ProductionMode)

	// S3 API routes — ordering matters: /:bucket must come after GET /
	r.GET("/", auth, h.listBuckets)
	r.PUT("/:bucket", auth, h.createBucket)
	r.HEAD("/:bucket", auth, h.headBucket)
	r.DELETE("/:bucket", auth, h.deleteBucket)
	r.GET("/:bucket", auth, h.listObjects)
	r.PUT("/:bucket/*key", auth, h.putObject)
	r.GET("/:bucket/*key", auth, h.getObject)
	r.HEAD("/:bucket/*key", auth, h.headObject)
	r.DELETE("/:bucket/*key", auth, h.deleteObject)

	return &Server{cfg: cfg, engine: r}
}

// Handler returns the underlying http.Handler (useful for testing with httptest).
func (s *Server) Handler() http.Handler { return s.engine }

// Start begins serving S3 requests, blocking until ctx is cancelled.
// If TLSCertFile and TLSKeyFile are both set in Config, the server listens
// over HTTPS using those credentials.
func (s *Server) Start(ctx context.Context) error {
	srv := &http.Server{
		Addr:    s.cfg.ListenAddr,
		Handler: s.engine,
	}
	if s.cfg.TLSCertFile != "" && s.cfg.TLSKeyFile != "" {
		cert, err := tls.LoadX509KeyPair(s.cfg.TLSCertFile, s.cfg.TLSKeyFile)
		if err != nil {
			return fmt.Errorf("s3server: load TLS credentials: %w", err)
		}
		srv.TLSConfig = &tls.Config{
			Certificates: []tls.Certificate{cert},
			MinVersion:   tls.VersionTLS12,
		}
	}
	errCh := make(chan error, 1)
	go func() {
		var err error
		if srv.TLSConfig != nil {
			err = srv.ListenAndServeTLS("", "") // cert/key already in TLSConfig
		} else {
			err = srv.ListenAndServe()
		}
		if err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
		close(errCh)
	}()
	select {
	case <-ctx.Done():
		return srv.Shutdown(context.Background())
	case err := <-errCh:
		if err != nil {
			return fmt.Errorf("s3server: %w", err)
		}
		return nil
	}
}
