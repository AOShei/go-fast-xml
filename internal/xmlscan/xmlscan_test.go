package xmlscan

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"testing"
	"testing/iotest"
)

// dump writes every token of the scan, one per line.
func dump(s *Scanner) string {
	var b strings.Builder
	for {
		k := s.Next()
		switch k {
		case EOF:
			if s.Err() != nil {
				fmt.Fprintf(&b, "ERR %v\n", s.Err())
			}
			return b.String()
		case Start:
			fmt.Fprintf(&b, "S %s {%s}", s.Name(), s.URI())
			for i := range s.NumAttr() {
				uri, local, v := s.AttrAt(i)
				fmt.Fprintf(&b, " %s{%s}%s=%q", s.AttrNameAt(i), uri, local, v)
			}
			b.WriteByte('\n')
		case End:
			fmt.Fprintf(&b, "E %s {%s}\n", s.Name(), s.URI())
		case Text:
			fmt.Fprintf(&b, "T %q\n", s.Text())
		case Comment:
			fmt.Fprintf(&b, "C %q %v\n", s.Text(), s.Final())
		case PI:
			fmt.Fprintf(&b, "P %s %q\n", s.Target(), s.Text())
		case Doctype:
			fmt.Fprintf(&b, "D %q\n", s.Text())
		}
	}
}

// merged is dump with adjacent Text and Comment pieces joined, so that
// scans that cut text in different places compare equal.
func merged(s *Scanner) string {
	var b strings.Builder
	var text, comment []byte
	inText := false
	flush := func() {
		if inText {
			fmt.Fprintf(&b, "T %q\n", text)
			text, inText = text[:0], false
		}
	}
	for {
		k := s.Next()
		if k != Text {
			flush()
		}
		switch k {
		case EOF:
			if s.Err() != nil {
				fmt.Fprintf(&b, "ERR\n")
			}
			return b.String()
		case Start:
			fmt.Fprintf(&b, "S %s {%s}", s.Name(), s.URI())
			for i := range s.NumAttr() {
				uri, local, v := s.AttrAt(i)
				fmt.Fprintf(&b, " {%s}%s=%q", uri, local, v)
			}
			b.WriteByte('\n')
		case End:
			fmt.Fprintf(&b, "E %s\n", s.Name())
		case Text:
			text, inText = append(text, s.Text()...), true
		case Comment:
			comment = append(comment, s.Text()...)
			if s.Final() {
				fmt.Fprintf(&b, "C %q\n", comment)
				comment = comment[:0]
			}
		case PI:
			fmt.Fprintf(&b, "P %s %q\n", s.Target(), s.Text())
		case Doctype:
			fmt.Fprintf(&b, "D %q\n", s.Text())
		}
	}
}

func TestTokens(t *testing.T) {
	src := `<?xml version="1.0"?>
<!DOCTYPE r [<!ENTITY e "x>y">]>
<r xmlns="urn:d" xmlns:p="urn:p" a="1" p:b="2&amp;3"><!-- c --><p:x>t&lt;<![CDATA[<raw>]]></p:x><y xmlns="">z</y><?pi data?></r>`
	got := dump(NewBytes([]byte(src), nil))
	want := `P xml "version=\"1.0\""
T "\n"
D " r [<!ENTITY e \"x>y\">]"
T "\n"
S r {urn:d} xmlns{http://www.w3.org/2000/xmlns/}xmlns="urn:d" xmlns:p{http://www.w3.org/2000/xmlns/}p="urn:p" a{}a="1" p:b{urn:p}b="2&3"
C " c " true
S p:x {urn:p}
T "t<"
T "<raw>"
E p:x {urn:p}
S y {} xmlns{http://www.w3.org/2000/xmlns/}xmlns=""
T "z"
E y {}
P pi "data"
E r {urn:d}
`
	if got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
}

func TestSelfDeclaredNamespaceOnEnd(t *testing.T) {
	s := NewBytes([]byte(`<a:x xmlns:a="urn:a"/>`), nil)
	if s.Next() != Start || string(s.URI()) != "urn:a" {
		t.Fatal("start")
	}
	if s.Next() != End || string(s.URI()) != "urn:a" || s.Space() != SpaceUnknown {
		t.Fatalf("end: %q", s.URI())
	}
}

func TestOffsets(t *testing.T) {
	src := `<a x="1">hi<!--c--><b/></a>`
	s := NewBytes([]byte(src), nil)
	var spans []string
	for s.Next() != EOF {
		a, b := s.Span()
		spans = append(spans, src[a:b])
	}
	want := []string{`<a x="1">`, "hi", "<!--c-->", "<b/>", "", "</a>"}
	if fmt.Sprint(spans) != fmt.Sprint(want) {
		t.Errorf("spans %q, want %q", spans, want)
	}
}

func TestRecovery(t *testing.T) {
	for _, c := range []struct{ src, want string }{
		{`<a><b>x</a>`, "S a {}\nS b {}\nT \"x\"\nE b {}\nE a {}\n"},
		{`<a>x</c></a>`, "S a {}\nT \"x\"\nE a {}\n"},
		{`<a>1 < 2 & 3</a>`, "S a {}\nT \"1 \"\nT \"<\"\nT \" 2 & 3\"\nE a {}\n"},
		{`<a>x`, "S a {}\nT \"x\"\nE a {}\n"},
		{`<a b c=d>x</a>`, "S a {} c{}c=\"d\"\nT \"x\"\nE a {}\n"},
		{`<a><!-- open`, "S a {}\nC \" open\" true\nE a {}\n"},
		{`<a>x<b`, "S a {}\nT \"x\"\nT \"<b\"\nE a {}\n"},
		{`<a <b>x</b></a>`, "S a {}\nS b {}\nT \"x\"\nE b {}\nE a {}\n"},
	} {
		s := NewBytes([]byte(c.src), nil)
		if got := dump(s); got != c.want {
			t.Errorf("%q:\n%s\nwant\n%s", c.src, got, c.want)
		}
		if s.Stats.Repairs == 0 {
			t.Errorf("%q: no repair counted", c.src)
		}
	}
}

func resolver(ents map[string]string) Resolver {
	return func(name []byte) ([]byte, Resolution) {
		if v, ok := ents[string(name)]; ok {
			return []byte(v), Expand
		}
		if string(name) == "eacute" {
			return []byte("\u00e9"), Insert
		}
		return nil, Keep
	}
}

func TestEntities(t *testing.T) {
	ents := map[string]string{"a": "A<b>&c;</b>", "c": "C", "loop": "x&loop;"}
	src := `<r k="&c;&eacute;&nope;">&a;|&eacute;|&nope;|&loop;</r>`
	s := NewBytes([]byte(src), &Options{Resolve: resolver(ents), ExpansionLimit: -1})
	got := merged(s)
	want := "S r {} {}k=\"C\u00e9&nope;\"\nT \"A\"\nS b {}\nT \"C\"\nE b\nT \"|\u00e9|&nope;|x&loop;\"\nE r\n"
	if got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
	if s.Stats.EntitiesRecursive != 1 {
		t.Errorf("recursive %d", s.Stats.EntitiesRecursive)
	}
}

func TestEntityOffsets(t *testing.T) {
	src := `<r>x&a;y</r>`
	s := NewBytes([]byte(src), &Options{Resolve: resolver(map[string]string{"a": "<b>in</b>"}), ExpansionLimit: -1})
	for s.Next() != EOF {
		if s.Kind() == Text && string(s.Text()) == "in" {
			if a, b := s.Span(); src[a:b] != "&a;" || !s.InEntity() {
				t.Errorf("span %q", src[a:b])
			}
		}
	}
}

func TestBillionLaughs(t *testing.T) {
	ents := map[string]string{"l0": "lol"}
	for i := 1; i <= 9; i++ {
		ents[fmt.Sprintf("l%d", i)] = strings.Repeat(fmt.Sprintf("&l%d;", i-1), 10)
	}
	s := NewBytes([]byte(`<r>&l9;</r>`), &Options{Resolve: resolver(ents), ExpansionLimit: 1 << 20})
	n := 0
	for s.Next() != EOF {
		n += len(s.Text())
	}
	if s.Stats.EntitiesCut == 0 || s.Stats.Expanded > 1<<20 {
		t.Errorf("cut %d, expanded %d", s.Stats.EntitiesCut, s.Stats.Expanded)
	}
	s = NewBytes([]byte(`<r a="&l9;"/>`), &Options{Resolve: resolver(ents), ExpansionLimit: 1 << 20})
	s.Next()
	if _, _, v := s.AttrAt(0); len(v) > 2<<20 || s.Stats.EntitiesCut == 0 {
		t.Errorf("attribute %d bytes, cut %d", len(v), s.Stats.EntitiesCut)
	}
	// Nesting deeper than the bound.
	deep := map[string]string{"d0": "x"}
	for i := 1; i < 40; i++ {
		deep[fmt.Sprintf("d%d", i)] = fmt.Sprintf("&d%d;", i-1)
	}
	s = NewBytes([]byte(`<r>&d39;</r>`), &Options{Resolve: resolver(deep), ExpansionLimit: -1})
	if got := merged(s); !strings.Contains(got, "&d") || s.Stats.EntitiesCut != 1 {
		t.Errorf("deep: %s, cut %d", got, s.Stats.EntitiesCut)
	}
}

func TestNoExpansionWhenLimitZero(t *testing.T) {
	s := NewBytes([]byte(`<r>&a;</r>`), &Options{Resolve: resolver(map[string]string{"a": "x"})})
	if got := merged(s); !strings.Contains(got, `"&a;"`) {
		t.Errorf("got %s", got)
	}
}

func TestDepthFlattened(t *testing.T) {
	src := strings.Repeat("<a>", 10) + "x" + strings.Repeat("</a>", 10)
	s := NewBytes([]byte(src), &Options{MaxDepth: 3})
	got := merged(s)
	if want := "S a {}\nS a {}\nS a {}\nT \"x\"\nE a\nE a\nE a\n"; got != want {
		t.Errorf("got\n%s", got)
	}
	if s.Stats.Flattened != 7 {
		t.Errorf("flattened %d", s.Stats.Flattened)
	}
}

func TestAttrsCut(t *testing.T) {
	var b strings.Builder
	b.WriteString("<a")
	for i := range 10 {
		fmt.Fprintf(&b, ` x%d="%d"`, i, i)
	}
	b.WriteString("/>")
	s := NewBytes([]byte(b.String()), &Options{MaxAttrs: 4})
	s.Next()
	if s.NumAttr() != 4 || s.Stats.AttrsCut != 6 {
		t.Errorf("attrs %d, cut %d", s.NumAttr(), s.Stats.AttrsCut)
	}
}

func TestTokenTooLarge(t *testing.T) {
	src := `<a x="` + strings.Repeat("v", 1000) + `"/>`
	s := New(strings.NewReader(src), &Options{MaxTokenBytes: 256, BufferSize: 64})
	for s.Next() != EOF {
	}
	if s.Err() == nil {
		t.Error("no error")
	}
}

// TestChunked: reading through a small window, one byte at a time, gives
// the same tokens as reading from memory.
func TestChunked(t *testing.T) {
	src := `<?xml version="1.0"?><!DOCTYPE r [<!ENTITY a "<i>A</i>">]><r xmlns:p="urn:p">` +
		strings.Repeat("text &amp; more &#x263A; caf\u00e9\r\n", 50) +
		`<!--` + strings.Repeat("comment ", 40) + `--><![CDATA[` + strings.Repeat("cdata ", 40) + `]]>&a;<p:q a="&lt;"/></r>`
	opts := func(buf int) *Options {
		return &Options{BufferSize: buf, Resolve: resolver(map[string]string{"a": "<i>A</i>"}), ExpansionLimit: -1}
	}
	want := merged(NewBytes([]byte(src), opts(0)))
	for _, size := range []int{16, 64, 100, 4096} {
		got := merged(New(iotest.OneByteReader(strings.NewReader(src)), opts(size)))
		if got != want {
			t.Errorf("buffer %d:\n%s\nwant\n%s", size, got, want)
		}
	}
}

func FuzzScanner(f *testing.F) {
	for _, s := range []string{
		`<a b="c">d<e/>&amp;&x;<!--f--><![CDATA[g]]><?h i?></a>`,
		`<!DOCTYPE a [<!ENTITY x "<b>y</b>">]><a>&x;</a>`,
		`<a><b></a></b>< & <`,
		"<a xmlns='u' xmlns:p='v'><p:b p:c='1'>\r\n</p:b></a>",
	} {
		f.Add([]byte(s))
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		opts := func(buf int) *Options {
			return &Options{BufferSize: buf, MaxTokenBytes: 1 << 16, MaxDepth: 64,
				Resolve: resolver(map[string]string{"x": "<b>y</b>&x;", "y": "&x;&x;"}), ExpansionLimit: 1 << 12}
		}
		whole := NewBytes(data, opts(0))
		want := merged(whole)
		got := merged(New(bytes.NewReader(data), opts(16)))
		if !strings.Contains(want, "ERR") && !strings.Contains(got, "ERR") && got != want {
			t.Fatalf("chunked scan differs:\n%s\nwant\n%s", got, want)
		}
		if whole.Stats.Expanded > 1<<12 {
			t.Fatalf("expanded %d", whole.Stats.Expanded)
		}
	})
}

var _ = io.EOF
