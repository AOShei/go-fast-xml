package charset

import (
	"math"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestSniff(t *testing.T) {
	for _, c := range []struct {
		in       string
		enc      Encoding
		from     string
		declared string
		unsup    bool
	}{
		{"\xEF\xBB\xBF<p>x", UTF8, FromBOM, "", false},
		{"\xFF\xFE<\x00p\x00", UTF16LE, FromBOM, "", false},
		{"\xFE\xFF\x00<\x00p", UTF16BE, FromBOM, "", false},
		{`<meta charset="windows-1252"><p>`, Windows1252, FromMeta, "windows-1252", false},
		{`<META CHARSET=ISO-8859-1>`, Windows1252, FromMeta, "iso-8859-1", false},
		{`<meta http-equiv="Content-Type" content="text/html; charset=utf-8">`, UTF8, FromMeta, "utf-8", false},
		{`<meta content="text/html; charset=utf-8">`, UTF8, FromDetected, "", false},
		{`<meta charset="utf-16">`, UTF8, FromMeta, "utf-16", false},
		{`<meta charset="koi8-r">`, KOI8R, FromMeta, "koi8-r", false},
		{`<meta charset="iso-2022-jp">`, Windows1252, FromMeta, "iso-2022-jp", true},
		{`<meta charset="shift_jis">`, ShiftJIS, FromMeta, "shift_jis", false},
		{`<meta charset="ISO-8859-9">`, Windows1254, FromMeta, "iso-8859-9", false},
		{`<meta charset="gb18030">`, GB18030, FromMeta, "gb18030", false},
		{`<meta charset="utf-7">`, Windows1252, FromMeta, "utf-7", true},
		{`<!-- <meta charset="shift_jis"> --><p>caf` + "\xC3\xA9", UTF8, FromDetected, "", false},
		{`<div title="<meta charset=koi8-r>">x`, UTF8, FromDetected, "", false},
		{"<p>caf\xE9", Windows1252, FromDetected, "", false},
	} {
		r := Sniff([]byte(c.in), "")
		if r.Enc != c.enc || r.From != c.from || r.Declared != c.declared || r.Unsupported != c.unsup {
			t.Errorf("Sniff(%q) = %+v", c.in, r)
		}
	}
}

func TestDecodeMap(t *testing.T) {
	for _, c := range []struct {
		src  string
		want string
	}{
		{"a\xE9b\x80c", "aéb€c"},
		{"\xFF\xFEa\x00\xE9\x00=\xD8\x00\xDEz\x00", "aé\U0001F600z"},
		{"\xFE\xFF\x00a\xD8\x00", "a\xEF\xBF\xBD"},
		{"<meta charset=utf-8>\xC3(", "<meta charset=utf-8>\xEF\xBF\xBD("},
	} {
		src := []byte(c.src)
		r := Sniff(src, "")
		out, m, _, cut := Decode(src, r, math.MaxInt64)
		if string(out) != c.want || cut {
			t.Errorf("Decode(%q) = %q, want %q", c.src, out, c.want)
			continue
		}
		// Every character's offset maps to a source offset in order,
		// and the end maps to the end.
		prev := int64(-1)
		for i := range string(out) {
			s := m.Source(i)
			if s <= prev || s >= int64(len(src)) {
				t.Errorf("%q: Source(%d) = %d after %d", c.src, i, s, prev)
			}
			prev = s
		}
		if got := m.Source(len(out)); got != int64(len(src)) {
			t.Errorf("%q: Source(end) = %d, want %d", c.src, got, len(src))
		}
	}
}

func TestDecodeLong(t *testing.T) {
	// Long enough for several checkpoints.
	src := make([]byte, 20000)
	for i := range src {
		src[i] = byte('a' + i%3)
		if i%7 == 0 {
			src[i] = 0xE9
		}
	}
	out, m, _, _ := Decode(src, Result{Enc: Windows1252}, math.MaxInt64)
	dec := 0
	for i := range src {
		if got := m.Source(dec); got != int64(i) {
			t.Fatalf("Source(%d) = %d, want %d", dec, got, i)
		}
		r, n := utf8.DecodeRune(out[dec:])
		_ = r
		dec += n
	}
}

func TestDecodeLimit(t *testing.T) {
	out, _, _, cut := Decode([]byte("\xE9\xE9\xE9"), Result{Enc: Windows1252}, 5)
	if !cut || len(out) != 4 {
		t.Errorf("got %q, cut %v", out, cut)
	}
	out, _, _, cut = Decode([]byte("ab\xC3\xA9"), Result{Enc: UTF8}, 3)
	if !cut || string(out) != "ab" {
		t.Errorf("got %q, cut %v", out, cut)
	}
}

// Run with -fuzzminimizetime=2s: see FuzzExtract in the root package.
func FuzzCharset(f *testing.F) {
	f.Add([]byte(`<meta charset="utf-8"><p>x`))
	f.Add([]byte("\xFF\xFEa\x00"))
	f.Add([]byte("<p>caf\xE9"))
	f.Fuzz(func(t *testing.T, b []byte) {
		r := Sniff(b, "")
		out, m, _, _ := Decode(b, r, 1<<20)
		if !utf8.Valid(out) {
			t.Fatal("invalid UTF-8")
		}
		prev := int64(-1)
		for i := range string(out) {
			s := m.Source(i)
			if s < prev || s > int64(len(b)) {
				t.Fatalf("Source(%d) = %d after %d", i, s, prev)
			}
			prev = s
		}
		sameInAnyOrder(t, m, min(len(out), 4096), uint64(len(b)))
	})
}

func TestTransport(t *testing.T) {
	for _, c := range []struct {
		in, tr string
		enc    Encoding
		from   string
		bad    string
	}{
		{"<p>x", "windows-1251", Windows1251, FromTransport, ""},
		{"<p>x", ` "X-CP1251" `, Windows1251, FromTransport, ""},
		{"<p>x", "sjis", ShiftJIS, FromTransport, ""},
		{"\xEF\xBB\xBF<p>x", "windows-1251", UTF8, FromBOM, ""},
		{`<meta charset="shift_jis"><p>x`, "windows-1251", Windows1251, FromTransport, ""},
		{`<meta charset="shift_jis"><p>x`, "x-unknown", ShiftJIS, FromMeta, "x-unknown"},
		{"<p>x", "iso-2022-jp", UTF8, FromDetected, "iso-2022-jp"},
		{"<p>x", "", UTF8, FromDetected, ""},
	} {
		r := Sniff([]byte(c.in), c.tr)
		if r.Enc != c.enc || r.From != c.from || r.Transport != c.bad {
			t.Errorf("Sniff(%q, %q) = %+v", c.in, c.tr, r)
		}
	}
	if r := SniffText([]byte("x\xCF"), "cp1251"); r.Enc != Windows1251 || r.From != FromTransport {
		t.Errorf("SniffText = %+v", r)
	}
}

func TestDecodeLegacy(t *testing.T) {
	for _, c := range []struct {
		enc     Encoding
		src     string
		want    string
		invalid int
	}{
		{Windows1251, "\xCF\xF0\xE8\xE2\xE5\xF2!", "Привет!", 0},
		{Windows1251, "\x98", "\u0098", 0}, // undefined in 0x80-0x9F: the C1 control
		{Windows874, "\xA1\xDB", "ก�", 1},
		{Windows1254, "\xF0", "ğ", 0},
		{Macintosh, "\x80\xA5", "Ä•", 0},
		{ShiftJIS, "a\x93\xFA\x96\x7B\x8C\xEAb\xB1", "a日本語bｱ", 0},
		{ShiftJIS, "\x81\x30x", "�0x", 1}, // the ASCII trail byte is read again
		{ShiftJIS, "x\x93", "x�", 1},
		{GBK, "\xC4\xE3\xBA\xC3", "你好", 0},
		{EUCKR, "\xC7\xD1\xB1\xDB", "한글", 0},
		{Big5, "\xA4\xA4\xA4\xE5", "中文", 0},
	} {
		src := []byte(c.src)
		out, m, invalid, cut := Decode(src, Result{Enc: c.enc}, math.MaxInt64)
		if string(out) != c.want || invalid != c.invalid || cut {
			t.Errorf("%s %q: got %q (%d invalid), want %q (%d)", c.enc, c.src, out, invalid, c.want, c.invalid)
			continue
		}
		prev := int64(-1)
		for i := range string(out) {
			s := m.Source(i)
			if s <= prev || s >= int64(len(src)) {
				t.Errorf("%q: Source(%d) = %d after %d", c.src, i, s, prev)
			}
			prev = s
		}
		if got := m.Source(len(out)); got != int64(len(src)) {
			t.Errorf("%q: Source(end) = %d, want %d", c.src, got, len(src))
		}
	}
}

func TestDecodeLongDoubleByte(t *testing.T) {
	// Several checkpoints of mixed one- and two-byte characters.
	var src []byte
	var starts []int
	for i := 0; i < 6000; i++ {
		starts = append(starts, len(src))
		if i%3 == 0 {
			src = append(src, 'a')
		} else {
			src = append(src, 0x93, 0xFA)
		}
	}
	out, m, invalid, _ := Decode(src, Result{Enc: ShiftJIS}, math.MaxInt64)
	if invalid != 0 {
		t.Fatalf("%d invalid", invalid)
	}
	k := 0
	for i := range string(out) {
		if got := m.Source(i); got != int64(starts[k]) {
			t.Fatalf("Source(%d) = %d, want %d", i, got, starts[k])
		}
		k++
	}
}

// Run with -fuzztime=30s or less.
func FuzzDoubleByte(f *testing.F) {
	f.Add([]byte("a\x93\xFA\x96\x7B\x8C\xEAb\xB1\x81"))
	f.Add([]byte("\xC4\xE3\xBA\xC3\xFF\x80"))
	f.Fuzz(func(t *testing.T, b []byte) {
		for _, enc := range []Encoding{ShiftJIS, GBK, EUCKR, Big5, EUCJP, GB18030} {
			out, m, _, cut := Decode(b, Result{Enc: enc}, 1<<20)
			if !utf8.Valid(out) {
				t.Fatal("invalid UTF-8")
			}
			prev := int64(-1)
			for i, r := range string(out) {
				s := m.Source(i)
				if s == prev && enc == Big5 && (r == 0x0304 || r == 0x030C) {
					continue // the mark of a Big5 pair shares its letter's bytes
				}
				if s <= prev || s >= int64(len(b)) {
					t.Fatalf("%s: Source(%d) = %d after %d", enc, i, s, prev)
				}
				prev = s
			}
			if s := m.Source(len(out)); !cut && s != int64(len(b)) {
				t.Fatalf("%s: Source(end) = %d, want %d", enc, s, len(b))
			}
			sameInAnyOrder(t, m, min(len(out), 4096), uint64(len(b)))
		}
	})
}

// sameInAnyOrder checks that m gives every offset of the decoded text
// (n bytes) the same source offset whatever order they are asked in: in
// order, backwards, and shuffled, each against a Map that starts from its
// checkpoints alone.
func sameInAnyOrder(t *testing.T, m *Map, n int, seed uint64) {
	t.Helper()
	want := make([]int64, n+1)
	for i := range want {
		m.last = checkpoint{}
		want[i] = m.Source(i)
	}
	order := make([]int, 0, 3*(n+1))
	for i := 0; i <= n; i++ {
		order = append(order, i)
	}
	for i := n; i >= 0; i-- {
		order = append(order, i)
	}
	x := seed | 1
	for range n + 1 {
		x ^= x << 13
		x ^= x >> 7
		x ^= x << 17
		order = append(order, int(x%uint64(n+1)))
	}
	m.last = checkpoint{}
	for _, i := range order {
		if got := m.Source(i); got != want[i] {
			t.Fatalf("Source(%d) = %d, want %d", i, got, want[i])
		}
	}
}

// TestSourceAnyOrder: the remembered position of the last call never
// changes an answer, in any encoding, across checkpoints.
func TestSourceAnyOrder(t *testing.T) {
	src := make([]byte, 3*checkpointEvery)
	x := uint32(1)
	for i := range src {
		x = x*1664525 + 1013904223
		src[i] = byte(x >> 24)
		if x&3 == 0 {
			src[i] = 'a' + byte(x>>8)%26
		}
	}
	for _, enc := range []Encoding{UTF8, Windows1252, Windows1251, UTF16LE, UTF16BE, ShiftJIS, GBK, EUCKR, Big5} {
		out, m, _, _ := Decode(src, Result{Enc: enc}, math.MaxInt64)
		sameInAnyOrder(t, m, len(out), uint64(enc)+7)
	}
}

// BenchmarkSourceInOrder asks for the start and end of every 40-byte
// paragraph of 4 MiB of windows-1252 in order, as the blocks of a
// document are: one pass over the source, not 4 KiB of decoding a call.
func BenchmarkSourceInOrder(b *testing.B) {
	src := []byte(strings.Repeat("caf\xE9 na\xEFve r\xE9sum\xE9 text here.\n\n", 4<<20/40))
	out, m, _, _ := Decode(src, Result{Enc: Windows1252}, math.MaxInt64)
	b.ResetTimer()
	for range b.N {
		m.last = checkpoint{}
		for i := 0; i < len(out); i += 40 {
			m.Source(i)
			m.Source(min(i+38, len(out)))
		}
	}
}

// TestLateMeta: a declaration past the first 1024 bytes is used when the
// input is not UTF-8, and not when it is.
func TestLateMeta(t *testing.T) {
	pad := "<!-- " + strings.Repeat("x", 1100) + " -->"
	r := Sniff([]byte(pad+"<meta charset=\"windows-1250\"><p>P\xf8\xedli\x9a"), "")
	if r.Enc != Windows1250 || r.From != FromMeta || r.Guessed {
		t.Errorf("not UTF-8: %+v", r)
	}
	r = Sniff([]byte(pad+"<meta charset=\"windows-1250\"><p>caf\xc3\xa9"), "")
	if r.Enc != UTF8 {
		t.Errorf("UTF-8: %+v", r)
	}
	r = Sniff([]byte(pad+"<meta charset=\"euc-jp\"><p>\xc6\xfc\xcb\xdc"), "")
	if r.Enc != EUCJP || r.From != FromMeta {
		t.Errorf("EUC-JP: %+v", r)
	}
}

// TestTransportNewLabels: the labels of the encodings added with
// singlebyte.go and cjk.go work as the transport-layer encoding too.
func TestTransportNewLabels(t *testing.T) {
	for label, want := range map[string]Encoding{"koi8-r": KOI8R, "ISO-8859-2": ISO8859_2, "euc-jp": EUCJP, "gb18030": GB18030, "cp866": IBM866} {
		if r := SniffText([]byte("x"), label); r.Enc != want || r.From != FromTransport {
			t.Errorf("%s: %+v", label, r)
		}
	}
}
