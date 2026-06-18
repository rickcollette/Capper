package lb

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// Store persists LB configuration in SQLite.
type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store { return &Store{db: db} }

// InitSchema creates the lb tables if they do not exist.
func InitSchema(db *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS lb_load_balancers (
			id           TEXT PRIMARY KEY,
			name         TEXT NOT NULL,
			project      TEXT NOT NULL DEFAULT 'default',
			network_id   TEXT NOT NULL DEFAULT '',
			mode         TEXT NOT NULL DEFAULT 'tcp',
			listen_addr  TEXT NOT NULL DEFAULT '',
			status       TEXT NOT NULL DEFAULT 'active',
			created_at   TEXT NOT NULL,
			algorithm    TEXT NOT NULL DEFAULT '',
			selector     TEXT NOT NULL DEFAULT '',
			tls_cert_name TEXT NOT NULL DEFAULT '',
			UNIQUE(name, project)
		)`,
		`CREATE TABLE IF NOT EXISTS lb_backends (
			id      TEXT PRIMARY KEY,
			lb_id   TEXT NOT NULL REFERENCES lb_load_balancers(id) ON DELETE CASCADE,
			address TEXT NOT NULL,
			UNIQUE(lb_id, address)
		)`,
	}
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			return err
		}
	}
	// Additive migrations for existing databases.
	for _, alter := range []string{
		`ALTER TABLE lb_load_balancers ADD COLUMN algorithm TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE lb_load_balancers ADD COLUMN selector TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE lb_load_balancers ADD COLUMN tls_cert_name TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE lb_load_balancers ADD COLUMN service_alias TEXT NOT NULL DEFAULT ''`,
	} {
		if _, err := db.Exec(alter); err != nil {
			if !isDupeCol(err) {
				return err
			}
		}
	}
	return nil
}

func isDupeCol(err error) bool {
	s := err.Error()
	return strings.Contains(s, "duplicate column") || strings.Contains(s, "already exists")
}

func (s *Store) Insert(lb LoadBalancer) error {
	_, err := s.db.Exec(
		`INSERT INTO lb_load_balancers
		 (id, name, project, network_id, mode, listen_addr, status, algorithm, selector, tls_cert_name, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		lb.ID, lb.Name, lb.Project, lb.NetworkID, lb.Mode, lb.ListenAddr, lb.Status,
		string(lb.Algorithm), lb.Selector, lb.TLSCertName, lb.CreatedAt,
	)
	return err
}

const lbCols = `id, name, project, network_id, mode, listen_addr, status, algorithm, selector, tls_cert_name, service_alias, created_at`

func (s *Store) Get(nameOrID, project string) (LoadBalancer, error) {
	var row *sql.Row
	q := `SELECT ` + lbCols + ` FROM lb_load_balancers WHERE (id=? OR name=?)`
	if project == "" {
		row = s.db.QueryRow(q+` LIMIT 1`, nameOrID, nameOrID)
	} else {
		row = s.db.QueryRow(q+` AND project=? LIMIT 1`, nameOrID, nameOrID, project)
	}
	return scanLB(row)
}

func (s *Store) List(project string) ([]LoadBalancer, error) {
	var (
		rows *sql.Rows
		err  error
	)
	q := `SELECT ` + lbCols + ` FROM lb_load_balancers`
	if project == "" {
		rows, err = s.db.Query(q + ` ORDER BY name`)
	} else {
		rows, err = s.db.Query(q+` WHERE project=? ORDER BY name`, project)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []LoadBalancer
	for rows.Next() {
		lb, err := scanLB(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, lb)
	}
	return out, rows.Err()
}

func (s *Store) ListActive() ([]LoadBalancer, error) {
	rows, err := s.db.Query(
		`SELECT `+lbCols+` FROM lb_load_balancers WHERE status='active' AND listen_addr != ''`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []LoadBalancer
	for rows.Next() {
		lb, err := scanLB(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, lb)
	}
	return out, rows.Err()
}

func (s *Store) UpdateStatus(id string, status LBStatus) error {
	_, err := s.db.Exec(`UPDATE lb_load_balancers SET status=? WHERE id=?`, status, id)
	return err
}

func (s *Store) UpdateListenAddr(id, addr string) error {
	_, err := s.db.Exec(`UPDATE lb_load_balancers SET listen_addr=? WHERE id=?`, addr, id)
	return err
}

// SetMeta updates algorithm, selector, and TLS cert name for an LB.
func (s *Store) SetMeta(id string, selector, tlsCertName string, algo LBAlgorithm) error {
	_, err := s.db.Exec(
		`UPDATE lb_load_balancers SET selector=?, tls_cert_name=?, algorithm=? WHERE id=?`,
		selector, tlsCertName, string(algo), id,
	)
	return err
}

func (s *Store) SetServiceAlias(id, alias string) error {
	_, err := s.db.Exec(`UPDATE lb_load_balancers SET service_alias=? WHERE id=?`, alias, id)
	return err
}

func (s *Store) Delete(nameOrID, project string) error {
	var res sql.Result
	var err error
	if project == "" {
		res, err = s.db.Exec(`DELETE FROM lb_load_balancers WHERE id=? OR name=?`, nameOrID, nameOrID)
	} else {
		res, err = s.db.Exec(`DELETE FROM lb_load_balancers WHERE (id=? OR name=?) AND project=?`,
			nameOrID, nameOrID, project)
	}
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("lb %q not found", nameOrID)
	}
	return nil
}

func (s *Store) AddBackend(lbID, address string) (Backend, error) {
	id := newID()
	_, err := s.db.Exec(
		`INSERT INTO lb_backends (id, lb_id, address) VALUES (?, ?, ?)
		 ON CONFLICT(lb_id, address) DO NOTHING`,
		id, lbID, address,
	)
	if err != nil {
		return Backend{}, err
	}
	return Backend{ID: id, LBID: lbID, Address: address, Healthy: true}, nil
}

func (s *Store) RemoveBackend(lbID, address string) error {
	res, err := s.db.Exec(`DELETE FROM lb_backends WHERE lb_id=? AND address=?`, lbID, address)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("backend %q not found on lb %q", address, lbID)
	}
	return nil
}

func (s *Store) ListBackends(lbID string) ([]Backend, error) {
	rows, err := s.db.Query(
		`SELECT id, lb_id, address FROM lb_backends WHERE lb_id=? ORDER BY address`,
		lbID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Backend
	for rows.Next() {
		var b Backend
		b.Healthy = true // health state is live, not persisted
		if err := rows.Scan(&b.ID, &b.LBID, &b.Address); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

type rowScanner interface{ Scan(dest ...any) error }

func scanLB(s rowScanner) (LoadBalancer, error) {
	var lb LoadBalancer
	var algo, selector, tlsCert, serviceAlias string
	if err := s.Scan(&lb.ID, &lb.Name, &lb.Project, &lb.NetworkID, &lb.Mode,
		&lb.ListenAddr, &lb.Status, &algo, &selector, &tlsCert, &serviceAlias, &lb.CreatedAt); err != nil {
		if err == sql.ErrNoRows {
			return LoadBalancer{}, fmt.Errorf("lb not found")
		}
		return LoadBalancer{}, fmt.Errorf("lb: scan: %w", err)
	}
	lb.Algorithm = LBAlgorithm(algo)
	lb.Selector = selector
	lb.TLSCertName = tlsCert
	lb.ServiceAlias = serviceAlias
	return lb, nil
}

func newID() string {
	return "lb_" + fmt.Sprintf("%d", time.Now().UnixNano())
}
