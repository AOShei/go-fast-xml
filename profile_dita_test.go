package gofastxml

import (
	"strings"
	"testing"

	"github.com/AOShei/go-fast-core"
)

func TestDitaCensus(t *testing.T) { jatsCensus(t, "xml-dita") }

func TestDitaFixture(t *testing.T) {
	doc, data := jatsExtractFixture(t, "dita/task.dita", "dita")
	want := `# body
heading1:Installing ^install
paragraph:How to install. ^install
paragraph:Have Java. ^install
list_item1(1.):Unzip the zip. ^install
paragraph:Anywhere. ^install
list_item1(2.):Run it. ^install
paragraph:Done. ^install
code:dita --help ^install
table: ^install
paragraph:Reused. ^install
# stored
note:install
note:Java
`
	if got := jatsBriefAll(doc); got != want {
		t.Errorf("blocks:\n%s\nwant:\n%s", got, want)
	}
	if md := doc.Metadata; md.Title != "Installing" || strings.Join(md.Authors, "|") != "Writer" || strings.Join(md.Keywords, "|") != "setup" {
		t.Errorf("metadata %+v", md)
	}
	bs := blocks(doc)
	if bs[2].Attrs["dita.in"] != "prereq" || bs[4].Attrs["dita.step"] != "info" || bs[6].Attrs["dita.note"] != "tip" ||
		bs[7].Attrs["language"] != "bash" || bs[9].Attrs["dita.conref"] != "common.dita#c/p1" || bs[0].Attrs["dita.type"] != "task" {
		t.Errorf("blocks %s", dump(doc))
	}
	if tb := bs[8].Table; tb == nil || mustJSON(tb.Rows) != `[["Option"],["-o"]]` || tb.HeaderRows != 1 {
		t.Errorf("table %+v", bs[8])
	}
	if st := doc.Parts[1].Blocks; st[0].Attrs[gofastcore.AttrStored] != gofastcore.StoredCustomXML || st[1].Attrs["href"] != "java.dita" {
		t.Errorf("stored %s", dump(doc))
	}
	if bad := ditaCase.cite(data, doc); bad != "" {
		t.Error(bad)
	}
	m, mdata := jatsExtractFixture(t, "dita/guide.ditamap", "dita")
	wantMap := `# body
heading1:Guide ^guide
list_item1(install.dita):Installing ^guide
list_item2:sub/more.dita ^guide
list_item1:https://example.org ^guide
`
	if got := jatsBriefAll(m); got != wantMap {
		t.Errorf("map blocks:\n%s\nwant:\n%s", got, wantMap)
	}
	mb := blocks(m)
	if mb[1].Attrs["dita.href"] != "install.dita" || mb[1].Attrs["dita.navtitle"] != "Install" || mb[2].Attrs["dita.scope"] != "local" ||
		mb[3].Attrs["dita.keys"] != "site" || mb[3].Attrs["dita.scope"] != "external" {
		t.Errorf("map %s", dump(m))
	}
	ditaPrep(m)
	if bad := ditaCase.cite(mdata, m); bad != "" {
		t.Error(bad)
	}
	for _, d := range []*gofastcore.Document{doc, m} {
		for _, w := range d.AllWarnings() {
			if w.Code == WarnUnmappedElements {
				t.Errorf("unmapped: %s", w.Message)
			}
		}
	}
}

var ditaCase = &jatsOracleCase{profile: "dita", format: "xml-dita", skip: map[string]string{
	// Index terms are stored data (custom_xml), not body text.
	"indexterm": "stored",
	// Metadata of the prolog and of a map's topicmeta.
	"keywords/keyword": "Metadata.Keywords", "prodinfo": "metadata", "topicmeta/author": "Metadata.Authors",
}, prep: ditaPrep}

// ditaPrep takes out of the oracles what a map's references take from
// their attributes: the Marker (href or navtitle) and, for a reference
// with no text of its own, the text the Marker became.
func ditaPrep(doc *gofastcore.Document) {
	for pi := range doc.Parts {
		for bi := range doc.Parts[pi].Blocks {
			b := &doc.Parts[pi].Blocks[bi]
			if b.Kind != gofastcore.KindListItem {
				continue
			}
			if b.Marker != "" && (b.Marker == b.Attrs["dita.href"] || b.Marker == b.Attrs["dita.navtitle"]) {
				b.Marker = ""
			}
			if b.Marker == "" && b.Text != "" && (b.Text == b.Attrs["dita.href"] || b.Text == b.Attrs["dita.navtitle"]) {
				b.Attrs[AttrAttribute] = "href"
			}
		}
	}
}

func TestDitaCorpus(t *testing.T) { ditaCase.run(t) }

func TestDitaOracles(t *testing.T) { ditaCase.oracles(t) }
