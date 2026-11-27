package gofastxml

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/AOShei/go-fast-core"
)

// extract reads src and checks the document is valid.
func extract(t *testing.T, src string, x *Extractor) *gofastcore.Document {
	t.Helper()
	if x == nil {
		x = &Extractor{}
	}
	doc, err := x.Extract(context.Background(), strings.NewReader(src), int64(len(src)), nil)
	if err != nil {
		t.Fatalf("%q: %v", clip(src), err)
	}
	if err := doc.Validate(); err != nil {
		t.Fatalf("%q: invalid: %v", clip(src), err)
	}
	return doc
}

func clip(s string) string {
	if len(s) > 80 {
		return s[:80] + "..."
	}
	return s
}

// lines writes each block as kind, attrs of interest and text.
func lines(doc *gofastcore.Document) string {
	var b strings.Builder
	for _, bl := range doc.Parts[0].Blocks {
		switch {
		case bl.Kind == gofastcore.KindNote && bl.Attrs[gofastcore.AttrStored] != "":
			fmt.Fprintf(&b, "comment %q\n", bl.Text)
		case bl.Kind == gofastcore.KindNote:
			fmt.Fprintf(&b, "@%s %q (%s)\n", bl.Attrs[AttrAttribute], bl.Text, bl.Attrs[AttrElement])
		default:
			fmt.Fprintf(&b, "%s %q (%s)", bl.Kind, bl.Text, bl.Attrs[AttrElement])
			if bl.Continued {
				b.WriteString(" continued")
			}
			b.WriteByte('\n')
		}
	}
	return b.String()
}

func warned(doc *gofastcore.Document, code string) string {
	for _, w := range doc.AllWarnings() {
		if w.Code == code {
			return w.Message
		}
	}
	return ""
}

func TestGeneric(t *testing.T) {
	for _, c := range []struct{ name, src, want string }{
		{"containers and text elements",
			`<doc><title>T</title><sec><p>One</p><p>Two</p></sec></doc>`,
			"paragraph \"T\" (doc/title)\nparagraph \"One\" (doc/sec/p)\nparagraph \"Two\" (doc/sec/p)\n"},
		{"inline elements are part of the block",
			"<doc><p>Hello <b>bold</b> and <i>it<sup>2</sup></i>.\n  More</p></doc>",
			"paragraph \"Hello bold and it2. More\" (doc/p)\n"},
		{"direct text after children takes them in",
			`<doc><p><b>Bold</b> <i>it</i> tail</p></doc>`,
			"paragraph \"Bold it tail\" (doc/p)\n"},
		{"white space inside children is kept",
			`<doc><p><b>a </b>b<c> d</c></p></doc>`,
			"paragraph \"a b d\" (doc/p)\n"},
		{"attributes are notes before the block",
			`<doc lang="x"><p n="1" empty="">One <a href="u">link</a></p></doc>`,
			"@lang \"x\" (doc)\n@n \"1\" (doc/p)\nparagraph \"One link\" (doc/p)\n@href \"u\" (doc/p/a)\n"},
		{"comments stay in place",
			`<doc><!-- first --><p>One<!-- inside --></p><!--
  multi
  line --></doc>`,
			"comment \"first\"\nparagraph \"One\" (doc/p)\ncomment \"inside\"\ncomment \"multi\\n  line\"\n"},
		{"xml:space preserve keeps line breaks",
			"<doc><pre xml:space=\"preserve\">  line 1\n\tline 2  \n</pre><p>a\nb</p></doc>",
			"paragraph \"line 1\\n\\tline 2\" (doc/pre)\nparagraph \"a b\" (doc/p)\n"},
		{"prefixes as written in the path",
			`<r:doc xmlns:r="urn:r" xmlns:x="urn:x"><x:p x:a="v">t</x:p></r:doc>`,
			"@x:a \"v\" (r:doc/x:p)\nparagraph \"t\" (r:doc/x:p)\n"},
		{"namespace declarations, xml:lang and schema hints are not notes",
			`<doc xmlns="urn:d" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:schemaLocation="urn:d d.xsd" xml:lang="fr" xml:space="default"><p xml:lang="en">t</p></doc>`,
			"@xml:lang \"en\" (doc/p)\nparagraph \"t\" (doc/p)\n"},
		{"CDATA is text",
			`<doc><code><![CDATA[a < b]]></code></doc>`,
			"paragraph \"a < b\" (doc/code)\n"},
		{"ligatures and control references",
			"<doc><p>\uFB01ne&#1;&#x7;x&#9;y</p></doc>",
			"paragraph \"fine\\u0001x\\ty\" (doc/p)\n"},
	} {
		doc := extract(t, c.src, nil)
		want := c.want
		if c.name == "ligatures and control references" {
			want = "paragraph \"finex y\" (doc/p)\n" // a tab is white space, collapsed
		}
		if got := lines(doc); got != want {
			t.Errorf("%s:\n%s\nwant\n%s", c.name, got, want)
		}
	}
}

func TestControlsWarned(t *testing.T) {
	doc := extract(t, "<doc><p>a&#1;b\x02c</p></doc>", nil)
	if msg := warned(doc, gofastcore.WarnTextControlCharactersRemoved); !strings.HasPrefix(msg, "2 ") {
		t.Errorf("message %q", msg)
	}
}

func TestAttributesNone(t *testing.T) {
	doc := extract(t, `<doc a="1"><p b="2">t</p></doc>`, &Extractor{Attributes: AttributesNone})
	if got := lines(doc); got != "paragraph \"t\" (doc/p)\n" {
		t.Errorf("got %s", got)
	}
	if len(doc.Warnings)+len(doc.Parts[0].Warnings) != 0 {
		t.Errorf("warnings %+v %+v", doc.Warnings, doc.Parts[0].Warnings)
	}
}

func TestAnchorsAndOffsets(t *testing.T) {
	src := `<doc><p xml:id="a1">One</p><p id="b2">Two <b>x</b></p><!--c--></doc>`
	doc := extract(t, src, &Extractor{Attributes: AttributesNone})
	bs := doc.Parts[0].Blocks
	if len(bs) != 3 {
		t.Fatalf("%d blocks", len(bs))
	}
	for i, want := range []string{`<p xml:id="a1">One</p>`, `<p id="b2">Two <b>x</b></p>`, `<!--c-->`} {
		l := bs[i].Loc
		if got := src[l.Offset : l.Offset+l.Length]; got != want {
			t.Errorf("block %d: %q", i, got)
		}
	}
	if bs[0].Loc.Anchor != "a1" || bs[1].Loc.Anchor != "b2" {
		t.Errorf("anchors %q %q", bs[0].Loc.Anchor, bs[1].Loc.Anchor)
	}
	// With a byte order mark, offsets are still into the file.
	src = "\xEF\xBB\xBF<doc><p>x</p></doc>"
	doc = extract(t, src, nil)
	if l := doc.Parts[0].Blocks[0].Loc; src[l.Offset:l.Offset+l.Length] != "<p>x</p>" {
		t.Errorf("loc with BOM %+v", l)
	}
}

func TestMetadata(t *testing.T) {
	doc := extract(t, `<!DOCTYPE article PUBLIC "-//X//DTD Y//EN" "y.dtd"><article xmlns="urn:a" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:noNamespaceSchemaLocation="a.xsd" xml:lang="pt-BR"><p>t</p></article>`, nil)
	m := doc.Metadata
	if m.Language != "pt-BR" || m.Extra[ExtraRoot] != "article" || m.Extra[ExtraRootNamespace] != "urn:a" ||
		m.Extra[ExtraNoNamespaceSchemaLocation] != "a.xsd" || m.Extra[ExtraDoctypePublic] != "-//X//DTD Y//EN" || m.Extra[ExtraDoctypeSystem] != "y.dtd" {
		t.Errorf("metadata %+v", m)
	}
	if msg := warned(doc, gofastcore.WarnInputExternalReference); !strings.Contains(msg, "external DTD subset (1)") {
		t.Errorf("external reference %q", msg)
	}
	if doc.Source.Format != "xml" {
		t.Errorf("format %q", doc.Source.Format)
	}
}

func TestEntities(t *testing.T) {
	src := `<!DOCTYPE doc [
<!ENTITY prod "Widget&#x2122;">
<!ENTITY para "<p>From an entity: &prod;</p>">
<!ENTITY chap SYSTEM "chap.xml">
]>
<doc>&para;<p>&prod; &eacute; &chap;</p></doc>`
	doc := extract(t, src, nil)
	want := "paragraph \"From an entity: Widget\u2122\" (doc/p)\nparagraph \"Widget\u2122 &eacute; &chap;\" (doc/p)\n"
	if got := lines(doc); got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
	if msg := warned(doc, WarnEntityUndeclared); !strings.HasPrefix(msg, "1 reference") {
		t.Errorf("undeclared %q", msg)
	}
	if msg := warned(doc, gofastcore.WarnInputExternalReference); !strings.Contains(msg, "external entity reference (1)") {
		t.Errorf("external %q", msg)
	}
	// With an external subset, the HTML 4 names are assumed.
	doc = extract(t, `<!DOCTYPE doc SYSTEM "doc.dtd"><doc><p>caf&eacute; &mdash; &nosuch;</p></doc>`, nil)
	if got := lines(doc); got != "paragraph \"caf\u00e9 \u2014 &nosuch;\" (doc/p)\n" {
		t.Errorf("assumed: %s", got)
	}
	if msg := warned(doc, WarnEntitiesAssumed); !strings.HasPrefix(msg, "2 references") {
		t.Errorf("assumed %q", msg)
	}
	if msg := warned(doc, WarnEntityUndeclared); !strings.HasPrefix(msg, "1 reference") {
		t.Errorf("undeclared %q", msg)
	}
}

func TestProcessingInstructions(t *testing.T) {
	doc := extract(t, `<?xml version="1.0"?><?xml-stylesheet href="s.xsl" type="text/xsl"?><doc><?app one?><?app two?><p>t</p><x:include xmlns:x="http://www.w3.org/2001/XInclude" href="o.xml"/></doc>`, nil)
	if msg := warned(doc, gofastcore.WarnInputUnreadContent); msg != "2 processing instructions, not read as text" {
		t.Errorf("unread %q", msg)
	}
	var kinds []string
	for _, w := range doc.AllWarnings() {
		if w.Code == gofastcore.WarnInputExternalReference {
			kinds = append(kinds, w.Message)
		}
	}
	if len(kinds) != 2 || !strings.Contains(kinds[0], "xml-stylesheet") || !strings.Contains(kinds[1], "XInclude") {
		t.Errorf("external %q", kinds)
	}
}

func TestEncodings(t *testing.T) {
	doc := extract(t, "<?xml version=\"1.0\" encoding=\"windows-1252\"?><doc><p>caf\xE9 \x93q\x94</p></doc>", nil)
	if got := doc.Parts[0].Blocks[0].Text; got != "caf\u00e9 \u201cq\u201d" {
		t.Errorf("windows-1252 %q", got)
	}
	u16 := []byte{0xFF, 0xFE}
	for _, r := range `<doc><p>Ω x</p></doc>` {
		u16 = append(u16, byte(r), byte(r>>8))
	}
	doc = extract(t, string(u16), nil)
	if got := doc.Parts[0].Blocks[0].Text; got != "Ω x" {
		t.Errorf("utf-16 %q", got)
	}
	doc = extract(t, "<?xml version=\"1.0\" encoding=\"UTF-8\"?><doc><p>a\xFFb</p></doc>", nil)
	if got := doc.Parts[0].Blocks[0].Text; got != "a\uFFFDb" || warned(doc, gofastcore.WarnTextUnmappedGlyphs) == "" {
		t.Errorf("invalid utf-8 %q", got)
	}
	_, err := ExtractBytes(context.Background(), []byte(`<?xml version="1.0" encoding="ISO-2022-KR"?><a/>`), nil)
	if !errors.Is(err, gofastcore.ErrUnsupported) {
		t.Errorf("unsupported: %v", err)
	}
}

// ISO-2022-JP is read (core's charset decodes it), and locations are
// bytes of the file although the encoding keeps a state: the character
// set an escape sequence selects holds across the 32 KiB pieces the
// input is decoded in, and the map back to the source starts each piece
// in the state it was decoded in.
func TestISO2022JP(t *testing.T) {
	const kanji, ascii = "\x1b$B", "\x1b(B"
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="ISO-2022-JP"?><doc>`)
	var starts []int
	for i := range 3000 {
		starts = append(starts, b.Len())
		b.WriteString("<p>")
		if i%2 == 0 {
			// 日本 and then ASCII in one paragraph.
			b.WriteString(kanji + "F|K\\" + ascii + " abc")
		} else {
			// A long run in the two-byte set, which crosses pieces
			// without an escape sequence at their start.
			b.WriteString(kanji + strings.Repeat("F|", 40) + ascii)
		}
		b.WriteString("</p>")
	}
	b.WriteString("</doc>")
	src := b.String()
	if len(src) < 4*(32<<10) {
		t.Fatalf("the document is %d bytes: too short to cross pieces", len(src))
	}
	doc, err := ExtractBytes(context.Background(), []byte(src), nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := doc.Validate(); err != nil {
		t.Fatal(err)
	}
	if got := doc.Metadata.Extra[ExtraEncoding]; got != "iso-2022-jp" {
		t.Errorf("read as %q", got)
	}
	blocks := doc.Parts[0].Blocks
	if len(blocks) != len(starts) {
		t.Fatalf("%d blocks, want %d", len(blocks), len(starts))
	}
	for i, bl := range blocks {
		want := "日本 abc"
		if i%2 == 1 {
			want = strings.Repeat("日", 40)
		}
		if bl.Text != want {
			t.Fatalf("block %d: %q, want %q", i, bl.Text, want)
		}
		end := len(src) - len("</doc>")
		if i+1 < len(starts) {
			end = starts[i+1]
		}
		if int(bl.Loc.Offset) != starts[i] || int(bl.Loc.Offset+bl.Loc.Length) != end {
			t.Fatalf("block %d: at %d+%d, want %d to %d", i, bl.Loc.Offset, bl.Loc.Length, starts[i], end)
		}
	}
}

func TestMalformed(t *testing.T) {
	for _, src := range []string{"", "just text", "<!-- only a comment -->", "<?xml version=\"1.0\"?>"} {
		_, err := ExtractBytes(context.Background(), []byte(src), nil)
		if !errors.Is(err, gofastcore.ErrMalformed) {
			t.Errorf("%q: %v", src, err)
		}
	}
	doc := extract(t, `<doc><p>one<p>two</doc>`, nil)
	if warned(doc, gofastcore.WarnInputRepaired) == "" {
		t.Error("no repair reported")
	}
	if got := lines(doc); got != "paragraph \"onetwo\" (doc/p)\n" {
		t.Errorf("got %s", got)
	}
}

func TestNoText(t *testing.T) {
	doc := extract(t, `<doc><empty/></doc>`, nil)
	if warned(doc, gofastcore.WarnPartNoText) == "" {
		t.Error("no part.no_text")
	}
}

func TestPartsSelection(t *testing.T) {
	_, err := Extract(context.Background(), strings.NewReader("<a/>"), 4, &gofastcore.Options{Parts: gofastcore.PartRange{First: 2}})
	if !errors.Is(err, gofastcore.ErrNoPartsSelected) {
		t.Errorf("err %v", err)
	}
}

func TestTextOutsideRoot(t *testing.T) {
	doc := extract(t, `stray <doc><p>in</p></doc> after`, nil)
	if got := lines(doc); got != "paragraph \"stray\" ()\nparagraph \"in\" (doc/p)\nparagraph \"after\" ()\n" {
		t.Errorf("got %s", got)
	}
}
