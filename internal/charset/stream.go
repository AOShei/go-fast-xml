// Written for go-fast-xml; not part of go-fast-html's copy.

package charset

import "unicode/utf8"

// MaxChar is the most bytes one character takes in any encoding decoded
// here (a GB18030 four-byte sequence, a UTF-16 surrogate pair).
const MaxChar = 4

// DecodeChunk decodes src in enc to UTF-8, appending to dst, as Decode
// does but for one piece of a stream. Unless final, it stops before the
// last MaxChar-1 bytes, which may be the start of a character the next
// piece completes. It returns the extended dst, the source bytes used and
// the number of byte sequences that were not a character and became
// U+FFFD.
func DecodeChunk(dst, src []byte, enc Encoding, final bool) (out []byte, used, invalid int) {
	return decodeChunk(dst, src, enc, final)
}

// Step returns how many bytes of UTF-8 DecodeChunk makes of the character
// at src[s:] and how many source bytes it takes. It must agree with
// DecodeChunk.
func Step(src []byte, s int, enc Encoding) (dec, n int) {
	if b := src[s]; b < 0x80 && enc != UTF16LE && enc != UTF16BE {
		return 1, 1
	}
	ch, n, _ := decodeAt(src, s, enc)
	dec = runeLen(ch)
	if mark := markAt(src, s, enc); mark != 0 {
		dec += runeLen(mark)
	}
	return dec, n
}

// runeLen is the length utf8.AppendRune gives r: U+FFFD's for a rune it
// cannot encode.
func runeLen(r rune) int {
	if n := utf8.RuneLen(r); n > 0 {
		return n
	}
	return 3
}

func decodeChunk(dst, src []byte, enc Encoding, final bool) (out []byte, used, invalid int) {
	stop := len(src)
	if !final {
		stop -= MaxChar - 1
	}
	s := 0
	for s < stop {
		if b := src[s]; b < 0x80 && enc != UTF16LE && enc != UTF16BE {
			// ASCII is itself in every byte-oriented encoding here.
			dst = append(dst, b)
			s++
			continue
		}
		ch, n, bad := decodeAt(src, s, enc)
		if bad {
			invalid++
		}
		dst = utf8.AppendRune(dst, ch)
		if mark := markAt(src, s, enc); mark != 0 {
			dst = utf8.AppendRune(dst, mark)
		}
		s += n
	}
	return dst, s, invalid
}
