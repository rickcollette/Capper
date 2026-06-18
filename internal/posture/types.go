package posture

// Severity classifies a posture finding.
type Severity string

const (
	SeverityHigh   Severity = "high"
	SeverityMedium Severity = "medium"
	SeverityLow    Severity = "low"
	SeverityInfo   Severity = "info"
)

// Finding is a single posture check result.
type Finding struct {
	ID         string   `json:"id"`
	ScanID     string   `json:"scanId"`
	Project    string   `json:"project"`
	Check      string   `json:"check"`      // e.g. "open-ports", "world-writable", "suid"
	Severity   Severity `json:"severity"`
	Target     string   `json:"target"`     // path, address, or resource description
	Detail     string   `json:"detail"`
	ScannedAt  string   `json:"scannedAt"`
}

// ScanResult groups findings from a single scan run.
type ScanResult struct {
	ScanID    string    `json:"scanId"`
	Project   string    `json:"project"`
	ScannedAt string    `json:"scannedAt"`
	Findings  []Finding `json:"findings"`
}
