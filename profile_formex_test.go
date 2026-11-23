package gofastxml

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AOShei/go-fast-core"
)

// formexSpec is the formex oracle: the bibliographic data is metadata;
// the .doc.xml records hold only the title as body text.
var formexSpec = &aknOracleSpec{
	skip: map[string]string{
		"BIB.INSTANCE": "metadata", "BIB.DOC": "metadata", "PUBLICATION.REF": "metadata", "FMX": "metadata",
		"ITEM.REF": "metadata",
	},
	outOfLine: map[string]bool{"NOTE": true},
	blocks: map[string]bool{"P": true, "TI": true, "STI": true, "TI.ART": true, "STI.ART": true, "ALINEA": true, "TXT": true,
		"PREAMBLE.INIT": true, "PREAMBLE.FINAL": true, "GR.CONSID.INIT": true, "VISA": true, "PL.DATE": true, "ITEM": true,
		"CONSID": true, "DLIST.ITEM": true, "QUOT.S": true, "ANNOTATION": true},
	labels:     map[string]bool{"NO.P": true, "NO.PARAG": true},
	items:      map[string]bool{"ITEM": true, "CONSID": true, "DLIST.ITEM": true},
	closers:    map[string]bool{"LIST": true, "DLIST": true, "TBL": true},
	splits:     map[string]bool{"TBL": true, "INCL.ELEMENT": true},
	cells:      map[string]bool{"CELL": true},
	attrMarker: map[gofastcore.Kind]bool{},
	// Quotation marks written by code point ("value" with "codepoint").
	attrChar: map[string]string{"QUOT.START": "CODE", "QUOT.END": "CODE"},
}

func TestFormexCorpus(t *testing.T) { aknCorpus(t, "xml-formex", "formex") }

func TestFormexOracles(t *testing.T) { aknOracles(t, "xml-formex", formexSpec) }

// TestFormexSample checks every rule kind the formex profile uses on a
// synthetic regulation, with exact blocks.
func TestFormexSample(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "profiles", "formex", "sample.xml"))
	if err != nil {
		t.Fatal(err)
	}
	doc, err := ExtractBytes(context.Background(), data, &gofastcore.Options{Spans: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := doc.Validate(); err != nil {
		t.Fatal(err)
	}
	m := doc.Metadata
	if m.Extra[ExtraProfile] != "formex" || m.Language != "EN" || m.Extra["fx.date"] != "20260102" ||
		m.Extra["fx.document_ref"] != "L_2026001EN.01000101.doc.xml" || m.Extra["fx.oj.no.oj"] != "1" || m.Extra["fx.root"] != "ACT" {
		t.Errorf("metadata %+v", m)
	}
	var b strings.Builder
	for _, p := range doc.Parts {
		fmt.Fprintf(&b, "[%s %s]\n%s", p.Kind, p.Name, brief(p.Blocks))
	}
	want := `[body ]
footnote:Text with EEA relevance.
heading1:Regulation (EU) 2026/1 of 1 January 2026 on samples
[chapter PREAMBLE]
paragraph:THE EUROPEAN COMMISSION,
paragraph:Having regard to the Treaty,
paragraph:Whereas:
list_item1((1)):Samples are useful.
paragraph:HAS ADOPTED THIS REGULATION:
[body ENACTING.TERMS]
heading2:CHAPTER I
paragraph:General provisions
heading3:Article 1
paragraph:Subject matter
footnote:OJ L 1, 1.1.2026, p. 1.
paragraph(1.):This Regulation lays down rules.
paragraph(2.):It applies to:
(a) apples;
(b) pears.
heading3:Article 2
paragraph:Rates:
caption:Table 1
table:
[chapter FINAL]
paragraph:This Regulation shall be binding in its entirety.
paragraph:Done at Brussels, 1 January 2026.
paragraph:For the Commission
`
	if got := b.String(); got != want {
		t.Errorf("blocks:\n%s\nwant:\n%s", got, want)
	}
	var all []gofastcore.Block
	for _, x := range doc.AllBlocks() {
		all = append(all, *x)
	}
	find := func(text string) gofastcore.Block {
		for _, x := range all {
			if strings.HasPrefix(x.Text, text) {
				return x
			}
		}
		t.Fatalf("no block %q", text)
		return gofastcore.Block{}
	}
	if x := find("This Regulation lays"); x.Attrs[AttrAnchorParent] != "001.001" {
		t.Errorf("paragraph %+v", x)
	}
	if x := find("OJ L 1"); x.Loc.Anchor != "E0002" || x.Attrs["fx.numbering"] != "ARAB" {
		t.Errorf("note %+v", x)
	}
	if x := find("Subject matter"); x.Attrs["fx.element"] != "STI.ART" || x.Attrs[AttrAnchorParent] != "001" {
		t.Errorf("subject %+v", x)
	}
	if x := find("Regulation (EU)"); len(x.Spans) != 1 || x.Spans[0].Style != gofastcore.StyleItalic {
		t.Errorf("title %+v", x)
	}
	tb := find("")
	for _, x := range all {
		if x.Kind == gofastcore.KindTable {
			tb = x
		}
	}
	if got := mustJSON(tb.Table.Rows); got != `[["Fruit","Rate"],["none",""]]` || tb.Table.HeaderRows != 1 || tb.Loc.Anchor != "0001" {
		t.Errorf("table %s %+v", got, tb.Table)
	}
	if bad := aknCiteCheck(data, doc); bad != "" {
		t.Error(bad)
	}
	src, _ := formexSpec.sourceText(data, map[string]bool{})
	if got := formexSpec.docText(doc); got != src {
		t.Errorf("oracle (a):\n doc:  %s\n file: %s", got, src)
	}
}
