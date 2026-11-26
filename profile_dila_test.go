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

// The xml-dila corpus set holds DILA's case law (JADE, CASS, INCA, CAPP,
// CONSTIT), CNIL deliberations and KALI collective agreements; a file's
// name begins with its collection.

// dilaCollection is the collection of a corpus file: its name's prefix.
func dilaCollection(path string) string {
	c, _, _ := strings.Cut(filepath.Base(path), "-")
	return c
}

// dilaProfile is the profile a collection is read with.
var dilaProfile = map[string]string{
	"jade": "dila-juri", "cass": "dila-juri", "inca": "dila-juri", "capp": "dila-juri", "constit": "dila-juri",
	"kali": "legifrance",
}

// TestDilaCorpus reads every file of the xml-dila dev and held-out sets
// and reports aggregate counts per collection; unmapped names on dev
// only. It fails on a panic, an error, a Validate failure, a timeout or a
// file not read with its collection's profile.
func TestDilaCorpus(t *testing.T) {
	for _, set := range []string{"dev", "heldout"} {
		dir := aknCorpusDir(set, "xml-dila")
		if dir == "" {
			t.Logf("%s: no xml-dila corpus", set)
			continue
		}
		tallies := map[string]*tally{}
		unmapped := map[string]map[string]int{}
		unmappedTotal := map[string]int{}
		start := time.Now()
		for _, path := range spFiles(dir) {
			coll := dilaCollection(path)
			res := corpusOne(path)
			if tallies[coll] == nil {
				tallies[coll] = newTally()
			}
			tallies[coll].add(coll, res)
			label := path
			if set == "heldout" {
				label = "a held-out " + coll + " file"
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
			case dilaProfile[coll] != "" && res.doc.Metadata.Extra[ExtraProfile] != dilaProfile[coll]:
				t.Errorf("%s: read with %q, not %q", label, res.doc.Metadata.Extra[ExtraProfile], dilaProfile[coll])
			}
			if res.doc == nil {
				continue
			}
			for _, w := range res.doc.AllWarnings() {
				if w.Code != WarnUnmappedElements && w.Code != WarnUnmappedAttributes {
					continue
				}
				for _, sm := range countRE.FindAllStringSubmatch(w.Message, -1) {
					n, _ := strconv.Atoi(sm[2])
					unmappedTotal[coll+" "+w.Code] += n
					if set == "dev" {
						k := coll + " " + w.Code
						if unmapped[k] == nil {
							unmapped[k] = map[string]int{}
						}
						unmapped[k][strings.TrimSpace(sm[1])] += n
					}
				}
			}
		}
		colls := make([]string, 0, len(tallies))
		for c := range tallies {
			colls = append(colls, c)
		}
		sort.Strings(colls)
		var b strings.Builder
		for _, c := range colls {
			fmt.Fprintf(&b, "-- %s\n%s", c, tallies[c])
			for _, code := range []string{WarnUnmappedElements, WarnUnmappedAttributes} {
				k := c + " " + code
				fmt.Fprintf(&b, "%s: %d in all", code, unmappedTotal[k])
				if set == "dev" && len(unmapped[k]) > 0 {
					b.WriteString(": " + byCount(unmapped[k]))
				}
				b.WriteString("\n")
			}
		}
		t.Logf("%s: xml-dila in %v\n%s", set, time.Since(start).Round(time.Millisecond), b.String())
	}
}

// dilaJuriSpec is the dila-juri oracle: META is metadata but for the
// decision's title, which is also its heading.
var dilaJuriSpec = &aknOracleSpec{
	skip: map[string]string{
		"META_COMMUN": "metadata", "META_JURI_JUDI": "metadata", "META_JURI_ADMIN": "metadata", "META_JURI_CONSTIT": "metadata",
		"DATE_DEC": "metadata", "JURIDICTION": "metadata", "NUMERO": "metadata", "SOLUTION": "metadata",
	},
	outOfLine:  map[string]bool{},
	blocks:     map[string]bool{"p": true, "LIEN": true, "SCT": true, "ANA": true, "li": true, "blockquote": true},
	labels:     map[string]bool{},
	items:      map[string]bool{"li": true},
	closers:    map[string]bool{"ul": true, "ol": true, "table": true},
	splits:     map[string]bool{"table": true},
	cells:      map[string]bool{"td": true, "th": true},
	attrMarker: map[gofastcore.Kind]bool{},
	only:       func(path string) bool { return dilaProfile[dilaCollection(path)] == "dila-juri" },
}

func TestDilaJuriOracles(t *testing.T) { aknOracles(t, "xml-dila", dilaJuriSpec) }

// dilaLegifranceSpec is the legifrance oracle on the KALI and CNIL files
// of xml-dila: the metadata blocks are not body text, but for the titles
// that are also headings (TITREFULL, SECTION_TA/TITRE_TA, an IDCC's
// TITRE).
var dilaLegifranceSpec = &aknOracleSpec{
	skip: map[string]string{
		"META_COMMUN": "metadata", "META_ARTICLE": "metadata", "META_TEXTE_CHRONICLE": "metadata", "/ID": "metadata",
		"CONTEXTE": "metadata (lf.texte.*, lf.contexte)", "META_TEXTE_VERSION/TITRE": "metadata", "ETAT": "metadata",
		"DATE_DEBUT": "metadata", "DATE_FIN": "metadata", "APPLI_GEO": "metadata", "CODES_NOMENCLATURES": "metadata",
		"LIBELLE_EXTENSION": "metadata", "MCS_TXT": "metadata", "META_CONTENEUR/NUM": "metadata",
		"META_CONTENEUR/DATE_PUBLI": "metadata", "ACTS_PRO": "metadata", "NUMS_BROCH": "metadata",
	},
	outOfLine:  map[string]bool{},
	blocks:     map[string]bool{"p": true, "LIEN": true, "LIEN_SECTION_TA": true, "CONDITION_DIFFERE": true, "li": true, "blockquote": true},
	labels:     map[string]bool{},
	items:      map[string]bool{"li": true},
	closers:    map[string]bool{"ul": true, "ol": true, "table": true},
	splits:     map[string]bool{"table": true},
	cells:      map[string]bool{"td": true, "th": true},
	attrMarker: map[gofastcore.Kind]bool{},
	only:       func(path string) bool { return dilaProfile[dilaCollection(path)] == "legifrance" },
}

func TestDilaLegifranceOracles(t *testing.T) { aknOracles(t, "xml-dila", dilaLegifranceSpec) }

// TestDilaJuriSample checks the dila-juri profile on a synthetic Cour de
// cassation decision, with exact blocks.
func TestDilaJuriSample(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "profiles", "dila-juri", "sample.xml"))
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
	for k, v := range map[string]string{
		ExtraProfile: "dila-juri", "juri.root": "TEXTE_JURI_JUDI", "juri.id": "JURITEXT000000000001",
		"juri.ecli": "ECLI:FR:CCASS:2026:C100012", "juri.juridiction": "Cour de cassation", "juri.numero": "12",
		"juri.solution": "Rejet", "juri.formation": "CHAMBRE_CIVILE_1", "juri.numero_affaire": "25-10.001\n25-10.002",
		"juri.publi_bull": "Bull. 2026, I, n° 1", "juri.publi_bull_publie": "oui", "juri.president": "M. Exemple",
		"juri.rapporteur": "Mme Rapporteure", "juri.avocats": "SCP Exemple", "juri.date_dec_att": "2025-03-04",
	} {
		if m.Extra[k] != v {
			t.Errorf("Extra[%s] = %q, want %q", k, m.Extra[k], v)
		}
	}
	if !strings.HasPrefix(m.Title, "Cour de cassation, civile") || m.Created.Format("2006-01-02") != "2026-01-02" {
		t.Errorf("title %q created %v", m.Title, m.Created)
	}
	if len(doc.Parts) != 1 {
		t.Fatalf("%d parts", len(doc.Parts))
	}
	bs := blocks(doc)
	var b strings.Builder
	for _, x := range bs {
		fmt.Fprintf(&b, "%s%d:%q\n", x.Kind, x.Level, x.Text)
	}
	want := `heading1:"Cour de cassation, civile, Chambre civile 1, 2 janvier 2026, 25-10.001, Publié au bulletin"
paragraph0:"LA COUR,\n\nAttendu que l'exemple est bien fondé ;"
paragraph0:"Par ces motifs :\nREJETTE le pourvoi."
paragraph0:"EXEMPLE - Titrage"
paragraph0:"Le sommaire de l'exemple."
paragraph0:"Cass. civ. 1, 1er janvier 2020"
paragraph0:"Code civil - art. 1240"
`
	if got := b.String(); got != want {
		t.Errorf("blocks:\n%s\nwant:\n%s", got, want)
	}
	for i, x := range bs {
		if x.Attrs[AttrAnchorParent] != "JURITEXT000000000001" {
			t.Errorf("block %d anchor_parent %q", i, x.Attrs[AttrAnchorParent])
		}
	}
	if x := bs[1]; x.Attrs["juri.field"] != "BLOC_TEXTUEL" || len(x.Spans) != 1 || x.Spans[0].Style != gofastcore.StyleBold {
		t.Errorf("text %+v", x)
	}
	if x := bs[3]; x.Attrs["juri.part"] != "SCT" || x.Attrs["juri.id"] != "1" || x.Attrs["juri.type"] != "PRINCIPAL" || x.Attrs["juri.field"] != "SOMMAIRE" {
		t.Errorf("sct %+v", x)
	}
	if x := bs[4]; x.Attrs["juri.part"] != "ANA" || x.Attrs["juri.id"] != "1" {
		t.Errorf("ana %+v", x)
	}
	if x := bs[5]; x.Attrs["juri.field"] != "CITATION_JP" {
		t.Errorf("citation %+v", x)
	}
	if x := bs[6]; x.Attrs["juri.group"] != "LIENS" || x.Attrs["juri.typelien"] != "CITATION" || x.Attrs["juri.sens"] != "source" ||
		x.Attrs["juri.id"] != "LEGIARTI000006436298" || x.Attrs["juri.cidtexte"] != "LEGITEXT000006070721" ||
		x.Attrs["juri.datesignatexte"] != "1804-03-21" || x.Attrs["juri.naturetexte"] != "CODE" || x.Attrs["juri.num"] != "1240" {
		t.Errorf("link %+v", x)
	}
	if bad := aknCiteCheck(data, doc); bad != "" {
		t.Error(bad)
	}
	src, _ := dilaJuriSpec.sourceText(data, map[string]bool{})
	if got := dilaJuriSpec.docText(doc); got != src {
		t.Errorf("oracle (a):\n doc:  %s\n file: %s", got, src)
	}
}

// TestLegifranceKali checks the KALI elements of the legifrance profile
// on synthetic files: an article, a text version and an IDCC container.
func TestLegifranceKali(t *testing.T) {
	read := func(src string) (*gofastcore.Document, []gofastcore.Block) {
		t.Helper()
		doc, err := ExtractBytes(context.Background(), []byte(src), nil)
		if err != nil {
			t.Fatal(err)
		}
		if err := doc.Validate(); err != nil {
			t.Fatal(err)
		}
		if doc.Metadata.Extra[ExtraProfile] != "legifrance" {
			t.Fatalf("profile %q", doc.Metadata.Extra[ExtraProfile])
		}
		for _, w := range doc.AllWarnings() {
			if w.Code == WarnUnmappedElements || w.Code == WarnUnmappedAttributes {
				t.Errorf("warning %+v", w)
			}
		}
		return doc, blocks(doc)
	}
	doc, bs := read(`<ARTICLE><META><META_COMMUN><ID>KALIARTI000000000001</ID><ORIGINE>KALI</ORIGINE></META_COMMUN><META_SPEC><META_ARTICLE>` +
		`<NUM>2</NUM><ETAT>VIGUEUR_ETEN</ETAT><CALIPSOS><CALIPSO>SALAIRES</CALIPSO><CALIPSO>DUREE</CALIPSO></CALIPSOS>` +
		`<HISTORIQUE>Modifié par avenant n° 1</HISTORIQUE><DATE_DEBUT>2022-07-01</DATE_DEBUT><DATE_FIN>2999-01-01</DATE_FIN>` +
		`<DATE_DEB_EXT>2999-01-01</DATE_DEB_EXT><DATE_FIN_EXT>2999-01-02</DATE_FIN_EXT></META_ARTICLE></META_SPEC></META>` +
		`<CONTEXTE><TEXTE cid="KALITEXT000000000002"><TITRE_TXT id_txt="KALITEXT000000000002">Avenant n° 61</TITRE_TXT></TEXTE>` +
		`<CONTENEUR cid="KALICONT000000000003" date_publi="1988-05-18" etat="VIGUEUR_ETEN" nature="IDCC" num="1501" titre="Convention collective" url="conteneur/x.xml"/></CONTEXTE>` +
		`<BLOC_TEXTUEL><CONTENU><p>Texte.</p></CONTENU></BLOC_TEXTUEL><CONDITION_DIFFERE>Entrera en vigueur au JORF.</CONDITION_DIFFERE>` +
		`<INFORMATIONS_COMPLEMENTAIRES><CONTENU>Précision.</CONTENU></INFORMATIONS_COMPLEMENTAIRES></ARTICLE>`)
	m := doc.Metadata.Extra
	for k, v := range map[string]string{
		"lf.calipso": "SALAIRES\nDUREE", "lf.historique": "Modifié par avenant n° 1", "lf.date_deb_ext": "2999-01-01",
		"lf.date_fin_ext": "2999-01-02", "lf.conteneur.cid": "KALICONT000000000003", "lf.conteneur.num": "1501",
		"lf.conteneur.nature": "IDCC", "lf.conteneur.titre": "Convention collective", "lf.conteneur.etat": "VIGUEUR_ETEN",
		"lf.conteneur.date_publi": "1988-05-18", "lf.conteneur.url": "conteneur/x.xml", "lf.texte.cid": "KALITEXT000000000002",
	} {
		if m[k] != v {
			t.Errorf("Extra[%s] = %q, want %q", k, m[k], v)
		}
	}
	if got := brief(bs); got != "paragraph:Texte.\nparagraph:Entrera en vigueur au JORF.\nparagraph~:Précision.\n" {
		t.Errorf("article blocks:\n%s", got)
	}
	if bs[1].Attrs["lf.field"] != "CONDITION_DIFFERE" || bs[2].Attrs["lf.field"] != "INFORMATIONS_COMPLEMENTAIRES" {
		t.Errorf("fields %v %v", bs[1].Attrs, bs[2].Attrs)
	}

	doc, bs = read(`<TEXTE_VERSION><META><META_COMMUN><ID>KALITEXT000000000004</ID></META_COMMUN><META_SPEC><META_TEXTE_VERSION>` +
		`<TITREFULL>Accord du 30 novembre 2021</TITREFULL><APPLI_GEO>A NATIONAL</APPLI_GEO><CODES_NOMENCLATURES><CODE>10-71A</CODE><CODE>56-10C</CODE></CODES_NOMENCLATURES>` +
		`<LIBELLE_EXTENSION>Etendu par arrêté du 23 mai 2022</LIBELLE_EXTENSION><LIBELLE_ELARGISSEMENT>Elargi</LIBELLE_ELARGISSEMENT>` +
		`<DESCRIPTION_FUSION><CONTENU><p>Fusion.</p></CONTENU></DESCRIPTION_FUSION><INFOS_RESTRUCTURATION_BRANCHE><CONTENU><p>Branche.</p></CONTENU></INFOS_RESTRUCTURATION_BRANCHE>` +
		`</META_TEXTE_VERSION></META_SPEC></META><SIGNATAIRES><SIGNPATRON><CONTENU><p>Patronat.</p></CONTENU></SIGNPATRON><SIGNSYNDIC><CONTENU><p>Syndicats.</p></CONTENU></SIGNSYNDIC>` +
		`<ORGADHERE><CONTENU><p>Adhésion.</p></CONTENU></ORGADHERE><ORGDENONCE><CONTENU><p>Dénonciation.</p></CONTENU></ORGDENONCE>` +
		`<SIGNEXT><CONTENU><p>Extension.</p></CONTENU></SIGNEXT><EXECUTION><CONTENU><p>Exécution.</p></CONTENU></EXECUTION></SIGNATAIRES></TEXTE_VERSION>`)
	m = doc.Metadata.Extra
	if m["lf.appli_geo"] != "A NATIONAL" || m["lf.code_nomenclature"] != "10-71A\n56-10C" || m["lf.libelle_extension"] != "Etendu par arrêté du 23 mai 2022" ||
		m["lf.libelle_elargissement"] != "Elargi" || doc.Metadata.Title != "Accord du 30 novembre 2021" {
		t.Errorf("text version metadata %v", doc.Metadata)
	}
	var b strings.Builder
	for _, x := range bs {
		fmt.Fprintf(&b, "%s|%s|%s\n", x.Text, x.Attrs["lf.field"], x.Attrs["lf.signataires"])
	}
	want := "Accord du 30 novembre 2021||\nFusion.|DESCRIPTION_FUSION|\nBranche.|INFOS_RESTRUCTURATION_BRANCHE|\n" +
		"Patronat.|SIGNATAIRES|SIGNPATRON\nSyndicats.|SIGNATAIRES|SIGNSYNDIC\nAdhésion.|SIGNATAIRES|ORGADHERE\n" +
		"Dénonciation.|SIGNATAIRES|ORGDENONCE\nExtension.|SIGNATAIRES|SIGNEXT\nExécution.|SIGNATAIRES|EXECUTION\n"
	if got := b.String(); got != want {
		t.Errorf("text version blocks:\n%s\nwant:\n%s", got, want)
	}

	doc, bs = read(`<IDCC><META><META_COMMUN><ID>KALICONT000000000005</ID><NATURE>IDCC</NATURE></META_COMMUN><META_SPEC><META_CONTENEUR>` +
		`<TITRE>Convention collective nationale du 6 mai 1970</TITRE><ETAT>PERIME</ETAT><NUM>562</NUM><DATE_PUBLI>1970-05-06</DATE_PUBLI></META_CONTENEUR></META_SPEC></META>` +
		`<STRUCTURE_TXT><TM niv="1"><TITRE_TM>Texte de base</TITRE_TM><LIEN_TXT idtxt="KALITEXT000000000006" titretxt="Convention du 6 mai 1970"/></TM></STRUCTURE_TXT>` +
		`<TXTS_ISO/><ACTS_PRO><ACT_PRO>Aide à domicile</ACT_PRO></ACTS_PRO><NUMS_BROCH><NUM_BROCH>3321</NUM_BROCH></NUMS_BROCH></IDCC>`)
	m = doc.Metadata.Extra
	if doc.Metadata.Title != "Convention collective nationale du 6 mai 1970" || m["lf.root"] != "IDCC" || m["lf.id"] != "KALICONT000000000005" ||
		m["lf.num"] != "562" || m["lf.act_pro"] != "Aide à domicile" || m["lf.num_broch"] != "3321" || m["lf.etat"] != "PERIME" {
		t.Errorf("idcc metadata %v", doc.Metadata)
	}
	if got := brief(bs); got != "heading1:Convention collective nationale du 6 mai 1970\nheading2:Texte de base\nparagraph:Convention du 6 mai 1970\n" {
		t.Errorf("idcc blocks:\n%s", got)
	}
	if x := bs[2]; x.Attrs["lf.idtxt"] != "KALITEXT000000000006" || x.Attrs["lf.group"] != "STRUCTURE_TXT" || x.Attrs[AttrAttribute] != "titretxt" {
		t.Errorf("idcc link %+v", x)
	}
}

// TestLateAnchorNested checks an anchor read from a grandchild that is
// also read for Extra (dila-juri's root and META/META_COMMUN/ID): both get
// the value, and the anchor reaches the blocks before it and after it,
// past the end of the elements between, without hiding nearer anchors.
func TestLateAnchorNested(t *testing.T) {
	p, err := LoadProfile(strings.NewReader(fuzzProfile))
	if err != nil {
		t.Fatal(err)
	}
	src := `<r><ja><p>before</p><jm><ji>I1</ji><ji>I2</ji></jm><p>after</p><s id="in"><p>x</p><ja><jm><ji>I3</ji></jm><p>y</p></ja></s><p>z</p></ja></r>`
	x := &Extractor{Profiles: []*Profile{p}, Profile: "fuzz"}
	doc, err := x.Extract(context.Background(), strings.NewReader(src), int64(len(src)), nil)
	if err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	for _, x := range blocks(doc) {
		fmt.Fprintf(&b, "%s@%s%s ", x.Text, x.Loc.Anchor, x.Attrs[AttrAnchorParent])
	}
	if got, want := b.String(), "before@I1 after@I1 x@in y@I3 z@I1 "; got != want {
		t.Errorf("anchors %q, want %q", got, want)
	}
	if m := doc.Metadata.Extra; m["x.ji"] != "I1\nI3" || m["x.j"] != "I2" {
		t.Errorf("extra %q %q", m["x.ji"], m["x.j"])
	}
}

// TestDilaJuriConstit checks a Conseil constitutionnel decision's own
// metadata (LOI_DEF's attributes as written, 2999-01-01 included) and
// that the anchor reaches blocks after the metadata.
func TestDilaJuriConstit(t *testing.T) {
	const src = `<?xml version="1.0" encoding="UTF-8"?>
<TEXTE_JURI_CONSTIT><META><META_COMMUN><ID>CONSTEXT000000000002</ID><NATURE>DC</NATURE></META_COMMUN>
<META_SPEC><META_JURI><TITRE>Décision n° 2026-1 DC</TITRE><DATE_DEC>2026-01-05</DATE_DEC><JURIDICTION>Conseil constitutionnel</JURIDICTION><NUMERO>2026-1</NUMERO><SOLUTION>Conformité</SOLUTION></META_JURI>
<META_JURI_CONSTIT><NOR>CSCX2600001S</NOR><NATURE_QUALIFIEE>Contrôle de constitutionnalité</NATURE_QUALIFIEE><LOI_DEF date="2999-01-01" nor="SUPPRIME" num=""/>
<TITRE_JO>Journal officiel du 6 janvier 2026</TITRE_JO><URL_CC>https://example.invalid/2026-1</URL_CC><ECLI>ECLI:FR:CC:2026:2026.1.DC</ECLI></META_JURI_CONSTIT></META_SPEC></META>
<TEXTE><BLOC_TEXTUEL><CONTENU>Le Conseil constitutionnel,<br/>Décide :</CONTENU></BLOC_TEXTUEL>
<SAISINES><CONTENU>Saisine des députés</CONTENU></SAISINES><OBSERVATIONS><CONTENU/></OBSERVATIONS></TEXTE></TEXTE_JURI_CONSTIT>`
	doc, err := ExtractBytes(context.Background(), []byte(src), nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := doc.Validate(); err != nil {
		t.Fatal(err)
	}
	m := doc.Metadata.Extra
	if m["juri.loi_def_date"] != "2999-01-01" || m["juri.loi_def_nor"] != "SUPPRIME" || m["juri.ecli"] != "ECLI:FR:CC:2026:2026.1.DC" ||
		m["juri.nor"] != "CSCX2600001S" || m["juri.titre_jo"] != "Journal officiel du 6 janvier 2026" || m["juri.id"] != "CONSTEXT000000000002" ||
		m["juri.nature_qualifiee"] != "Contrôle de constitutionnalité" || m["juri.url_cc"] != "https://example.invalid/2026-1" {
		t.Errorf("extra %v", m)
	}
	bs := blocks(doc)
	if len(bs) != 3 || bs[1].Text != "Le Conseil constitutionnel,\nDécide :" || bs[2].Attrs["juri.field"] != "SAISINES" {
		t.Fatalf("blocks %s", brief(bs))
	}
	for i, x := range bs {
		if x.Attrs[AttrAnchorParent] != "CONSTEXT000000000002" {
			t.Errorf("block %d anchor_parent %q", i, x.Attrs[AttrAnchorParent])
		}
	}
	for _, w := range doc.AllWarnings() {
		if w.Code == WarnUnmappedElements || w.Code == WarnUnmappedAttributes {
			t.Errorf("warning %+v", w)
		}
	}
}
