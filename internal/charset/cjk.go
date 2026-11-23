// GB18030 and EUC-JP, decoded a character at a time as the WHATWG
// Encoding Standard's decoders do. The tables gb18030-2.txt,
// gb18030-4.txt, jis0208.txt and jis0212.txt and gen/gen_cjk_extra.py are
// go-fast-email's (internal/charset/tables at v0.2.0; see NOTICE); the
// decoding follows go-fast-email's cjk.go, a character at a time so that
// the map back to the source works as for the other encodings.
// ISO-2022-JP, which keeps a state from one escape sequence to the next,
// is not decoded here.

package charset

import (
	_ "embed"
	"sort"
	"strconv"
	"strings"
	"sync"
	"unicode/utf8"
)

var (
	//go:embed gb18030-2.txt
	gb18030Text string
	//go:embed gb18030-4.txt
	gb18030RangesText string
	//go:embed jis0208.txt
	jis0208Text string
	//go:embed jis0212.txt
	jis0212Text string

	tableGB18030 = lazyTable{text: &gb18030Text}
	tableJIS0208 = lazyTable{text: &jis0208Text}
	tableJIS0212 = lazyTable{text: &jis0212Text}

	gb4Once   sync.Once
	gb4Ranges []gbRange
)

// gbRange is a run of GB18030 four-byte pointers whose code points rise
// with them from cp.
type gbRange struct {
	ptr uint32
	cp  rune
}

func gb18030Ranges() []gbRange {
	gb4Once.Do(func() {
		for line := range strings.SplitSeq(gb18030RangesText, "\n") {
			if line == "" || line[0] == '#' {
				continue
			}
			key, val, ok := strings.Cut(line, " ")
			if !ok {
				continue
			}
			p, err1 := strconv.ParseUint(key, 16, 32)
			v, err2 := strconv.ParseUint(strings.TrimSpace(val), 16, 32)
			if err1 != nil || err2 != nil || v > 0xFFFF {
				continue
			}
			gb4Ranges = append(gb4Ranges, gbRange{uint32(p), rune(v)})
		}
		sort.Slice(gb4Ranges, func(i, j int) bool { return gb4Ranges[i].ptr < gb4Ranges[j].ptr })
	})
	return gb4Ranges
}

// gbFourByte returns the code point of four-byte pointer p, or 0, as the
// Encoding Standard's "index gb18030 ranges code point".
func gbFourByte(p uint32) rune {
	switch {
	case p > 39419 && p < 189000 || p > 1237575:
		return 0
	case p >= 189000:
		return 0x10000 + rune(p-189000)
	case p == 7457:
		return 0xE7C7
	}
	r := gb18030Ranges()
	i := sort.Search(len(r), func(i int) bool { return r[i].ptr > p }) - 1
	if i < 0 {
		return 0
	}
	return r[i].cp + rune(p-r[i].ptr)
}

func isDigit(c byte) bool { return c >= 0x30 && c <= 0x39 }

// gb18030At decodes the character of GB18030 at src[s:], whose first byte
// is not ASCII: one byte (0x80, the euro sign), two, or four. Where a
// sequence is not valid, U+FFFD takes the first byte, and the bytes after
// it are read again, as the standard's decoder does.
func gb18030At(src []byte, s int) (rune, int, bool) {
	c := src[s]
	switch {
	case c == 0x80:
		return 0x20AC, 1, false
	case c == 0xFF, s+1 >= len(src):
		return utf8.RuneError, 1, true
	}
	c2 := src[s+1]
	if !isDigit(c2) {
		if c == 0xA8 && c2 == 0xBC {
			return 0x1E3F, 2, false
		}
		if c2 != 0x7F && c2 != 0xFF {
			if r := tableGB18030.get().pair(c, c2); r != 0 {
				return r, 2, false
			}
		}
		if c2 < 0x80 {
			return utf8.RuneError, 1, true // the ASCII byte is read again
		}
		return utf8.RuneError, 2, true
	}
	if s+3 >= len(src) {
		return utf8.RuneError, 1, true
	}
	c3, c4 := src[s+2], src[s+3]
	if c3 < 0x81 || c3 > 0xFE || !isDigit(c4) {
		return utf8.RuneError, 1, true
	}
	p := uint32(c-0x81)*12600 + uint32(c2-0x30)*1260 + uint32(c3-0x81)*10 + uint32(c4-0x30)
	if r := gbFourByte(p); r != 0 {
		return r, 4, false
	}
	return utf8.RuneError, 4, true
}

// eucJPAt decodes the character of EUC-JP at src[s:], whose first byte is
// not ASCII: a half-width katakana (0x8E and a byte), a character of JIS
// X 0212 (0x8F and two bytes) or of JIS X 0208 (two bytes). Where a
// sequence is not valid, U+FFFD takes the first byte, and an ASCII byte
// after it is read again.
func eucJPAt(src []byte, s int) (rune, int, bool) {
	c := src[s]
	if c != 0x8E && c != 0x8F && (c < 0xA1 || c == 0xFF) || s+1 >= len(src) {
		return utf8.RuneError, 1, true
	}
	c2 := src[s+1]
	switch {
	case c == 0x8E && c2 >= 0xA1 && c2 <= 0xDF:
		return 0xFF61 + rune(c2-0xA1), 2, false
	case c == 0x8F && c2 >= 0xA1 && c2 <= 0xFE:
		if s+2 >= len(src) {
			return utf8.RuneError, 1, true
		}
		c3 := src[s+2]
		if c3 >= 0xA1 && c3 <= 0xFE {
			if r := tableJIS0212.get().pair(c2, c3); r != 0 {
				return r, 3, false
			}
		}
		if c3 < 0x80 {
			return utf8.RuneError, 2, true // the ASCII byte is read again
		}
		return utf8.RuneError, 3, true
	case c >= 0xA1 && c <= 0xFE && c2 >= 0xA1 && c2 <= 0xFE:
		if r := tableJIS0208.get().pair(c, c2); r != 0 {
			return r, 2, false
		}
		return utf8.RuneError, 2, true
	}
	if c2 < 0x80 {
		return utf8.RuneError, 1, true
	}
	return utf8.RuneError, 2, true
}
