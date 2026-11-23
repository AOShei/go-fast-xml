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

// legifranceSpec is the legifrance oracle: META and CONTEXTE are
// metadata; the links of VERSIONS and STRUCT are blocks whose text is an
// attribute.
var legifranceSpec = &aknOracleSpec{
	skip: map[string]string{
		"META_COMMUN": "metadata", "META_ARTICLE": "metadata", "META_TEXTE_CHRONICLE": "metadata", "/ID": "metadata", "META_CONTENEUR": "metadata",
		"META_TEXTE_VERSION/TITRE": "metadata", "ETAT": "metadata", "DATE_DEBUT": "metadata", "DATE_FIN": "metadata",
		"AUTORITE": "metadata", "MINISTERE": "metadata", "MCS_TXT": "metadata", "ENTREPRISE": "metadata",
		"CONTEXTE": "metadata (lf.texte.*, lf.contexte)",
	},
	outOfLine:  map[string]bool{},
	blocks:     map[string]bool{"p": true, "LIEN": true, "LIEN_SECTION_TA": true, "li": true, "blockquote": true},
	labels:     map[string]bool{},
	items:      map[string]bool{"li": true},
	closers:    map[string]bool{"ul": true, "ol": true, "table": true},
	cells:      map[string]bool{"td": true, "th": true},
	attrMarker: map[gofastcore.Kind]bool{},
}

func TestLegifranceCorpus(t *testing.T) { aknCorpus(t, "xml-legifrance", "legifrance") }

func TestLegifranceOracles(t *testing.T) { aknOracles(t, "xml-legifrance", legifranceSpec) }

// TestLegifranceSample checks every rule kind the legifrance profile
// uses on a synthetic article, with exact blocks.
func TestLegifranceSample(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "profiles", "legifrance", "sample.xml"))
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
	if m.Extra[ExtraProfile] != "legifrance" || m.Extra["lf.id"] != "LEGIARTI000000000001" || m.Extra["lf.num"] != "L1-1" ||
		m.Extra["lf.etat"] != "VIGUEUR" || m.Extra["lf.date_debut"] != "2026-01-01" || m.Extra["lf.texte.cid"] != "LEGITEXT000000000002" ||
		m.Extra["lf.contexte"] != "Partie législative > Livre Ier" || m.Extra["lf.texte.titre"] != "Code de l'exemple" ||
		len(m.Keywords) != 1 || m.Keywords[0] != "exemple" {
		t.Errorf("metadata %+v", m)
	}
	var b strings.Builder
	for _, p := range doc.Parts {
		fmt.Fprintf(&b, "[%s %s]\n%s", p.Kind, p.Name, brief(p.Blocks))
	}
	want := `[body ]
paragraph:L1-1
paragraph:Le présent code s'applique aux 1 exemples.
table:
paragraph:Conformément à la loi.
paragraph:LOI n°2025-1 du 31 décembre 2025 - art. 1
`
	if got := b.String(); got != want {
		t.Errorf("blocks:\n%s\nwant:\n%s", got, want)
	}
	bs := blocks(doc)
	if x := bs[0]; x.Attrs["lf.id"] != "LEGIARTI000000000001" || x.Attrs["lf.version_etat"] != "VIGUEUR" || x.Attrs[AttrAttribute] != "num" || x.Attrs["lf.group"] != "VERSIONS" {
		t.Errorf("version %+v", x)
	}
	if x := bs[1]; x.Attrs["lf.field"] != "BLOC_TEXTUEL" || len(x.Spans) != 1 || x.Spans[0].Style != gofastcore.StyleSuper {
		t.Errorf("text %+v", x)
	}
	if got := mustJSON(bs[2].Table.Rows); got != `[["Cas","Taux"],["aucun",""]]` {
		t.Errorf("table %s", got)
	}
	if x := bs[3]; x.Attrs["lf.field"] != "NOTA" {
		t.Errorf("nota %+v", x)
	}
	if x := bs[4]; x.Attrs["lf.typelien"] != "CREATION" || x.Attrs["lf.sens"] != "cible" || x.Attrs["lf.id"] != "LEGIARTI000000000006" || x.Attrs["lf.group"] != "LIENS" {
		t.Errorf("link %+v", x)
	}
	if bad := aknCiteCheck(data, doc); bad != "" {
		t.Error(bad)
	}
	src, _ := legifranceSpec.sourceText(data, map[string]bool{})
	if got := legifranceSpec.docText(doc); got != src {
		t.Errorf("oracle (a):\n doc:  %s\n file: %s", got, src)
	}
}
