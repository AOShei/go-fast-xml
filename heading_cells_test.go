package gofastxml

import (
	"strings"
	"testing"

	"github.com/AOShei/go-fast-core"
)

// TestProfileSectionHeading: a section's heading from its own attribute,
// the first of a list present and not empty, located at the start tag and
// marked as attribute text.
func TestProfileSectionHeading(t *testing.T) {
	prof := head("sechead", `{"root": "r"}`) + `"rules": [
		{"match": "/r", "as": "transparent"},
		{"match": "f", "as": "section", "heading": [{"attr": "original"}, {"attr": "id"}], "anchor": {"attr": "id"}},
		{"match": "p", "as": "block", "kind": "paragraph"}]}`
	src := `<r><f id="f1" original="a.txt"><p>One</p><f id="f2"><p>Two</p></f></f><f original=""><p>Three</p></f></r>`
	doc := extractWith(t, prof, src, nil)
	var got []string
	for _, b := range blocks(doc) {
		s := b.Kind.String() + ":" + b.Text + " " + b.Loc.Anchor + " " + src[b.Loc.Offset:b.Loc.Offset+b.Loc.Length]
		if b.Kind == gofastcore.KindHeading {
			s += " level=" + string(rune('0'+b.Level)) + " attribute=" + b.Attrs[AttrAttribute]
		}
		got = append(got, s)
	}
	want := []string{
		`heading:a.txt f1 <f id="f1" original="a.txt"> level=1 attribute=original`,
		`paragraph:One  <p>One</p>`,
		`heading:f2 f2 <f id="f2"> level=2 attribute=id`,
		`paragraph:Two  <p>Two</p>`,
		`paragraph:Three  <p>Three</p>`,
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("got:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	for _, w := range doc.Warnings {
		if w.Code == "xml.unmapped_attributes" {
			t.Errorf("warning %s: %s", w.Code, w.Message)
		}
	}
	for _, bad := range []string{
		`{"match": "f", "as": "section", "heading": "lit"}`,
		`{"match": "f", "as": "section", "heading": {"attr": "a", "of": "r"}}`,
		`{"match": "f", "as": "section", "heading": {"child": "t"}}`,
		`{"match": "f", "as": "section", "heading": []}`,
		`{"match": "f", "as": "block", "heading": {"attr": "a"}}`,
	} {
		if _, err := LoadProfile(strings.NewReader(head("bad", `{"root": "x"}`) + `"rules": [` + bad + `]}`)); err == nil {
			t.Errorf("%s loaded", bad)
		}
	}
}

// TestProfileRowAroundRows: a row of attributes with no "rest" (an XLIFF
// group) is its start tag, ended there, so the rows inside its element
// follow it with their own locations; with none of its attributes it
// makes no row.
func TestProfileRowAroundRows(t *testing.T) {
	prof := head("grouprows", `{"root": "body"}`) + `"rules": [
		{"match": "/body", "as": "table", "header": ["id", "source", "other"]},
		{"match": "g", "as": "row", "anchor": {"attr": "id"}, "cells": [{"attr": "id"}]},
		{"match": "u", "as": "row", "anchor": {"attr": "id"}, "cells": [{"attr": "id"}], "rest": 3},
		{"match": "u/s", "as": "cell", "column": 2}]}`
	src := `<body><g id="g1"><u id="1"><s>a</s></u><g><u id="2"><s>b</s></u></g></g></body>`
	doc := extractWith(t, prof, src, nil)
	bs := blocks(doc)
	if len(bs) != 1 || bs[0].Table == nil {
		t.Fatalf("blocks:\n%s", dump(doc))
	}
	var got []string
	for i, r := range bs[0].Table.Rows {
		l := bs[0].Table.RowLocs[i]
		got = append(got, strings.Join(r, "|")+" "+src[l.Offset:l.Offset+l.Length]+" #"+l.Anchor)
	}
	want := []string{"id|source|other  #", `g1|| <g id="g1"> #g1`, `1|a| <u id="1"><s>a</s></u> #1`, `2|b| <u id="2"><s>b</s></u> #2`}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("got:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

// TestProfileCellsFromChildAttribute: a row's cell read from the first
// matching child's attribute (XLIFF target@state), which is still read by
// its own rule; the row needs "rest".
func TestProfileCellsFromChildAttribute(t *testing.T) {
	prof := head("childcells", `{"root": "body"}`) + `"rules": [
		{"match": "/body", "as": "table", "header": ["id", "state", "source", "target", "other"]},
		{"match": "u", "as": "row", "cells": [{"attr": "id"}, {"attr": "state", "child": "target"}], "rest": 5},
		{"match": "u/source", "as": "cell", "column": 3},
		{"match": "u/target", "as": "cell", "column": 4},
		{"match": "alt", "as": "transparent"},
		{"match": "alt/target", "as": "block", "kind": "paragraph"}]}`
	src := `<body><u id="1"><source>a</source><alt><target state="x">b</target></alt><target state="final">c</target></u><u id="2"><source>d</source></u></body>`
	doc := extractWith(t, prof, src, nil)
	bs := blocks(doc)
	if len(bs) != 1 || bs[0].Kind != gofastcore.KindTable {
		t.Fatalf("blocks:\n%s", dump(doc))
	}
	want := [][]string{{"id", "state", "source", "target", "other"}, {"1", "final", "a", "c", "b"}, {"2", "", "d", "", ""}}
	if got := mustJSON(bs[0].Table.Rows); got != mustJSON(want) {
		t.Errorf("rows %s", got)
	}
	if a := bs[0].Attrs[AttrAttribute]; a != "id target@state" {
		t.Errorf("attribute %q", a)
	}
	for _, w := range doc.Warnings {
		if w.Code == "xml.unmapped_attributes" && strings.Contains(w.Message, "target@state") && !strings.Contains(w.Message, "alt") {
			t.Errorf("warning %s: %s", w.Code, w.Message)
		}
	}
	for _, bad := range []string{
		`{"match": "u", "as": "row", "cells": [{"attr": "state", "child": "target"}]}`,
		`{"match": "u", "as": "row", "cells": [{"attr": "state", "codepoint": "hex"}], "rest": 2}`,
	} {
		if _, err := LoadProfile(strings.NewReader(head("bad", `{"root": "x"}`) + `"rules": [` + bad + `]}`)); err == nil {
			t.Errorf("%s loaded", bad)
		}
	}
}
