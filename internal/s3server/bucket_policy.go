package s3server

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// InitBucketPolicySchema creates the s3_bucket_policies table.
func InitBucketPolicySchema(db *sql.DB) error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS s3_bucket_policies (
		bucket     TEXT PRIMARY KEY,
		policy     TEXT NOT NULL DEFAULT '{}',
		updated_at TEXT NOT NULL
	)`)
	return err
}

// BucketPolicy is a simplified S3-style bucket policy.
type BucketPolicy struct {
	Version   string            `json:"Version"`
	Statement []PolicyStatement `json:"Statement"`
}

// PolicyStatement is a single allow/deny rule within a BucketPolicy.
type PolicyStatement struct {
	Effect    string            `json:"Effect"`
	Principal string            `json:"Principal"`
	Action    []string          `json:"Action"`
	Resource  []string          `json:"Resource"`
	Condition map[string]map[string]string `json:"Condition,omitempty"`
}

// Allows returns true if this policy permits principal to perform action on
// resource. An explicit Deny statement always returns false regardless of
// any Allow statements.
func (p *BucketPolicy) Allows(principal, action, resource string) bool {
	allowed := false
	for _, stmt := range p.Statement {
		if !stmtMatchesPrincipal(stmt.Principal, principal) {
			continue
		}
		if !stmtMatchesAction(stmt.Action, action) {
			continue
		}
		if !stmtMatchesResource(stmt.Resource, resource) {
			continue
		}
		if strings.EqualFold(stmt.Effect, "Deny") {
			return false
		}
		if strings.EqualFold(stmt.Effect, "Allow") {
			allowed = true
		}
	}
	return allowed
}

func stmtMatchesPrincipal(principal, target string) bool {
	return principal == "*" || principal == target
}

func stmtMatchesAction(actions []string, action string) bool {
	for _, a := range actions {
		if a == "*" || strings.EqualFold(a, action) {
			return true
		}
		if strings.HasSuffix(a, "*") && strings.HasPrefix(strings.ToLower(action), strings.ToLower(strings.TrimSuffix(a, "*"))) {
			return true
		}
	}
	return false
}

func stmtMatchesResource(resources []string, resource string) bool {
	for _, r := range resources {
		if r == "*" || r == resource {
			return true
		}
		if strings.HasSuffix(r, "*") && strings.HasPrefix(resource, strings.TrimSuffix(r, "*")) {
			return true
		}
	}
	return false
}

// GetBucketPolicy retrieves the policy for bucket from db.
// Returns an empty policy if none is set.
func GetBucketPolicy(db *sql.DB, bucket string) (BucketPolicy, error) {
	var raw string
	err := db.QueryRow(`SELECT policy FROM s3_bucket_policies WHERE bucket=?`, bucket).Scan(&raw)
	if err == sql.ErrNoRows {
		return BucketPolicy{Version: "2012-10-17"}, nil
	}
	if err != nil {
		return BucketPolicy{}, err
	}
	var p BucketPolicy
	if err := json.Unmarshal([]byte(raw), &p); err != nil {
		return BucketPolicy{}, fmt.Errorf("bucket policy for %q: malformed JSON: %w", bucket, err)
	}
	return p, nil
}

// PutBucketPolicy stores policy for bucket in db.
func PutBucketPolicy(db *sql.DB, bucket string, p BucketPolicy) error {
	raw, err := json.Marshal(p)
	if err != nil {
		return err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	_, err = db.Exec(
		`INSERT INTO s3_bucket_policies (bucket, policy, updated_at) VALUES (?,?,?)
		 ON CONFLICT(bucket) DO UPDATE SET policy=excluded.policy, updated_at=excluded.updated_at`,
		bucket, string(raw), now,
	)
	return err
}

// DeleteBucketPolicy removes the policy for bucket from db.
func DeleteBucketPolicy(db *sql.DB, bucket string) error {
	_, err := db.Exec(`DELETE FROM s3_bucket_policies WHERE bucket=?`, bucket)
	return err
}
