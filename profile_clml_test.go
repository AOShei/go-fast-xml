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

// clmlSpec is the clml oracle: Dublin Core values are metadata, the
// EUR-Lex record is stored.
var clmlSpec = &aknOracleSpec{
	prefixes: map[string]string{"http://purl.org/dc/elements/1.1/": "dc", "http://www.w3.org/1999/xhtml": "html",
		"http://www.legislation.gov.uk/namespaces/metadata": "ukm", "http://purl.org/dc/terms/": "dct"},
	skip: map[string]string{
		"dc:title": "metadata", "dc:identifier": "metadata", "dc:language": "metadata", "dc:publisher": "metadata",
		"dc:modified": "metadata", "dc:subject": "metadata", "dc:description": "metadata", "dc:contributor": "metadata",
		"dc:format": "metadata", "dc:type": "metadata", "dct:valid": "metadata", "dct:alternative": "metadata",
		"ukm:EURLexMetadata": "stored custom_xml", "dc:source": "metadata", "dc:date": "metadata",
	},
	outOfLine: map[string]bool{},
	blocks: map[string]bool{"Text": true, "Title": true, "Number": true, "LongTitle": true, "DateText": true, "Subtitle": true,
		"ListItem": true, "PersonName": true, "JobTitle": true, "Department": true, "AddressLine": true, "DateSigned": true,
		"LSseal": true, "Reference": true, "TableText": true},
	labels:     map[string]bool{"Pnumber": true},
	items:      map[string]bool{"ListItem": true},
	closers:    map[string]bool{"UnorderedList": true, "OrderedList": true, "html:table": true},
	cells:      map[string]bool{"html:td": true, "html:th": true},
	attrMarker: map[gofastcore.Kind]bool{gofastcore.KindListItem: true},
	attrText:   map[string]string{"ListItem": "NumberOverride"},
}

func TestClmlCorpus(t *testing.T) { aknCorpus(t, "xml-clml", "clml") }

func TestClmlOracles(t *testing.T) { aknOracles(t, "xml-clml", clmlSpec) }

// TestClmlSample checks every rule kind the clml profile uses on a
// synthetic act, with exact blocks.
func TestClmlSample(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "profiles", "clml", "sample.xml"))
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
	if m.Extra[ExtraProfile] != "clml" || m.Title != "Sample Act 2026" || m.Extra["ukm:Year"] != "2026" ||
		m.Extra["ukm:EnactmentDate"] != "2026-01-02" || m.Extra["clml.link"] != "http://www.legislation.gov.uk/ukpga/2026/1/data.xml" ||
		m.Extra["clml.id_uri"] != "http://www.legislation.gov.uk/id/ukpga/2026/1" {
		t.Errorf("metadata %+v", m)
	}
	var b strings.Builder
	for _, p := range doc.Parts {
		fmt.Fprintf(&b, "[%s %s]\n%s", p.Kind, p.Name, brief(p.Blocks))
	}
	want := `[chapter PrimaryPrelims]
heading1:Sample Act 2026
paragraph:2026 CHAPTER 1
paragraph:An Act to make provision about samples.
[body Body]
heading2(Part 1):Samples
heading3:Meaning of sample
paragraph:1
paragraph(1):A sample is anything listed in the 2025 Act.
paragraph(2):Samples include—
paragraph(a):apples, and
list_item1:pears
table:
[chapter Commentaries]
footnote:Words substituted by S.I. 2026/5.
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
	if x := find("A sample"); x.Attrs[AttrAnchorParent] != "section-1-1" || len(x.Spans) != 2 || x.Spans[1].Href != "http://www.legislation.gov.uk/id/ukpga/2025/9" {
		t.Errorf("paragraph %+v", x)
	}
	if x := find("Samples"); x.Attrs[AttrAnchorParent] != "part-1" || x.Attrs["clml.document_uri"] != "http://www.legislation.gov.uk/ukpga/2026/1/part/1" {
		t.Errorf("part heading %+v", x)
	}
	if x := find("Words substituted"); x.Loc.Anchor != "c1" || x.Attrs["clml.commentary_type"] != "F" {
		t.Errorf("commentary %+v", x)
	}
	for _, x := range all {
		if x.Kind == gofastcore.KindTable {
			if got := mustJSON(x.Table.Rows); got != `[["Fruit","Rate"],["apple","1"]]` || x.Table.HeaderRows != 1 {
				t.Errorf("table %s %+v", got, x.Table)
			}
		}
	}
	if bad := aknCiteCheck(data, doc); bad != "" {
		t.Error(bad)
	}
	src, _ := clmlSpec.sourceText(data, map[string]bool{})
	if got := clmlSpec.docText(doc); got != src {
		t.Errorf("oracle (a):\n doc:  %s\n file: %s", got, src)
	}
}
