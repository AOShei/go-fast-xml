package gofastxml

import (
	"strings"
	"testing"

	"github.com/AOShei/go-fast-core"
)

// Each row of a table says where its element is in the file, so that a
// row can be cited: rows from child cells, rows from attributes, CALS
// rows; a header row the profile gives has no place.
func TestRowLocs(t *testing.T) {
	check := func(t *testing.T, src string, tb *gofastcore.Table, wantTag string, skip int) {
		t.Helper()
		if len(tb.RowLocs) != len(tb.Rows) {
			t.Fatalf("%d row locations for %d rows", len(tb.RowLocs), len(tb.Rows))
		}
		for r, l := range tb.RowLocs {
			if r < skip {
				if l != (gofastcore.Location{}) {
					t.Errorf("row %d, from the profile, has a place: %+v", r, l)
				}
				continue
			}
			if l.Length <= 0 || l.Offset+l.Length > int64(len(src)) {
				t.Fatalf("row %d: %+v outside the source", r, l)
			}
			got := src[l.Offset : l.Offset+l.Length]
			if !strings.HasPrefix(got, "<"+wantTag) || !strings.HasSuffix(got, ">") {
				t.Errorf("row %d: source there is %q", r, got)
			}
			for _, cell := range tb.Rows[r] {
				if cell != "" && !strings.Contains(got, cell) {
					t.Errorf("row %d: cell %q not in %q", r, cell, got)
				}
			}
		}
	}
	t.Run("attributes", func(t *testing.T) {
		prof := head("attrs", `{"root": "Rs"}`) + `"rules": [
			{"match": "Rs", "as": "table", "header": ["from", "to"]},
			{"match": "R", "as": "row", "anchor": {"attr": "id"}, "cells": [{"attr": "from"}, {"attr": "to"}]}]}`
		src := `<Rs><R id="r1" from="F1" to="N1"/>
<R from="F2" to="N2"></R><R id="r3" from="F3"/></Rs>`
		doc := extractWith(t, prof, src, nil)
		bs := blocks(doc)
		if len(bs) != 1 || bs[0].Table == nil {
			t.Fatalf("blocks:\n%s", brief(bs))
		}
		tb := bs[0].Table
		if len(tb.Rows) != 4 {
			t.Fatalf("rows %s", mustJSON(tb.Rows))
		}
		check(t, src, tb, "R", 1)
		if tb.RowLocs[1].Anchor != "r1" || tb.RowLocs[2].Anchor != "" || tb.RowLocs[3].Anchor != "r3" {
			t.Errorf("anchors %+v", tb.RowLocs)
		}
		if bs[0].Origin != gofastcore.OriginNative {
			t.Errorf("origin %v", bs[0].Origin)
		}
	})
	t.Run("cells", func(t *testing.T) {
		prof := head("cells", `{"root": "doc"}`) + `"rules": [
			{"match": "T", "as": "table"},
			{"match": "Row", "as": "row"},
			{"match": "C", "as": "cell"}]}`
		src := `<doc><T><Row><C>a</C><C>b</C></Row>` + "\n" + `<Row><C>c</C><C>d</C></Row></T></doc>`
		doc := extractWith(t, prof, src, nil)
		check(t, src, blocks(doc)[0].Table, "Row", 0)
	})
	t.Run("cals", func(t *testing.T) {
		prof := head("cals", `{"root": "doc"}`) + `"rules": [
			{"match": "table", "as": "table", "model": "cals"}]}`
		src := `<doc><table><tgroup cols="2"><thead><row><entry>h1</entry><entry>h2</entry></row></thead>` +
			`<tbody><row><entry>a</entry><entry>b</entry></row><row><entry>c</entry><entry>d</entry></row></tbody></tgroup></table></doc>`
		doc := extractWith(t, prof, src, nil)
		check(t, src, blocks(doc)[0].Table, "row", 0)
	})
}
