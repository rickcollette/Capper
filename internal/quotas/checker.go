package quotas

import "fmt"

// Checker wraps Store to provide quota enforcement.
type Checker struct {
	store *Store
}

// NewChecker creates a Checker backed by the given Store.
func NewChecker(store *Store) *Checker { return &Checker{store: store} }

// Check returns an error if the accountID has used >= limit for the given quotaKey.
// delta is how many units this operation would add (usually 1).
func (c *Checker) Check(accountID, quotaKey string, delta int64) error {
	limit, err := c.store.GetQuota(accountID, quotaKey)
	if err != nil {
		return err
	}
	used, err := c.store.CurrentUsage(accountID, quotaKey)
	if err != nil {
		return err
	}
	if used+delta > limit {
		return fmt.Errorf("quota exceeded for %s: used %d of %d", quotaKey, used, limit)
	}
	return nil
}

// Record records resource usage for the given accountID after a successful create.
func (c *Checker) Record(accountID, quotaKey, resourceID string, delta int64) error {
	return c.store.RecordUsage(accountID, quotaKey, resourceID, delta)
}

// Release removes a resource from usage tracking after a successful delete.
func (c *Checker) Release(accountID, quotaKey, resourceID string) error {
	return c.store.ReleaseUsage(accountID, quotaKey, resourceID)
}
