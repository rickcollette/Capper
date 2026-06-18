package database

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"
)

type Manager struct {
	store *Store
}

func NewManager(s *Store) *Manager { return &Manager{store: s} }

func (m *Manager) Create(name, project, engine, version, networkID string, port int) (ManagedDB, string, error) {
	if name == "" {
		return ManagedDB{}, "", fmt.Errorf("database: name is required")
	}
	eng := DBEngine(engine)
	if !engineSupported(eng) {
		return ManagedDB{}, "", fmt.Errorf("database: unsupported engine %q", engine)
	}
	passBuf := make([]byte, 16)
	if _, err := rand.Read(passBuf); err != nil {
		return ManagedDB{}, "", fmt.Errorf("database: generate password: %w", err)
	}
	password := hex.EncodeToString(passBuf)
	secretName := name + "-password-" + hex.EncodeToString(passBuf[:4])
	now := time.Now().UTC().Format(time.RFC3339)
	if version == "" {
		version = DefaultVersions[eng]
	}
	if port == 0 {
		port = DefaultPorts[eng]
	}
	db := ManagedDB{
		ID:         newID(),
		Name:       name,
		Project:    project,
		Engine:     eng,
		Version:    version,
		Status:     DBStatusProvisioning,
		NetworkID:  networkID,
		SecretName: secretName,
		Port:       port,
		CreatedAt:  now,
	}
	if err := m.store.Insert(db); err != nil {
		return ManagedDB{}, "", fmt.Errorf("database: store: %w", err)
	}
	return db, password, nil
}

func engineSupported(eng DBEngine) bool {
	for _, e := range Engines {
		if e == eng {
			return true
		}
	}
	return false
}

func (m *Manager) UpdateInstanceID(id, instanceID string, status DBStatus) error {
	return m.store.UpdateInstanceID(id, instanceID, status)
}

func (m *Manager) Get(nameOrID, project string) (ManagedDB, error) {
	return m.store.Get(nameOrID, project)
}

func (m *Manager) List(project string) ([]ManagedDB, error) {
	return m.store.List(project)
}

func (m *Manager) Delete(nameOrID, project string) error {
	return m.store.Delete(nameOrID, project)
}

func (m *Manager) UpdateStatus(nameOrID, project string, status DBStatus) error {
	db, err := m.store.Get(nameOrID, project)
	if err != nil {
		return err
	}
	return m.store.UpdateStatus(db.ID, status)
}

// ---- backup lifecycle -------------------------------------------------------

// BackupExecutor is a function that performs the actual pg_dump / redis-cli SAVE
// for a database and returns the path and size of the backup file.
type BackupExecutor func(db ManagedDB) (path string, sizeBytes int64, err error)

// CreateBackup records a pending backup, calls executor, then marks it complete.
func (m *Manager) CreateBackup(nameOrID, project, backupType string, executor BackupExecutor) (DBBackup, error) {
	db, err := m.store.Get(nameOrID, project)
	if err != nil {
		return DBBackup{}, err
	}
	if executor == nil {
		return DBBackup{}, fmt.Errorf("database: backup executor is required")
	}
	backup := DBBackup{
		ID:      newID() + "_bk",
		DBID:    db.ID,
		Project: project,
		Type:    backupType,
		Status:  "pending",
	}
	if backup.Type == "" {
		backup.Type = "full"
	}
	if err := m.store.InsertBackup(backup); err != nil {
		return backup, fmt.Errorf("database: record backup: %w", err)
	}
	path, size, err := executor(db)
	if err != nil {
		_ = m.store.UpdateBackupStatus(backup.ID, "failed", 0)
		return backup, fmt.Errorf("database: backup executor: %w", err)
	}
	backup.Path = path
	backup.SizeBytes = size
	backup.Status = "complete"
	if serr := m.store.UpdateBackupStatus(backup.ID, "complete", size); serr != nil {
		return backup, serr
	}
	return backup, nil
}

// ListBackups returns all backups for a database.
func (m *Manager) ListBackups(nameOrID, project string) ([]DBBackup, error) {
	db, err := m.store.Get(nameOrID, project)
	if err != nil {
		return nil, err
	}
	return m.store.ListBackups(db.ID)
}

// DeleteBackup removes a backup record by ID.
func (m *Manager) DeleteBackup(backupID string) error {
	return m.store.DeleteBackup(backupID)
}

// RestoreExecutor restores backupID into targetConnectionString.
type RestoreExecutor func(backupID, project, targetConnectionString string) error

// RestoreIntoNew creates a target database record and restores a backup into
// the caller-provided target connection string. The database is marked running
// only after restore succeeds.
func (m *Manager) RestoreIntoNew(backupID, targetName, project, engine, version, networkID string, port int, targetConnectionString string, restore RestoreExecutor) (ManagedDB, error) {
	if restore == nil {
		return ManagedDB{}, fmt.Errorf("database: restore executor is required")
	}
	if targetConnectionString == "" {
		return ManagedDB{}, fmt.Errorf("database: target connection string is required")
	}
	db, _, err := m.Create(targetName, project, engine, version, networkID, port)
	if err != nil {
		return ManagedDB{}, err
	}
	if err := restore(backupID, project, targetConnectionString); err != nil {
		_ = m.store.UpdateStatus(db.ID, DBStatusStopped)
		return ManagedDB{}, fmt.Errorf("database: restore backup: %w", err)
	}
	if err := m.store.UpdateStatus(db.ID, DBStatusRunning); err != nil {
		return ManagedDB{}, err
	}
	return m.store.Get(db.ID, project)
}

// ---- read replicas (Block 8 Ph5) --------------------------------------------

// AddReplica creates a read replica of the primary database.
// The replica shares the same engine and version; network routing to the
// physical replica instance is handled outside this manager.
func (m *Manager) AddReplica(primaryNameOrID, project, replicaName string) (ManagedDB, error) {
	primary, err := m.store.Get(primaryNameOrID, project)
	if err != nil {
		return ManagedDB{}, fmt.Errorf("database: primary not found: %w", err)
	}
	now := time.Now().UTC().Format(time.RFC3339)
	replica := ManagedDB{
		ID:         newID(),
		Name:       replicaName,
		Project:    project,
		Engine:     primary.Engine,
		Version:    primary.Version,
		Status:     DBStatusProvisioning,
		NetworkID:  primary.NetworkID,
		SecretName: primary.SecretName,
		Port:       primary.Port,
		PrimaryID:  primary.ID,
		CreatedAt:  now,
	}
	if err := m.store.Insert(replica); err != nil {
		return ManagedDB{}, fmt.Errorf("database: add replica: %w", err)
	}
	return replica, nil
}

// DNSUpdater updates a DNS A record to point to the new primary.
// Injected to avoid a circular import with capper/internal/dns.
type DNSUpdater func(dnsName, newIP string) error

// PromoteReplica promotes a read replica to become the new primary.
// If dnsUpdater is provided, the database's DNS record is updated to point to
// the replica's new address (failover DNS record update).
func (m *Manager) PromoteReplica(replicaNameOrID, project string, dnsUpdater DNSUpdater) error {
	replica, err := m.store.Get(replicaNameOrID, project)
	if err != nil {
		return fmt.Errorf("database: replica not found: %w", err)
	}
	if replica.PrimaryID == "" {
		return fmt.Errorf("database: %q is not a replica", replica.Name)
	}
	// Demote old primary.
	if serr := m.store.UpdateStatus(replica.PrimaryID, DBStatusStopped); serr != nil {
		return fmt.Errorf("database: demote primary: %w", serr)
	}
	// Promote replica.
	if serr := m.store.ClearPrimaryID(replica.ID); serr != nil {
		return fmt.Errorf("database: clear primary ref: %w", serr)
	}
	if serr := m.store.UpdateStatus(replica.ID, DBStatusRunning); serr != nil {
		return serr
	}
	// Update DNS to point the service name at the promoted replica.
	if dnsUpdater != nil && replica.DNSName != "" {
		_ = dnsUpdater(replica.DNSName, replica.InstanceID) // best-effort
	}
	return nil
}
