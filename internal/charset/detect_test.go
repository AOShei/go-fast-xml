package charset

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// detectWant is what each file of testdata/detect (gen/gen_detect.py) is
// recognised as, by its codec; a codec not listed must not be decided,
// unless the encoding chosen gives the same text (a short Czech text
// whose letters windows-1252 has at the same bytes).
var detectWant = map[string]Encoding{
	"cp932": ShiftJIS, "gbk": GBK, "cp950": Big5, "cp949": EUCKR, "cp1251": Windows1251,
	"cp1252": Windows1252, "utf_16_le": UTF16LE, "utf_16_be": UTF16BE, "utf8bad": UTF8,
}

// decodable are the codecs of testdata/detect this package decodes, to
// compare the text when one is not decided.
var decodable = map[string]Encoding{"cp1250": Windows1250, "cp1253": Windows1253, "cp1254": Windows1254,
	"cp1255": Windows1255, "cp1258": Windows1258, "gb18030": GB18030, "euc_jp": EUCJP, "koi8_r": KOI8R,
	"koi8_u": KOI8U, "cp866": IBM866, "iso8859_2": ISO8859_2, "iso8859_5": ISO8859_5, "iso8859_7": ISO8859_7}

func TestDetect(t *testing.T) {
	files, err := filepath.Glob("testdata/detect/*")
	if err != nil || len(files) < 30 {
		t.Fatalf("testdata/detect: %d files, %v", len(files), err)
	}
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		codec := f[strings.LastIndexByte(f, '.')+1:]
		want, decided := detectWant[codec]
		// The whole file (three paragraphs) and its first 600 bytes.
		for _, s := range [][]byte{b, b[:min(len(b), 600)]} {
			r := SniffText(s, "")
			switch {
			case decided && (r.Unknown || r.Enc != want || !r.Guessed):
				t.Errorf("%s (%d bytes): %+v, want %s", f, len(s), r, want)
			case !decided && !r.Unknown:
				te, ok := decodable[codec]
				if !ok {
					t.Errorf("%s (%d bytes): decided %s, want not decided", f, len(s), r.Enc)
					continue
				}
				a, _, _, _ := Decode(s, Result{Enc: te}, 1<<30)
				g, _, _, _ := Decode(s, r, 1<<30)
				if string(a) != string(g) {
					t.Errorf("%s (%d bytes): decided %s, which gives other text; want not decided", f, len(s), r.Enc)
				}
			}
		}
	}
}

// TestDetectNotUsed: input that declares its encoding, or is valid UTF-8,
// is never judged.
func TestDetectNotUsed(t *testing.T) {
	for _, r := range []Result{
		SniffText([]byte("plain ascii"), ""),
		SniffText([]byte("caf\xC3\xA9"), ""),
		SniffText([]byte("caf\xE9"), "windows-1250"),
		SniffText([]byte("\xEF\xBB\xBFcaf\xE9"), ""),
		Sniff([]byte(`<meta charset="koi8-r"><p>\xF0\xF2`), ""),
	} {
		if r.Guessed || r.Unknown {
			t.Errorf("%+v", r)
		}
	}
	if r := SniffText([]byte("caf\xE9 au lait"), ""); !r.Guessed || r.Enc != Windows1252 {
		t.Errorf("undeclared windows-1252: %+v", r)
	}
	if r := Sniff([]byte("<p>caf\xE9 au lait"), ""); !r.Guessed || r.Enc != Windows1252 {
		t.Errorf("undeclared windows-1252 HTML: %+v", r)
	}
}

// TestDetectEdges: inputs that are not text, and the smallest ones.
func TestDetectEdges(t *testing.T) {
	for _, s := range []string{
		"\x00\x01\x02\xFF\xFE\x80binary\x00",
		"\xFF",
		"\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR",
		strings.Repeat("\xE9\xE8", 50), // only high bytes: not Latin text
	} {
		if r := detect([]byte(s)); !r.Unknown {
			t.Errorf("%q: %+v, want not decided", s, r)
		}
	}
	if r := detect([]byte("na\xEFve")); r.Unknown || r.Enc != Windows1252 {
		t.Errorf("one accented letter: %+v", r)
	}
}

// BenchmarkDetect: the time detection adds to a file that declares no
// encoding and is not UTF-8, for each kind of text, at most detectSample
// bytes read whatever the file's size.
func BenchmarkDetect(b *testing.B) {
	for _, f := range []string{"fr.cp1252", "ja.cp932", "zh.gbk", "zh-tw.cp950", "ko.cp949", "ru.cp1251", "el.cp1253", "cs.cp1250", "ru.utf8bad", "fr.utf_16_le"} {
		data, err := os.ReadFile("testdata/detect/" + f)
		if err != nil {
			b.Fatal(err)
		}
		b.Run(f, func(b *testing.B) {
			b.SetBytes(int64(len(data)))
			for range b.N {
				detect(data)
			}
		})
	}
}

// TestDecodeDeclared: each file of testdata/detect, decoded in the
// encoding it was written in, gives back testdata/text/LANG.txt exactly,
// and every offset maps back to the source in order.
func TestDecodeDeclared(t *testing.T) {
	codec := map[string]Encoding{
		"cp932": ShiftJIS, "euc_jp": EUCJP, "gbk": GBK, "gb18030": GB18030, "cp950": Big5, "cp949": EUCKR,
		"cp1250": Windows1250, "cp1251": Windows1251, "cp1252": Windows1252, "cp1253": Windows1253,
		"cp1254": Windows1254, "cp1255": Windows1255, "koi8_r": KOI8R, "koi8_u": KOI8U, "cp866": IBM866,
		"iso8859_2": ISO8859_2, "iso8859_5": ISO8859_5, "iso8859_7": ISO8859_7,
		"utf_16_le": UTF16LE, "utf_16_be": UTF16BE,
	}
	seen := map[Encoding]bool{}
	files, _ := filepath.Glob("testdata/detect/*")
	for _, f := range files {
		base := filepath.Base(f)
		lang, c, _ := strings.Cut(base, ".")
		enc, ok := codec[c]
		if !ok {
			continue
		}
		b, _ := os.ReadFile(f)
		want, err := os.ReadFile("testdata/text/" + lang + ".txt")
		if err != nil {
			t.Fatal(err)
		}
		out, m, invalid, _ := Decode(b, Result{Enc: enc}, 1<<30)
		if invalid != 0 || string(out) != string(want) {
			t.Errorf("%s as %s: %d invalid; %.60q", base, enc, invalid, out)
			continue
		}
		seen[enc] = true
		prev := int64(-1)
		for i := range string(out) {
			s := m.Source(i)
			if s <= prev || s >= int64(len(b)) {
				t.Fatalf("%s: Source(%d) = %d after %d", base, i, s, prev)
			}
			prev = s
		}
	}
	for _, e := range []Encoding{EUCJP, GB18030, KOI8R, KOI8U, IBM866, ISO8859_2, ISO8859_5, ISO8859_7} {
		if !seen[e] {
			t.Errorf("%s not tested", e)
		}
	}
}
