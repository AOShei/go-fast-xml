package gofastxml

import (
	"strings"
	"testing"

	"github.com/AOShei/go-fast-core"
)

func TestTeiCensus(t *testing.T) { jatsCensus(t, "xml-tei") }

func TestTeiFixture(t *testing.T) {
	doc, data := jatsExtractFixture(t, "tei/play.xml", "tei")
	want := `# body teiHeader
paragraph:Born digital.
# body body
heading1:Act 1 ^a1
heading2:Scene 2 ^a1s2
paragraph:Enter Hamlet. ^a1s2
paragraph(HAMLET):To be, or not to be #l1
paragraph:That is the question. #l2
note:12 ^a1s2
paragraph:One line ^a1s2
paragraph:Two line ^a1s2
paragraph:The teh the end
A note.. #p1
list_item1(1.):First ^a1s2
list_item1(2.):Second ^a1s2
table: ^a1s2
# stored
note:13
note:teh
note:the
`
	if got := jatsBriefAll(doc); got != want {
		t.Errorf("blocks:\n%s\nwant:\n%s", got, want)
	}
	m := doc.Metadata
	if m.Title != "A Play" || strings.Join(m.Authors, "|") != "Anon" || m.Extra["tei.publisher"] != "Test Press" ||
		m.Extra["tei.idno"] != "10.1/t" || m.Extra["tei.idno_type"] != "DOI" || m.Extra["tei.language_ident"] != "en" || m.Extra["tei.lang"] != "en" {
		t.Errorf("metadata %+v", m)
	}
	bs := blocks(doc)
	if a := bs[4].Attrs; a["tei.n"] != "1" || a["tei.who"] != "#ham" || a["tei.div1_n"] != "1" || a["tei.div2_n"] != "2" || a["tei.div2_type"] != "scene" {
		t.Errorf("line attrs %v", a)
	}
	if pb := bs[6]; pb.Attrs[gofastcore.AttrNoteType] != gofastcore.NoteAttribute || pb.Attrs[AttrAttribute] != "n" || pb.Attrs["tei.milestone"] != "pb" {
		t.Errorf("pb %+v", pb)
	}
	if tb := bs[12].Table; tb == nil || mustJSON(tb.Rows) != `[["H"],["v"]]` || tb.HeaderRows != 1 {
		t.Errorf("table %+v", bs[12])
	}
	st := doc.Parts[len(doc.Parts)-1].Blocks
	if st[0].Attrs["tei.milestone"] != "pb" || st[1].Attrs["tei.choice"] != "sic" || st[2].Attrs["tei.choice"] != "corr" {
		t.Errorf("stored %s", dump(doc))
	}
	if bad := teiCase.cite(data, doc); bad != "" {
		t.Error(bad)
	}
	for _, w := range doc.AllWarnings() {
		if w.Code == WarnUnmappedElements {
			t.Errorf("unmapped: %s", w.Message)
		}
	}
}

var teiCase = &jatsOracleCase{profile: "tei", format: "xml-tei", skip: map[string]string{
	// Metadata of the teiHeader (Metadata fields and Extra tei.*).
	"titleStmt": "metadata", "fileDesc/editionStmt": "metadata", "fileDesc/extent": "metadata",
	"fileDesc/publicationStmt": "metadata", "notesStmt": "metadata",
	"sourceDesc/bibl": "metadata", "sourceDesc/biblFull": "metadata", "sourceDesc/biblStruct": "metadata",
	"langUsage": "metadata", "textClass": "metadata", "profileDesc/textDesc": "metadata",
	"revisionDesc": "metadata",
	// Stored data: declarations of encoding.
	"encodingDesc/refsDecl": "stored",
}}

func TestTeiCorpus(t *testing.T) { teiCase.run(t) }

func TestTeiOracles(t *testing.T) { teiCase.oracles(t) }
