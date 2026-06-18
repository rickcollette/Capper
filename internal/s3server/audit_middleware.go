package s3server

import (
	"fmt"
	"time"

	"capper/internal/audit"

	"github.com/gin-gonic/gin"
)

// S3AuditEvent captures the fields recorded for every S3 operation.
type S3AuditEvent struct {
	Timestamp  time.Time
	AccessKey  string
	Method     string
	Bucket     string
	Key        string
	StatusCode int
	RemoteAddr string
	UserAgent  string
}

// S3AuditMiddleware records an audit event after every S3 handler runs.
// It reads the authenticated access key set by SigV4Auth and the response
// status written by the object handler.
func S3AuditMiddleware(recorder *audit.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		accessKeyRaw, _ := c.Get("s3AccessKey")
		accessKey := fmt.Sprint(accessKeyRaw)

		bucket := c.Param("bucket")
		key := c.Param("key")

		decision := "allow"
		if c.Writer.Status() >= 400 {
			decision = "deny"
		}

		resource := bucket
		if key != "" {
			resource = bucket + key
		}

		event := audit.Event{
			ID:          audit.NewID(),
			ActorType:   "s3-access-key",
			ActorID:     accessKey,
			Action:      "s3:" + c.Request.Method,
			ResourceCRN: "crn:capper:s3:::" + resource,
			Decision:    decision,
			SourceIP:    c.ClientIP(),
			UserAgent:   c.Request.UserAgent(),
			CreatedAt:   time.Now().UTC().Format(time.RFC3339),
		}
		_ = recorder.Record(event)
	}
}
