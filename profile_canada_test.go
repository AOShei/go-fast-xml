package gofastxml

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/AOShei/go-fast-core"
)

// canadaSpec is the canada-laws oracle: the identification fields that
// are metadata only are not body text; every other element's text is.
var canadaSpec = &aknOracleSpec{
	prefixes: map[string]string{"http://www.w3.org/1998/Math/MathML": "mml"},
	skip: map[string]string{
		"Identification/ShortTitle": "metadata", "Identification/RunningHead": "metadata",
		"Identification/BillNumber": "metadata", "Identification/Parliament": "metadata",
		"Identification/BillHistory": "metadata", "Identification/Chapter": "metadata",
		"Identification/BillRefNumber": "metadata", "Identification/InstrumentNumber": "metadata",
		"Identification/RegistrationDate": "metadata", "Identification/ConsolidationDate": "metadata",
		"Identification/RegulationMakerOrder": "metadata", "Identification/StatuteYear": "metadata",
	},
	outOfLine: map[string]bool{"HistoricalNote": true, "Note": true, "MarginalNote": true, "OriginatingRef": true},
	blocks: map[string]bool{"Text": true, "MarginalNote": true, "Heading": true, "GroupHeading": true, "ScheduleFormHeading": true,
		"TitleText": true, "Repealed": true, "Reserved": true, "Footnote": true, "HistoricalNote": true, "Note": true, "OriginatingRef": true,
		"FormulaText": true, "FormulaConnector": true, "Caption": true, "title": true, "AlternateText": true,
		"BilingualItemEn": true, "BilingualItemFr": true, "SignatureLine": true, "SignatureTitle": true, "SignatureName": true,
		"LongTitle": true, "ShortTitle": true, "RunningHead": true, "Item": true, "EnablingAuthority": true, "MathMLBlock": true},
	labels:  map[string]bool{"Label": true, "FormulaTerm": true},
	items:   map[string]bool{"Item": true},
	closers: map[string]bool{"List": true, "table": true, "RecentAmendments": true},
	splits:  map[string]bool{"table": true, "Image": true},
	cells:   map[string]bool{"entry": true, "AmendmentCitation": true, "AmendmentDate": true},
}

// canadaKind is the kind of a corpus file: its name's prefix.
func canadaKind(path string) string {
	base := filepath.Base(path)
	for _, k := range []string{"canada-eng-acts", "canada-eng-regulations", "canada-fra-lois", "canada-fra-reglements",
		"canada-annual-statutes"} {
		if strings.HasPrefix(base, k) {
			return k
		}
	}
	return "other"
}

// TestCanadaCorpus reads every file of the xml-canada dev and held-out
// sets and reports aggregate counts per kind (file-name prefix), held-out
// files never named. It fails on a panic, an error, a Validate failure, a
// timeout or a file not read with the canada-laws profile.
func TestCanadaCorpus(t *testing.T) {
	deadline := time.Now().Add(90 * time.Second)
	for _, set := range []string{"dev", "heldout"} {
		dir := aknCorpusDir(set, "xml-canada")
		if dir == "" {
			t.Logf("%s: no xml-canada corpus", set)
			continue
		}
		files := spFiles(dir)
		if testing.Short() && set == "dev" {
			files = files[:min(len(files), 40)]
		}
		byKind := map[string]*tally{}
		unmapped := map[string]map[string]int{WarnUnmappedElements: {}, WarnUnmappedAttributes: {}}
		for _, path := range files {
			if time.Now().After(deadline) {
				t.Logf("%s: stopped at the time limit", set)
				break
			}
			kind := canadaKind(path)
			if byKind[kind] == nil {
				byKind[kind] = newTally()
			}
			res := corpusOne(path)
			byKind[kind].add(kind, res)
			label := path
			if set == "heldout" {
				label = "a held-out " + kind + " file"
			}
			switch {
			case res.panic != "":
				t.Errorf("%s: panic: %s", label, firstLine(res.panic, set))
			case res.timeout:
				t.Errorf("%s: no result within the time limit", label)
			case res.err != nil:
				t.Errorf("%s: error: %s", label, firstLine(res.err.Error(), set))
			case res.invalid != nil:
				t.Errorf("%s: Validate: %s", label, firstLine(res.invalid.Error(), set))
			case res.doc.Metadata.Extra[ExtraProfile] != "canada-laws":
				t.Errorf("%s: not read with the canada-laws profile", label)
			}
			if res.doc == nil || set != "dev" {
				continue
			}
			for _, w := range res.doc.AllWarnings() {
				if m, ok := unmapped[w.Code]; ok {
					for _, sm := range countRE.FindAllStringSubmatch(w.Message, -1) {
						n, _ := strconv.Atoi(sm[2])
						m[strings.TrimSpace(sm[1])] += n
					}
				}
			}
		}
		kinds := make([]string, 0, len(byKind))
		for k := range byKind {
			kinds = append(kinds, k)
		}
		sort.Strings(kinds)
		for _, k := range kinds {
			t.Logf("%s %s:\n%s", set, k, byKind[k])
		}
		if set == "dev" {
			for _, code := range []string{WarnUnmappedElements, WarnUnmappedAttributes} {
				t.Logf("dev: %s: %s", code, byCount(unmapped[code]))
			}
		}
	}
}

// TestCanadaOracles checks every dev file, English and French alike:
// (a) the body's text is the file's text apart from the metadata-only
// identification fields; (b) every block's text is at its offsets.
func TestCanadaOracles(t *testing.T) {
	if testing.Short() {
		t.Skip("reads the whole dev set")
	}
	aknOracles(t, "xml-canada", canadaSpec)
}

func canadaExtract(t *testing.T, name string) (*gofastcore.Document, []byte) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "profiles", "canada", name))
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
	return doc, data
}

// TestCanadaSample checks the rules on a synthetic act, with exact blocks.
func TestCanadaSample(t *testing.T) {
	doc, data := canadaExtract(t, "sample-act.xml")
	m := doc.Metadata
	if m.Extra[ExtraProfile] != "canada-laws" || m.Title != "An Act respecting samples" || m.Language != "en" ||
		m.Extra["lims.ShortTitle"] != "Sample Act" || m.Extra["lims.ShortTitle.status"] != "official" ||
		m.Extra["lims.Stages"] != "2026 1 2" || m.Extra["lims.Stages.stage"] != "assented-to" ||
		m.Extra["lims.ConsolidatedNumber"] != "S-0.1" || m.Extra["lims.AnnualStatuteNumber"] != "7" ||
		m.Extra["lims.pit-date"] != "2026-01-05" || m.Extra["lims.current-date"] != "2026-02-01" ||
		m.Extra["lims.id"] != "100" || m.Extra["lims.in-force"] != "yes" {
		t.Errorf("metadata %+v", m)
	}
	var b strings.Builder
	for _, p := range doc.Parts {
		fmt.Fprintf(&b, "[%s]\n%s", p.Kind, brief(p.Blocks))
	}
	want := `[body]
heading1:An Act respecting samples
paragraph:His Majesty enacts as follows:
heading1(PART 1):General
heading6:Definitions
paragraph(1):The following definitions apply in this Act.
paragraph:sample means a thing listed in the Other Act. (échantillon)
paragraph:2026, c. 7, s. 1
2026, c. 9, s. 4
paragraph:2026, c. 9, s. 5
heading6:Rates
paragraph:2
paragraph((1)):The rate is set out in the table to this section*.
paragraph((a)):for apples, per kilogram; and
table:
footnote(*):Rates in dollars.
paragraph((2)):[Repealed, 2026, c. 9, s. 6]
paragraph(3):The formula is
formula:A × B
paragraph:where
paragraph(A):is the weight; and
paragraph((b)):for pears, par unit.
paragraph:[Repealed, 2026, c. 9, s. 7]
paragraph:(Section 2)
heading2(SCHEDULE):Fruit
list_item1(1):Apples
paragraph:Apple Board
paragraph:Office des pommes
image:
table:
`
	if got := b.String(); !strings.HasPrefix(got, want) {
		t.Errorf("blocks:\n%s\nwant:\n%s", got, want)
	}
	all := doc.AllBlocks()
	find := func(text string) *gofastcore.Block {
		for _, x := range all {
			if strings.HasPrefix(x.Text, text) {
				return x
			}
		}
		t.Fatalf("no block %q", text)
		return nil
	}
	if x := find("Definitions"); x.Loc.Anchor != "109" || x.Attrs["canada.element"] != "Section" ||
		x.Attrs["lims:lastAmendedDate"] != "2026-01-05" || x.Attrs["canada.type"] != "normal" {
		t.Errorf("marginal note %+v", x)
	}
	if x := find("General"); x.Loc.Anchor != "107" || x.Attrs["canada.element"] != "Heading" {
		t.Errorf("heading %+v", x)
	}
	if x := find("sample means"); x.Attrs[AttrAnchorParent] != "110" || x.Attrs["canada.element"] != "Definition" ||
		len(x.Spans) == 0 || x.Spans[0].Href != "S-0.2" {
		t.Errorf("definition %+v", x)
	}
	if x := find("The rate is"); x.Attrs["canada.in-force"] != "no" || x.Attrs[AttrAnchorParent] != "122" ||
		x.Attrs["canada.element"] != "Subsection" {
		t.Errorf("subsection %+v", x)
	}
	// The section's number, a paragraph of its own before the numbered
	// subsection, is the section's: not the subsection's attributes.
	for _, x := range all {
		if x.Text == "2" && (x.Attrs[AttrAnchorParent] != "120" || x.Attrs["canada.in-force"] != "" || x.Attrs["canada.element"] != "") {
			t.Errorf("section number %+v", x)
		}
	}
	if x := find("Rates in"); x.Loc.Anchor != "fn1" || x.Attrs["canada.status"] != "official" {
		t.Errorf("footnote %+v", x)
	}
	if x := find("for pears"); x.Attrs["canada.amendment"] != "AmendedText" {
		t.Errorf("amended text %+v", x)
	}
	if x := find("A × B"); x.Attrs["notation"] != "plain" {
		t.Errorf("formula %+v", x)
	}
	var tables []*gofastcore.Block
	for _, x := range all {
		switch x.Kind {
		case gofastcore.KindTable:
			tables = append(tables, x)
		case gofastcore.KindImage:
			if x.Image == nil || x.Image.ID != "fruit.gif" {
				t.Errorf("image %+v", x)
			}
		}
	}
	if len(tables) != 2 {
		t.Fatalf("%d tables", len(tables))
	}
	if got := mustJSON(tables[0].Table.Rows); got != `[["Fruit","Rate"],["apple","1 $"]]` || tables[0].Table.HeaderRows != 1 ||
		tables[0].Loc.Anchor != "125" || len(tables[0].Table.RowLocs) != 2 {
		t.Errorf("table %s %+v", got, tables[0].Table)
	}
	if got := mustJSON(tables[1].Table.Rows); got != `[["2026, c. 9","2026-01-05"]]` || len(tables[1].Table.RowLocs) != 1 {
		t.Errorf("recent amendments %s %+v", got, tables[1].Table)
	}
	if bad := aknCiteCheck(data, doc); bad != "" {
		t.Error(bad)
	}
	src, _ := canadaSpec.sourceText(data, map[string]bool{})
	if got := canadaSpec.docText(doc); got != src {
		t.Errorf("oracle (a):\n doc:  %s\n file: %s", got, src)
	}
}

// TestCanadaSelection: the root names have no namespace, so the profile
// needs the lims namespace declared on the root, or a Justice Canada
// DOCTYPE (the annual statutes' Bill roots declare no lims).
func TestCanadaSelection(t *testing.T) {
	for _, c := range []struct {
		name, src, want string
	}{
		{"lims", `<Statute xmlns:lims="http://justice.gc.ca/lims" xml:lang="fr"><Body><Section><Label>1</Label><Text>Texte.</Text></Section></Body></Statute>`, "canada-laws"},
		{"regulation", `<Regulation xmlns:lims="http://justice.gc.ca/lims"><Body/></Regulation>`, "canada-laws"},
		{"bill doctype", `<!DOCTYPE Bill PUBLIC "-//Justice Canada//DTD Bill Exchange v2.5.0//EN" "BillExchange.dtd"><Bill xml:lang="en"><Body/></Bill>`, "canada-laws"},
		{"bare statute", `<Statute xml:lang="en"><Body><Section><Label>1</Label><Text>Text.</Text></Section></Body></Statute>`, ""},
		{"bare bill", `<Bill><Body/></Bill>`, ""},
		{"other namespace", `<Statute xmlns:lims="http://example.org/lims"><Body/></Statute>`, ""},
	} {
		doc, err := ExtractBytes(context.Background(), []byte(c.src), nil)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if got := doc.Metadata.Extra[ExtraProfile]; got != c.want {
			t.Errorf("%s: profile %q, want %q", c.name, got, c.want)
		}
	}
}

// TestCanadaFrench: a French file has the same structure; nothing pairs
// it with the English one.
func TestCanadaFrench(t *testing.T) {
	doc, _ := canadaExtract(t, "sample-loi.xml")
	if doc.Metadata.Language != "fr" || doc.Metadata.Title != "Loi concernant les échantillons" {
		t.Errorf("metadata %+v", doc.Metadata)
	}
	want := `heading1:Loi concernant les échantillons
heading6:Définitions
paragraph(1):Les définitions qui suivent s’appliquent à la présente loi.
paragraph:échantillon Chose énumérée. (sample)
`
	wantBrief(t, doc, want)
}
