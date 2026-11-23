package charset

import (
	"math"
	"strings"
	"testing"
)

// Big5 is Big5 with HKSCS, as the Encoding standard's index is: code page
// 950 first, then the HKSCS table, then four pairs that decode to a letter
// and a combining mark.
func TestBig5HKSCS(t *testing.T) {
	for _, c := range []struct{ src, want string }{
		{"\x87\x40", "䏰"},
		{"\xFE\xFE", "秔"},
		{"\x87\x45", "\U00027267"}, // above U+FFFF, from big5-hkscs.txt
		{"\x88\x62", "Ê̄"},
		{"\x88\x64", "Ê̌"},
		{"\x88\xA3", "ê̄"},
		{"\x88\xA5", "ê̌"},
		{"\xA1\x45", "‧"}, // code page 950 wins
		{"a\x88\x62b\x87\x45c", "aÊ̄b\U00027267c"},
	} {
		src := []byte(c.src)
		out, m, invalid, cut := Decode(src, Result{Enc: Big5}, math.MaxInt64)
		if string(out) != c.want || invalid != 0 || cut {
			t.Errorf("% X: got %q (%d invalid), want %q", src, out, invalid, c.want)
			continue
		}
		if got := m.Source(len(out)); got != int64(len(src)) {
			t.Errorf("% X: Source(end) = %d, want %d", src, got, len(src))
		}
		for i := range string(out) {
			if s := m.Source(i); s < 0 || s >= int64(len(src)) {
				t.Errorf("% X: Source(%d) = %d", src, i, s)
			}
		}
	}
	// Both code points of a pair map to the pair's first byte.
	out, m, _, _ := Decode([]byte("a\x88\x62b"), Result{Enc: Big5}, math.MaxInt64)
	if i := strings.Index(string(out), "̄"); m.Source(i) != 1 || m.Source(i+2) != 3 {
		t.Errorf("pair offsets: Source(%d)=%d, Source(%d)=%d", i, m.Source(i), i+2, m.Source(i+2))
	}
	// A limit inside a pair cuts before it.
	out, _, _, cut := Decode([]byte("\x88\x62"), Result{Enc: Big5}, 3)
	if string(out) != "" || !cut {
		t.Errorf("cut inside pair: %q %v", out, cut)
	}
}
