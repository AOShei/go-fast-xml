package gofastxml

import (
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
