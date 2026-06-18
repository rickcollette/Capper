package database

type DBEngine string

const (
	EnginePostgres DBEngine = "postgres"
	EngineRedis    DBEngine = "redis"
	EngineMariaDB  DBEngine = "mariadb"
)

type DBStatus string

const (
	DBStatusRunning      DBStatus = "running"
	DBStatusStopped      DBStatus = "stopped"
	DBStatusProvisioning DBStatus = "provisioning"
)

type ManagedDB struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Project    string   `json:"project"`
	Engine     DBEngine `json:"engine"`
	Version    string   `json:"version,omitempty"`
	Status     DBStatus `json:"status"`
	NetworkID  string   `json:"networkId,omitempty"`
	InstanceID string   `json:"instanceId,omitempty"`
	VolumeID   string   `json:"volumeId,omitempty"`
	SecretName string   `json:"secretName,omitempty"`
	DNSName    string   `json:"dnsName,omitempty"`
	Port       int      `json:"port"`
	PrimaryID  string   `json:"primaryId,omitempty"` // set for read replicas
	CreatedAt  string   `json:"createdAt"`
}

// DBBackup is a point-in-time backup of a managed database.
type DBBackup struct {
	ID        string `json:"id"`
	DBID      string `json:"dbId"`
	Project   string `json:"project"`
	Type      string `json:"type"`      // "full" | "incremental"
	Path      string `json:"path"`
	Status    string `json:"status"`    // "pending" | "complete" | "failed"
	SizeBytes int64  `json:"sizeBytes"`
	CreatedAt string `json:"createdAt"`
}

// DefaultPorts maps engine types to their standard ports.
var DefaultPorts = map[DBEngine]int{
	EnginePostgres: 5432,
	EngineRedis:    6379,
	EngineMariaDB:  3306,
}

// DefaultVersions maps engine types to their default versions.
var DefaultVersions = map[DBEngine]string{
	EnginePostgres: "16",
	EngineRedis:    "7",
	EngineMariaDB:  "11",
}
