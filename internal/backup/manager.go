package backup

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"database/sql"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Manager performs store backups and manages policies.
type Manager struct {
	store      *Store
	db         *sql.DB
	storeDir   string // root directory containing capper.db and instances/
	runCommand commandRunner
}

func NewManager(s *Store, db *sql.DB, storeDir string) *Manager {
	return &Manager{store: s, db: db, storeDir: storeDir, runCommand: execCommand}
}

type commandRunner func(context.Context, string, ...string) error

func execCommand(ctx context.Context, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %s: %w: %s", name, strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return nil
}

// SetCommandRunner overrides command execution for tests.
func (m *Manager) SetCommandRunner(r func(context.Context, string, ...string) error) {
	if r == nil {
		m.runCommand = execCommand
		return
	}
	m.runCommand = r
}

// BackupStore creates a hot backup of the SQLite DB (via VACUUM INTO) and
// a gzipped tar of the instance JSON files, placed in destDir.
// Returns the backup record on success.
func (m *Manager) BackupStore(project, destDir string) (BackupRecord, error) {
	if destDir == "" {
		destDir = filepath.Join(m.storeDir, "backups")
	}
	if err := os.MkdirAll(destDir, 0o700); err != nil {
		return BackupRecord{}, fmt.Errorf("backup: mkdir: %w", err)
	}
	ts := time.Now().UTC().Format("20060102T150405Z")
	dbDest := filepath.Join(destDir, "capper_"+ts+".db")

	// SQLite hot backup via VACUUM INTO (supported by modernc.org/sqlite).
	if _, err := m.db.Exec(`VACUUM INTO ?`, dbDest); err != nil {
		return BackupRecord{}, fmt.Errorf("backup: vacuum: %w", err)
	}

	// Tar the instance JSON files.
	tarDest := filepath.Join(destDir, "instances_"+ts+".tar.gz")
	if err := tarDir(filepath.Join(m.storeDir, "instances"), tarDest); err != nil {
		// Non-fatal — instances dir may not exist.
		_ = os.Remove(tarDest)
	}

	fi, err := os.Stat(dbDest)
	if err != nil {
		return BackupRecord{}, err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	rec := BackupRecord{
		ID:        newID(),
		Type:      BackupTypeStore,
		Project:   project,
		Path:      dbDest,
		SizeBytes: fi.Size(),
		CreatedAt: now,
	}
	if err := m.store.InsertRecord(rec); err != nil {
		return BackupRecord{}, fmt.Errorf("backup: record: %w", err)
	}
	return rec, nil
}

// ListRecords returns all backup records for the project.
func (m *Manager) ListRecords(project string) ([]BackupRecord, error) {
	return m.store.ListRecords(project)
}

// Restore copies the backup DB file over the live DB. The caller must ensure
// the store is closed before calling this.
func (m *Manager) Restore(backupID, project string) error {
	records, err := m.store.ListRecords(project)
	if err != nil {
		return err
	}
	var rec *BackupRecord
	for i := range records {
		if records[i].ID == backupID || records[i].Path == backupID {
			rec = &records[i]
			break
		}
	}
	if rec == nil {
		return fmt.Errorf("backup record %q not found", backupID)
	}
	liveDB := filepath.Join(m.storeDir, "capper.db")
	return copyFile(rec.Path, liveDB)
}

// CreatePolicy stores a new backup policy.
func (m *Manager) CreatePolicy(name, project, targetPath string, btype BackupType, intervalSecs, retention int) (Policy, error) {
	return m.CreatePolicyWithSource(name, project, targetPath, "", btype, intervalSecs, retention)
}

func (m *Manager) CreatePolicyWithSource(name, project, targetPath, source string, btype BackupType, intervalSecs, retention int) (Policy, error) {
	p := Policy{
		ID:           newID(),
		Name:         name,
		Project:      project,
		Type:         btype,
		TargetPath:   targetPath,
		Source:       source,
		IntervalSecs: intervalSecs,
		Retention:    retention,
		CreatedAt:    time.Now().UTC().Format(time.RFC3339),
	}
	if err := m.store.InsertPolicy(p); err != nil {
		return Policy{}, fmt.Errorf("backup: policy: %w", err)
	}
	return p, nil
}

func (m *Manager) ListPolicies(project string) ([]Policy, error) {
	return m.store.ListPolicies(project)
}

func (m *Manager) DeletePolicy(nameOrID, project string) error {
	return m.store.DeletePolicy(nameOrID, project)
}

// RunDuePolicies executes any scheduled store-backup policies that are overdue.
func (m *Manager) RunDuePolicies(project string) error {
	policies, err := m.store.ListDuePolicies()
	if err != nil {
		return err
	}
	for _, p := range policies {
		var err error
		switch p.Type {
		case BackupTypeStore:
			_, err = m.BackupStore(p.Project, p.TargetPath)
		case BackupTypeDatabase:
			_, err = m.BackupDatabase(p.Project, p.Name, p.Source, p.TargetPath)
		default:
			continue
		}
		if err != nil {
			continue
		}
		now := time.Now().UTC().Format(time.RFC3339)
		_ = m.store.UpdatePolicyLastRun(p.ID, now)
		// Enforce retention.
		if p.Retention > 0 {
			m.enforceRetention(p)
		}
	}
	return nil
}

func (m *Manager) enforceRetention(p Policy) {
	records, err := m.store.ListRecords(p.Project)
	if err != nil || len(records) <= p.Retention {
		return
	}
	sort.Slice(records, func(i, j int) bool {
		return records[i].CreatedAt > records[j].CreatedAt
	})
	for _, r := range records[p.Retention:] {
		_ = os.Remove(r.Path)
		_ = m.store.DeleteRecord(r.ID)
	}
}

func tarDir(srcDir, destFile string) error {
	f, err := os.Create(destFile)
	if err != nil {
		return err
	}
	defer f.Close()
	gw := gzip.NewWriter(f)
	defer gw.Close()
	tw := tar.NewWriter(gw)
	defer tw.Close()
	return filepath.WalkDir(srcDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		info, ierr := d.Info()
		if ierr != nil {
			return nil
		}
		hdr, herr := tar.FileInfoHeader(info, "")
		if herr != nil {
			return nil
		}
		rel, _ := filepath.Rel(srcDir, path)
		hdr.Name = rel
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		rf, ferr := os.Open(path)
		if ferr != nil {
			return nil
		}
		defer rf.Close()
		_, cerr := io.Copy(tw, rf)
		return cerr
	})
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

// BackupDatabase creates a pg_dump custom-format backup for a managed Postgres
// database. connectionString is passed directly to pg_dump as the database
// target, so callers may use a libpq URI or service name.
func (m *Manager) BackupDatabase(project, dbName, connectionString, destDir string) (BackupRecord, error) {
	if connectionString == "" {
		return BackupRecord{}, fmt.Errorf("backup: database connection string is required")
	}
	if destDir == "" {
		destDir = filepath.Join(m.storeDir, "backups", "databases")
	}
	if err := os.MkdirAll(destDir, 0o700); err != nil {
		return BackupRecord{}, fmt.Errorf("backup: mkdir database: %w", err)
	}
	ts := time.Now().UTC()
	fname := fmt.Sprintf("%s-%s.pgdump", safeArtifactName(dbName), ts.Format("20060102-150405"))
	dest := filepath.Join(destDir, fname)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	if err := m.runCommand(ctx, "pg_dump", "--format=custom", "--file", dest, connectionString); err != nil {
		_ = os.Remove(dest)
		return BackupRecord{}, fmt.Errorf("backup: pg_dump: %w", err)
	}
	fi, err := os.Stat(dest)
	if err != nil {
		return BackupRecord{}, fmt.Errorf("backup: stat database dump: %w", err)
	}
	rec := BackupRecord{
		ID:        fmt.Sprintf("bkp_%d", ts.UnixNano()),
		Type:      BackupTypeDatabase,
		Project:   project,
		Path:      dest,
		SizeBytes: fi.Size(),
		CreatedAt: ts.UTC().Format(time.RFC3339),
	}
	if err := m.store.InsertRecord(rec); err != nil {
		return BackupRecord{}, fmt.Errorf("backup: record database backup: %w", err)
	}
	return rec, nil
}

// RestoreDatabase restores a database backup into targetConnectionString using
// pg_restore. The target should be an already-created destination database.
func (m *Manager) RestoreDatabase(backupID, project, targetConnectionString string) error {
	if targetConnectionString == "" {
		return fmt.Errorf("backup: target database connection string is required")
	}
	records, err := m.store.ListRecords(project)
	if err != nil {
		return err
	}
	var rec *BackupRecord
	for i := range records {
		if records[i].Type == BackupTypeDatabase && (records[i].ID == backupID || records[i].Path == backupID) {
			rec = &records[i]
			break
		}
	}
	if rec == nil {
		return fmt.Errorf("database backup %q not found", backupID)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	if err := m.runCommand(ctx, "pg_restore", "--clean", "--if-exists", "--dbname", targetConnectionString, rec.Path); err != nil {
		return fmt.Errorf("backup: pg_restore: %w", err)
	}
	return nil
}

// ListDatabaseBackups returns all database backup records for the project.
func (m *Manager) ListDatabaseBackups(project string) ([]BackupRecord, error) {
	all, err := m.ListRecords(project)
	if err != nil {
		return nil, err
	}
	var out []BackupRecord
	for _, r := range all {
		if r.Type == BackupTypeDatabase {
			out = append(out, r)
		}
	}
	return out, nil
}

func safeArtifactName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "database"
	}
	var b strings.Builder
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	return b.String()
}
