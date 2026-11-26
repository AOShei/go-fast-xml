package gofastxml

import (
	"context"
	"flag"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/AOShei/go-fast-core"
)

// The procurement profiles (eforms, ted, boamp) write a notice's values
// as tables of names and values, so their tests share pubmed's dump and
// its preparation for oracle (a): the profile's header rows, the name
// column and the attribute rows are not character data of the file.

var procUpdate = flag.Bool("proc.update", false, "rewrite testdata/profiles/{eforms,ted,boamp}/*.want")

// procFixture reads a fixture with the built-in profiles and compares
// its dump, metadata and warnings with the .want file beside it
// (-proc.update writes it from the output, to be reviewed before it is
// committed), and checks every block and row against its span.
func procFixture(t *testing.T, c *jatsOracleCase, name string) *gofastcore.Document {
	t.Helper()
	doc, data := jatsExtractFixture(t, c.profile+"/"+name+".xml", c.profile)
	got := "title: " + doc.Metadata.Title + "\n"
	if !doc.Metadata.Created.IsZero() {
		got += "created: " + doc.Metadata.Created.Format(time.RFC3339) + "\n"
	}
	keys := make([]string, 0, len(doc.Metadata.Extra))
	for k := range doc.Metadata.Extra {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		got += "extra " + k + "=" + doc.Metadata.Extra[k] + "\n"
	}
	got += pubmedDump(doc, data)
	for _, w := range doc.AllWarnings() {
		got += "warning " + w.Code + ": " + w.Message + "\n"
	}
	wantPath := filepath.Join("testdata", "profiles", c.profile, name+".want")
	if *procUpdate {
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
	if bad := c.cite(data, doc); bad != "" {
		t.Errorf("cite: %s", bad)
	}
	return doc
}

// procDocText is jatsDocText for profiles with no lists: the text of
// the body's blocks and cells in order, white space removed, nothing
// taken off a cell's start.
func procDocText(doc *gofastcore.Document) string {
	var b strings.Builder
	for _, p := range doc.Parts {
		if p.Kind == gofastcore.PartStored {
			continue
		}
		for _, x := range p.Blocks {
			if x.Attrs[gofastcore.AttrStored] != "" || x.Attrs[AttrAttribute] != "" || x.Kind == gofastcore.KindImage {
				continue
			}
			b.WriteString(noSpace(x.Marker))
			if x.Kind == gofastcore.KindTable {
				for _, row := range x.Table.Rows {
					for _, c := range row {
						b.WriteString(noSpace(c))
					}
				}
				continue
			}
			b.WriteString(noSpace(x.Text))
		}
	}
	return b.String()
}

// procSelection checks which profile each source is read with.
func procSelection(t *testing.T, cases map[string]string) {
	t.Helper()
	for src, want := range cases {
		doc, err := ExtractBytes(context.Background(), []byte(src), nil)
		if err != nil {
			t.Fatal(err)
		}
		if got := doc.Metadata.Extra[ExtraProfile]; got != want {
			t.Errorf("%s: profile %q, want %q", src, got, want)
		}
	}
}

// eformsCase: the values of the notice's own header (its identifier,
// version, type, dates) go to Metadata, not to the body.
var eformsCase = &jatsOracleCase{profile: "eforms", format: "xml-eforms", prep: pubmedPrep, mixed: true, rawCells: true, skip: map[string]string{
	"/*/UBLVersionID":             "Extra eforms.ubl_version",
	"/*/CustomizationID":          "Extra eforms.customization_id",
	"/*/ProfileID":                "Extra eforms.profile_id",
	"/*/ID":                       "Extra eforms.notice_id",
	"/*/ContractFolderID":         "Extra eforms.contract_folder_id",
	"/*/IssueDate":                "Metadata.Created",
	"/*/IssueTime":                "Extra eforms.issue_time",
	"/*/VersionID":                "Extra eforms.version_id",
	"/*/RegulatoryDomain":         "Extra eforms.regulatory_domain",
	"/*/NoticeTypeCode":           "Extra eforms.notice_type",
	"/*/NoticeLanguageCode":       "Extra eforms.notice_language",
	"/*/RequestedPublicationDate": "Extra eforms.requested_publication_date",
	"/*/PlannedDate":              "Extra eforms.planned_date",
}}

func TestEformsCensus(t *testing.T)  { jatsCensus(t, "xml-eforms") }
func TestEformsCorpus(t *testing.T)  { eformsCase.run(t) }
func TestEformsOracles(t *testing.T) { eformsCase.oracles(t) }

func TestEformsFixture(t *testing.T) {
	doc := procFixture(t, eformsCase, "award")
	if doc.Metadata.Title != "Fourniture de pain pour la cantine" {
		t.Errorf("title %q", doc.Metadata.Title)
	}
}

// The profile is chosen by the three notice roots in their UBL
// namespaces; the same names elsewhere, and other UBL documents, are not
// read with it.
func TestEformsSelection(t *testing.T) {
	procSelection(t, map[string]string{
		`<ContractNotice xmlns="urn:oasis:names:specification:ubl:schema:xsd:ContractNotice-2"/>`:                 "eforms",
		`<n:ContractAwardNotice xmlns:n="urn:oasis:names:specification:ubl:schema:xsd:ContractAwardNotice-2"/>`:   "eforms",
		`<PriorInformationNotice xmlns="urn:oasis:names:specification:ubl:schema:xsd:PriorInformationNotice-2"/>`: "eforms",
		`<ContractNotice/>`:                     "",
		`<ContractNotice xmlns="urn:example"/>`: "",
		`<Invoice xmlns="urn:oasis:names:specification:ubl:schema:xsd:Invoice-2"/>`:                                                    "ubl",
		`<BusinessRegistrationInformationNotice xmlns="http://data.europa.eu/p27/eforms-business-registration-information-notice/1"/>`: "",
	})
}

// tedPrep is pubmedPrep, after the language column of the two tables of
// translated titles and names, read from the LG attribute, is removed.
func tedPrep(doc *gofastcore.Document) {
	for pi := range doc.Parts {
		for i := range doc.Parts[pi].Blocks {
			x := &doc.Parts[pi].Blocks[i]
			if x.Kind != gofastcore.KindTable || len(x.Table.Rows) == 0 || x.Table.Rows[0][0] != "language" {
				continue
			}
			tb := *x.Table
			tb.Rows = nil
			for _, row := range x.Table.Rows[1:] {
				tb.Rows = append(tb.Rows, row[1:])
			}
			x.Table = &tb
			x.Attrs = map[string]string{}
		}
	}
	pubmedPrep(doc)
}

// tedCase: the country of an address written as a sentence is the VALUE
// attribute of an empty element, written where it stands.
var tedCase = &jatsOracleCase{profile: "ted", format: "xml-ted", prep: tedPrep, rawCells: true,
	attrText: map[string]string{"ADDRESS_NOT_STRUCT/COUNTRY": "VALUE"}}

func TestTedCensus(t *testing.T)  { jatsCensus(t, "xml-ted") }
func TestTedCorpus(t *testing.T)  { tedCase.run(t) }
func TestTedOracles(t *testing.T) { tedCase.oracles(t) }

func TestTedFixtureForm(t *testing.T) {
	doc := procFixture(t, tedCase, "f03")
	if doc.Metadata.Title != "Fourniture de pain pour la cantine" {
		t.Errorf("title %q", doc.Metadata.Title)
	}
}

func TestTedFixtureText(t *testing.T) { procFixture(t, tedCase, "text") }

// The profile is chosen by the root TED_EXPORT in the namespaces of the
// schema's versions; with no namespace or another, it is not.
func TestTedSelection(t *testing.T) {
	procSelection(t, map[string]string{
		`<TED_EXPORT xmlns="http://publications.europa.eu/resource/schema/ted/R2.0.9/publication"/>`:  "ted",
		`<TED_EXPORT xmlns="http://publications.europa.eu/resource/schema/ted/R2.0.8/publication"/>`:  "ted",
		`<TED_EXPORT xmlns="http://publications.europa.eu/TED_schema/Export"/>`:                       "ted",
		`<TED_EXPORT xmlns="http://formex.publications.europa.eu/ted/schema/export/R2.0.9.S01.E01"/>`: "ted",
		`<TED_EXPORT/>`:                     "",
		`<TED_EXPORT xmlns="urn:example"/>`: "",
	})
}
