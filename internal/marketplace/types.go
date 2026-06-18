package marketplace

// MarketplaceListing is a published image available for installation.
type MarketplaceListing struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	Version      string            `json:"version"`
	Description  string            `json:"description,omitempty"`
	Digest       string            `json:"digest,omitempty"`
	Status       string            `json:"status"` // pending, approved, rejected, quarantined
	Labels       map[string]string `json:"labels,omitempty"`
	CreatedAt    string            `json:"createdAt"`
	UpdatedAt    string            `json:"updatedAt"`
	ScanStatus     string            `json:"scanStatus,omitempty"`
	ScanFindings   int               `json:"scanFindings,omitempty"`
	ScanResults    string            `json:"scanResults,omitempty"` // JSON blob
	ScanSeverities map[string]int    `json:"scanSeverities,omitempty"`
	ScanScannedAt  string            `json:"scanScannedAt,omitempty"`
	SBOMDigest     string            `json:"sbomDigest,omitempty"`
}

const (
	StatusPending     = "pending"
	StatusApproved    = "approved"
	StatusRejected    = "rejected"
	StatusQuarantined = "quarantined"
)
