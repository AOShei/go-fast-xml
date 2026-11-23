package gofastxml

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/AOShei/go-fast-core"
)

// spDir returns the Service-Public directory of a corpus set, or "".
func spDir(set string) string {
	root := os.Getenv("GOFAST_CORPUS")
	if root == "" {
		root = filepath.Join("..", ".testdata", "corpus")
	}
	dir := filepath.Join(root, set, "xml-servicepublic")
	if _, err := os.Stat(dir); err != nil {
		return ""
	}
	return dir
}

func spFiles(dir string) []string {
	var files []string
	filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(p, ".xml") {
			files = append(files, p)
		}
		return nil
	})
	sort.Strings(files)
	return files
}

var countRE = regexp.MustCompile(`([^:;]+) \((\d+)\)`)

// TestServicePublicCorpus reads every Service-Public file of the dev and
// held-out sets with the built-in profile and reports aggregate counts:
// files, errors, Validate failures, blocks by kind, warnings by code, and
// the elements and attributes no rule maps (by name on dev only; held-out
// files are counted, never looked at). It fails on a panic, an error, a
// Validate failure, a file not read with the profile, or a timeout.
func TestServicePublicCorpus(t *testing.T) {
	for _, set := range []string{"dev", "heldout"} {
		dir := spDir(set)
		if dir == "" {
			t.Logf("%s: no Service-Public corpus", set)
			continue
		}
		files := spFiles(dir)
		if testing.Short() && set == "dev" {
			files = files[:min(len(files), 300)]
		}
		tally := newTally()
		unmapped := map[string]map[string]int{WarnUnmappedElements: {}, WarnUnmappedAttributes: {}}
		unmappedTotal := map[string]int{}
		start := time.Now()
		for _, path := range files {
			res := corpusOne(path)
			tally.add("xml-servicepublic", res)
			label := path
			if set == "heldout" {
				label = "a held-out file"
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
			case res.doc.Metadata.Extra[ExtraProfile] != "service-public":
				t.Errorf("%s: not read with the service-public profile", label)
			}
			if res.doc == nil {
				continue
			}
			for _, w := range res.doc.AllWarnings() {
				m, ok := unmapped[w.Code]
				if !ok {
					continue
				}
				for _, sm := range countRE.FindAllStringSubmatch(w.Message, -1) {
					n, _ := strconv.Atoi(sm[2])
					unmappedTotal[w.Code] += n
					if set == "dev" {
						m[strings.TrimSpace(sm[1])] += n
					}
				}
			}
		}
		t.Logf("%s: %d files in %v\n%s", set, len(files), time.Since(start).Round(time.Millisecond), tally)
		for _, code := range []string{WarnUnmappedElements, WarnUnmappedAttributes} {
			line := fmt.Sprintf("%s: %s: %d in all", set, code, unmappedTotal[code])
			if set == "dev" {
				line += ": " + byCount(unmapped[code])
			}
			t.Log(line)
		}
	}
}

func byCount(m map[string]int) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return m[keys[i]] > m[keys[j]] || m[keys[i]] == m[keys[j]] && keys[i] < keys[j] })
	var b strings.Builder
	for _, k := range keys {
		fmt.Fprintf(&b, "%s=%d ", k, m[k])
	}
	return b.String()
}

// spSample is a small synthetic fiche with the shapes of decisions S1 to
// S7.
const spSample = `<?xml version="1.0" encoding="UTF-8"?><Publication xmlns:dc="http://purl.org/dc/elements/1.1/" ID="F1" type="Fiche" spUrl="https://example.org/F1">` +
	`<dc:title>Titre de la fiche</dc:title><dc:description>Resume.</dc:description><dc:date>modified 2026-11-03</dc:date>` +
	`<FilDAriane><Niveau ID="A">Accueil</Niveau><Niveau ID="B">Famille</Niveau></FilDAriane>` +
	`<Texte><Chapitre ID="C1"><Titre><Paragraphe>Chapitre un</Paragraphe></Titre>` +
	`<ANoter><Titre>A noter</Titre><Paragraphe>Voir <LienInterne LienPublication="F2" type="Fiche" audience="Particuliers" commentaireLien="Autre fiche">ceci</LienInterne>.</Paragraphe></ANoter>` +
	`<BlocCas affichage="radio"><Cas><Titre>Cas A</Titre><TitreRiche><Paragraphe>Cas A</Paragraphe></TitreRiche><Condition><estVrai var="T1"/></Condition><Paragraphe>Dans le cas A.</Paragraphe></Cas></BlocCas>` +
	`</Chapitre></Texte><Reference type="Texte de reference" URL="https://legifrance.example" ID="R1"><Titre>Code civil</Titre></Reference></Publication>`

// TestServicePublicDecisions checks S1 to S7 on spSample, from several
// goroutines at once (a profile is shared by concurrent Extract calls).
func TestServicePublicDecisions(t *testing.T) {
	check := func() string {
		doc, err := ExtractBytes(context.Background(), []byte(spSample), &gofastcore.Options{Spans: true})
		if err != nil {
			return err.Error()
		}
		if err := doc.Validate(); err != nil {
			return err.Error()
		}
		m := doc.Metadata
		if m.Title != "Titre de la fiche" || m.Extra["dc:date"] != "modified 2026-11-03" || !m.Modified.IsZero() ||
			m.Extra["description"] != "Resume." || m.Extra["sp.id"] != "F1" || m.Extra["sp.url"] != "https://example.org/F1" ||
			m.Extra["sp.breadcrumb"] != "Accueil > Famille" || m.Extra[ExtraProfile] != "service-public" {
			return fmt.Sprintf("metadata %+v", m)
		}
		got := brief(blocks(doc))
		want := "heading1:Titre de la fiche\nheading2:Chapitre un\nparagraph(A noter):Voir ceci.\nheading3:Cas A\nparagraph:Dans le cas A.\nlist_item1:Code civil\n"
		if got != want {
			return "blocks:\n" + got
		}
		bs := blocks(doc)
		if bs[2].Attrs["sp.box"] != "ANoter" || bs[2].Attrs[AttrAnchorParent] != "C1" || bs[2].Spans[0].Href != "F2" {
			return fmt.Sprintf("box %+v", bs[2])
		}
		if bs[3].Attrs["sp.condition"] != "estVrai(var=T1)" || bs[4].Attrs["sp.case_group"] != "radio" || bs[3].Attrs["sp.title_plain"] != "" {
			return fmt.Sprintf("case %+v", bs[3].Attrs)
		}
		if bs[5].Attrs["sp.group"] != "Reference" || bs[5].Loc.Anchor != "R1" || bs[5].Attrs["sp.url"] != "https://legifrance.example" {
			return fmt.Sprintf("reference %+v", bs[5])
		}
		st := doc.Parts[len(doc.Parts)-1]
		if st.Kind != gofastcore.PartStored || st.Blocks[0].Text != "Autre fiche" || st.Blocks[0].Attrs["sp.audience"] != "Particuliers" {
			return "stored " + dump(doc)
		}
		if bad := citeCheck([]byte(spSample), doc); bad != "" {
			return bad
		}
		return ""
	}
	errs := make(chan string, 8)
	for range 8 {
		go func() { errs <- check() }()
	}
	for range 8 {
		if e := <-errs; e != "" {
			t.Fatal(e)
		}
	}
}

// spOracleSkip lists the elements whose character data is not block or
// cell text in the body, by design, as "parent/name" or "name" (any
// parent); the root's own children are "/name". The oracle leaves their
// text (and their descendants') out of the file's side.
var spOracleSkip = map[string]string{
	// Metadata only (C1). dc:title is not here: it is also the level 1
	// heading (S1).
	"dc:creator": "metadata", "dc:subject": "metadata", "dc:publisher": "metadata", "dc:contributor": "metadata",
	"dc:date": "metadata (S2)", "dc:type": "metadata", "dc:format": "metadata", "dc:identifier": "metadata",
	"dc:source": "metadata", "dc:relation": "metadata", "dc:coverage": "metadata", "dc:rights": "metadata",
	"dc:language":    "metadata",
	"dc:description": "metadata only (S3)",
	"/SurTitre":      "metadata", "/Audience": "metadata", "/Canal": "metadata", "/NumeroCerfa": "metadata",
	"/AutreNumero": "metadata", "FilDAriane": "metadata", "/Theme": "metadata", "/SousThemePere": "metadata",
	"DossierPere/Titre": "metadata",
	// S4: the plain title is dropped when it repeats TitreRiche, else kept
	// in Attrs["sp.title_plain"].
	"Cas/Titre": "S4", "Situation/Titre": "S4",
	// Read for a value of Attrs (A4 child).
	"PivotLocal": "Attrs sp.pivot", "Chapitre/TitreAlternatif": "Attrs sp.title_alt",
	"SousChapitre/TitreAlternatif": "Attrs sp.title_alt", "ServiceEnLigne/TitreAlternatif": "Attrs sp.title_alt",
	// The alternative text of an image is its block's text, which stands
	// where the image starts.
	"TexteDeRemplacement": "image alt",
}

// spSkipped reports whether the element at the end of stack is skipped.
func spSkipped(stack []string) bool {
	n := len(stack)
	name := stack[n-1]
	if _, ok := spOracleSkip[name]; ok {
		return true
	}
	if n >= 2 {
		if _, ok := spOracleSkip[stack[n-2]+"/"+name]; ok {
			return true
		}
	}
	if n == 2 {
		if _, ok := spOracleSkip["/"+name]; ok {
			return true
		}
	}
	return false
}

// spSourceText is the character data of the file in order, without the
// skipped elements, white space removed.
func spSourceText(data []byte, used map[string]bool) (string, error) {
	d := xml.NewDecoder(bytes.NewReader(data))
	var stack []string
	skip := 0
	var b strings.Builder
	for {
		tok, err := d.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}
		switch tk := tok.(type) {
		case xml.StartElement:
			name := tk.Name.Local
			if tk.Name.Space == "http://purl.org/dc/elements/1.1/" {
				name = "dc:" + name
			}
			stack = append(stack, name)
			if skip > 0 {
				skip++
			} else if spSkipped(stack) {
				skip = 1
				used[strings.Join(stack[max(len(stack)-2, 0):], "/")] = true
			}
		case xml.EndElement:
			stack = stack[:len(stack)-1]
			if skip > 0 {
				skip--
			}
		case xml.CharData:
			if skip == 0 && len(stack) > 0 {
				b.WriteString(noSpace(gofastcore.ExpandLigatures(string(tk))))
			}
		}
	}
	return b.String(), nil
}

var cellMarkerRE = regexp.MustCompile(`(^|\n)\d+\. `)

// spDocText is the text of the body's blocks and cells in order, white
// space removed: not stored data, not text from an attribute (a block
// with Attrs["attribute"], the Redirections table among them, whose
// cells are attributes and whose header is the profile's), not the
// text of an image (its alt, see spOracleSkip), not the numbers of
// ordered list items, which the file does not hold.
func spDocText(doc *gofastcore.Document) string {
	var b strings.Builder
	for _, p := range doc.Parts {
		if p.Kind == gofastcore.PartStored {
			continue
		}
		for _, x := range p.Blocks {
			switch {
			case x.Attrs[gofastcore.AttrStored] != "", x.Attrs[AttrAttribute] != "", x.Kind == gofastcore.KindImage:
				continue
			case x.Kind == gofastcore.KindTable:
				for _, row := range x.Table.Rows {
					for _, c := range row {
						b.WriteString(noSpace(cellMarkerRE.ReplaceAllString(c, "$1")))
					}
				}
				continue
			}
			if x.Marker != "" && !(x.Kind == gofastcore.KindListItem && x.Ordered) {
				b.WriteString(noSpace(x.Marker))
			}
			b.WriteString(noSpace(x.Text))
		}
	}
	return b.String()
}

// TestServicePublicOracles checks, for every Service-Public dev file, the
// two oracles of C6 and A6: (a) the body's block and cell text in order
// is the file's character data in order, white space aside, apart from
// the elements spOracleSkip lists; (b) every block's text is in the
// source at its Loc.Offset and Loc.Length, markup stripped. Every
// exception must be met at least once.
func TestServicePublicOracles(t *testing.T) {
	dir := spDir("dev")
	if dir == "" {
		t.Skip("no Service-Public dev corpus")
	}
	files := spFiles(dir)
	if testing.Short() {
		files = files[:min(len(files), 300)]
	}
	used := map[string]bool{}
	fails := 0
	for _, path := range files {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		doc, err := ExtractBytes(context.Background(), data, nil)
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		src, err := spSourceText(data, used)
		if err != nil {
			t.Fatalf("%s: encoding/xml: %v", path, err)
		}
		if got := spDocText(doc); got != src && fails < 10 {
			fails++
			i := 0
			for i < len(got) && i < len(src) && got[i] == src[i] {
				i++
			}
			t.Errorf("%s: (a) text differs at byte %d:\n doc:  %q\n file: %q", path, i, around(got, i), around(src, i))
		}
		if bad := citeCheck(data, doc); bad != "" && fails < 10 {
			fails++
			t.Errorf("%s: (b) %s", path, around(bad, 0))
		}
	}
	if !testing.Short() {
		for k := range spOracleSkip {
			met := false
			for u := range used {
				if u == k || strings.HasSuffix(u, "/"+k) || strings.HasPrefix(k, "/") && strings.HasSuffix(u, k) {
					met = true
				}
			}
			if !met {
				t.Errorf("exception %q is met nowhere in the dev set", k)
			}
		}
	}
}

// around returns the part of s around byte i.
func around(s string, i int) string {
	from := max(i-40, 0)
	return strings.ToValidUTF8(s[from:min(len(s), i+200)], "")
}
