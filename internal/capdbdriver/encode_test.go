package capdbdriver

import (
	"database/sql/driver"
	"testing"
	"time"
)

func TestEncodeLiteral(t *testing.T) {
	tm := time.Date(2026, 6, 12, 15, 4, 5, 123456789, time.UTC)
	cases := []struct {
		in   driver.Value
		want string
	}{
		{nil, "NULL"},
		{int64(42), "42"},
		{int64(-7), "-7"},
		{float64(1.5), "1.5"},
		{true, "1"},
		{false, "0"},
		{"hello", "'hello'"},
		{"it's", "'it''s'"},
		{[]byte{0xde, 0xad, 0xbe, 0xef}, "x'deadbeef'"},
		{[]byte{}, "x''"},
		{tm, "'2026-06-12 15:04:05.123456789+00:00'"},
	}
	for _, c := range cases {
		got, err := encodeLiteral(c.in)
		if err != nil {
			t.Fatalf("encodeLiteral(%v): %v", c.in, err)
		}
		if got != c.want {
			t.Errorf("encodeLiteral(%v) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestSubstitute(t *testing.T) {
	cases := []struct {
		query string
		args  []driver.Value
		want  string
	}{
		{"SELECT * FROM t WHERE id=?", []driver.Value{"x"}, "SELECT * FROM t WHERE id='x'"},
		{"INSERT INTO t VALUES(?,?,?)", []driver.Value{int64(1), "a", nil}, "INSERT INTO t VALUES(1,'a',NULL)"},
		// '?' inside a string literal must be left alone
		{"SELECT '?' , ?", []driver.Value{int64(9)}, "SELECT '?' , 9"},
		// '?' inside a quoted identifier
		{`SELECT "a?b" FROM t WHERE x=?`, []driver.Value{int64(1)}, `SELECT "a?b" FROM t WHERE x=1`},
		// '?' inside a line comment
		{"SELECT 1 -- ?\nWHERE x=?", []driver.Value{int64(2)}, "SELECT 1 -- ?\nWHERE x=2"},
		// '?' inside a block comment
		{"SELECT /* ? */ ?", []driver.Value{int64(3)}, "SELECT /* ? */ 3"},
		// escaped quote inside string
		{"SELECT 'a''?b', ?", []driver.Value{int64(4)}, "SELECT 'a''?b', 4"},
	}
	for _, c := range cases {
		got, err := substitute(c.query, c.args)
		if err != nil {
			t.Fatalf("substitute(%q): %v", c.query, err)
		}
		if got != c.want {
			t.Errorf("substitute(%q) = %q, want %q", c.query, got, c.want)
		}
	}
}

func TestRejectNULBytes(t *testing.T) {
	if _, err := encodeLiteral("ab\x00cd"); err == nil {
		t.Error("expected error for string arg with NUL byte")
	}
	if _, err := substitute("SELECT ?", []driver.Value{"x\x00y"}); err == nil {
		t.Error("expected error for NUL in string arg via substitute")
	}
	if _, err := substitute("SELECT 1 \x00", nil); err == nil {
		t.Error("expected error for NUL in query text")
	}
	// A blob with a NUL is fine (hex-encoded, no truncation).
	if _, err := encodeLiteral([]byte{0x00, 0x01}); err != nil {
		t.Errorf("blob with NUL should be allowed: %v", err)
	}
}

func TestSubstituteArgMismatch(t *testing.T) {
	if _, err := substitute("SELECT ?", nil); err == nil {
		t.Error("expected error for too few args")
	}
	if _, err := substitute("SELECT 1", []driver.Value{int64(1)}); err == nil {
		t.Error("expected error for too many args")
	}
	if _, err := substitute("SELECT ?1", []driver.Value{int64(1)}); err == nil {
		t.Error("expected error for numbered placeholder")
	}
}

func TestCountPlaceholders(t *testing.T) {
	cases := []struct {
		query string
		want  int
	}{
		{"SELECT ?", 1},
		{"INSERT INTO t VALUES(?,?,?)", 3},
		{"SELECT '?' , ?", 1},
		{"SELECT /* ?? */ ?", 1},
		{"SELECT 1", 0},
	}
	for _, c := range cases {
		if got := countPlaceholders(c.query); got != c.want {
			t.Errorf("countPlaceholders(%q) = %d, want %d", c.query, got, c.want)
		}
	}
}
