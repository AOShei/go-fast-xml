package gofastxml

import (
	"context"
	"flag"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/AOShei/go-fast-core"
)

func TestPubmedCensus(t *testing.T) { jatsCensus(t, "xml-pubmed") }

// pubmedDump writes every block of the body: kind, level, marker, text
// (a table's rows one per line, cells joined by " | "), the anchor, and
// the Attrs, sorted; a table's RowLocs as "@offset+length" per row.
func pubmedDump(doc *gofastcore.Document, src []byte) string {
	var b strings.Builder
	for _, p := range doc.Parts {
		for _, x := range p.Blocks {
			b.WriteString(x.Kind.String())
			if x.Level > 0 {
				b.WriteString(strconv.Itoa(int(x.Level)))
			}
			if x.Marker != "" {
				b.WriteString("(" + x.Marker + ")")
			}
			if x.Loc.Anchor != "" {
				b.WriteString(" #" + x.Loc.Anchor)
			}
			keys := make([]string, 0, len(x.Attrs))
			for k := range x.Attrs {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				b.WriteString(" " + k + "=" + x.Attrs[k])
			}
			if x.Kind == gofastcore.KindTable {
				b.WriteString("\n")
				for i, row := range x.Table.Rows {
					b.WriteString("  " + strings.Join(row, " | "))
					if l := x.Table.RowLocs[i]; l.Length > 0 {
						b.WriteString("  <" + string(src[l.Offset:min(l.Offset+l.Length, l.Offset+24)]))
					}
					b.WriteString("\n")
				}
				continue
			}
			b.WriteString(": " + x.Text + "\n")
		}
	}
	return b.String()
}

// pubmedCase: the PMID that opens a record is its heading and its
// anchor, so no character data of the file is left out of oracle (a).
var pubmedCase = &jatsOracleCase{profile: "pubmed", format: "xml-pubmed", prep: pubmedPrep}

// pubmedPrep leaves in the document only text the file holds as
// character data, for oracle (a): the profile's header rows, the name
// column of name and value tables and their attribute rows, the id
// column of a rest table, the Source written after an identifier
// ("with"), and a paragraph's Marker (AbstractText@Label) are removed.
func pubmedPrep(doc *gofastcore.Document) {
	for pi := range doc.Parts {
		bs := doc.Parts[pi].Blocks
		for i := range bs {
			x := &bs[i]
			if x.Kind == gofastcore.KindParagraph {
				x.Marker = ""
			}
			if x.Kind != gofastcore.KindTable || x.Attrs[AttrHeaderSource] != "profile" || len(x.Table.Rows) == 0 {
				continue
			}
			head := x.Table.Rows[0]
			var rows [][]string
			for _, row := range x.Table.Rows[1:] {
				switch {
				case len(head) == 2 || len(head) == 3 && head[1] == "name":
					// name, value (with the id first in a rest table).
					n := len(head)
					if len(row) < n || strings.Contains(row[n-2], "@") {
						continue
					}
					rows = append(rows, []string{row[n-1]})
				default:
					// An author: the identifier's Source follows it in
					// the identifier and affiliation columns.
					row = append([]string(nil), row...)
					for _, c := range []int{5, 6} {
						if c < len(row) {
							row[c] = pubmedSourceRE.ReplaceAllString(row[c], "")
						}
					}
					rows = append(rows, row)
				}
			}
			tb := *x.Table
			tb.Rows = rows
			x.Table = &tb
		}
	}
}

// pubmedSourceRE is an Identifier@Source written after its identifier,
// the values the DTD's documentation and the dev set give.
var pubmedSourceRE = regexp.MustCompile(` (?:ORCID|ROR|GRID|ISNI|RINGGOLD)\b`)

func TestPubmedCorpus(t *testing.T) { pubmedCase.run(t) }

func TestPubmedOracles(t *testing.T) { pubmedCase.oracles(t) }

var pubmedUpdate = flag.Bool("pubmed.update", false, "rewrite testdata/profiles/pubmed/*.want")

// pubmedFixture reads a fixture with the built-in profiles and compares
// its dump and its warnings with the .want file beside it, written by
// hand from the fixture and checked line by line (-pubmed.update writes
// it from the output, to be reviewed before it is committed).
func pubmedFixture(t *testing.T, name string) *gofastcore.Document {
	t.Helper()
	doc, data := jatsExtractFixture(t, "pubmed/"+name+".xml", "pubmed")
	got := pubmedDump(doc, data)
	for _, w := range doc.AllWarnings() {
		got += "warning " + w.Code + ": " + w.Message + "\n"
	}
	wantPath := filepath.Join("testdata", "profiles", "pubmed", name+".want")
	if *pubmedUpdate {
		if err := os.WriteFile(wantPath, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(wantPath)
	if err != nil {
		t.Fatal(err)
	}
	if got != string(want) {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
	if bad := pubmedCase.cite(data, doc); bad != "" {
		t.Errorf("cite: %s", bad)
	}
	return doc
}

func TestPubmedFixtureBook(t *testing.T) { pubmedFixture(t, "book") }

// The profile is chosen by the root PubmedArticleSet in no namespace,
// with or without the DOCTYPE; in a namespace, or another root of the
// DTD, the file stays generic.
func TestPubmedSelection(t *testing.T) {
	for _, c := range []struct{ src, want string }{
		{`<PubmedArticleSet><PubmedArticle><MedlineCitation><PMID>1</PMID></MedlineCitation></PubmedArticle></PubmedArticleSet>`, "pubmed"},
		{`<!DOCTYPE PubmedArticleSet PUBLIC "-//NLM//DTD PubMedArticle, 1st January 2025//EN" "x.dtd"><PubmedArticleSet/>`, "pubmed"},
		{`<PubmedArticleSet xmlns="urn:example"><PubmedArticle/></PubmedArticleSet>`, ""},
		{`<p:PubmedArticleSet xmlns:p="urn:example"/>`, ""},
		{`<PubmedArticle><MedlineCitation><PMID>1</PMID></MedlineCitation></PubmedArticle>`, ""},
		{`<MedlineCitationSet><MedlineCitation><PMID>1</PMID></MedlineCitation></MedlineCitationSet>`, ""},
	} {
		doc, err := ExtractBytes(context.Background(), []byte(c.src), nil)
		if err != nil {
			t.Fatal(err)
		}
		if got := doc.Metadata.Extra[ExtraProfile]; got != c.want {
			t.Errorf("%s: profile %q, want %q", c.src, got, c.want)
		}
	}
}

func TestPubmedFixtureArticle(t *testing.T) {
	doc := pubmedFixture(t, "article")
	if doc.Metadata.Title != "" || len(doc.Metadata.Authors) != 0 {
		t.Errorf("metadata filled from a record: %+v", doc.Metadata)
	}
}
