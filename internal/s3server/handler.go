package s3server

import (
	"encoding/xml"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// BucketProvider is the interface the S3 server uses for bucket lifecycle.
// internal/storage.Manager implements this interface.
type BucketProvider interface {
	ListBucketsForS3() ([]BucketEntry, error)
	CreateBucketForS3(name string) error
	DeleteBucketForS3(name string, force bool) error
	BucketExistsForS3(name string) bool
	// BucketQuotaBytes returns the configured quota for the bucket in bytes.
	// A return value of 0 means no quota is enforced.
	BucketQuotaBytes(name string) int64
}

// ObjectAuthorizer enforces IAM access control on object operations.
// Implement this interface and pass it via Config.ObjectAuth to enable
// per-request authorization. If nil, authorization is skipped (dev mode).
//
// action is one of: "s3:PutObject", "s3:GetObject", "s3:DeleteObject".
// resource is the full bucket/key path, e.g., "my-bucket/some/key.txt".
// The principal is derived from the access key set by SigV4Auth.
type ObjectAuthorizer interface {
	AuthorizeObject(accessKey, action, resource string) error
}

// BucketEntry is a minimal bucket record for S3 API responses.
type BucketEntry struct {
	Name      string
	CreatedAt string
}

var (
	bucketNameRe = regexp.MustCompile(`^[a-z0-9][a-z0-9\-]{1,61}[a-z0-9]$`)
	ipAddrRe     = regexp.MustCompile(`^\d+\.\d+\.\d+\.\d+$`)
)

func validateBucketName(name string) error {
	if len(name) < 3 || len(name) > 63 {
		return fmt.Errorf("bucket name must be 3-63 characters")
	}
	if ipAddrRe.MatchString(name) {
		return fmt.Errorf("bucket name must not be an IP address")
	}
	if !bucketNameRe.MatchString(name) {
		return fmt.Errorf("bucket name must be lowercase alphanumeric or hyphens")
	}
	if strings.Contains(name, "..") || strings.Contains(name, ".-") || strings.Contains(name, "-.") {
		return fmt.Errorf("bucket name contains invalid character sequence")
	}
	return nil
}

// Handler holds the services for S3 request handling.
type Handler struct {
	objSvc  *ObjectService
	buckets BucketProvider
	auth    ObjectAuthorizer
}

func newHandler(objSvc *ObjectService, buckets BucketProvider) *Handler {
	return &Handler{objSvc: objSvc, buckets: buckets}
}

// WithObjectAuth attaches an authorizer to the handler (optional).
func (h *Handler) WithObjectAuth(a ObjectAuthorizer) { h.auth = a }

// authorizeObject checks IAM for object operations. Returns false and writes
// a 403 response if authorization fails; returns true to continue.
func (h *Handler) authorizeObject(c *gin.Context, action, bucket, key string) bool {
	if h.auth == nil {
		return true
	}
	accessKey, _ := c.Get("s3AccessKey")
	ak, _ := accessKey.(string)
	resource := bucket + "/" + key
	if err := h.auth.AuthorizeObject(ak, action, resource); err != nil {
		writeS3Error(c, http.StatusForbidden, ErrCodeAccessDenied,
			"IAM authorization denied: "+err.Error(), "/"+resource)
		c.Abort()
		return false
	}
	return true
}

// ---- bucket handlers --------------------------------------------------------

// listBuckets serves GET /
func (h *Handler) listBuckets(c *gin.Context) {
	bkts, err := h.buckets.ListBucketsForS3()
	if err != nil {
		writeS3Error(c, http.StatusInternalServerError, ErrCodeInternalError, err.Error(), "")
		return
	}
	s3bkts := make([]S3Bucket, 0, len(bkts))
	for _, b := range bkts {
		s3bkts = append(s3bkts, S3Bucket{Name: b.Name, CreationDate: b.CreatedAt})
	}
	writeXML(c, http.StatusOK, ListAllMyBucketsResult{
		Owner:   S3Owner{ID: "capper", DisplayName: "capper"},
		Buckets: S3Buckets{Bucket: s3bkts},
	})
}

// createBucket serves PUT /:bucket
func (h *Handler) createBucket(c *gin.Context) {
	name := c.Param("bucket")
	if err := validateBucketName(name); err != nil {
		writeS3Error(c, http.StatusBadRequest, ErrCodeInvalidBucketName, err.Error(), "/"+name)
		return
	}
	if h.buckets.BucketExistsForS3(name) {
		writeS3Error(c, http.StatusConflict, ErrCodeBucketAlreadyExists, "Bucket already exists", "/"+name)
		return
	}
	if err := h.buckets.CreateBucketForS3(name); err != nil {
		writeS3Error(c, http.StatusInternalServerError, ErrCodeInternalError, err.Error(), "/"+name)
		return
	}
	c.Header("Location", "/"+name)
	c.Status(http.StatusOK)
}

// headBucket serves HEAD /:bucket
func (h *Handler) headBucket(c *gin.Context) {
	name := c.Param("bucket")
	if !h.buckets.BucketExistsForS3(name) {
		c.Status(http.StatusNotFound)
		return
	}
	c.Status(http.StatusOK)
}

// deleteBucket serves DELETE /:bucket
func (h *Handler) deleteBucket(c *gin.Context) {
	name := c.Param("bucket")
	if !h.buckets.BucketExistsForS3(name) {
		writeS3Error(c, http.StatusNotFound, ErrCodeNoSuchBucket, "The specified bucket does not exist.", "/"+name)
		return
	}
	if err := h.buckets.DeleteBucketForS3(name, false); err != nil {
		if s3e, ok := err.(*S3Err); ok && s3e.Code == ErrCodeBucketNotEmpty {
			writeS3Error(c, http.StatusConflict, s3e.Code, s3e.Message, "/"+name)
		} else if s3e != nil {
			writeS3Error(c, http.StatusInternalServerError, s3e.Code, s3e.Message, "/"+name)
		} else {
			writeS3Error(c, http.StatusInternalServerError, ErrCodeInternalError, err.Error(), "/"+name)
		}
		return
	}
	c.Status(http.StatusNoContent)
}

// listObjects serves GET /:bucket (list objects in bucket)
func (h *Handler) listObjects(c *gin.Context) {
	bucket := c.Param("bucket")
	if !h.buckets.BucketExistsForS3(bucket) {
		writeS3Error(c, http.StatusNotFound, ErrCodeNoSuchBucket, "The specified bucket does not exist.", "/"+bucket)
		return
	}
	prefix := c.Query("prefix")
	delimiter := c.Query("delimiter")
	res, err := h.objSvc.ListObjects(bucket, prefix, delimiter, 1000)
	if err != nil {
		writeS3Error(c, http.StatusInternalServerError, ErrCodeInternalError, err.Error(), "/"+bucket)
		return
	}
	contents := make([]S3ObjectContent, 0, len(res.Objects))
	for _, obj := range res.Objects {
		contents = append(contents, S3ObjectContent{
			Key:          obj.Key,
			LastModified: obj.LastModified.UTC().Format(time.RFC3339),
			ETag:         `"` + obj.ETag + `"`,
			Size:         obj.Size,
			StorageClass: "STANDARD",
		})
	}
	prefixes := make([]S3CommonPrefix, 0, len(res.CommonPrefixes))
	for _, cp := range res.CommonPrefixes {
		prefixes = append(prefixes, S3CommonPrefix{Prefix: cp})
	}
	writeXML(c, http.StatusOK, ListBucketResult{
		Name:           bucket,
		Prefix:         prefix,
		Delimiter:      delimiter,
		MaxKeys:        1000,
		IsTruncated:    res.IsTruncated,
		KeyCount:       len(contents),
		Contents:       contents,
		CommonPrefixes: prefixes,
	})
}

// ---- object handlers --------------------------------------------------------

// putObject serves PUT /:bucket/*key
func (h *Handler) putObject(c *gin.Context) {
	bucket, key := bucketKey(c)
	if !h.authorizeObject(c, "s3:PutObject", bucket, key) {
		return
	}
	if !h.buckets.BucketExistsForS3(bucket) {
		writeS3Error(c, http.StatusNotFound, ErrCodeNoSuchBucket, "The specified bucket does not exist.", "/"+bucket+"/"+key)
		return
	}

	// Pre-check quota using Content-Length when available.
	if quota := h.buckets.BucketQuotaBytes(bucket); quota > 0 {
		currentSize := h.objSvc.BucketSizeBytes(bucket)
		if cl := c.Request.ContentLength; cl > 0 && currentSize+cl > quota {
			writeS3Error(c, http.StatusForbidden, ErrCodeBucketQuotaExceeded,
				fmt.Sprintf("bucket quota of %d bytes would be exceeded", quota), "/"+bucket+"/"+key)
			return
		}
	}

	meta, err := h.objSvc.PutObject(c.Request.Context(), bucket, key, c.Request.Body, c.GetHeader("Content-Type"))
	if err != nil {
		writeS3Error(c, http.StatusInternalServerError, ErrCodeInternalError, err.Error(), "/"+bucket+"/"+key)
		return
	}

	// Post-check quota to catch cases where Content-Length was absent or wrong.
	if quota := h.buckets.BucketQuotaBytes(bucket); quota > 0 {
		if h.objSvc.BucketSizeBytes(bucket) > quota {
			_ = h.objSvc.DeleteObject(bucket, key)
			writeS3Error(c, http.StatusForbidden, ErrCodeBucketQuotaExceeded,
				fmt.Sprintf("bucket quota of %d bytes exceeded", quota), "/"+bucket+"/"+key)
			return
		}
	}

	c.Header("ETag", `"`+meta.ETag+`"`)
	c.Status(http.StatusOK)
}

// getObject serves GET /:bucket/*key
func (h *Handler) getObject(c *gin.Context) {
	bucket, key := bucketKey(c)
	if !h.authorizeObject(c, "s3:GetObject", bucket, key) {
		return
	}
	meta, rc, err := h.objSvc.GetObject(bucket, key)
	if err != nil {
		s3ObjectError(c, err, bucket, key)
		return
	}
	defer rc.Close()
	c.Header("Content-Type", meta.ContentType)
	c.Header("ETag", `"`+meta.ETag+`"`)
	c.Header("Last-Modified", time.Unix(meta.LastModified, 0).UTC().Format(http.TimeFormat))
	c.DataFromReader(http.StatusOK, meta.Size, meta.ContentType, rc, nil)
}

// headObject serves HEAD /:bucket/*key
func (h *Handler) headObject(c *gin.Context) {
	bucket, key := bucketKey(c)
	meta, err := h.objSvc.HeadObject(bucket, key)
	if err != nil {
		s3ObjectError(c, err, bucket, key)
		return
	}
	c.Header("Content-Type", meta.ContentType)
	c.Header("Content-Length", fmt.Sprintf("%d", meta.Size))
	c.Header("ETag", `"`+meta.ETag+`"`)
	c.Header("Last-Modified", time.Unix(meta.LastModified, 0).UTC().Format(http.TimeFormat))
	c.Status(http.StatusOK)
}

// deleteObject serves DELETE /:bucket/*key
func (h *Handler) deleteObject(c *gin.Context) {
	bucket, key := bucketKey(c)
	if !h.authorizeObject(c, "s3:DeleteObject", bucket, key) {
		return
	}
	if err := h.objSvc.DeleteObject(bucket, key); err != nil && err != ErrNoSuchKey {
		writeS3Error(c, http.StatusInternalServerError, ErrCodeInternalError, err.Error(), "/"+bucket+"/"+key)
		return
	}
	// S3 returns 204 even when the key does not exist
	c.Status(http.StatusNoContent)
}

// ---- helpers ----------------------------------------------------------------

func bucketKey(c *gin.Context) (bucket, key string) {
	bucket = c.Param("bucket")
	key = strings.TrimPrefix(c.Param("key"), "/")
	return
}

func s3ObjectError(c *gin.Context, err error, bucket, key string) {
	if err == ErrNoSuchKey {
		writeS3Error(c, http.StatusNotFound, ErrCodeNoSuchKey, "The specified key does not exist.", "/"+bucket+"/"+key)
	} else if err == ErrNoSuchBucket {
		writeS3Error(c, http.StatusNotFound, ErrCodeNoSuchBucket, "The specified bucket does not exist.", "/"+bucket)
	} else {
		writeS3Error(c, http.StatusInternalServerError, ErrCodeInternalError, err.Error(), "/"+bucket+"/"+key)
	}
}

func writeXML(c *gin.Context, status int, v any) {
	data, err := xml.Marshal(v)
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}
	c.Data(status, "application/xml", append([]byte(xml.Header), data...))
}

func writeS3Error(c *gin.Context, status int, code, message, resource string) {
	writeXML(c, status, S3Error{Code: code, Message: message, Resource: resource})
}
