package charset

import (
	"bytes"
	"unicode/utf8"
)

// The encoding of text that declares none and is not UTF-8 is recognised
// from its bytes only when one encoding is certain. The tests are of a
// kind that the text of one language passes by a wide margin and the
// same bytes read in another encoding fail by a wide margin:
//
//   - UTF-8 with damaged bytes: its multi-byte sequences, which other
//     encodings' text almost never forms, far outnumber the bytes that
//     are not valid.
//   - UTF-16 with no byte order mark: the bytes at every other offset
//     are a handful of values (0x00 for ASCII, 0x04 for Cyrillic), and
//     the text decodes with no unpaired surrogate or control character.
//     Han text, whose characters spread over many values, is not
//     recognised.
//   - Shift_JIS: Japanese writes its grammar in kana, which no other
//     encoding's bytes give when read as Shift_JIS. Only full-width kana
//     count: Shift_JIS's single bytes 0xA1 to 0xDF are half-width
//     katakana, and they are the common lead bytes of the other code
//     pages, so Chinese or Korean read as Shift_JIS gives many of those.
//   - EUC-KR: Korean is written in Hangul syllables, with few Hanja and
//     few lone jamo; Chinese read as EUC-KR gives as many Hanja as
//     Hangul, or bytes it does not have, and the kana of EUC-JP give
//     jamo.
//   - GBK and Big5: most characters of Chinese text are among its 1,000
//     most frequent (commonHan); the bytes of one read as the other give
//     scattered rare characters.
//   - windows-1251: о е а и н т с р are about half of the letters of
//     Russian, Ukrainian, Bulgarian and Serbian, and there are capitals;
//     Greek or KOI8-R read the same way give a quarter or less, and
//     Hebrew, nearer, has a third and no bytes where the capitals are
//     (but its punctuation at 0xD4 to 0xD8).
//
// A double-byte code page is allowed one byte sequence it does not have
// in a hundred characters (a stray byte); another encoding's text read
// as it gives a great many, or none.
//
// Text whose bytes above 0x7F stand alone, most of them beside an ASCII
// letter or a space, as the accented letters of the languages of western
// Europe do (è, the word, between two spaces), is
// windows-1252, the Encoding standard's fallback, unless something in
// it says it is another code page of the Latin script, whose letters
// windows-1252 shows as other letters or as symbols:
//
//   - a byte windows-1252 leaves undefined (ť of windows-1250 is 0x9D);
//   - a symbol between two ASCII letters (ł ą ż ș of windows-1250 are
//     ³ ¹ ¿ º);
//   - the Icelandic letters ð þ ý Ð Þ Ý, one in twenty bytes above 0x7F
//     or more: they are ğ ş ı Ğ Ş İ of Turkish, ý of Czech and Slovak, ţ
//     of Romanian, and đ ư of Vietnamese (Icelandic itself is then not
//     decided; a word or two transliterated, hýdor, is let pass);
//   - õ beside ö or ü, each one in thirty or more, or õ with neither ã
//     nor ç: Hungarian's ő (Portuguese, the language of windows-1252 that
//     writes õ, writes ã and ç far more; Estonian is then not decided);
//   - two letters no language of windows-1252 writes in one text, and
//     that the letters of windows-1250 give together: ø beside ì ù ò ï
//     or ý (Czech ř beside ě ů ň ď ý), õ or ö beside û (Hungarian ő ö ű), ã
//     beside î (Romanian ă î), ñ beside ê or æ (Polish ń ę ć), ý beside ä
//     or ô (Slovak);
//   - two or more valid UTF-8 sequences: UTF-8 too damaged to be called
//     that, or text of two encodings.
//
// Anything else is not decided: read as windows-1252 it would be
// garbage. A zero byte is not in text either.

// detectSample is how much of the input the tests read: from just before
// its first byte above 0x7F.
const detectSample = 32 << 10

// detect decides the encoding of b, which declares none and is not
// valid UTF-8, or has zero bytes (see unmarkedUTF16).
func detect(b []byte) Result {
	if enc, ok := utf16Unmarked(b[:min(len(b), detectSample)]); ok {
		return Result{Enc: enc, From: FromDetected, Guessed: true}
	}
	s := sampleOf(b)
	multi, invalid := utf8Counts(s)
	if multi >= 4 && invalid*4 <= multi {
		// UTF-8 with some damaged bytes: at least four multi-byte
		// characters, and at most one byte that is not valid for every
		// four of them.
		return Result{Enc: UTF8, From: FromDetected, Guessed: true}
	}
	st := scanBytes(s)
	unknown := Result{Enc: Windows1252, From: FromDetected, Unknown: true}
	if st.high == 0 || st.zero > 0 {
		return unknown
	}
	if st.inRuns*2 < st.high && st.byLetter*2 >= st.high {
		// Bytes above 0x7F mostly alone: a language written in Latin
		// letters.
		icelandic := st.count[0xF0-0x80] + st.count[0xFE-0x80] + st.count[0xFD-0x80] +
			st.count[0xD0-0x80] + st.count[0xDE-0x80] + st.count[0xDD-0x80]
		otilde := st.count[0xF5-0x80] + st.count[0xD5-0x80]
		umlaut := st.count[0xF6-0x80] + st.count[0xFC-0x80] + st.count[0xD6-0x80] + st.count[0xDC-0x80]
		if st.undefined1252 > 0 || st.symbolInWord > 0 || icelandic >= 2 && icelandic*20 >= st.high ||
			otilde > 0 && umlaut > 0 && otilde*30 >= st.high && umlaut*30 >= st.high ||
			otilde > 0 && st.count[0xE3-0x80]+st.count[0xC3-0x80]+st.count[0xE7-0x80]+st.count[0xC7-0x80] == 0 ||
			foreignPair(&st) || multi >= 2 {
			return unknown
		}
		return Result{Enc: Windows1252, From: FromDetected, Guessed: true}
	}
	var found []Encoding
	for _, enc := range []Encoding{ShiftJIS, EUCKR, GBK, Big5} {
		if dbcsLikely(s, enc) {
			found = append(found, enc)
		}
	}
	if cyrillicLikely(st) {
		found = append(found, Windows1251)
	}
	if len(found) == 1 {
		return Result{Enc: found[0], From: FromDetected, Guessed: true}
	}
	return unknown
}

// sampleOf returns up to detectSample bytes of b, starting at a byte that
// begins a character in every encoding tested: after the last byte below
// 0x40 (no trail byte of a double-byte code page is) before the first
// byte above 0x7F. A character cut at the end is left to the tests.
func sampleOf(b []byte) []byte {
	first := 0
	for first < len(b) && b[first] < 0x80 {
		first++
	}
	start := first
	for start > 0 && first-start < 1024 && b[start-1] >= 0x40 {
		start--
	}
	if start > 0 && b[start-1] >= 0x40 {
		start = first
	}
	return b[start:min(len(b), start+detectSample)]
}

// byteStats is what one pass over the sample counts.
type byteStats struct {
	high          int      // bytes above 0x7F
	inRuns        int      // of them, those in a run of three or more
	zero          int      // zero bytes
	undefined1252 int      // 0x81, 0x8D, 0x8F, 0x90, 0x9D
	symbolInWord  int      // a byte that is not a letter in windows-1252, between ASCII letters
	byLetter      int      // a byte beside an ASCII letter or a space
	count         [128]int // of each byte above 0x7F
}

func scanBytes(s []byte) byteStats {
	var st byteStats
	run := 0
	for i, c := range s {
		if c < 0x80 {
			if c == 0 {
				st.zero++
			}
			if run >= 3 {
				st.inRuns += run
			}
			run = 0
			continue
		}
		st.high++
		st.count[c-0x80]++
		run++
		switch c {
		case 0x81, 0x8D, 0x8F, 0x90, 0x9D:
			st.undefined1252++
		}
		before := i > 0 && asciiLetter(s[i-1])
		after := i+1 < len(s) && asciiLetter(s[i+1])
		if before || after || i > 0 && s[i-1] == ' ' || i+1 < len(s) && s[i+1] == ' ' {
			st.byLetter++
		}
		if before && after && !letter1252(c) {
			st.symbolInWord++
		}
	}
	if run >= 3 {
		st.inRuns += run
	}
	return st
}

// foreignPairs are pairs of windows-1252 letters, lower case, that no
// language written in windows-1252 has in one text, and that the text of
// a language of windows-1250 read as windows-1252 gives.
var foreignPairs = [][2]byte{
	{0xF8, 0xEC}, {0xF8, 0xF9}, {0xF8, 0xF2}, {0xF8, 0xEF}, {0xF8, 0xFD}, // ø: ì ù ò ï ý
	{0xF5, 0xFB}, {0xFB, 0xF6}, // õ û, û ö
	{0xE3, 0xEE},               // ã î
	{0xF1, 0xEA}, {0xF1, 0xE6}, // ñ: ê æ
	{0xFD, 0xE4}, {0xFD, 0xF4}, // ý: ä ô
}

// foreignPair reports whether both letters of a foreign pair, in either
// case, are in the text.
func foreignPair(st *byteStats) bool {
	has := func(c byte) bool { return st.count[c-0x80]+st.count[c-0x20-0x80] > 0 }
	for _, p := range foreignPairs {
		if has(p[0]) && has(p[1]) {
			return true
		}
	}
	return false
}

func asciiLetter(c byte) bool { return c|0x20 >= 'a' && c|0x20 <= 'z' }

// letter1252 reports whether byte c above 0x7F is a letter in
// windows-1252, or a character found inside words: an apostrophe, a soft
// hyphen, a middle dot (Catalan l·l), a dash, × and ÷ (2×x).
func letter1252(c byte) bool {
	switch c {
	case 0x8A, 0x8C, 0x8E, 0x9A, 0x9C, 0x9E, 0x9F, 0x83, 0xB5,
		0x92, 0x91, 0xAD, 0xB7, 0x96, 0x97:
		return true
	}
	return c >= 0xC0
}

// unmarkedUTF16 reports whether b, which declares no encoding, is to be
// tested for UTF-16 before UTF-8: its first detectSample bytes hold a
// zero byte, which no text in UTF-8 has. UTF-16 of ASCII or Cyrillic
// has every byte below 0x80, and so is valid UTF-8.
func unmarkedUTF16(b []byte) bool {
	return bytes.IndexByte(b[:min(len(b), detectSample)], 0) >= 0
}

// utf16Unmarked recognises UTF-16 without a byte order mark in s, which
// starts at the start of the input: the bytes at the offsets of one
// parity, the high bytes of the code units, are nearly all one of four
// values, those at the other are not, and the text decodes with no
// unpaired surrogate and no control character but tab and line ends.
func utf16Unmarked(s []byte) (Encoding, bool) {
	s = s[:len(s)&^1]
	if len(s) < 32 {
		return 0, false
	}
	var hist [2][256]int
	for i, c := range s {
		hist[i&1][c]++
	}
	few := func(h *[256]int) bool {
		var top [4]int
		for _, n := range h {
			for k := range top {
				if n > top[k] {
					copy(top[k+1:], top[k:3])
					top[k] = n
					break
				}
			}
		}
		return (top[0]+top[1]+top[2]+top[3])*10 >= len(s)/2*9
	}
	var enc Encoding
	switch even, odd := few(&hist[0]), few(&hist[1]); {
	case odd && !even:
		enc = UTF16LE
	case even && !odd:
		enc = UTF16BE
	default:
		return 0, false
	}
	for i := 0; i < len(s); {
		r, n := utf16At(s, i, enc == UTF16BE)
		if r == utf8.RuneError && i+n+2 <= len(s) || r < 0x20 && r != '\t' && r != '\n' && r != '\r' || r == 0xFFFE || r == 0xFFFF {
			return 0, false
		}
		i += n
	}
	return enc, true
}

// utf8Counts counts the valid multi-byte UTF-8 sequences in s and the
// bytes that begin none, but for a sequence cut by the end of s.
func utf8Counts(s []byte) (multi, bad int) {
	for i := 0; i < len(s); {
		if s[i] < 0x80 {
			i++
			continue
		}
		r, n := utf8.DecodeRune(s[i:])
		switch {
		case r == utf8.RuneError && n <= 1:
			if len(s)-i >= utf8.UTFMax || utf8.FullRune(s[i:]) {
				bad++
			}
		default:
			multi++
		}
		i += max(n, 1)
	}
	return multi, bad
}

// dbcsChars is how many characters past ASCII dbcsLikely reads: enough
// to judge any text, and a bound on its time.
const dbcsChars = 4096

// dbcsLikely reports whether s reads as text of the language enc is made
// for: at most one byte sequence it does not have in a hundred
// characters (more than four in the first 400 end the test), enough
// characters to judge, and the share of them the language's text has.
func dbcsLikely(s []byte, enc Encoding) bool {
	var chars, bad, kana, jamo, hangul, hangulExt, han, common int
	for i := 0; i < len(s); {
		if s[i] < 0x80 {
			i++
			continue
		}
		if i+1 == len(s) && isLead(enc.codePage(), s[i]) {
			break // a character cut by the end of the sample
		}
		r, n, invalid := decodeAt(s, i, enc)
		if invalid {
			bad++
			if bad*100 > max(chars, 400) {
				return false
			}
		}
		chars++
		if chars > dbcsChars {
			break
		}
		switch {
		case r >= 0x3040 && r <= 0x30FF:
			kana++
		case r >= 0x3131 && r <= 0x318E:
			jamo++
		case r >= 0xAC00 && r <= 0xD7A3:
			hangul++
			if n == 2 && (s[i] < 0xB0 || s[i+1] < 0xA1) {
				hangulExt++ // not one of KS X 1001's 2,350
			}
		case r >= 0x4E00 && r < 0xA000:
			han++
			if commonHan[(r-0x4E00)/32]&(1<<((r-0x4E00)%32)) != 0 {
				common++
			}
		}
		i += n
	}
	if chars < 16 || bad*100 > chars {
		return false
	}
	switch enc {
	case ShiftJIS:
		return kana*5 >= chars
	case EUCKR:
		return hangul*10 >= chars*6 && han*10 <= chars && hangulExt*20 <= hangul && jamo*20 <= chars
	default: // GBK, Big5
		return kana*20 < chars && hangul == 0 && han*10 >= chars*6 && common*2 >= han
	}
}

// cyrillicLikely reports whether the bytes above 0x7F read as a language
// of the Cyrillic script in windows-1251: most are letters (0xC0 to
// 0xFF), о е а и н т с р (0xEE 0xE5 0xE0 0xE8 0xED 0xF2 0xF1 0xF0) are
// 45 in a hundred of them or more, and there are capitals (0xC0 to 0xDF,
// not counting 0xD4 to 0xD8).
func cyrillicLikely(st byteStats) bool {
	letters, top, capitals := 0, 0, 0
	for c := 0xC0; c <= 0xFF; c++ {
		letters += st.count[c-0x80]
	}
	for _, c := range []byte{0xEE, 0xE5, 0xE0, 0xE8, 0xED, 0xF2, 0xF1, 0xF0} {
		top += st.count[c-0x80]
	}
	for c := 0xC0; c <= 0xDF; c++ {
		if c < 0xD4 || c > 0xD8 {
			capitals += st.count[c-0x80]
		}
	}
	return letters >= 40 && letters*10 >= st.high*7 && top*100 >= letters*45 && capitals > 0
}
