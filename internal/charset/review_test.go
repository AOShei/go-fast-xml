package charset

import (
	"math"
	"testing"
)

// r builds a string of code points, so that this file holds no Private
// Use Area or look-alike characters, literal or escaped.
func r(cps ...rune) string { return string(cps) }

// The WHATWG Shift_JIS decoder reads 0x80 as U+0080 and the user-defined
// area (lead bytes 0xF0 to 0xF9, pointers 8836 to 10715) as U+E000 to
// U+E757, which Microsoft's code page 932 also does. Without them a
// user-defined character with an ASCII trail byte became U+FFFD followed
// by a letter the author did not write.
func TestShiftJISUserDefined(t *testing.T) {
	for _, c := range []struct{ src, want string }{
		{"\x80", r(0x80)},
		{"\xF0\x40", r(0xE000)},
		{"\xF0\x7E\xF0\x80", r(0xE03E, 0xE03F)},
		{"\xF9\xFC", r(0xE757)},
		{"\xF0\x7F", r(0xFFFD, 0x7F)},
	} {
		out, _, _, _ := Decode([]byte(c.src), Result{Enc: ShiftJIS}, math.MaxInt64)
		if string(out) != c.want {
			t.Errorf("%q: got %+q, want %+q", c.src, out, c.want)
		}
	}
}

// The WHATWG gbk decoder (the gb18030 decoder) reads GBK's three
// user-defined areas as the Private Use Area, as GB 18030 maps them:
// 0xAAA1-0xAFFE to U+E000-U+E233, 0xF8A1-0xFEFE to U+E234-U+E4C5, and
// 0xA140-0xA7A0 to U+E4C6-U+E765. Code page 936's table leaves them out.
func TestGBKUserDefined(t *testing.T) {
	for _, c := range []struct{ src, want string }{
		{"\xAA\xA1", r(0xE000)},
		{"\xAA\xFE\xAB\xA1", r(0xE05D, 0xE05E)},
		{"\xAF\xFE", r(0xE233)},
		{"\xF8\xA1", r(0xE234)},
		{"\xFE\xFE", r(0xE4C5)},
		{"\xA1\x40", r(0xE4C6)},
		{"\xA1\x7E\xA1\x80", r(0xE504, 0xE505)},
		{"\xA1\xA0\xA2\x40", r(0xE525, 0xE526)},
		{"\xA7\xA0", r(0xE765)},
		{"\xA1\x7F", r(0xFFFD, 0x7F)},
		{"\xA1\xA1", r(0x3000)}, // GB 2312 proper is unchanged
	} {
		out, _, _, _ := Decode([]byte(c.src), Result{Enc: GBK}, math.MaxInt64)
		if string(out) != c.want {
			t.Errorf("%q: got %+q, want %+q", c.src, out, c.want)
		}
	}
}

// Labels match ASCII case-insensitively only, as the Encoding standard
// says: KELVIN SIGN (U+212A) and LATIN CAPITAL LETTER I WITH DOT ABOVE
// (U+0130) are not the letters k and i, though Unicode lowercases them so.
func TestLabelASCIICase(t *testing.T) {
	for _, l := range []string{r(0x212A) + "orean", "sh" + r(0x130) + "ft_jis", "B" + r(0x130) + "G5"} {
		if e, ok := Lookup(l); ok {
			t.Errorf("Lookup(%+q) = %s", l, e)
		}
		if res := Sniff([]byte("<p>x"), l); res.From == FromTransport {
			t.Errorf("Sniff(transport %+q) = %+v", l, res)
		}
		meta := []byte(`<meta charset="` + l + `"><p>x`)
		if res := Sniff(meta, ""); !res.Unsupported {
			t.Errorf("Sniff(meta %+q) = %+v", l, res)
		}
	}
	if e, ok := Lookup(" SHIFT_JIS\t"); !ok || e != ShiftJIS {
		t.Errorf("Lookup(SHIFT_JIS) = %s, %v", e, ok)
	}
	if res := Sniff([]byte("<p>x"), ` "Big5" `); res.Enc != Big5 {
		t.Errorf("Sniff(transport Big5) = %+v", res)
	}
}
