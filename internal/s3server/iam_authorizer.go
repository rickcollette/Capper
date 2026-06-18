package s3server

import (
	"database/sql"
	"fmt"
	"strings"

	"capper/internal/iam"
)

// CapperObjectAuthorizer implements ObjectAuthorizer using the Capper IAM stack
// plus bucket-level policy evaluation.
//
// Authorization order:
//  1. Resolve access key → IAM principal (returns 403 if unknown).
//  2. IAM policy check (deny-by-default).
//  3. Bucket policy check — an explicit Deny overrides an IAM Allow.
type CapperObjectAuthorizer struct {
	iam *iam.Manager
	db  *sql.DB
}

// NewCapperObjectAuthorizer creates an authorizer backed by the given IAM manager
// and the application database (for bucket policy lookups).
func NewCapperObjectAuthorizer(iamMgr *iam.Manager, db *sql.DB) *CapperObjectAuthorizer {
	return &CapperObjectAuthorizer{iam: iamMgr, db: db}
}

// AuthorizeObject implements ObjectAuthorizer.
// action is one of "s3:GetObject", "s3:PutObject", "s3:DeleteObject".
// resource is "<bucket>/<key>" or "<bucket>/".
func (a *CapperObjectAuthorizer) AuthorizeObject(accessKey, action, resource string) error {
	principalType, principalID, err := a.iam.LookupByS3AccessKey(accessKey)
	if err != nil {
		return fmt.Errorf("s3: authorize: %w", err)
	}

	// IAM check.
	if err := a.iam.Authorize(principalType, principalID, action, "crn:capper:s3:::"+resource); err != nil {
		return err
	}

	// Bucket policy check — an explicit Deny overrides the IAM Allow above.
	bucket := resource
	if idx := strings.IndexByte(resource, '/'); idx >= 0 {
		bucket = resource[:idx]
	}
	bp, err := GetBucketPolicy(a.db, bucket)
	if err != nil {
		return fmt.Errorf("s3: bucket policy lookup: %w", err)
	}
	if len(bp.Statement) > 0 && !bp.Allows(principalID, action, resource) {
		return fmt.Errorf("s3: bucket policy denies %s on %s", action, resource)
	}
	return nil
}
