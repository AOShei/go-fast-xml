package gofastxml

import (
	"strings"
	"testing"
)

// Tests of the phase 3 integration: what the three profile branches'
// engine changes do together.

// TestProfileLabelBeforeTableInBlock: a label that is the first thing in
// a paragraph after a table split it, followed by another table, is not
// lost: it is a block of its own before the table, in file order.
func TestProfileLabelBeforeTableInBlock(t *testing.T) {
	prof := head("tablelabel", `{"root": "doc"}`) + `"rules": [
		{"match": "doc", "as": "transparent"},
		{"match": "p", "as": "block", "kind": "paragraph"},
		{"match": "mod", "as": "inline"},
		{"match": "tblock", "as": "section"},
		{"match": "num", "as": "label"},
		{"match": "table", "as": "table"},
		{"match": "tr", "as": "row"},
		{"match": "td", "as": "cell"}]}`
	src := `<doc><p>for Table 1 <mod><tblock><num>TABLE 1</num><table><tr><td>a</td></tr></table></tblock>` +
		`<tblock><num>TABLE 1A</num><table><tr><td>b</td></tr></table></tblock></mod></p></doc>`
	doc := extractWith(t, prof, src, nil)
	wantBrief(t, doc, "paragraph:for Table 1 TABLE 1\ntable:\nparagraph:TABLE 1A\ntable:\n")
}

// TestProfileSplit: a rule with "split" met inside an open paragraph ends
// it there and is read on its own; text after it is a new block. A
// nested block without it is still a line of the paragraph; in a cell a
// split element is read into the cell.
func TestProfileSplit(t *testing.T) {
	prof := head("split", `{"root": "doc"}`) + `"rules": [
		{"match": "doc", "as": "transparent"},
		{"match": "p", "as": "block", "kind": "paragraph"},
		{"match": "fig", "as": "transparent", "split": true},
		{"match": "fig/label", "as": "label"},
		{"match": "caption", "as": "block", "kind": "caption"},
		{"match": "fn", "as": "block", "kind": "footnote", "split": true},
		{"match": "q", "as": "block", "kind": "quote"},
		{"match": "table", "as": "table"},
		{"match": "tr", "as": "row"},
		{"match": "td", "as": "cell"}]}`
	src := `<doc><p>See <fig><label>Fig. 1</label><caption>A cat</caption></fig> and<fn>Note</fn> more<q>joined</q></p>` +
		`<table><tr><td>a<fn>in cell</fn></td></tr></table></doc>`
	doc := extractWith(t, prof, src, nil)
	wantBrief(t, doc, "paragraph:See\ncaption(Fig. 1):A cat\nparagraph:and\nfootnote:Note\nparagraph:more\njoined\ntable:\n")
	if got := blocks(doc)[5].Table.Rows[0][0]; got != "a\nin cell" {
		t.Errorf("cell %q", got)
	}
	if _, err := LoadProfile(strings.NewReader(head("bad", `{"root": "doc"}`) + `"rules": [{"match": "a", "as": "block", "kind": "footnote", "split": true, "out_of_line": true}]}`)); err == nil {
		t.Error("split with out_of_line loaded")
	}
	if _, err := LoadProfile(strings.NewReader(head("bad", `{"root": "doc"}`) + `"rules": [{"match": "a", "as": "inline", "split": true}]}`)); err == nil {
		t.Error("split on an inline loaded")
	}
}

// TestProfileBreak: a "break" rule writes a line break inside a block as
// core's hard break "\n", as go-fast-html does for <br>: white space
// around it dropped, two breaks two "\n", none at a block's ends; in a
// cell too; in a metadata value or a label a space; between blocks
// nothing.
func TestProfileBreak(t *testing.T) {
	prof := head("brk", `{"root": "doc"}`) + `"rules": [
		{"match": "doc", "as": "transparent"},
		{"match": "title", "as": "metadata", "field": "title"},
		{"match": "p", "as": "block", "kind": "paragraph"},
		{"match": "num", "as": "label"},
		{"match": "br", "as": "break"},
		{"match": "table", "as": "table"},
		{"match": "tr", "as": "row"},
		{"match": "td", "as": "cell"}]}`
	src := `<doc><title>Two<br/>lines</title><br/><p><br/>EDI<br/>über <br/> x<br/><br/>y<br/></p>` +
		`<p><num>1<br/>a</num>text</p><table><tr><td>c<br/>d</td></tr></table></doc>`
	doc := extractWith(t, prof, src, nil)
	wantBrief(t, doc, "paragraph:EDI\nüber\nx\n\ny\nparagraph(1 a):text\ntable:\n")
	if doc.Metadata.Title != "Two lines" {
		t.Errorf("title %q", doc.Metadata.Title)
	}
	if got := blocks(doc)[2].Table.Rows[0][0]; got != "c\nd" {
		t.Errorf("cell %q", got)
	}
	if _, err := LoadProfile(strings.NewReader(head("bad", `{"root": "doc"}`) + `"rules": [{"match": "br", "as": "break", "style": "bold"}]}`)); err == nil {
		t.Error("a break with a style loaded")
	}
}

// TestProfileNothingDropped: (a) the attribute of a child as a value
// source, and the attributes of unmapped elements inside a metadata
// value kept as notes; (b) a label is not lost before an auto-numbered
// item, a block with no other text, or a table; (c) titles after the
// first in Extra.
func TestProfileNothingDropped(t *testing.T) {
	prof := head("nodrop", `{"root": "doc"}`) + `"rules": [
		{"match": "doc", "as": "transparent"},
		{"match": "title", "as": "metadata", "field": "title"},
		{"match": "subject", "as": "metadata", "field": "subject"},
		{"match": "target", "as": "metadata", "field": "extra", "key": "x.target",
			"extra": {"x.gender": {"attr": "code", "child": "patient/gender"}, "x.root": {"attr": "root", "child": "tid"}}},
		{"match": "tid", "as": "transparent"},
		{"match": "sec", "as": "section", "attrs": {"x.code": {"attr": "code", "child": "code"}}},
		{"match": "sec/code", "as": "transparent"},
		{"match": "h", "as": "heading", "level": 1},
		{"match": "p", "as": "block", "kind": "paragraph"},
		{"match": "num", "as": "label"},
		{"match": "ol", "as": "list", "ordered": true},
		{"match": "li", "as": "item"},
		{"match": "table", "as": "table"},
		{"match": "tr", "as": "row"},
		{"match": "td", "as": "cell"}]}`
	src := `<doc><title>One</title><title>Two</title><subject>S1</subject><subject>S2</subject>` +
		`<target><tid root="1.2.3"/><patient><name>Ann</name><gender code="F" system="x"/><birth value="1950"/></patient></target>` +
		`<sec><code code="11348-0"/><h>Hist</h>` +
		`<num>(a)</num><ol><li>item</li></ol>` +
		`<p><num>2</num></p>` +
		`<num>Table 1</num><table><tr><td>c</td></tr></table></sec></doc>`
	doc := extractWith(t, prof, src, nil)
	wantBrief(t, doc, "note~:x\nnote~:1950\nheading1:Hist\nlist_item1((a)):item\nparagraph:2\nparagraph:Table 1\ntable:\n")
	x := doc.Metadata.Extra
	if doc.Metadata.Title != "One" || x["title"] != "Two" || doc.Metadata.Subject != "S1" || x["subject"] != "S2" {
		t.Errorf("title %q subject %q extra %v", doc.Metadata.Title, doc.Metadata.Subject, x)
	}
	if x["x.target"] != "Ann" || x["x.gender"] != "F" || x["x.root"] != "1.2.3" {
		t.Errorf("extra %v", x)
	}
	if bs := blocks(doc); bs[2].Attrs["x.code"] != "11348-0" {
		t.Errorf("section attrs %v", bs[2].Attrs)
	}
	for _, w := range doc.AllWarnings() {
		if w.Code == WarnUnmappedAttributes && strings.Contains(w.Message, "gender@code") {
			t.Errorf("read attribute counted unread: %s", w.Message)
		}
	}
}

// TestProfileNoteInTableInBlock: an out_of_line note inside a table that
// split a paragraph is in file order: the table's caption, the note, the
// table, then the paragraph's text after the table.
func TestProfileNoteInTableInBlock(t *testing.T) {
	prof := head("tablenote", `{"root": "doc"}`) + `"rules": [
		{"match": "doc", "as": "transparent"},
		{"match": "P", "as": "block", "kind": "paragraph"},
		{"match": "TBL", "as": "table"},
		{"match": "TBL/TITLE", "as": "block", "kind": "caption"},
		{"match": "NOTE", "as": "block", "kind": "footnote", "out_of_line": true},
		{"match": "ROW", "as": "row"},
		{"match": "CELL", "as": "cell"}]}`
	src := `<doc><P>See:<TBL><TITLE>Table 2</TITLE><NOTE>A note</NOTE><ROW><CELL>c</CELL></ROW></TBL>after end</P></doc>`
	doc := extractWith(t, prof, src, nil)
	wantBrief(t, doc, "paragraph:See:\ncaption:Table 2\nfootnote:A note\ntable:\nparagraph:after end\n")
}
