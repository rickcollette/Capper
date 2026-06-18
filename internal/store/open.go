package store

import (
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"
)

// openDB opens the control-plane database. By default it uses the in-process,
// pure-Go modernc SQLite driver (single-node, hermetic). Setting
// CAPPER_DB_DRIVER=capdb selects the networked CapDB backend instead, which
// requires a binary built with `-tags capdb` (see internal/capdbdriver).
//
//	CAPPER_DB_DRIVER=sqlite|capdb   (default: sqlite)
//	CAPPER_DB_DSN=capdb://user:pass@host:5432/capper   (required for capdb)
//	CAPPER_DB_MAX_OPEN_CONNS, CAPPER_DB_MAX_IDLE_CONNS, CAPPER_DB_CONN_MAX_LIFETIME_SECS
func openDB(paths Paths) (*sql.DB, error) {
	switch driver := os.Getenv("CAPPER_DB_DRIVER"); driver {
	case "", "sqlite":
		// WAL + a busy timeout so concurrent writers (renewal scheduler,
		// control daemon, autoscaler, background ACME orders) wait for the lock
		// instead of failing with "database is locked".
		//
		// CAPPER_DB_SYNCHRONOUS tunes durability vs. throughput (default NORMAL,
		// the WAL default — safe against corruption, may lose the last commit on
		// power loss). Set FULL for strict durability.
		sync, err := resolveSynchronous()
		if err != nil {
			return nil, err
		}
		return sql.Open("sqlite", fmt.Sprintf(
			"%s?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=synchronous(%s)",
			paths.DB, sync))
	case "capdb":
		return openCapDB()
	default:
		return nil, fmt.Errorf("store: unknown CAPPER_DB_DRIVER %q (want sqlite or capdb)", driver)
	}
}

// resolveSynchronous validates CAPPER_DB_SYNCHRONOUS and returns the SQLite
// synchronous mode to apply (default NORMAL).
func resolveSynchronous() (string, error) {
	switch v := strings.ToUpper(strings.TrimSpace(os.Getenv("CAPPER_DB_SYNCHRONOUS"))); v {
	case "":
		return "NORMAL", nil
	case "OFF", "NORMAL", "FULL", "EXTRA":
		return v, nil
	default:
		return "", fmt.Errorf("store: invalid CAPPER_DB_SYNCHRONOUS %q (want OFF, NORMAL, FULL, or EXTRA)", v)
	}
}

func openCapDB() (*sql.DB, error) {
	if !slices.Contains(sql.Drivers(), "capdb") {
		return nil, fmt.Errorf("store: CAPPER_DB_DRIVER=capdb but the capdb driver is not compiled in; rebuild with `-tags capdb` (see docs/src/operator-guide/capdb-backend.md)")
	}
	dsn := os.Getenv("CAPPER_DB_DSN")
	if dsn == "" {
		return nil, fmt.Errorf("store: CAPPER_DB_DRIVER=capdb requires CAPPER_DB_DSN (e.g. capdb://user:pass@host:5432/capper)")
	}
	// Keep the DB credential out of the persisted DSN/env: if a token file is
	// configured, read it and inject the token into the DSN at connect time only.
	if tf := os.Getenv("CAPPER_DB_TOKEN_FILE"); tf != "" {
		b, err := os.ReadFile(tf)
		if err != nil {
			return nil, fmt.Errorf("store: reading CAPPER_DB_TOKEN_FILE %q: %w", tf, err)
		}
		token := strings.TrimSpace(string(b))
		sep := "?"
		if strings.Contains(dsn, "?") {
			sep = "&"
		}
		dsn = dsn + sep + "token=" + url.QueryEscape(token)
	}
	db, err := sql.Open("capdb", dsn)
	if err != nil {
		return nil, err
	}
	// Client-side pool over the networked backend — the whole reason for the
	// move. The WAL/busy_timeout pragmas are SQLite-local and become
	// capdb-server configuration (the server owns the file + its native pool).
	//
	// IMPORTANT: keep MaxOpenConns <= the server's --pool-max (default 8). Each
	// open client connection becomes a server session that checks out a pooled
	// SQLite handle; if the client opens more connections than the server pool
	// has handles, the surplus block on the server-side acquire. `capper aio`
	// launches capdb-server with --pool-max sized to MaxOpenConns to keep them
	// aligned (see internal/cli/aio.go, CapDBPoolMax).
	maxOpen := envInt("CAPPER_DB_MAX_OPEN_CONNS", 8)
	db.SetMaxOpenConns(maxOpen)
	db.SetMaxIdleConns(envInt("CAPPER_DB_MAX_IDLE_CONNS", maxOpen))
	db.SetConnMaxLifetime(time.Duration(envInt("CAPPER_DB_CONN_MAX_LIFETIME_SECS", 300)) * time.Second)

	// Wait for the server with a bounded exponential backoff so a control plane
	// that starts just before (or during a restart of) capdb-server comes up
	// instead of crashing. The driver already bounds each connect attempt
	// (connect_timeout DSN param, default 10s).
	retries := envInt("CAPPER_DB_STARTUP_RETRIES", 30)
	var perr error
	for i := 0; i < retries; i++ {
		if perr = db.Ping(); perr == nil {
			return db, nil
		}
		backoff := time.Duration(100*(1<<min(i, 5))) * time.Millisecond // cap ~3.2s
		time.Sleep(backoff)
	}
	db.Close()
	return nil, fmt.Errorf("store: capdb server unreachable after %d attempts: %w", retries, perr)
}

func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}
