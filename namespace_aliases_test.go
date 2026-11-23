package gofastxml

import (
	"context"
	"strings"
	"testing"
)

// TestProfileNamespaceAliases: elements of an older namespace of a schema
// are matched by the rules written for the current one.
func TestProfileNamespaceAliases(t *testing.T) {
	prof := head("alias", `{"root": "doc", "namespace": "urn:new"}, {"root": "doc", "namespace": "urn:old"}`) +
		`"default_namespace": "urn:new", "namespace_aliases": {"urn:old": "urn:new"}, "rules": [
		{"match": "/doc", "as": "transparent"},
		{"match": "h", "as": "heading", "level": 1},
		{"match": "p", "as": "block", "kind": "paragraph"}]}`
	for _, ns := range []string{"urn:new", "urn:old"} {
		src := `<doc xmlns="` + ns + `"><h>T</h><p>Para.</p><x:p xmlns:x="urn:other">Other.</x:p></doc>`
		doc := extractWith(t, prof, src, nil)
		wantBrief(t, doc, "heading1:T\nparagraph:Para.\nparagraph~:Other.\n")
	}
	for _, bad := range []string{`{"": "urn:new"}`, `{"urn:a": "urn:a"}`, `{"urn:a": "urn:b", "urn:b": "urn:c"}`} {
		p := head("alias", `{"root": "doc"}`) + `"namespace_aliases": ` + bad + `, "rules": []}`
		if _, err := LoadProfile(strings.NewReader(p)); err == nil {
			t.Errorf("%s loaded", bad)
		}
	}
}

// TestProfileOutOfLine: a footnote met inside a paragraph is a block of
// its own, before the paragraph (made where the footnote ends), and the
// paragraph keeps its waiting label and the rest of its text. In a cell
// it stays in the cell.
func TestProfileOutOfLine(t *testing.T) {
	prof := head("ool", `{"root": "doc"}`) + `"rules": [
		{"match": "/doc", "as": "transparent"},
		{"match": "sec", "as": "section", "anchor": {"attr": "id"}},
		{"match": "num", "as": "label"},
		{"match": "p", "as": "block", "kind": "paragraph"},
		{"match": "fn", "as": "block", "kind": "footnote", "out_of_line": true, "marker": {"attr": "n"}, "anchor": {"attr": "id"}},
		{"match": "t", "as": "table"}, {"match": "tr", "as": "row"}, {"match": "td", "as": "cell"}]}`
	src := `<doc><sec id="s1"><num>(1)</num><p>Before<fn n="1" id="f1"><p>Note one.</p><p>Second.</p></fn> after.</p>` +
		`<t><tr><td>In<fn n="2">inline</fn></td></tr></t></sec></doc>`
	// Not extractWith: the paragraph's span holds the note's text too.
	doc, err := (&Extractor{Profiles: []*Profile{mustLoad(t, prof)}}).Extract(context.Background(), strings.NewReader(src), int64(len(src)), nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := doc.Validate(); err != nil {
		t.Fatal(err)
	}
	wantBrief(t, doc, "footnote(1):Note one.\nSecond.\nparagraph((1)):Before after.\ntable:\n")
	bs := blocks(doc)
	if bs[0].Loc.Anchor != "f1" || bs[1].Attrs[AttrAnchorParent] != "s1" {
		t.Errorf("anchors %+v %+v", bs[0].Loc, bs[1].Attrs)
	}
	if c := bs[2].Table.Rows[0][0]; c != "In\ninline" {
		t.Errorf("cell %q", c)
	}
	// A label met inside a block that has text, a marker or a waiting
	// label, or inside a cell, is text of the block (it was dropped);
	// a note in a label is out of line too.
	src = `<doc><sec><num>(b)</num><p>Insert<sec><num>(4)</num><p>new text</p></sec></p>` +
		`<num>c.<fn n="3">Note.</fn></num><p>After.</p>` +
		`<t><tr><td><num>1.</num> cell</td></tr></t></sec></doc>`
	doc, err = (&Extractor{Profiles: []*Profile{mustLoad(t, prof)}}).Extract(context.Background(), strings.NewReader(src), int64(len(src)), nil)
	if err != nil {
		t.Fatal(err)
	}
	wantBrief(t, doc, "paragraph((b)):Insert (4)\nnew text\nfootnote(3):Note.\nparagraph(c.):After.\ntable:\n")
	if c := blocks(doc)[3].Table.Rows[0][0]; c != "1. cell" {
		t.Errorf("cell %q", c)
	}
	bad := head("ool", `{"root": "doc"}`) + `"rules": [{"match": "x", "as": "heading", "level": 1, "out_of_line": true}]}`
	if _, err := LoadProfile(strings.NewReader(bad)); err == nil {
		t.Error("out_of_line on a heading loaded")
	}
}
