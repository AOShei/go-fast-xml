package gofastxml

import (
	"context"
	"strings"
	"testing"
)

// TestRowLocsInFileBytes: a table's row locations are offsets in the
// file, as block locations are, in a file not in UTF-8 too (windows-1252
// with non-ASCII text before the rows: each byte 0xE9 is two bytes of the
// decoded text).
func TestRowLocsInFileBytes(t *testing.T) {
	for name, src := range map[string]string{
		"xliff": "<?xml version=\"1.0\" encoding=\"windows-1252\"?>\n<xliff version=\"1.2\" xmlns=\"urn:oasis:names:tc:xliff:document:1.2\"><file original=\"caf\xe9\xe9\xe9.txt\" source-language=\"fr\"><header><note>\xe9t\xe9 \xe9t\xe9</note></header><body>" +
			"<trans-unit id=\"1\"><source>\xe9l\xe8ve</source></trans-unit><trans-unit id=\"2\"><source>caf\xe9</source></trans-unit></body></file></xliff>",
		"tmx": "<?xml version=\"1.0\" encoding=\"windows-1252\"?>\n<tmx version=\"1.4\"><header srclang=\"fr\" creationtool=\"\xe9\xe9\xe9\"/><body>" +
			"<tu tuid=\"1\"><tuv xml:lang=\"fr\"><seg>\xe9l\xe8ve</seg></tuv></tu><tu tuid=\"2\"><tuv xml:lang=\"fr\"><seg>caf\xe9</seg></tuv></tu></body></tmx>",
	} {
		doc, err := ExtractBytes(context.Background(), []byte(src), nil)
		if err != nil {
			t.Fatal(err)
		}
		if p := doc.Metadata.Extra[ExtraProfile]; p != name {
			t.Fatalf("%s: profile %q", name, p)
		}
		rows := 0
		for _, b := range blocks(doc) {
			if b.Table == nil {
				continue
			}
			for i, l := range b.Table.RowLocs {
				if i < b.Table.HeaderRows {
					continue
				}
				raw := src[l.Offset : l.Offset+l.Length]
				if !strings.HasPrefix(raw, "<t") || !strings.HasSuffix(raw, "</trans-unit>") && !strings.HasSuffix(raw, "</tu>") {
					t.Errorf("%s: row %d at %+v is %q", name, i, l, raw)
				}
				rows++
			}
		}
		if rows != 2 {
			t.Errorf("%s: %d rows", name, rows)
		}
	}
}
