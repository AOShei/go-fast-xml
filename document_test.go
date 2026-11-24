package gofastxml

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"testing"

	"github.com/AOShei/go-fast-core"
)

// handed is what a fake child extractor was given.
type handed struct {
	name, mime string
	data       string
}

// fakeChild returns a Child that records what it is given and returns a
// document of one paragraph holding it; err, when set, is returned
// instead.
func fakeChild(got *[]handed, err error) gofastcore.ChildFunc {
	return func(ctx context.Context, name, mime string, r io.ReaderAt, size int64, opts *gofastcore.Options) (*gofastcore.Document, error) {
		b := make([]byte, size)
		if _, e := r.ReadAt(b, 0); e != nil && e != io.EOF {
			return nil, e
		}
		*got = append(*got, handed{name, mime, string(b)})
		if err != nil {
			return nil, err
		}
		d := gofastcore.New("fake")
		d.Parts = []gofastcore.Part{{Kind: gofastcore.PartBody, Number: 1,
			Blocks: []gofastcore.Block{{Kind: gofastcore.KindParagraph, Origin: gofastcore.OriginNative, Text: string(b)}}}}
		return d, nil
	}
}

// docHas reports whether a document has a warning of that code.
func docHas(d *gofastcore.Document, code string) bool {
	for _, w := range d.Warnings {
		if w.Code == code {
			return true
		}
	}
	return false
}

const docProf =`{"profile": "docs", "format": 1, "match": [{"root": "doc"}], "rules": [
	{"match": "/doc", "as": "transparent"},
	{"match": "sec", "as": "section", "anchor": {"attr": "id"}},
	{"match": "sec/t", "as": "heading", "level": "nesting"},
	{"match": "d", "as": "document", "media_type": "text/html"},
	{"match": "m", "as": "document", "media_type": "text/html", "when": "markup"},
	{"match": "content", "as": "document", "media_type": {"attr": "type", "map": {"html": "text/html", "xhtml": "application/xhtml+xml"}}},
	{"match": "content", "as": "block", "kind": "paragraph"},
	{"match": "p", "as": "block", "kind": "paragraph"},
	{"match": "pt", "as": "part"}]}`

// docExtract reads src with docProf and the fake child (none when got is
// nil), and checks what every test of embedded documents checks.
func docExtract(t *testing.T, src string, got *[]handed, err error, opts *gofastcore.Options) *gofastcore.Document {
	t.Helper()
	if opts == nil {
		opts = &gofastcore.Options{}
	}
	if got != nil {
		opts.Child = fakeChild(got, err)
	}
	doc := extractWith(t, docProf, src, opts)
	checkChildren(t, []byte(src), doc)
	return doc
}

// checkChildren checks link 2 of every child: its element's bytes are at
// xml.parent_offset, and each child is named by a section's "children".
func checkChildren(t *testing.T, src []byte, doc *gofastcore.Document) {
	t.Helper()
	linked := map[int]bool{}
	for _, b := range blocks(doc) {
		if v := b.Attrs[AttrChildren]; v != "" {
			for _, s := range strings.Split(v, ",") {
				i, err := strconv.Atoi(s)
				if err != nil || i < 0 || i >= len(doc.Children) || linked[i] {
					t.Errorf("children %q", v)
				}
				linked[i] = true
			}
		}
	}
	for i, c := range doc.Children {
		if !childExtracted(c) {
			// A stub: the parent's Validate checks it.
		} else if err := c.Validate(); err != nil {
			t.Errorf("child %d: %v", i, err)
		}
		off, err1 := strconv.ParseInt(c.Metadata.Extra[ExtraParentOffset], 10, 64)
		n, err2 := strconv.ParseInt(c.Metadata.Extra[ExtraParentLength], 10, 64)
		if err1 != nil || err2 != nil || off < 0 || n <= 0 || off+n > int64(len(src)) {
			t.Errorf("child %d: parent offsets %v", i, c.Metadata.Extra)
			continue
		}
		el := c.Metadata.Extra[ExtraParentElement]
		name := el[strings.LastIndexByte(el, '/')+1:]
		span := string(src[off : off+n])
		if !strings.HasPrefix(span, "<"+name) || !strings.HasSuffix(span, ">") {
			t.Errorf("child %d: %q at its parent offsets is not its element %s", i, span, el)
		}
	}
}

func TestDocumentConstant(t *testing.T) {
	src := `<doc><sec id="i1"><t>Title</t><d>&lt;p&gt;Hi &amp;amp; you&lt;/p&gt;</d><p>After.</p></sec></doc>`
	var got []handed
	doc := docExtract(t, src, &got, nil, nil)
	wantBrief(t, doc, "heading1:Title\nparagraph:After.\n")
	if len(got) != 1 || got[0] != (handed{"sec/d#i1", "text/html; charset=utf-8", "<p>Hi &amp; you</p>"}) {
		t.Fatalf("handed %q", got)
	}
	if len(doc.Children) != 1 {
		t.Fatalf("children %d", len(doc.Children))
	}
	c := doc.Children[0]
	off := strings.Index(src, "<d>")
	want := map[string]string{ExtraParentOffset: strconv.Itoa(off), ExtraParentLength: strconv.Itoa(strings.Index(src, "</d>") + 4 - off),
		ExtraParentAnchor: "i1", ExtraParentElement: "doc/sec/d"}
	for k, v := range want {
		if c.Metadata.Extra[k] != v {
			t.Errorf("%s = %q, want %q", k, c.Metadata.Extra[k], v)
		}
	}
	if c.Source.Name != "sec/d#i1" || c.Source.Format != "fake" {
		t.Errorf("source %+v", c.Source)
	}
	if h := blocks(doc)[0]; h.Attrs[AttrChildren] != "0" {
		t.Errorf("heading attrs %v", h.Attrs)
	}
}

func TestDocumentByAttribute(t *testing.T) {
	xhtml := `<div xmlns="http://www.w3.org/1999/xhtml"><p>One &amp; <b>two</b></p><p>Three</p></div>`
	src := `<doc><sec id="e"><t>T</t><content type="html">&lt;i&gt;x&lt;/i&gt;</content>` +
		`<content type="xhtml">` + xhtml + `</content><content type="text">&lt;i&gt;y&lt;/i&gt;</content><content>plain</content></sec></doc>`
	var got []handed
	doc := docExtract(t, src, &got, nil, nil)
	wantBrief(t, doc, "heading1:T\nparagraph:<i>y</i>\nparagraph:plain\n")
	if len(got) != 2 || got[0].data != "<i>x</i>" || got[1].data != xhtml || got[1].mime != "application/xhtml+xml; charset=utf-8" {
		t.Fatalf("handed %q", got)
	}
	if h := blocks(doc)[0]; h.Attrs[AttrChildren] != "0,1" {
		t.Errorf("heading attrs %v", h.Attrs)
	}
	// Without a child extractor the XHTML is kept as its text.
	doc = docExtract(t, src, nil, nil, nil)
	wantBrief(t, doc, "heading1:T\nparagraph:<i>x</i>\nparagraph:One & two Three\nparagraph:<i>y</i>\nparagraph:plain\n")
	bs := blocks(doc)
	if bs[1].Attrs[AttrMediaType] != "text/html" || bs[2].Attrs[AttrMediaType] != "application/xhtml+xml" || bs[3].Attrs[AttrMediaType] != "" {
		t.Errorf("media types %v %v %v", bs[1].Attrs, bs[2].Attrs, bs[3].Attrs)
	}
	if len(doc.Children) != 2 || !docHas(doc.Children[0], gofastcore.WarnChildNotExtracted) {
		t.Errorf("stubs %d", len(doc.Children))
	}
}

func TestDocumentXHTMLNotUTF8(t *testing.T) {
	src := "<?xml version=\"1.0\" encoding=\"ISO-8859-1\"?><doc><sec><t>T</t><content type=\"xhtml\"><div a=\"1&amp;2\">caf\xe9<br/>&lt;</div></content></sec></doc>"
	var got []handed
	docExtract(t, src, &got, nil, nil)
	if len(got) != 1 || got[0].data != "<div a=\"1&amp;2\">café<br/>&lt;</div>" {
		t.Fatalf("handed %q", got)
	}
}

func TestDocumentWhenMarkup(t *testing.T) {
	src := `<doc><sec id="s"><t>T</t><m>Plain text, a &lt; b and c &gt; d &amp; e.</m><m><![CDATA[<b>bold</b>]]></m>` +
		`<m>caf&amp;eacute;</m><m>x &amp;#233; y</m><m>&lt;!-- c --&gt;</m><m>   </m></sec></doc>`
	var got []handed
	doc := docExtract(t, src, &got, nil, nil)
	wantBrief(t, doc, "heading1:T\nparagraph:Plain text, a < b and c > d & e.\n")
	want := []string{"<b>bold</b>", "caf&eacute;", "x &#233; y", "<!-- c -->"}
	if len(got) != len(want) {
		t.Fatalf("handed %q", got)
	}
	for i, w := range want {
		if got[i].data != w {
			t.Errorf("handed %d %q, want %q", i, got[i].data, w)
		}
	}
	if bs := blocks(doc); bs[0].Attrs[AttrChildren] != "0,1,2,3" || bs[1].Attrs[AttrMediaType] != "" {
		t.Errorf("attrs %v %v", bs[0].Attrs, bs[1].Attrs)
	}
}

func TestDocumentNotExtracted(t *testing.T) {
	src := `<doc><sec id="i1"><t>Title</t><d>&lt;p&gt;Hi&lt;/p&gt;</d></sec><sec><d>&lt;b&gt;x&lt;/b&gt;</d></sec><sec><d>&lt;i&gt;y&lt;/i&gt;</d></sec></doc>`
	for _, c := range []struct {
		name string
		err  error
		code string
	}{{"no child", nil, gofastcore.WarnChildNotExtracted}, {"unsupported", gofastcore.ErrUnsupported, gofastcore.WarnChildUnsupported},
		{"failed", errors.New("broken"), gofastcore.WarnChildFailed}} {
		t.Run(c.name, func(t *testing.T) {
			var got []handed
			p := &got
			if c.err == nil {
				p = nil
			}
			doc := docExtract(t, src, p, c.err, nil)
			wantBrief(t, doc, "heading1:Title\nparagraph:<p>Hi</p>\nparagraph:<b>x</b>\nparagraph:<i>y</i>\n")
			bs := blocks(doc)
			if bs[0].Attrs[AttrChildren] != "0" || bs[1].Attrs[AttrMediaType] != "text/html" {
				t.Errorf("attrs %v %v", bs[0].Attrs, bs[1].Attrs)
			}
			// A section with no heading: on its first block.
			if bs[2].Attrs[AttrChildren] != "1" || bs[3].Attrs[AttrChildren] != "2" {
				t.Errorf("attrs %v %v", bs[2].Attrs, bs[3].Attrs)
			}
			if len(doc.Children) != 3 || !docHas(doc.Children[0], c.code) || doc.Children[0].Source.Name != "sec/d#i1" {
				t.Fatalf("children %+v", doc.Children)
			}
		})
	}
	// Extracted, a section with no heading and no block has no
	// "children": the child's own keys are the link.
	var got []handed
	doc := docExtract(t, src, &got, nil, nil)
	wantBrief(t, doc, "heading1:Title\n")
	if doc.Children[1].Metadata.Extra[ExtraParentAnchor] != "" || doc.Children[1].Metadata.Extra[ExtraParentElement] != "doc/sec/d" {
		t.Errorf("extra %v", doc.Children[1].Metadata.Extra)
	}
}

func TestDocumentDepthLimit(t *testing.T) {
	src := `<doc><sec><t>T</t><d>&lt;p&gt;Hi&lt;/p&gt;</d></sec></doc>`
	var got []handed
	opts := &gofastcore.Options{Depth: 2, Limits: gofastcore.Limits{MaxDepth: 2}}
	doc := docExtract(t, src, &got, nil, opts)
	wantBrief(t, doc, "heading1:T\nparagraph:<p>Hi</p>\n")
	if len(got) != 0 || len(doc.Children) != 1 || !docHas(doc.Children[0], gofastcore.WarnChildNotExtracted) {
		t.Errorf("handed %q, children %d", got, len(doc.Children))
	}
}

func TestDocumentMaxChildren(t *testing.T) {
	defer func(n int) { maxChildren = n }(maxChildren)
	maxChildren = 2
	src := `<doc><sec><t>T</t><d>&lt;a&gt;</d><d>&lt;b&gt;</d><d>&lt;c&gt;</d><d>&lt;e&gt;</d></sec></doc>`
	var got []handed
	doc := docExtract(t, src, &got, nil, nil)
	wantBrief(t, doc, "heading1:T\nparagraph:<c>\nparagraph:<e>\n")
	if len(got) != 2 || len(doc.Children) != 2 {
		t.Errorf("handed %q", got)
	}
	n := 0
	for _, w := range doc.Parts[0].Warnings {
		if w.Code == gofastcore.WarnLimitExceeded && w.Severity == gofastcore.SeverityError {
			n++
			if !strings.Contains(w.Message, "2 embedded documents past 2") {
				t.Errorf("message %q", w.Message)
			}
		}
	}
	if n != 1 {
		t.Errorf("warnings %v", doc.Parts[0].Warnings)
	}
}

func TestDocumentSizeBound(t *testing.T) {
	long := strings.Repeat("<p>abc</p>", MaxBlockBytes/10+100)
	src := `<doc><sec><t>T</t><d>` + strings.NewReplacer("<", "&lt;", ">", "&gt;").Replace(long) + `</d></sec></doc>`
	var got []handed
	doc := docExtract(t, src, &got, nil, nil)
	if len(got) != 1 || len(got[0].data) != MaxBlockBytes || got[0].data != long[:MaxBlockBytes] {
		t.Fatalf("handed %d bytes", len(got[0].data))
	}
	if !doc.Parts[0].Has(gofastcore.WarnLimitExceeded) {
		t.Errorf("warnings %v", doc.Parts[0].Warnings)
	}
	// Without a child extractor the paragraph is cut the same way.
	doc = docExtract(t, src, nil, nil, nil)
	bs := blocks(doc)
	if len(bs) != 2 || len(bs[1].Text) > MaxBlockBytes || !doc.Parts[0].Has(gofastcore.WarnLimitExceeded) {
		t.Errorf("%d blocks, %d bytes", len(bs), len(bs[len(bs)-1].Text))
	}
}

func TestDocumentParts(t *testing.T) {
	src := `<doc><pt><sec><t>A</t><d>&lt;a&gt;</d></sec></pt><pt><sec><t>B</t><d>&lt;b&gt;</d></sec></pt></doc>`
	var got []handed
	doc := docExtract(t, src, &got, nil, &gofastcore.Options{Parts: gofastcore.PartRange{First: 2, Last: 2}})
	if len(got) != 1 || got[0].data != "<b>" || len(doc.Children) != 1 {
		t.Fatalf("handed %q", got)
	}
	if h := blocks(doc)[0]; h.Text != "B" || h.Attrs[AttrChildren] != "0" {
		t.Errorf("heading %+v", h)
	}
}

func TestDocumentCancel(t *testing.T) {
	src := `<doc><sec><t>T</t><d>&lt;a&gt;</d></sec></doc>`
	p, err := LoadProfile(strings.NewReader(docProf))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	opts := &gofastcore.Options{Child: func(ctx context.Context, name, mime string, r io.ReaderAt, size int64, opts *gofastcore.Options) (*gofastcore.Document, error) {
		cancel()
		return nil, ctx.Err()
	}}
	if _, err := (&Extractor{Profiles: []*Profile{p}}).Extract(ctx, strings.NewReader(src), int64(len(src)), opts); !errors.Is(err, context.Canceled) {
		t.Errorf("err %v", err)
	}
}

func TestDocumentRuleRefused(t *testing.T) {
	for _, r := range []string{
		`{"match": "d", "as": "document"}`,
		`{"match": "d", "as": "document", "media_type": "text/plain"}`,
		`{"match": "d", "as": "document", "media_type": {"attr": "type", "map": {"html": "text/xml"}}}`,
		`{"match": "d", "as": "document", "media_type": {"attr": "type", "map": {}}}`,
		`{"match": "d", "as": "document", "media_type": {"attr": "type"}}`,
		`{"match": "d", "as": "document", "media_type": {"map": {"html": "text/html"}}}`,
		`{"match": "d", "as": "document", "media_type": "text/html", "when": "always"}`,
		`{"match": "d", "as": "document", "media_type": "text/html", "when": true}`,
		`{"match": "d", "as": "block", "kind": "paragraph", "when": "markup"}`,
		`{"match": "d", "as": "document", "media_type": "text/html", "kind": "paragraph"}`,
	} {
		prof := head("bad", `{"root": "doc"}`) + `"rules": [` + r + `]}`
		if _, err := LoadProfile(strings.NewReader(prof)); err == nil {
			t.Errorf("loaded: %s", r)
		}
	}
}

// TestDocumentMany checks that many children are handed over one by one,
// in file order.
func TestDocumentMany(t *testing.T) {
	var b strings.Builder
	b.WriteString("<doc>")
	for i := range 200 {
		fmt.Fprintf(&b, `<sec id="s%d"><t>T%d</t><d>&lt;p&gt;%d&lt;/p&gt;</d></sec>`, i, i, i)
	}
	b.WriteString("</doc>")
	var got []handed
	doc := docExtract(t, b.String(), &got, nil, nil)
	for i, c := range doc.Children {
		if c.Parts[0].Blocks[0].Text != fmt.Sprintf("<p>%d</p>", i) || blocks(doc)[i].Attrs[AttrChildren] != strconv.Itoa(i) {
			t.Fatalf("child %d", i)
		}
	}
}
