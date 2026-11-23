package generic

import (
	"strings"
	"testing"
)

// Characters written by their numbers, to keep them visible in the source.
var (
	nbsp = string(rune(0xA0))
	nel  = string(rune(0x85))
	c1   = string(rune(0x90))
	ffi  = string(rune(0xFB03))
	repl = string(rune(0xFFFD))
	eAc  = string(rune(0xE9))
)

func TestBuilder(t *testing.T) {
	for _, c := range []struct {
		in       []string
		preserve bool
		want     string
		lead     bool
		trail    bool
	}{
		{[]string{"  a \n\t b  ", " c "}, false, "a b c", true, true},
		{[]string{nbsp + "x" + nbsp + " y" + nbsp}, false, "x" + nbsp + " y", true, true},
		{[]string{"\n  line 1\n\tline 2  \n"}, true, "line 1\n\tline 2", true, true},
		{[]string{"a\x00b\x1bc" + nel + "d" + c1 + "e"}, false, "abc de", false, false},
		{[]string{ffi + "x", "\xFFy"}, false, "ffix" + repl + "y", false, false},
	} {
		var b Builder
		for _, s := range c.in {
			b.Append([]byte(s), c.preserve)
		}
		if got := b.String(); got != c.want || b.Lead() != c.lead || b.Trail() != c.trail {
			t.Errorf("%q: %q lead %v trail %v", c.in, got, b.Lead(), b.Trail())
		}
	}
}

func TestSplit(t *testing.T) {
	var b Builder
	b.Append([]byte(strings.Repeat("word ", 100)), false)
	piece := b.Split(42)
	if len(piece) > 42 || strings.HasSuffix(piece, " ") || strings.HasPrefix(b.String(), " ") {
		t.Errorf("piece %q, rest %q", piece, b.String()[:10])
	}
	if got := piece + " " + b.String(); got != strings.TrimSpace(strings.Repeat("word ", 100)) {
		t.Errorf("pieces do not join back")
	}
	b.Reset()
	b.Append([]byte(strings.Repeat(eAc, 50)), false)
	if piece := b.Split(41); len(piece) != 40 {
		t.Errorf("cut inside a character: %d", len(piece))
	}
}
