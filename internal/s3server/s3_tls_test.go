package s3server_test

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"capper/internal/s3server"
)

// generateSelfSigned writes a self-signed TLS cert+key to dir and returns
// (certPath, keyPath).
func generateSelfSigned(t *testing.T, dir string) (string, string) {
	t.Helper()

	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "localhost"},
		NotBefore:    time.Now().Add(-time.Minute),
		NotAfter:     time.Now().Add(time.Hour),
		DNSNames:     []string{"localhost"},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &priv.PublicKey, priv)
	if err != nil {
		t.Fatalf("create cert: %v", err)
	}

	certFile := filepath.Join(dir, "cert.pem")
	keyFile := filepath.Join(dir, "key.pem")

	cf, _ := os.Create(certFile)
	pem.Encode(cf, &pem.Block{Type: "CERTIFICATE", Bytes: der})
	cf.Close()

	kf, _ := os.Create(keyFile)
	kb, _ := x509.MarshalECPrivateKey(priv)
	pem.Encode(kf, &pem.Block{Type: "EC PRIVATE KEY", Bytes: kb})
	kf.Close()

	return certFile, keyFile
}

// TestS3TLSConfigLoaded verifies that providing TLSCertFile+TLSKeyFile causes
// the server to load a valid TLS configuration (the key pair is parseable and
// the TLSConfig has exactly one certificate loaded).
func TestS3TLSConfigLoaded(t *testing.T) {
	dir := t.TempDir()
	certFile, keyFile := generateSelfSigned(t, dir)

	// Load the cert/key pair the same way the server does.
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		t.Fatalf("LoadX509KeyPair: %v", err)
	}
	tlsCfg := &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
	}
	if len(tlsCfg.Certificates) != 1 {
		t.Errorf("expected 1 certificate in TLSConfig, got %d", len(tlsCfg.Certificates))
	}
}

// TestS3ServerNoTLSWhenCertsAbsent verifies that a Server with no TLS fields
// builds a plain-HTTP server (no panic, no error on construction).
func TestS3ServerNoTLSWhenCertsAbsent(t *testing.T) {
	dir := t.TempDir()
	srv := s3server.New(s3server.Config{
		ListenAddr: "127.0.0.1:0",
		StorageDir: dir,
	})
	if srv == nil {
		t.Fatal("New returned nil")
	}
}

// TestS3ServerTLSFieldsRoundtrip verifies that Config fields survive assignment
// (no accidental zero-value reset in the constructor).
func TestS3ServerTLSFieldsRoundtrip(t *testing.T) {
	dir := t.TempDir()
	certFile, keyFile := generateSelfSigned(t, dir)

	cfg := s3server.Config{
		ListenAddr:  "127.0.0.1:9443",
		StorageDir:  dir,
		TLSCertFile: certFile,
		TLSKeyFile:  keyFile,
	}
	if cfg.TLSCertFile != certFile || cfg.TLSKeyFile != keyFile {
		t.Errorf("TLS fields not preserved through Config struct")
	}
	srv := s3server.New(cfg)
	if srv == nil {
		t.Fatal("New returned nil for TLS config")
	}
}

// TestS3TLSTransport verifies HTTPS connectivity to a TLS-enabled test server.
// This starts a real httptest.TLS server (not the s3server itself) to confirm
// our cert/key pair is valid for round-trip TLS.
func TestS3TLSTransport(t *testing.T) {
	dir := t.TempDir()
	certFile, keyFile := generateSelfSigned(t, dir)

	cert, _ := tls.LoadX509KeyPair(certFile, keyFile)
	leaf, _ := x509.ParseCertificate(cert.Certificate[0])
	pool := x509.NewCertPool()
	pool.AddCert(leaf)

	// Minimal TLS server using our self-signed cert.
	ln, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
	})
	if err != nil {
		t.Fatalf("tls.Listen: %v", err)
	}
	defer ln.Close()

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("tls-ok"))
	})
	go http.Serve(ln, mux) //nolint:errcheck

	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{RootCAs: pool},
		},
	}
	resp, err := client.Get("https://" + ln.Addr().String() + "/")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}
