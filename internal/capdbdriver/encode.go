// Package capdbdriver implements a database/sql driver for CapDB, the
// connection-pooled, network-capable SQLite fork vendored in capdb/.
//
// The networked CapDB client (capdb_net_*) speaks the SQLite SQL dialect but
// does NOT expose server-side parameter binding: its PREPARE/STEP protocol
// messages carry no bound values. A database/sql driver, however, must accept
// positional arguments. This file bridges that gap by substituting `?`
// placeholders with safely-encoded SQL literals on the client before the
// statement is sent to the server.
//
// This file deliberately carries no cgo and no build tag, so the encoding logic
// is unit-testable on any platform without a C toolchain. The cgo driver itself
// lives in driver.go behind the `capdb` build tag.
package capdbdriver

import (
	"database/sql/driver"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// timeLayout matches how modernc.org/sqlite renders time.Time, so values
// round-trip identically across the two backends.
const timeLayout = "2006-01-02 15:04:05.999999999-07:00"

// substitute replaces each positional `?` placeholder in query with the
// SQL-literal encoding of the corresponding arg. Placeholders inside string
// literals ('...'), quoted identifiers ("..." and [...]), and comments
// (-- ... and /* ... */) are left untouched. It returns an error if the number
// of placeholders does not match len(args).
func substitute(query string, args []driver.Value) (string, error) {
	if strings.IndexByte(query, 0) >= 0 {
		return "", fmt.Errorf("capdb: query contains a NUL byte (not supported)")
	}
	var b strings.Builder
	b.Grow(len(query) + 16*len(args))

	argi := 0
	i := 0
	n := len(query)
	for i < n {
		c := query[i]
		switch {
		case c == '\'':
			// single-quoted string literal: copy through, honoring '' escapes
			j := i + 1
			for j < n {
				if query[j] == '\'' {
					if j+1 < n && query[j+1] == '\'' {
						j += 2
						continue
					}
					j++
					break
				}
				j++
			}
			b.WriteString(query[i:j])
			i = j
		case c == '"':
			j := i + 1
			for j < n && query[j] != '"' {
				j++
			}
			if j < n {
				j++ // include closing quote
			}
			b.WriteString(query[i:j])
			i = j
		case c == '[':
			j := i + 1
			for j < n && query[j] != ']' {
				j++
			}
			if j < n {
				j++
			}
			b.WriteString(query[i:j])
			i = j
		case c == '-' && i+1 < n && query[i+1] == '-':
			j := i + 2
			for j < n && query[j] != '\n' {
				j++
			}
			b.WriteString(query[i:j])
			i = j
		case c == '/' && i+1 < n && query[i+1] == '*':
			j := i + 2
			for j+1 < n && !(query[j] == '*' && query[j+1] == '/') {
				j++
			}
			if j+1 < n {
				j += 2
			} else {
				j = n
			}
			b.WriteString(query[i:j])
			i = j
		case c == '?':
			// CapDB/SQLite also accept ?NNN / :name / @name / $name, but Capper
			// uses only plain positional `?`; reject the numbered form so a
			// silent mis-bind can't happen.
			if i+1 < n && query[i+1] >= '0' && query[i+1] <= '9' {
				return "", fmt.Errorf("capdb: numbered placeholders (?NNN) are not supported; use positional ?")
			}
			if argi >= len(args) {
				return "", fmt.Errorf("capdb: not enough arguments for placeholders (have %d)", len(args))
			}
			lit, err := encodeLiteral(args[argi])
			if err != nil {
				return "", err
			}
			b.WriteString(lit)
			argi++
			i++
		default:
			b.WriteByte(c)
			i++
		}
	}
	if argi != len(args) {
		return "", fmt.Errorf("capdb: %d arguments provided but %d placeholders consumed", len(args), argi)
	}
	return b.String(), nil
}

// encodeLiteral renders a single driver.Value as a SQLite SQL literal.
// driver.Value is one of: int64, float64, bool, []byte, string, time.Time, nil.
func encodeLiteral(v driver.Value) (string, error) {
	switch t := v.(type) {
	case nil:
		return "NULL", nil
	case int64:
		return strconv.FormatInt(t, 10), nil
	case float64:
		return strconv.FormatFloat(t, 'g', -1, 64), nil
	case bool:
		if t {
			return "1", nil
		}
		return "0", nil
	case []byte:
		return encodeBlob(t), nil
	case string:
		if strings.IndexByte(t, 0) >= 0 {
			return "", fmt.Errorf("capdb: string argument contains a NUL byte (not supported)")
		}
		return encodeString(t), nil
	case time.Time:
		return encodeString(t.Format(timeLayout)), nil
	default:
		return "", fmt.Errorf("capdb: unsupported argument type %T", v)
	}
}

// encodeString wraps s in single quotes, doubling embedded quotes. Callers must
// reject NUL bytes first (see encodeLiteral): the wire layer length-prefixes
// strings using strlen(), so an embedded NUL would silently truncate the
// statement on the server.
func encodeString(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

// encodeBlob renders b as an SQLite blob literal: x'deadbeef'.
func encodeBlob(b []byte) string {
	const hexdigits = "0123456789abcdef"
	var sb strings.Builder
	sb.Grow(len(b)*2 + 3)
	sb.WriteString("x'")
	for _, c := range b {
		sb.WriteByte(hexdigits[c>>4])
		sb.WriteByte(hexdigits[c&0x0f])
	}
	sb.WriteByte('\'')
	return sb.String()
}

// countPlaceholders counts positional `?` placeholders outside of literals and
// comments. Used by Stmt.NumInput.
func countPlaceholders(query string) int {
	count := 0
	i := 0
	n := len(query)
	for i < n {
		c := query[i]
		switch {
		case c == '\'':
			j := i + 1
			for j < n {
				if query[j] == '\'' {
					if j+1 < n && query[j+1] == '\'' {
						j += 2
						continue
					}
					j++
					break
				}
				j++
			}
			i = j
		case c == '"':
			j := i + 1
			for j < n && query[j] != '"' {
				j++
			}
			i = j + 1
		case c == '[':
			j := i + 1
			for j < n && query[j] != ']' {
				j++
			}
			i = j + 1
		case c == '-' && i+1 < n && query[i+1] == '-':
			j := i + 2
			for j < n && query[j] != '\n' {
				j++
			}
			i = j
		case c == '/' && i+1 < n && query[i+1] == '*':
			j := i + 2
			for j+1 < n && !(query[j] == '*' && query[j+1] == '/') {
				j++
			}
			i = j + 2
		case c == '?':
			if i+1 < n && query[i+1] >= '0' && query[i+1] <= '9' {
				// numbered placeholder; not counted as positional
				i++
				continue
			}
			count++
			i++
		default:
			i++
		}
	}
	return count
}
