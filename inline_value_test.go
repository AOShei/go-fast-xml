package gofastxml

import (
	"context"
	"strings"
	"testing"

	"github.com/AOShei/go-fast-core"
)

// extractNoCite reads src with the profile and checks the document, but
// not citing: text written from an attribute is not in the span's
// character data.
func extractNoCite(t *testing.T, prof, src string, opts *gofastcore.Options) *gofastcore.Document {
	t.Helper()
	p, err := LoadProfile(strings.NewReader(prof))
	if err != nil {
		t.Fatal(err)
	}
	doc, err := (&Extractor{Profiles: []*Profile{p}}).Extract(context.Background(), strings.NewReader(src), int64(len(src)), opts)
	if err != nil {
		t.Fatal(err)
	}
	if err := doc.Validate(); err != nil {
		t.Fatalf("Validate: %v\n%s", err, dump(doc))
	}
	return doc
}

// TestInlineValue: an inline rule with "value" writes the element's
// attribute in place in the open block's text (TEI p/pb@n).
func TestInlineValue(t *testing.T) {
	prof := head("iv", `{"root": "TEI"}`) + `"rules": [
		{"match": "p", "as": "block", "kind": "paragraph"},
		{"match": "head", "as": "heading", "level": 1},
		{"match": "p/pb", "as": "inline", "value": {"attr": "n"}, "space": true},
		{"match": "head/pb", "as": "inline", "value": {"attr": "n"}, "space": true},
		{"match": "hi", "as": "inline", "style": "italic"}]}`
	src := `<TEI><head>Book<pb n="iv"/></head><p>The end<pb n="13"/>of the <hi>page</hi> and<pb/>more.</p></TEI>`
	doc := extractNoCite(t, prof, src, &gofastcore.Options{Spans: true})
	wantBrief(t, doc, "heading1:Book iv\nparagraph:The end 13 of the page and more.\n")
	p := blocks(doc)[1]
	if len(p.Spans) != 1 || p.Text[p.Spans[0].Start:p.Spans[0].End] != "page" {
		t.Errorf("spans %+v", p.Spans)
	}
	if l, at := p.Loc, int64(strings.Index(src, "<p>")); l.Offset != at || l.Offset+l.Length != int64(strings.Index(src, "</p>")+4) {
		t.Errorf("loc %+v", l)
	}
	for _, w := range doc.AllWarnings() {
		if w.Code == WarnUnmappedAttributes {
			t.Errorf("unexpected %s", w.Message)
		}
	}
}

// TestInlineValueCodepoint: {"attr", "codepoint": "hex"} writes the
// character whose code point the attribute holds in hex (Formex
// QUOT.START@CODE); surrogates, values past U+10FFFF, controls and
// anything not hex write nothing and count the attribute as unmapped.
func TestInlineValueCodepoint(t *testing.T) {
	prof := head("cp", `{"root": "ACT"}`) + `"rules": [
		{"match": "P", "as": "block", "kind": "paragraph"},
		{"match": "QUOT.START", "as": "inline", "value": {"attr": "CODE", "codepoint": "hex"}},
		{"match": "QUOT.END", "as": "inline", "value": {"attr": "CODE", "codepoint": "hex"}}]}`
	src := `<ACT><P>He said <QUOT.START CODE="2018"/>yes<QUOT.END CODE="2019"/> and <QUOT.START CODE="0022"/>no<QUOT.END CODE="1F600"/>.</P>` +
		`<P>Bad <QUOT.START CODE="D800"/>a<QUOT.END CODE="110000"/>b<QUOT.START CODE="000A"/>c<QUOT.END CODE="zz"/>d<QUOT.START CODE=""/>e<QUOT.END CODE="0085"/>.</P></ACT>`
	doc := extractNoCite(t, prof, src, nil)
	wantBrief(t, doc, "paragraph:He said ‘yes’ and \"no\U0001F600.\nparagraph:Bad abcde.\n")
	var unmapped string
	for _, w := range doc.AllWarnings() {
		if w.Code == WarnUnmappedAttributes {
			unmapped = w.Message
		}
	}
	if !strings.Contains(unmapped, "QUOT.START@CODE") || !strings.Contains(unmapped, "QUOT.END@CODE") {
		t.Errorf("unmapped attributes: %q", unmapped)
	}
}

// TestInlineValueRefused: an inline's value is the element's own
// attribute; a literal, a child or an ancestor's attribute is refused.
func TestInlineValueRefused(t *testing.T) {
	for _, v := range []string{`"text"`, `{"child": "x"}`, `{"structure": "x"}`, `{"attr": "a", "of": "p"}`, `{"attr": "a", "child": "x"}`,
		`{"attr": "a", "codepoint": "dec"}`, `{"child": "x", "codepoint": "hex"}`} {
		prof := head("bad", `{"root": "r"}`) + `"rules": [{"match": "q", "as": "inline", "value": ` + v + `}]}`
		if _, err := LoadProfile(strings.NewReader(prof)); err == nil {
			t.Errorf("%s: loaded", v)
		}
	}
	prof := head("ok", `{"root": "r"}`) + `"rules": [{"match": "q", "as": "block", "kind": "paragraph", "value": {"attr": "a", "codepoint": "hex"}}]}`
	if _, err := LoadProfile(strings.NewReader(prof)); err != nil {
		t.Error(err)
	}
}
