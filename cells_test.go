package gofastxml

import (
	"strings"
	"testing"

	"github.com/AOShei/go-fast-core"
)

// TestProfileCellsFromAttributes: a row whose data is in its attributes,
// under a header of fixed text declared by the profile.
func TestProfileCellsFromAttributes(t *testing.T) {
	prof := head("attrcells", `{"root": "Redirections"}`) + `"rules": [
		{"match": "/Redirections", "as": "table", "header": ["from", "to", "toUrl"]},
		{"match": "Redirection", "as": "row", "cells": [{"attr": "from"}, {"attr": "to"}, {"attr": "toUrl"}]},
		{"match": "RedirectionExterne", "as": "row", "cells": [{"attr": "from"}, {"attr": "to"}, {"attr": "toUrl"}]}]}`
	src := `<Redirections><Redirection from="F1" to="N2" date="d"/><RedirectionExterne from="F3" toUrl="https://x/?a=1&amp;b=2"/><Redirection from=" F4 "/></Redirections>`
	doc := extractWith(t, prof, src, nil)
	bs := blocks(doc)
	if len(bs) != 1 || bs[0].Kind != gofastcore.KindTable {
		t.Fatalf("blocks:\n%s", dump(doc))
	}
	b := bs[0]
	want := [][]string{{"from", "to", "toUrl"}, {"F1", "N2", ""}, {"F3", "", "https://x/?a=1&b=2"}, {"F4", "", ""}}
	if got := mustJSON(b.Table.Rows); got != mustJSON(want) {
		t.Errorf("rows %s", got)
	}
	// Every row and cell is the file's: the table is native; only its
	// header row's text is the profile's.
	if b.Table.HeaderRows != 1 || b.Origin != gofastcore.OriginNative || b.Attrs[AttrHeaderSource] != "profile" {
		t.Errorf("header: rows %d origin %v attrs %v", b.Table.HeaderRows, b.Origin, b.Attrs)
	}
	if b.Attrs[AttrAttribute] != "from to toUrl" {
		t.Errorf("attribute %q", b.Attrs[AttrAttribute])
	}
	if b.Loc.Offset != 0 || b.Loc.Length != int64(len(src)) {
		t.Errorf("loc %+v", b.Loc)
	}
	for _, w := range doc.Warnings {
		if w.Code == "xml.unmapped_elements" || w.Code == "xml.unmapped_attributes" && strings.Contains(w.Message, "from") {
			t.Errorf("warning %s: %s", w.Code, w.Message)
		}
	}
	for _, bad := range []string{
		`{"match": "R", "as": "row", "cells": ["lit"]}`,
		`{"match": "R", "as": "row", "cells": [{"attr": "a", "of": "T", "child": "b"}]}`,
		`{"match": "S", "as": "section", "table_header": []}`,
		`{"match": "S", "as": "heading", "level": 1, "table_header": ["a"]}`,
		`{"match": "R", "as": "row", "cells": [{"child": "a"}]}`,
		`{"match": "R", "as": "row", "cells": []}`,
		`{"match": "T", "as": "table", "header": [1]}`,
		`{"match": "T", "as": "cell", "cells": [{"attr": "a"}]}`,
	} {
		if _, err := LoadProfile(strings.NewReader(head("bad", `{"root": "x"}`) + `"rules": [` + bad + `]}`)); err == nil {
			t.Errorf("%s loaded", bad)
		}
	}
}
