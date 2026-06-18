package s3server

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// ObjectMeta holds per-object metadata stored as JSON in an xl.meta file.
type ObjectMeta struct {
	Size         int64  `json:"size"`
	ETag         string `json:"etag"`         // sha256 hex of content
	LastModified int64  `json:"lastModified"` // unix timestamp
	ContentType  string `json:"contentType"`
}

// ObjectInfo is a summary returned by listing operations.
type ObjectInfo struct {
	Key          string
	Size         int64
	ETag         string
	LastModified time.Time
	ContentType  string
}

// ListResult is the result of a ListObjects call.
type ListResult struct {
	Bucket         string
	Prefix         string
	Delimiter      string
	IsTruncated    bool
	MaxKeys        int
	Objects        []ObjectInfo
	CommonPrefixes []string
}

// ObjectService manages objects on the local filesystem.
//
// Layout:
//
//	<storageDir>/
//	  <bucket>/
//	    <key>/
//	      xl.meta   JSON ObjectMeta
//	      data      object contents
type ObjectService struct {
	storageDir string
}

// NewObjectService creates an ObjectService rooted at storageDir.
func NewObjectService(storageDir string) *ObjectService {
	return &ObjectService{storageDir: storageDir}
}

func (s *ObjectService) bucketDir(bucket string) (string, error) {
	if err := validateStorageBucketName(bucket); err != nil {
		return "", err
	}
	root, err := filepath.Abs(s.storageDir)
	if err != nil {
		return "", err
	}
	dir := filepath.Join(root, bucket)
	if err := ensureContained(root, dir); err != nil {
		return "", err
	}
	return dir, nil
}

func (s *ObjectService) objectDir(bucket, key string) (string, error) {
	if err := validateObjectKey(key); err != nil {
		return "", err
	}
	bucketDir, err := s.bucketDir(bucket)
	if err != nil {
		return "", err
	}
	dir := filepath.Join(bucketDir, filepath.FromSlash(key))
	if err := ensureContained(bucketDir, dir); err != nil {
		return "", err
	}
	return dir, nil
}

// PutObject streams r into bucket/key, writes xl.meta, returns metadata.
func (s *ObjectService) PutObject(ctx context.Context, bucket, key string, r io.Reader, contentType string) (*ObjectMeta, error) {
	dir, err := s.objectDir(bucket, key)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("s3server: mkdir %s: %w", dir, err)
	}
	dataPath := filepath.Join(dir, "data")
	f, err := os.Create(dataPath)
	if err != nil {
		_ = os.RemoveAll(dir)
		return nil, fmt.Errorf("s3server: create data: %w", err)
	}
	h := sha256.New()
	mw := io.MultiWriter(f, h)
	written, copyErr := copyWithCtx(ctx, mw, r)
	_ = f.Close()
	if copyErr != nil {
		_ = os.RemoveAll(dir)
		return nil, fmt.Errorf("s3server: write data: %w", copyErr)
	}
	if contentType == "" {
		contentType = inferContentType(key)
	}
	meta := &ObjectMeta{
		Size:         written,
		ETag:         hex.EncodeToString(h.Sum(nil)),
		LastModified: time.Now().Unix(),
		ContentType:  contentType,
	}
	if err := writeMetaJSON(filepath.Join(dir, "xl.meta"), meta); err != nil {
		_ = os.RemoveAll(dir)
		return nil, err
	}
	return meta, nil
}

// PutObjectFromFile is a convenience wrapper that opens srcPath and calls PutObject.
func (s *ObjectService) PutObjectFromFile(bucket, key, srcPath string) (*ObjectMeta, error) {
	f, err := os.Open(srcPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return s.PutObject(context.Background(), bucket, key, f, "")
}

// GetObject returns the object metadata and a ReadCloser for the data.
// Caller must close the ReadCloser.
func (s *ObjectService) GetObject(bucket, key string) (*ObjectMeta, io.ReadCloser, error) {
	dir, err := s.objectDir(bucket, key)
	if err != nil {
		return nil, nil, err
	}
	meta, err := readMetaJSON(filepath.Join(dir, "xl.meta"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, ErrNoSuchKey
		}
		return nil, nil, err
	}
	f, err := os.Open(filepath.Join(dir, "data"))
	if err != nil {
		return nil, nil, err
	}
	return meta, f, nil
}

// GetObjectToFile copies the object to destPath and returns the metadata.
func (s *ObjectService) GetObjectToFile(bucket, key, destPath string) (*ObjectMeta, error) {
	meta, rc, err := s.GetObject(bucket, key)
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return nil, err
	}
	out, err := os.Create(destPath)
	if err != nil {
		return nil, err
	}
	defer out.Close()
	if _, err := io.Copy(out, rc); err != nil {
		return nil, err
	}
	return meta, nil
}

// HeadObject returns metadata without opening the data file.
func (s *ObjectService) HeadObject(bucket, key string) (*ObjectMeta, error) {
	dir, err := s.objectDir(bucket, key)
	if err != nil {
		return nil, err
	}
	meta, err := readMetaJSON(filepath.Join(dir, "xl.meta"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNoSuchKey
		}
		return nil, err
	}
	return meta, nil
}

// DeleteObject removes the object directory (xl.meta + data).
func (s *ObjectService) DeleteObject(bucket, key string) error {
	dir, err := s.objectDir(bucket, key)
	if err != nil {
		return err
	}
	if _, err := os.Stat(filepath.Join(dir, "xl.meta")); os.IsNotExist(err) {
		return ErrNoSuchKey
	}
	return os.RemoveAll(dir)
}

// ObjectExists reports whether the object has an xl.meta.
func (s *ObjectService) ObjectExists(bucket, key string) bool {
	dir, err := s.objectDir(bucket, key)
	if err != nil {
		return false
	}
	_, err = os.Stat(filepath.Join(dir, "xl.meta"))
	return err == nil
}

// BucketSizeBytes returns the total number of bytes stored across all objects
// in the bucket. Returns 0 on error or if the bucket is empty.
func (s *ObjectService) BucketSizeBytes(bucketName string) int64 {
	bucketDir, err := s.bucketDir(bucketName)
	if err != nil {
		return 0
	}
	var total int64
	_ = filepath.WalkDir(bucketDir, func(path string, d os.DirEntry, werr error) error {
		if werr != nil || d.IsDir() || d.Name() != "xl.meta" {
			return nil
		}
		meta, merr := readMetaJSON(path)
		if merr == nil {
			total += meta.Size
		}
		return nil
	})
	return total
}

// BucketHasObjects reports whether the bucket directory contains any entries.
func (s *ObjectService) BucketHasObjects(bucketName string) bool {
	dir, err := s.bucketDir(bucketName)
	if err != nil {
		return false
	}
	entries, err := os.ReadDir(dir)
	return err == nil && len(entries) > 0
}

// ListObjects returns objects in bucket matching prefix/delimiter up to maxKeys.
func (s *ObjectService) ListObjects(bucket, prefix, delimiter string, maxKeys int) (*ListResult, error) {
	if maxKeys <= 0 {
		maxKeys = 1000
	}
	bucketDir, err := s.bucketDir(bucket)
	if err != nil {
		return nil, err
	}
	result := &ListResult{
		Bucket:         bucket,
		Prefix:         prefix,
		Delimiter:      delimiter,
		MaxKeys:        maxKeys,
		Objects:        make([]ObjectInfo, 0),
		CommonPrefixes: make([]string, 0),
	}
	seenPrefixes := make(map[string]bool)
	err = filepath.WalkDir(bucketDir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || d.Name() != "xl.meta" {
			return nil
		}
		objDir, relErr := filepath.Rel(bucketDir, filepath.Dir(path))
		if relErr != nil {
			return nil
		}
		key := filepath.ToSlash(objDir)
		if prefix != "" && !strings.HasPrefix(key, prefix) {
			return nil
		}
		if delimiter != "" {
			after := key
			if prefix != "" {
				after = key[len(prefix):]
			}
			if idx := strings.Index(after, delimiter); idx >= 0 {
				cp := prefix + after[:idx+len(delimiter)]
				if !seenPrefixes[cp] {
					seenPrefixes[cp] = true
					result.CommonPrefixes = append(result.CommonPrefixes, cp)
				}
				return nil
			}
		}
		if len(result.Objects) >= maxKeys {
			result.IsTruncated = true
			return filepath.SkipAll
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil
		}
		var meta ObjectMeta
		if jsonErr := json.Unmarshal(data, &meta); jsonErr != nil {
			return nil
		}
		result.Objects = append(result.Objects, ObjectInfo{
			Key:          key,
			Size:         meta.Size,
			ETag:         meta.ETag,
			LastModified: time.Unix(meta.LastModified, 0),
			ContentType:  meta.ContentType,
		})
		return nil
	})
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	sort.Slice(result.Objects, func(i, j int) bool {
		return result.Objects[i].Key < result.Objects[j].Key
	})
	sort.Strings(result.CommonPrefixes)
	return result, nil
}

// ---- filesystem helpers -----------------------------------------------------

func writeMetaJSON(path string, meta *ObjectMeta) error {
	data, err := json.Marshal(meta)
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func validateStorageBucketName(bucket string) error {
	if bucket == "" {
		return fmt.Errorf("s3server: bucket name is required")
	}
	if filepath.IsAbs(bucket) || strings.Contains(bucket, "\\") {
		return fmt.Errorf("s3server: unsafe bucket name %q", bucket)
	}
	for _, part := range strings.Split(bucket, "/") {
		if part == "" || part == "." || part == ".." {
			return fmt.Errorf("s3server: unsafe bucket name %q", bucket)
		}
	}
	return nil
}

func validateObjectKey(key string) error {
	if key == "" {
		return fmt.Errorf("s3server: object key is required")
	}
	cleanKey := filepath.FromSlash(key)
	if filepath.IsAbs(cleanKey) || strings.Contains(cleanKey, "\\") {
		return fmt.Errorf("s3server: unsafe object key %q", key)
	}
	for _, part := range strings.Split(cleanKey, string(os.PathSeparator)) {
		if part == "" || part == "." || part == ".." {
			return fmt.Errorf("s3server: unsafe object key %q", key)
		}
	}
	return nil
}

func ensureContained(root, path string) error {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return err
	}
	if rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return fmt.Errorf("s3server: path escapes storage root")
	}
	return nil
}

func readMetaJSON(path string) (*ObjectMeta, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var meta ObjectMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, err
	}
	return &meta, nil
}

func inferContentType(key string) string {
	name := strings.ToLower(filepath.Base(key))
	switch {
	case strings.HasSuffix(name, ".json"):
		return "application/json"
	case strings.HasSuffix(name, ".txt"):
		return "text/plain"
	case strings.HasSuffix(name, ".html"), strings.HasSuffix(name, ".htm"):
		return "text/html"
	case strings.HasSuffix(name, ".xml"):
		return "application/xml"
	case strings.HasSuffix(name, ".png"):
		return "image/png"
	case strings.HasSuffix(name, ".jpg"), strings.HasSuffix(name, ".jpeg"):
		return "image/jpeg"
	case strings.HasSuffix(name, ".tar.gz"), strings.HasSuffix(name, ".tgz"):
		return "application/gzip"
	case strings.HasSuffix(name, ".tar.zst"):
		return "application/zstd"
	case strings.HasSuffix(name, ".tar"):
		return "application/x-tar"
	case strings.HasSuffix(name, ".zip"):
		return "application/zip"
	case strings.HasSuffix(name, ".cap"):
		return "application/x-capper"
	default:
		return "application/octet-stream"
	}
}

func copyWithCtx(ctx context.Context, dst io.Writer, src io.Reader) (int64, error) {
	buf := make([]byte, 32*1024)
	var written int64
	for {
		select {
		case <-ctx.Done():
			return written, ctx.Err()
		default:
		}
		nr, er := src.Read(buf)
		if nr > 0 {
			nw, ew := dst.Write(buf[:nr])
			written += int64(nw)
			if ew != nil {
				return written, ew
			}
		}
		if er != nil {
			if er == io.EOF {
				return written, nil
			}
			return written, er
		}
	}
}
