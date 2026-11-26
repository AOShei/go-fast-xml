package gofastxml

import (
	"strings"
	"testing"

	"github.com/AOShei/go-fast-core"
)

// anchor_of: a heading takes the anchor its record read from a child.
func TestAnchorOf(t *testing.T) {
	prof := head("anchorof", `{"root": "set"}`) + `"rules": [
		{"match": "/set", "as": "transparent"},
		{"match": "rec", "as": "section", "anchor": {"child": "id"}},
		{"match": "rec/title", "as": "heading", "level": "nesting", "anchor": {"anchor_of": "rec"}},
		{"match": "rec/p", "as": "block", "kind": "paragraph"}]}`
	src := `<set><rec><id>A1</id><p>Before.</p><title>One</title><p>After.</p></rec><rec><title>Two</title></rec></set>`
	doc := extractWith(t, prof, src, nil)
	var got []string
	for _, b := range blocks(doc) {
		got = append(got, b.Text+"#"+b.Loc.Anchor+"^"+b.Attrs[AttrAnchorParent])
	}
	if want := "Before.#^A1 One#A1^ After.#^A1 Two#^"; strings.Join(got, " ") != want {
		t.Errorf("got %q, want %q", strings.Join(got, " "), want)
	}
	for _, bad := range []string{
		`{"match": "p", "as": "block", "kind": "paragraph", "attrs": {"k": {"anchor_of": "rec"}}}`,
		`{"match": "p", "as": "block", "kind": "paragraph", "anchor": {"anchor_of": "rec", "attr": "id"}}`,
	} {
		if _, err := LoadProfile(strings.NewReader(head("x", `{"root": "set"}`) + `"rules": [` + bad + `]}`)); err == nil {
			t.Errorf("loaded: %s", bad)
		}
	}
}

// A row with a rest_table writes its own attributes as the rest table's
// first rows for that row, named <row>/@<attribute>, cited at the row.
func TestRestTableRowAttributes(t *testing.T) {
	prof := head("rowattrs", `{"root": "list"}`) + `"rules": [
		{"match": "/list", "as": "table", "header": ["name"]},
		{"match": "list/person", "as": "row", "rest_table": {"names": "path", "id": 1, "header": ["id", "name", "value"]}},
		{"match": "person/name", "as": "cell", "column": 1}]}`
	src := `<list><person valid="Y" lang="fr"><name>Doe</name><note>x</note></person><person><name>Roe</name></person></list>`
	doc := extractWith(t, prof, src, nil)
	bs := blocks(doc)
	if len(bs) != 2 || bs[1].Kind != gofastcore.KindTable {
		t.Fatalf("blocks: %s", brief(bs))
	}
	var rows []string
	for i, r := range bs[1].Table.Rows {
		l := bs[1].Table.RowLocs[i]
		rows = append(rows, strings.Join(r, "|")+"@"+src[l.Offset:l.Offset+l.Length])
	}
	want := `id|name|value@ Doe|person/@valid|Y@<person valid="Y" lang="fr"> Doe|person/@lang|fr@<person valid="Y" lang="fr"> Doe|note|x@<note>x</note>`
	if got := strings.Join(rows, " "); got != want {
		t.Errorf("rest rows:\n%s\nwant:\n%s", got, want)
	}
	for _, w := range doc.AllWarnings() {
		if w.Code == WarnUnmappedAttributes {
			t.Errorf("unexpected %s: %s", w.Code, w.Message)
		}
	}
}
