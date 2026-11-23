package gofastxml

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/AOShei/go-fast-core"
)

// rowLocCheck checks the row locations of every table of doc against the
// file src: each non-empty location lies inside the table block's Loc and
// inside the file, starts at a "<", and the rows are in file order, none
// overlapping the one before. It returns the first problem, or "", and
// the number of rows checked.
func rowLocCheck(src []byte, doc *gofastcore.Document) (string, int) {
	rows := 0
	for pi := range doc.Parts {
		for bi := range doc.Parts[pi].Blocks {
			b := &doc.Parts[pi].Blocks[bi]
			if b.Table == nil || len(b.Table.RowLocs) == 0 {
				continue
			}
			where := fmt.Sprintf("part %d block %d", pi, bi)
			if len(b.Table.RowLocs) != len(b.Table.Rows) {
				return fmt.Sprintf("%s: %d row locations for %d rows", where, len(b.Table.RowLocs), len(b.Table.Rows)), rows
			}
			tStart, tEnd := b.Loc.Offset, b.Loc.Offset+b.Loc.Length
			prevEnd := int64(-1)
			for r, l := range b.Table.RowLocs {
				if l == (gofastcore.Location{}) {
					continue
				}
				rows++
				end := l.Offset + l.Length
				switch {
				case l.Offset < 0 || l.Length <= 0 || end > int64(len(src)):
					return fmt.Sprintf("%s row %d: %+v outside the file (%d bytes)", where, r, l, len(src)), rows
				case l.Offset < tStart || end > tEnd:
					return fmt.Sprintf("%s row %d: %+v outside the table's [%d,%d)", where, r, l, tStart, tEnd), rows
				case l.Offset < prevEnd:
					return fmt.Sprintf("%s row %d: starts at %d, before the end of the row before (%d)", where, r, l.Offset, prevEnd), rows
				case src[l.Offset] != '<':
					return fmt.Sprintf("%s row %d: the source there is %q", where, r, around(string(src[l.Offset:end]), 0)), rows
				}
				prevEnd = end
			}
		}
	}
	return "", rows
}

// The check finds each kind of fault.
func TestRowLocCheck(t *testing.T) {
	src := []byte(`<t><r>a</r><r>b</r></t>`)
	doc := func(locs ...gofastcore.Location) *gofastcore.Document {
		rows := make([][]string, len(locs))
		return &gofastcore.Document{Parts: []gofastcore.Part{{Blocks: []gofastcore.Block{{Kind: gofastcore.KindTable,
			Loc: gofastcore.Location{Offset: 0, Length: int64(len(src))}, Table: &gofastcore.Table{Rows: rows, RowLocs: locs}}}}}}
	}
	r1, r2 := gofastcore.Location{Offset: 3, Length: 8}, gofastcore.Location{Offset: 11, Length: 8}
	if bad, n := rowLocCheck(src, doc(gofastcore.Location{}, r1, r2)); bad != "" || n != 2 {
		t.Errorf("good rows: %q, %d", bad, n)
	}
	for name, d := range map[string]*gofastcore.Document{
		"order":   doc(r2, r1),
		"file":    doc(gofastcore.Location{Offset: 20, Length: 10}),
		"table":   doc(gofastcore.Location{Offset: 0, Length: 3}),
		"markup":  doc(gofastcore.Location{Offset: 4, Length: 3}),
		"overlap": doc(r1, gofastcore.Location{Offset: 6, Length: 5}),
	} {
		if name == "table" {
			d.Parts[0].Blocks[0].Loc = gofastcore.Location{Offset: 3, Length: 16}
		}
		if bad, _ := rowLocCheck(src, d); bad == "" {
			t.Errorf("%s: not found", name)
		}
	}
}

// TestCorpusRowLocs runs rowLocCheck over every dev file of every XML
// corpus.
func TestCorpusRowLocs(t *testing.T) {
	formats := []string{"xml-akn", "xml-cda", "xml-clml", "xml-dita", "xml-docbook", "xml-formex", "xml-jats",
		"xml-legifrance", "xml-patent", "xml-s1000d", "xml-servicepublic", "xml-tei"}
	seen := false
	start := time.Now()
	for _, format := range formats {
		dir := jatsCorpusDir("dev", format)
		if dir == "" {
			continue
		}
		seen = true
		files, tables, rows, fails := 0, 0, 0, 0
		for _, path := range jatsCorpusFiles(dir) {
			if time.Since(start) > 90*time.Second {
				t.Logf("stopped after 90 s, at %s", format)
				return
			}
			fi, err := os.Stat(path)
			if err != nil || fi.Size() > 64<<20 {
				continue
			}
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			doc, err := ExtractBytes(ctx, data, nil)
			cancel()
			if err != nil {
				if !errors.Is(err, gofastcore.ErrMalformed) {
					t.Errorf("%s: %v", path, err)
				}
				continue
			}
			files++
			for _, p := range doc.Parts {
				for _, b := range p.Blocks {
					if b.Table != nil && len(b.Table.RowLocs) > 0 {
						tables++
					}
				}
			}
			bad, n := rowLocCheck(data, doc)
			rows += n
			if bad != "" && fails < 5 {
				fails++
				t.Errorf("%s: %s", path, bad)
			}
		}
		t.Logf("%s: %d files, %d tables with row locations, %d rows checked", format, files, tables, rows)
	}
	if !seen {
		t.Skip("no dev corpus")
	}
}
