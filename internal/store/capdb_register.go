//go:build capdb

package store

// Building with `-tags capdb` links the CapDB cgo driver and registers it as
// the database/sql driver name "capdb", making CAPPER_DB_DRIVER=capdb usable.
// Without the tag the default build stays pure-Go (modernc SQLite only).
import _ "capper/internal/capdbdriver"
