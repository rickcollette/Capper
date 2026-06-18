package backup

// BackupType identifies what was backed up.
type BackupType string

const (
	BackupTypeStore    BackupType = "store"    // SQLite DB + instance JSON files
	BackupTypeVolume   BackupType = "volume"   // storage volume snapshot
	BackupTypeDatabase BackupType = "database" // pg_dump of a managed Postgres instance
)

// BackupRecord describes a completed backup.
type BackupRecord struct {
	ID        string     `json:"id"`
	Type      BackupType `json:"type"`
	Project   string     `json:"project"`
	Path      string     `json:"path"` // absolute path to the backup artifact
	SizeBytes int64      `json:"sizeBytes"`
	CreatedAt string     `json:"createdAt"`
}

// Policy is a scheduled backup policy.
type Policy struct {
	ID           string     `json:"id"`
	Name         string     `json:"name"`
	Project      string     `json:"project"`
	Type         BackupType `json:"type"`
	TargetPath   string     `json:"targetPath"`       // destination directory
	Source       string     `json:"source,omitempty"` // database connection string or future source ref
	IntervalSecs int        `json:"intervalSecs"`     // run every N seconds (0 = manual only)
	Retention    int        `json:"retention"`        // keep last N backups (0 = keep all)
	LastRunAt    string     `json:"lastRunAt,omitempty"`
	CreatedAt    string     `json:"createdAt"`
}
