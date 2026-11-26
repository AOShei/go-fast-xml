package gofastxml

import (
	"strings"
	"testing"

	"github.com/AOShei/go-fast-core"
)

// TestProfilePairs: a "pairs" table writes every value inside its element
// as a row of the file's name for it and its text, attributes as rows of
// their own after their element's; a "rest_table" does the same for what
// a row's column cells do not take, after the table, with the row's id.
func TestProfilePairs(t *testing.T) {
	prof := head("pairs", `{"root": "Doc"}`) + `"rules": [
		{"match": "/Doc", "as": "transparent"},
		{"match": "Party", "as": "table", "pairs": "path", "header": ["name", "value"], "mark": {"x.party": "p"}},
		{"match": "Ref", "as": "table", "pairs": "path", "header": ["name", "value"]},
		{"match": "Journal", "as": "table", "pairs": "local"},
		{"match": "Lines", "as": "table", "header": ["id", "amount"]},
		{"match": "Line", "as": "row", "rest_table": {"names": "path", "id": 1, "header": ["line", "name", "value"]}},
		{"match": "Line/ID", "as": "cell", "column": 1},
		{"match": "Line/Price/Amount", "as": "cell", "column": 2, "with": [{"attr": "currency"}]},
		{"match": "Total", "as": "table", "one_row": true, "header": ["sum"], "rest_table": {"names": "local"}},
		{"match": "Total/Sum", "as": "cell", "column": 1},
		{"match": "Name", "as": "block", "kind": "paragraph"}]}`
	src := `<Doc><Party><Name>Acme</Name><Tax><ID scheme="VA">FR1</ID></Tax><Addr><Country>FR</Country></Addr></Party>` +
		`<Ref k="1">R-9</Ref>` +
		`<Journal><Title>T</Title><Issue><Volume>3</Volume></Issue></Journal>` +
		`<Lines><Line><ID>1</ID><Price><Amount currency="EUR">4.50</Amount><Base unit="C62">1</Base></Price><Note>n</Note></Line>` +
		`<Line><Note>m</Note><ID>2</ID></Line></Lines>` +
		`<Total><Sum>9</Sum><Paid>1</Paid></Total><Name>after</Name></Doc>`
	doc := extractWith(t, prof, src, nil)
	if err := doc.Validate(); err != nil {
		t.Fatal(err)
	}
	if bad := citeCheck([]byte(src), doc); bad != "" {
		t.Errorf("cite: %s", bad)
	}
	bs := blocks(doc)
	want := [][][]string{
		{{"name", "value"}, {"Party/Name", "Acme"}, {"Party/Tax/ID", "FR1"}, {"Party/Tax/ID/@scheme", "VA"}, {"Party/Addr/Country", "FR"}},
		{{"name", "value"}, {"Ref", "R-9"}, {"Ref/@k", "1"}},
		{{"Title", "T"}, {"Volume", "3"}},
		{{"id", "amount"}, {"1", "4.50 EUR"}, {"2", ""}},
		{{"line", "name", "value"}, {"1", "Price/Base", "1"}, {"1", "Price/Base/@unit", "C62"}, {"1", "Note", "n"}, {"2", "Note", "m"}},
		{{"sum"}, {"9"}},
		{{"Paid", "1"}},
	}
	if len(bs) != len(want)+1 {
		t.Fatalf("%d blocks:\n%s", len(bs), dump(doc))
	}
	for i, w := range want {
		b := bs[i]
		if b.Table == nil || mustJSON(b.Table.Rows) != mustJSON(w) {
			t.Errorf("block %d: %s, want %s", i, dump(doc), mustJSON(w))
			continue
		}
		if len(b.Table.RowLocs) != len(w) {
			t.Errorf("block %d: %d RowLocs", i, len(b.Table.RowLocs))
		}
	}
	if bs[len(want)].Text != "after" {
		t.Errorf("last block %q", bs[len(want)].Text)
	}
	if bs[0].Attrs["x.party"] != "p" {
		t.Errorf("party table attrs %v", bs[0].Attrs)
	}
	// Each row cites the value's element.
	cite := func(b gofastcore.Block, row int) string {
		l := b.Table.RowLocs[row]
		return src[l.Offset : l.Offset+l.Length]
	}
	for _, c := range []struct {
		block, row int
		want       string
	}{
		{0, 2, `<ID scheme="VA">FR1</ID>`}, {0, 3, `<ID scheme="VA">FR1</ID>`}, {1, 2, `<Ref k="1">R-9</Ref>`},
		{4, 1, `<Base unit="C62">1</Base>`}, {4, 4, `<Note>m</Note>`}, {6, 0, `<Paid>1</Paid>`},
	} {
		if got := cite(bs[c.block], c.row); got != c.want {
			t.Errorf("block %d row %d cites %q, want %q", c.block, c.row, got, c.want)
		}
	}
	for _, w := range doc.AllWarnings() {
		if w.Code == WarnUnmappedElements || w.Code == WarnUnmappedAttributes {
			t.Errorf("warning %s: %s", w.Code, w.Message)
		}
	}
	for _, bad := range []string{
		`{"match": "T", "as": "table", "pairs": "names"}`,
		`{"match": "T", "as": "table", "pairs": true}`,
		`{"match": "T", "as": "table", "pairs": "path", "one_row": true}`,
		`{"match": "T", "as": "table", "pairs": "path", "model": "cals"}`,
		`{"match": "T", "as": "table", "rest_table": {"names": "path"}}`,
		`{"match": "T", "as": "row", "rest_table": {"names": "path", "other": 1}}`,
		`{"match": "T", "as": "row", "rest_table": {"id": 1}}`,
		`{"match": "T", "as": "row", "rest_table": {"names": "path", "id": 65}}`,
		`{"match": "T", "as": "row", "rest": 2, "rest_table": {"names": "path"}}`,
		`{"match": "T", "as": "block", "pairs": "path"}`,
		`{"match": "T", "as": "cell", "rest_table": {"names": "path"}}`,
	} {
		if _, err := LoadProfile(strings.NewReader(head("bad", `{"root": "x"}`) + `"rules": [` + bad + `]}`)); err == nil {
			t.Errorf("%s loaded", bad)
		}
	}
}

// TestProfilePairsParent: "pairs": "parent" names every value from the
// element the table's element stands in, so that a table that is one
// value of a record says whose it is.
func TestProfilePairsParent(t *testing.T) {
	prof := head("pairsparent", `{"root": "Doc"}`) + `"rules": [
		{"match": "/Doc", "as": "transparent"},
		{"match": "Project", "as": "transparent"},
		{"match": "Project/*", "as": "table", "pairs": "parent"},
		{"match": "Project/Name", "as": "heading", "level": 1},
		{"match": "/Doc/Top", "as": "table", "pairs": "parent"}]}`
	src := `<Doc><Project><ID s="x">7</ID><Name>N</Name><Class><Code list="cpv">15</Code></Class></Project><Top>t</Top></Doc>`
	doc := extractWith(t, prof, src, nil)
	if err := doc.Validate(); err != nil {
		t.Fatal(err)
	}
	if bad := citeCheck([]byte(src), doc); bad != "" {
		t.Errorf("cite: %s", bad)
	}
	var got []string
	for _, b := range blocks(doc) {
		if b.Kind == gofastcore.KindTable {
			for _, row := range b.Table.Rows {
				got = append(got, strings.Join(row, "="))
			}
		}
	}
	want := "Project/ID=7 Project/ID/@s=x Project/Class/Code=15 Project/Class/Code/@list=cpv Doc/Top=t"
	if strings.Join(got, " ") != want {
		t.Errorf("got  %s\nwant %s", strings.Join(got, " "), want)
	}
}

// TestProfilePairsEmpty: with "pairs_empty" an element with no content
// and no attributes, whose presence is the value (TED's PT_OPEN), is a
// row with an empty value; without it, it gives nothing.
func TestProfilePairsEmpty(t *testing.T) {
	src := `<Doc><Proc><PT_OPEN/><GPA></GPA><Lang V="EN"/><Date>2022</Date><Box> </Box></Proc></Doc>`
	for _, c := range []struct{ key, want string }{
		{`, "pairs_empty": true`, "Proc/PT_OPEN= Proc/GPA= Proc/Lang/@V=EN Proc/Date=2022 Proc/Box="},
		{``, "Proc/Lang/@V=EN Proc/Date=2022"},
	} {
		prof := head("pairsempty", `{"root": "Doc"}`) + `"rules": [
		{"match": "/Doc", "as": "transparent"},
		{"match": "Proc", "as": "table", "pairs": "path"` + c.key + `}]}`
		doc := extractWith(t, prof, src, nil)
		if err := doc.Validate(); err != nil {
			t.Fatal(err)
		}
		var got []string
		for _, b := range blocks(doc) {
			if b.Kind == gofastcore.KindTable {
				for i, row := range b.Table.Rows {
					got = append(got, strings.Join(row, "="))
					if l := b.Table.RowLocs[i]; l.Length == 0 {
						t.Errorf("row %v has no location", row)
					}
				}
			}
		}
		if strings.Join(got, " ") != c.want {
			t.Errorf("%q: got  %s\nwant %s", c.key, strings.Join(got, " "), c.want)
		}
	}
}
