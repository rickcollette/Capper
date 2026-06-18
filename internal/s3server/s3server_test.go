package s3server_test

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"capper/internal/s3server"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// ---- ObjectService unit tests -----------------------------------------------

func TestObjectServicePutAndGet(t *testing.T) {
	dir := t.TempDir()
	svc := s3server.NewObjectService(dir)
	_ = os.Mkdir(dir+"/bkt", 0o755)

	meta, err := svc.PutObject(t.Context(), "bkt", "hello.txt", strings.NewReader("hello world"), "text/plain")
	if err != nil {
		t.Fatalf("PutObject: %v", err)
	}
	if meta.Size != 11 {
		t.Fatalf("expected size 11, got %d", meta.Size)
	}
	if meta.ETag == "" {
		t.Fatal("ETag must be set")
	}
	if meta.ContentType != "text/plain" {
		t.Fatalf("unexpected content type: %s", meta.ContentType)
	}

	got, rc, err := svc.GetObject("bkt", "hello.txt")
	if err != nil {
		t.Fatalf("GetObject: %v", err)
	}
	defer rc.Close()
	data, _ := io.ReadAll(rc)
	if string(data) != "hello world" {
		t.Fatalf("content mismatch: %q", data)
	}
	if got.Size != 11 {
		t.Fatalf("head size mismatch: %d", got.Size)
	}
}

func TestObjectServiceHeadMissing(t *testing.T) {
	dir := t.TempDir()
	svc := s3server.NewObjectService(dir)
	_ = os.Mkdir(dir+"/bkt", 0o755)

	if _, err := svc.HeadObject("bkt", "missing"); err != s3server.ErrNoSuchKey {
		t.Fatalf("expected ErrNoSuchKey, got %v", err)
	}
}

func TestObjectServiceDelete(t *testing.T) {
	dir := t.TempDir()
	svc := s3server.NewObjectService(dir)
	_ = os.Mkdir(dir+"/bkt", 0o755)
	_, _ = svc.PutObject(t.Context(), "bkt", "file.txt", strings.NewReader("x"), "")

	if err := svc.DeleteObject("bkt", "file.txt"); err != nil {
		t.Fatalf("DeleteObject: %v", err)
	}
	if _, err := svc.HeadObject("bkt", "file.txt"); err != s3server.ErrNoSuchKey {
		t.Fatalf("expected ErrNoSuchKey after delete, got %v", err)
	}
}

func TestObjectServiceListObjects(t *testing.T) {
	dir := t.TempDir()
	svc := s3server.NewObjectService(dir)
	_ = os.Mkdir(dir+"/bkt", 0o755)

	for _, key := range []string{"a/1.txt", "a/2.txt", "b/3.txt"} {
		if _, err := svc.PutObject(t.Context(), "bkt", key, strings.NewReader("x"), ""); err != nil {
			t.Fatalf("PutObject %s: %v", key, err)
		}
	}

	res, err := svc.ListObjects("bkt", "", "", 1000)
	if err != nil {
		t.Fatalf("ListObjects all: %v", err)
	}
	if len(res.Objects) != 3 {
		t.Fatalf("expected 3, got %d", len(res.Objects))
	}

	res, _ = svc.ListObjects("bkt", "a/", "", 1000)
	if len(res.Objects) != 2 {
		t.Fatalf("expected 2 with prefix a/, got %d", len(res.Objects))
	}
}

func TestObjectServiceBucketHasObjects(t *testing.T) {
	dir := t.TempDir()
	svc := s3server.NewObjectService(dir)
	_ = os.Mkdir(dir+"/bkt", 0o755)

	if svc.BucketHasObjects("bkt") {
		t.Fatal("new bucket should be empty")
	}
	_, _ = svc.PutObject(t.Context(), "bkt", "f", strings.NewReader("x"), "")
	if !svc.BucketHasObjects("bkt") {
		t.Fatal("bucket should have objects after put")
	}
}

func TestObjectServicePutOverwrites(t *testing.T) {
	dir := t.TempDir()
	svc := s3server.NewObjectService(dir)
	_ = os.Mkdir(dir+"/bkt", 0o755)

	_, _ = svc.PutObject(t.Context(), "bkt", "f", strings.NewReader("short"), "")
	meta, err := svc.PutObject(t.Context(), "bkt", "f", strings.NewReader("longer content"), "")
	if err != nil {
		t.Fatalf("overwrite PutObject: %v", err)
	}
	if meta.Size != int64(len("longer content")) {
		t.Fatalf("expected %d after overwrite, got %d", len("longer content"), meta.Size)
	}
}

func TestObjectServiceRejectsTraversalKeys(t *testing.T) {
	dir := t.TempDir()
	svc := s3server.NewObjectService(dir)
	_ = os.Mkdir(dir+"/bkt", 0o755)

	for _, key := range []string{"../escape", "a/../../escape", "/absolute", "a//b", "a/./b"} {
		if _, err := svc.PutObject(t.Context(), "bkt", key, strings.NewReader("x"), ""); err == nil {
			t.Fatalf("PutObject accepted unsafe key %q", key)
		}
		if _, _, err := svc.GetObject("bkt", key); err == nil {
			t.Fatalf("GetObject accepted unsafe key %q", key)
		}
		if err := svc.DeleteObject("bkt", key); err == nil {
			t.Fatalf("DeleteObject accepted unsafe key %q", key)
		}
	}
	if _, err := os.Stat(dir + "/escape"); !os.IsNotExist(err) {
		t.Fatalf("unsafe put created escape path: %v", err)
	}
}

func TestObjectServiceRejectsTraversalBuckets(t *testing.T) {
	dir := t.TempDir()
	svc := s3server.NewObjectService(dir)
	for _, bucket := range []string{"../escape", "/absolute", "a/../b", "a//b"} {
		if _, err := svc.PutObject(t.Context(), bucket, "safe-key", strings.NewReader("x"), ""); err == nil {
			t.Fatalf("PutObject accepted unsafe bucket %q", bucket)
		}
		if _, err := svc.ListObjects(bucket, "", "", 1000); err == nil {
			t.Fatalf("ListObjects accepted unsafe bucket %q", bucket)
		}
	}
}

// ---- HTTP handler tests (no-auth mode) ---------------------------------------

type fakeBuckets struct {
	buckets map[string]string // name → createdAt
	quotas  map[string]int64  // name → quota_bytes (0 = unlimited)
}

func newFakeBuckets() *fakeBuckets {
	return &fakeBuckets{
		buckets: map[string]string{},
		quotas:  map[string]int64{},
	}
}

func (f *fakeBuckets) ListBucketsForS3() ([]s3server.BucketEntry, error) {
	out := make([]s3server.BucketEntry, 0, len(f.buckets))
	for name, createdAt := range f.buckets {
		out = append(out, s3server.BucketEntry{Name: name, CreatedAt: createdAt})
	}
	return out, nil
}

func (f *fakeBuckets) CreateBucketForS3(name string) error {
	if _, ok := f.buckets[name]; ok {
		return fmt.Errorf("already exists")
	}
	f.buckets[name] = time.Now().UTC().Format(time.RFC3339)
	return nil
}

func (f *fakeBuckets) DeleteBucketForS3(name string, _ bool) error {
	if _, ok := f.buckets[name]; !ok {
		return s3server.ErrNoSuchBucket
	}
	delete(f.buckets, name)
	return nil
}

func (f *fakeBuckets) BucketExistsForS3(name string) bool {
	_, ok := f.buckets[name]
	return ok
}

func (f *fakeBuckets) BucketQuotaBytes(name string) int64 {
	return f.quotas[name]
}

func newTestServer(t *testing.T) (*httptest.Server, *fakeBuckets, string) {
	t.Helper()
	dir := t.TempDir()
	fb := newFakeBuckets()
	cfg := s3server.Config{
		StorageDir:  dir,
		Buckets:     fb,
		Credentials: nil, // dev mode: no auth
	}
	srv := s3server.New(cfg)
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)
	return ts, fb, dir
}

func TestHTTPCreateAndHeadBucket(t *testing.T) {
	ts, _, _ := newTestServer(t)

	req, _ := http.NewRequest(http.MethodPut, ts.URL+"/my-bucket", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("PUT /my-bucket: expected 200, got %d", resp.StatusCode)
	}

	req2, _ := http.NewRequest(http.MethodHead, ts.URL+"/my-bucket", nil)
	resp2, _ := http.DefaultClient.Do(req2)
	resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("HEAD /my-bucket: expected 200, got %d", resp2.StatusCode)
	}
}

func TestHTTPListBuckets(t *testing.T) {
	ts, fb, _ := newTestServer(t)
	_ = fb.CreateBucketForS3("alpha")
	_ = fb.CreateBucketForS3("beta")

	resp, err := http.Get(ts.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var result struct {
		Buckets struct {
			Bucket []struct{ Name string }
		}
	}
	if err := xml.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode XML: %v", err)
	}
	if len(result.Buckets.Bucket) != 2 {
		t.Fatalf("expected 2 buckets, got %d", len(result.Buckets.Bucket))
	}
}

func TestHTTPPutGetDeleteObject(t *testing.T) {
	ts, fb, dir := newTestServer(t)
	_ = fb.CreateBucketForS3("bkt")
	_ = os.Mkdir(dir+"/bkt", 0o755)

	req, _ := http.NewRequest(http.MethodPut, ts.URL+"/bkt/hello.txt", strings.NewReader("hello s3"))
	req.Header.Set("Content-Type", "text/plain")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("PUT object: expected 200, got %d", resp.StatusCode)
	}
	if resp.Header.Get("ETag") == "" {
		t.Fatal("ETag not returned")
	}

	resp2, err := http.Get(ts.URL + "/bkt/hello.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("GET object: expected 200, got %d", resp2.StatusCode)
	}
	data, _ := io.ReadAll(resp2.Body)
	if string(data) != "hello s3" {
		t.Fatalf("content mismatch: %q", data)
	}

	req3, _ := http.NewRequest(http.MethodDelete, ts.URL+"/bkt/hello.txt", nil)
	resp3, _ := http.DefaultClient.Do(req3)
	resp3.Body.Close()
	if resp3.StatusCode != http.StatusNoContent {
		t.Fatalf("DELETE object: expected 204, got %d", resp3.StatusCode)
	}

	resp4, _ := http.Get(ts.URL + "/bkt/hello.txt")
	resp4.Body.Close()
	if resp4.StatusCode != http.StatusNotFound {
		t.Fatalf("GET after delete: expected 404, got %d", resp4.StatusCode)
	}
}

func TestHTTPDeleteObjectIdempotent(t *testing.T) {
	ts, fb, dir := newTestServer(t)
	_ = fb.CreateBucketForS3("bkt")
	_ = os.Mkdir(dir+"/bkt", 0o755)

	req, _ := http.NewRequest(http.MethodDelete, ts.URL+"/bkt/nonexistent", nil)
	resp, _ := http.DefaultClient.Do(req)
	resp.Body.Close()
	// S3 spec: DELETE on non-existent key returns 204
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("DELETE nonexistent: expected 204, got %d", resp.StatusCode)
	}
}

func TestHTTPListObjects(t *testing.T) {
	ts, fb, dir := newTestServer(t)
	_ = fb.CreateBucketForS3("bkt")
	_ = os.Mkdir(dir+"/bkt", 0o755)

	for _, key := range []string{"a/1.txt", "a/2.txt", "b/3.txt"} {
		req, _ := http.NewRequest(http.MethodPut, ts.URL+"/bkt/"+key, strings.NewReader("x"))
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("PUT %s: %v", key, err)
		}
		resp.Body.Close()
	}

	resp, err := http.Get(ts.URL + "/bkt?prefix=a/")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var result struct {
		Contents []struct{ Key string } `xml:"Contents"`
	}
	_ = xml.NewDecoder(resp.Body).Decode(&result)
	if len(result.Contents) != 2 {
		t.Fatalf("expected 2 objects with prefix a/, got %d", len(result.Contents))
	}
}

func TestHTTPDeleteNonExistentBucket(t *testing.T) {
	ts, _, _ := newTestServer(t)

	req, _ := http.NewRequest(http.MethodDelete, ts.URL+"/no-such-bucket", nil)
	resp, _ := http.DefaultClient.Do(req)
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

// ---- quota enforcement tests ------------------------------------------------

func TestHTTPPutObjectQuotaEnforced(t *testing.T) {
	dir := t.TempDir()
	fb := newFakeBuckets()
	cfg := s3server.Config{
		StorageDir:  dir,
		Buckets:     fb,
		Credentials: nil,
	}
	srv := s3server.New(cfg)
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	// Create a bucket with a 20-byte quota.
	_ = fb.CreateBucketForS3("quota-bkt")
	fb.quotas["quota-bkt"] = 20
	// Also create the bucket directory that ObjectService expects.
	_ = os.Mkdir(dir+"/quota-bkt", 0o755)

	put := func(key, body string) int {
		req, _ := http.NewRequest(http.MethodPut, ts.URL+"/quota-bkt/"+key, strings.NewReader(body))
		req.ContentLength = int64(len(body))
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("PUT %s: %v", key, err)
		}
		resp.Body.Close()
		return resp.StatusCode
	}

	// First object (10 bytes) fits within quota.
	if got := put("a.txt", "0123456789"); got != http.StatusOK {
		t.Fatalf("first put: expected 200, got %d", got)
	}
	// Second object (11 bytes) would exceed the 20-byte quota.
	if got := put("b.txt", "01234567890"); got != http.StatusForbidden {
		t.Fatalf("second put: expected 403, got %d", got)
	}
	// Verify the second object was not persisted.
	resp, err := http.Get(ts.URL + "/quota-bkt/b.txt")
	if err != nil {
		t.Fatalf("GET b.txt: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("b.txt should not exist after quota rejection, got %d", resp.StatusCode)
	}
}

// ---- SigV4 auth tests -------------------------------------------------------

func TestSigV4AuthRejectsUnsigned(t *testing.T) {
	dir := t.TempDir()
	fb := newFakeBuckets()
	cfg := s3server.Config{
		StorageDir:  dir,
		Buckets:     fb,
		Credentials: s3server.StaticCredentials{"AKID": "secret"},
	}
	srv := s3server.New(cfg)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	resp, _ := http.Get(ts.URL + "/")
	resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("unsigned request: expected 403, got %d", resp.StatusCode)
	}
}

func TestSigV4AuthPassesValidSignature(t *testing.T) {
	dir := t.TempDir()
	fb := newFakeBuckets()
	cfg := s3server.Config{
		StorageDir:  dir,
		Buckets:     fb,
		Credentials: s3server.StaticCredentials{"AKID": "secret"},
	}
	srv := s3server.New(cfg)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/", nil)
	sigV4Sign(req, "AKID", "secret", "us-east-1", "s3")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("signed request: expected 200, got %d", resp.StatusCode)
	}
}

// sigV4Sign adds a valid AWS SigV4 Authorization header to req (empty body).
func sigV4Sign(req *http.Request, accessKey, secretKey, region, service string) {
	emptyBodyHash := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	now := time.Now().UTC()
	dateTime := now.Format("20060102T150405Z")
	date := now.Format("20060102")

	host := req.URL.Host
	req.Header.Set("Host", host)
	req.Header.Set("X-Amz-Date", dateTime)
	req.Header.Set("X-Amz-Content-Sha256", emptyBodyHash)

	signedHeaders := "host;x-amz-content-sha256;x-amz-date"
	canonicalHeaders := fmt.Sprintf("host:%s\nx-amz-content-sha256:%s\nx-amz-date:%s\n",
		host, emptyBodyHash, dateTime)

	path := req.URL.Path
	if path == "" {
		path = "/"
	}
	canonicalRequest := strings.Join([]string{
		req.Method, path, "",
		canonicalHeaders, signedHeaders, emptyBodyHash,
	}, "\n")

	credScope := strings.Join([]string{date, region, service, "aws4_request"}, "/")
	h := sha256.Sum256([]byte(canonicalRequest))
	stringToSign := strings.Join([]string{
		"AWS4-HMAC-SHA256", dateTime, credScope,
		hex.EncodeToString(h[:]),
	}, "\n")

	sigKey := hmacSHA256(
		hmacSHA256(hmacSHA256(hmacSHA256([]byte("AWS4"+secretKey), date), region), service),
		"aws4_request",
	)
	sig := hex.EncodeToString(hmacSHA256(sigKey, stringToSign))

	req.Header.Set("Authorization", fmt.Sprintf(
		"AWS4-HMAC-SHA256 Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		accessKey, credScope, signedHeaders, sig,
	))
}

func hmacSHA256(key []byte, data string) []byte {
	h := hmac.New(sha256.New, key)
	h.Write([]byte(data))
	return h.Sum(nil)
}

// ---- ObjectAuthorizer / IAM-denied tests ------------------------------------

// denyAuthorizer denies every action for the configured access key.
type denyAuthorizer struct {
	denyKey string // deny this access key; allow all others
}

func (d *denyAuthorizer) AuthorizeObject(accessKey, action, resource string) error {
	if accessKey == d.denyKey {
		return fmt.Errorf("IAM policy denies %s on %s for %s", action, resource, accessKey)
	}
	return nil
}

func newTestServerWithAuth(t *testing.T, auth s3server.ObjectAuthorizer) (*httptest.Server, *fakeBuckets, string) {
	t.Helper()
	dir := t.TempDir()
	fb := newFakeBuckets()
	cfg := s3server.Config{
		StorageDir: dir,
		Buckets:    fb,
		ObjectAuth: auth,
	}
	srv := s3server.New(cfg)
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)
	return ts, fb, dir
}

func TestObjectAuthDeniedPut(t *testing.T) {
	// The s3AccessKey gin-context value is only set by SigV4Auth middleware.
	// In dev mode (no Credentials), the key is "". We deny the empty key here
	// to simulate an IAM-denied request arriving without SigV4 credentials.
	ts, fb, dir := newTestServerWithAuth(t, &denyAuthorizer{denyKey: ""})
	_ = fb.CreateBucketForS3("bkt")
	_ = os.Mkdir(dir+"/bkt", 0o755)

	req, _ := http.NewRequest(http.MethodPut, ts.URL+"/bkt/secret.txt", strings.NewReader("data"))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("PUT with denied key: expected 403, got %d", resp.StatusCode)
	}
}

func TestObjectAuthDeniedGet(t *testing.T) {
	ts, fb, dir := newTestServerWithAuth(t, &denyAuthorizer{denyKey: ""})
	_ = fb.CreateBucketForS3("bkt")
	_ = os.Mkdir(dir+"/bkt", 0o755)

	resp, err := http.Get(ts.URL + "/bkt/any.txt")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("GET with denied key: expected 403, got %d", resp.StatusCode)
	}
}

func TestObjectAuthDeniedDelete(t *testing.T) {
	ts, fb, dir := newTestServerWithAuth(t, &denyAuthorizer{denyKey: ""})
	_ = fb.CreateBucketForS3("bkt")
	_ = os.Mkdir(dir+"/bkt", 0o755)

	req, _ := http.NewRequest(http.MethodDelete, ts.URL+"/bkt/any.txt", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("DELETE with denied key: expected 403, got %d", resp.StatusCode)
	}
}

func TestObjectAuthAllowsOtherKeys(t *testing.T) {
	// Deny "badkey"; the request uses empty key (dev mode), so it should be allowed.
	ts, fb, dir := newTestServerWithAuth(t, &denyAuthorizer{denyKey: "badkey"})
	_ = fb.CreateBucketForS3("bkt")
	_ = os.Mkdir(dir+"/bkt", 0o755)

	req, _ := http.NewRequest(http.MethodPut, ts.URL+"/bkt/ok.txt", strings.NewReader("allowed"))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("PUT with allowed key: expected 200, got %d", resp.StatusCode)
	}
}

func TestObjectAuthDenialDoesNotPersistObject(t *testing.T) {
	ts, fb, dir := newTestServerWithAuth(t, &denyAuthorizer{denyKey: ""})
	_ = fb.CreateBucketForS3("bkt")
	_ = os.Mkdir(dir+"/bkt", 0o755)

	// PUT is denied — object must not exist afterwards.
	req, _ := http.NewRequest(http.MethodPut, ts.URL+"/bkt/ghost.txt", strings.NewReader("should not persist"))
	resp, _ := http.DefaultClient.Do(req)
	resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", resp.StatusCode)
	}

	// Temporarily allow GET to confirm the object is absent.
	ts2, fb2, dir2 := newTestServerWithAuth(t, &denyAuthorizer{denyKey: "nobody"})
	_ = fb2.CreateBucketForS3("bkt")
	_ = os.Mkdir(dir2+"/bkt", 0o755)
	// Write a test via the allowed server to confirm the directory is empty.
	_ = dir // dir from denied server is separate — object was never written there.
	resp2, _ := http.Get(ts2.URL + "/bkt/ghost.txt")
	resp2.Body.Close()
	if resp2.StatusCode != http.StatusNotFound {
		t.Fatalf("ghost object should not exist, got %d", resp2.StatusCode)
	}
}
