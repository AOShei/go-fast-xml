package entity

import (
	"strings"
	"testing"
)

func TestTable(t *testing.T) {
	// The standard has 2231 entries: 2125 with ";" and 106 legacy. The
	// table was compared entry by entry with the WHATWG table (Python's
	// html.entities.html5).
	if n := Len(); n != 2125+106 {
		t.Errorf("table has %d entries, want 2231", n)
	}
	legacy := 0
	for k, v := range names {
		if v == "" {
			t.Errorf("%q has no replacement", k)
		}
		if !strings.HasSuffix(k, ";") {
			legacy++
		}
		if len(strings.TrimSuffix(k, ";")) > MaxName {
			t.Errorf("%q is longer than MaxName", k)
		}
	}
	if legacy != 106 {
		t.Errorf("%d legacy names, want 106", legacy)
	}
}

func TestNamed(t *testing.T) {
	// Expected replacements are written as UTF-8 byte escapes.
	for _, c := range []struct {
		in, repl string
		n        int
		semi, ok bool
	}{
		{"amp;", "&", 4, true, true},
		{"amp", "&", 3, false, true},
		{"ampx", "&", 3, false, true},
		{"notin;", "\xE2\x88\x89", 6, true, true},
		{"notit;", "\xC2\xAC", 3, false, true},
		{"nbsp", "\xC2\xA0", 4, false, true},
		{"CounterClockwiseContourIntegral;", "\xE2\x88\xB3", 32, true, true},
		{"NotEqualTilde;", "\xE2\x89\x82\xCC\xB8", 14, true, true},
		{"fjlig;", "fj", 6, true, true},
		{"xyz;", "", 0, false, false},
		{"alpha", "", 0, false, false},
		{"", "", 0, false, false},
	} {
		repl, n, semi, ok := Named([]byte(c.in))
		if repl != c.repl || n != c.n || semi != c.semi || ok != c.ok {
			t.Errorf("Named(%q) = %q, %d, %v, %v; want %q, %d, %v, %v", c.in, repl, n, semi, ok, c.repl, c.n, c.semi, c.ok)
		}
	}
}

func TestNumeric(t *testing.T) {
	for _, c := range []struct {
		v    int64
		want rune
	}{
		{0, 0xFFFD}, {0x41, 'A'}, {0x80, 0x20AC}, {0x81, 0x81}, {0x9F, 0x178},
		{0xD800, 0xFFFD}, {0x110000, 0xFFFD}, {0x1F600, 0x1F600},
	} {
		if got := Numeric(c.v); got != c.want {
			t.Errorf("Numeric(%#x) = %U, want %U", c.v, got, c.want)
		}
	}
}
