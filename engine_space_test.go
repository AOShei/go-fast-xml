package gofastxml

import (
	"strings"
	"testing"

	"github.com/AOShei/go-fast-core"
)

// TestProfileInlineSpace checks the inline "space" key: the fields of a
// name or a citation written with no white space between them are set
// off by one space, in a block, in a metadata value and in a value read
// from a child; white space the source has is not doubled; spans cover
// the element's text only.
func TestProfileInlineSpace(t *testing.T) {
	prof := head("space", `{"root": "doc"}`) + `"rules": [
		{"match": "/doc", "as": "transparent"},
		{"match": "contrib/name", "as": "metadata", "field": "author"},
		{"match": "contrib", "as": "block", "kind": "paragraph"},
		{"match": "surname", "as": "inline", "space": true},
		{"match": "given", "as": "inline", "space": true},
		{"match": "ref", "as": "block", "kind": "footnote", "attrs": {"x.first": {"child": "first"}}},
		{"match": "year", "as": "inline", "space": true, "style": "bold"},
		{"match": "title", "as": "inline", "space": true},
		{"match": "b", "as": "inline", "style": "bold"}]}`
	src := `<doc><contrib><name><surname>Krusche</surname><given>Adele</given></name>1</contrib>` +
		`<ref><first><surname>Lee</surname><given>K</given></first><surname>Smith</surname><given>J</given><year>2010</year><title>On <b>x</b></title>.</ref>` +
		`<ref><surname>Roe</surname> <given>P</given> (<year>2011</year>)</ref></doc>`
	doc := extractWith(t, prof, src, &gofastcore.Options{Spans: true})
	wantBrief(t, doc, "paragraph:1\nfootnote:Smith J 2010 On x .\nfootnote:Roe P ( 2011 )\n")
	if got := strings.Join(doc.Metadata.Authors, "|"); got != "Krusche Adele" {
		t.Errorf("authors %q", got)
	}
	bs := blocks(doc)
	if bs[1].Attrs["x.first"] != "Lee K" {
		t.Errorf("child value %v", bs[1].Attrs)
	}
	if sp := bs[1].Spans; len(sp) != 2 || bs[1].Text[sp[0].Start:sp[0].End] != "2010" || bs[1].Text[sp[1].Start:sp[1].End] != "x" {
		t.Errorf("spans %+v", sp)
	}
	if _, err := LoadProfile(strings.NewReader(head("bad", `{"root": "doc"}`) + `"rules": [{"match": "a", "as": "inline", "space": "yes"}]}`)); err == nil {
		t.Error("space must be a boolean")
	}
	if _, err := LoadProfile(strings.NewReader(head("bad", `{"root": "doc"}`) + `"rules": [{"match": "a", "as": "block", "kind": "paragraph", "space": true}]}`)); err == nil {
		t.Error("space is for inline rules only")
	}
}
