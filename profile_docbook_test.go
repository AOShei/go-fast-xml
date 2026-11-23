package gofastxml

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/AOShei/go-fast-core"
)

func TestDocbookCensus(t *testing.T) { jatsCensus(t, "xml-docbook") }

func TestDocbookFixture(t *testing.T) {
	doc, data := jatsExtractFixture(t, "docbook/book5.xml", "docbook")
	want := `# body
heading1:The Book ^b1
paragraph:About it. ^b1
heading2:First ^ch1
paragraph:See , site, bold and x. #p1
footnote:Note text. #fn1
heading3:Section ^s1
list_item1:One ^s1
list_item1(1.):Step ^s1
list_item1:Term ^s1
paragraph:Definition. ^s1
paragraph:Careful ^s1
paragraph:Noted. ^s1
code:fmt.Println("hi") ^s1
caption:Fig ^f1
image: ^f1
paragraph:Alt words ^f1
table: ^s1
caption:HTML ^s1
table: ^s1
paragraph:other.xml ^s1
note:Check this. ^s1
footnote:W10 N. Walsh DocBook 2010 #bib1
# stored
note:s1
note:first use
`
	if got := jatsBriefAll(doc); got != want {
		t.Errorf("blocks:\n%s\nwant:\n%s", got, want)
	}
	m := doc.Metadata
	if m.Title != "The Book" || strings.Join(m.Authors, "|") != "Norman Walsh" || m.Extra["db.subtitle"] != "A test" ||
		m.Extra["db.pubdate"] != "2010" || strings.Join(m.Keywords, "|") != "docbook" {
		t.Errorf("metadata %+v", m)
	}
	bs := blocks(doc)
	if p := bs[3]; len(p.Spans) != 3 || p.Spans[0].Href != "https://example.org" || p.Spans[1].Style != gofastcore.StyleBold {
		t.Errorf("spans %+v", p.Spans)
	}
	if bs[11].Attrs["db.admonition"] != "note" || bs[12].Attrs["language"] != "go" || bs[14].Image.ID != "a.png" ||
		bs[19].Attrs[AttrAttribute] != "href" || bs[20].Attrs[gofastcore.AttrNoteType] != "comment" {
		t.Errorf("blocks %s", dump(doc))
	}
	if tb := bs[16].Table; tb == nil || mustJSON(tb.Rows) != `[["H1","H2"],["a","b"]]` || tb.HeaderRows != 1 {
		t.Errorf("table %+v", tb)
	}
	st := doc.Parts[1].Blocks
	if st[0].Attrs["href"] != "s1" || st[1].Attrs[gofastcore.AttrStored] != gofastcore.StoredCustomXML {
		t.Errorf("stored %s", dump(doc))
	}
	if bad := docbookCase.cite(data, doc); bad != "" {
		t.Error(bad)
	}
	doc4, data4 := jatsExtractFixture(t, "docbook/article4.xml", "docbook")
	want4 := `# body
heading1:Old Article ^a1
heading2:Top ^s1
paragraph:Go to x. ^s1
heading3:Deeper ^s2
code:$ ls ^s2
`
	if got := jatsBriefAll(doc4); got != want4 {
		t.Errorf("blocks:\n%s\nwant:\n%s", got, want4)
	}
	if m := doc4.Metadata; strings.Join(m.Authors, "|") != "Eric Raymond" || m.Extra["db.copyright"] != "2000 ESR" {
		t.Errorf("metadata %+v", m)
	}
	if bad := docbookCase.cite(data4, doc4); bad != "" {
		t.Error(bad)
	}
	for _, d := range []*gofastcore.Document{doc, doc4} {
		for _, w := range d.AllWarnings() {
			if w.Code == WarnUnmappedElements {
				t.Errorf("unmapped: %s", w.Message)
			}
		}
	}
}

var docbookCase = &jatsOracleCase{profile: "docbook", format: "xml-docbook", skip: map[string]string{
	// Metadata of the root (Metadata fields and Extra db.*).
	"/*/info/subtitle": "metadata",
	"info/author":      "Metadata.Authors", "info/authorgroup": "Metadata.Authors", "bookinfo/authorgroup": "Metadata.Authors",
	"articleinfo/author": "Metadata.Authors", "info/editor": "metadata", "info/othercredit": "metadata",
	"info/pubdate": "metadata", "info/releaseinfo": "metadata", "info/copyright": "metadata",
	"info/legalnotice": "metadata", "info/revhistory": "metadata",
	"info/publisher": "metadata", "info/biblioid": "metadata", "info/titleabbrev": "metadata",
	"bookinfo/copyright": "metadata", "bookinfo/legalnotice": "metadata", "bookinfo/date": "metadata",
	"bookinfo/releaseinfo": "metadata", "bookinfo/keywordset": "Metadata.Keywords",
	"articleinfo/copyright": "metadata", "articleinfo/revhistory": "metadata",
	// Index terms are stored data (custom_xml), not body text.
	"indexterm": "stored",
}}

func TestDocbookCorpus(t *testing.T) { docbookCase.run(t) }

func TestDocbookOracles(t *testing.T) { docbookCase.oracles(t) }

// The docbook profile is generated from testdata/profiles/docbook/rules.json,
// which holds the rules once, for DocBook 4 (no namespace, identifiers in
// "id"). Each rule is written twice in the profile: as it is, and for
// DocBook 5, with every element name in the DocBook namespace (prefix
// "db") and "id" read as "xml:id". TestDocbookGenerated fails when the
// two disagree; go test -run TestDocbookGenerated -docbook.update writes
// the profile.
var docbookUpdate = flag.Bool("docbook.update", false, "write profiles/docbook.json from testdata/profiles/docbook/rules.json")

const docbookNS = "http://docbook.org/ns/docbook"

// docbookRoots are the root elements that select the profile: in the
// DocBook 5 namespace, or with a DocBook 4 public identifier.
var docbookRoots = []string{"set", "book", "part", "reference", "refentry", "article", "chapter", "appendix", "preface",
	"colophon", "dedication", "acknowledgements", "glossary", "bibliography", "index", "section", "sect1", "sect2",
	"sect3", "sect4", "sect5", "refsection", "simplesect", "topic", "partintro", "table", "informaltable", "figure",
	"example", "equation", "glossentry", "procedure", "sidebar", "note", "itemizedlist", "orderedlist", "variablelist",
	"para", "refsect1", "qandaset", "bibliodiv", "glossdiv"}

// docbookBareRoots are DocBook 4 roots also taken with no DOCTYPE: names
// no other built-in profile's schema uses for a root.
var docbookBareRoots = []string{"chapter", "appendix", "colophon", "preface", "sect1", "sect2", "sect3", "refentry", "reference"}

var docbookPublic = []string{"-//OASIS//DTD DocBook", "-//KDE//DTD DocBook", "-//FreeBSD//DTD DocBook"}

func docbookStep(s string) string {
	if s == "" || s[0] == '*' {
		return s
	}
	name, tests, _ := strings.Cut(s, "[")
	if tests != "" {
		tests = "[" + tests
	}
	if strings.Contains(name, ":") {
		return s
	}
	return "db:" + name + tests
}

func docbookSelector(sel string) string {
	steps := strings.Split(sel, "/")
	for i, s := range steps {
		steps[i] = docbookStep(s)
	}
	return strings.Join(steps, "/")
}

// docbookValue rewrites a value source (or a map of them) for DocBook 5.
func docbookValue(v any) any {
	m, ok := v.(map[string]any)
	if !ok {
		return v
	}
	out := map[string]any{}
	for k, x := range m {
		switch k {
		case "attr":
			if x == "id" {
				x = "xml:id"
			}
		case "of":
			x = docbookStep(x.(string))
		case "child", "structure":
			x = docbookSelector(x.(string))
		default:
			x = docbookValue(x)
		}
		out[k] = x
	}
	return out
}

func docbookMarshal(v any) string {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	enc.Encode(v)
	return strings.TrimSuffix(b.String(), "\n")
}

func docbookRule(r map[string]any) string {
	keys := make([]string, 0, len(r))
	for k := range r {
		if k != "match" && k != "as" {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	var b strings.Builder
	b.WriteString(`    {"match": ` + docbookMarshal(r["match"]) + `, "as": ` + docbookMarshal(r["as"]))
	for _, k := range keys {
		b.WriteString(`, "` + k + `": ` + docbookMarshal(r[k]))
	}
	b.WriteString("}")
	return b.String()
}

// docbookGenerate builds the profile from the template.
func docbookGenerate(t *testing.T) []byte {
	data, err := os.ReadFile(filepath.Join("testdata", "profiles", "docbook", "rules.json"))
	if err != nil {
		t.Fatal(err)
	}
	var tpl map[string]any
	if err := json.Unmarshal(data, &tpl); err != nil {
		t.Fatal(err)
	}
	ns := tpl["namespaces"].(map[string]any)
	ns["db"] = docbookNS
	var match []string
	for _, r := range docbookRoots {
		match = append(match, docbookMarshal(map[string]string{"root": r, "namespace": docbookNS}))
	}
	for _, p := range docbookPublic {
		for _, r := range docbookRoots {
			match = append(match, docbookMarshal(map[string]string{"root": r, "doctype_public": p}))
		}
	}
	for _, r := range docbookBareRoots {
		match = append(match, docbookMarshal(map[string]string{"root": r}))
	}
	var rules4, rules5 []string
	for _, x := range tpl["rules"].([]any) {
		r := x.(map[string]any)
		rules4 = append(rules4, docbookRule(r))
		r5 := map[string]any{}
		for k, v := range r {
			switch k {
			case "match":
				v = docbookSelector(v.(string))
			case "as", "kind", "type", "field", "key", "join", "style", "level", "ordered", "header", "model",
				"notation", "language", "also_heading", "space", "colspan", "rowspan":
			default:
				v = docbookValue(v)
			}
			r5[k] = v
		}
		rules5 = append(rules5, docbookRule(r5))
	}
	var b strings.Builder
	b.WriteString("{\n")
	b.WriteString(`  "profile": ` + docbookMarshal(tpl["profile"]) + ",\n")
	b.WriteString(`  "title": ` + docbookMarshal(tpl["title"]) + ",\n")
	b.WriteString(`  "format": 1,` + "\n")
	b.WriteString(`  "namespaces": ` + docbookMarshal(ns) + ",\n")
	b.WriteString(`  "match": [` + "\n    " + strings.Join(match, ",\n    ") + "\n  ],\n")
	b.WriteString(`  "heading_base": ` + docbookMarshal(tpl["heading_base"]) + ",\n")
	b.WriteString(`  "rules": [` + "\n" + strings.Join(rules5, ",\n") + ",\n\n" + strings.Join(rules4, ",\n") + "\n  ]\n}\n")
	return []byte(b.String())
}

func TestDocbookGenerated(t *testing.T) {
	want := docbookGenerate(t)
	path := filepath.Join("profiles", "docbook.json")
	if *docbookUpdate {
		if err := os.WriteFile(path, want, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("%s is not what testdata/profiles/docbook/rules.json gives; run go test -run TestDocbookGenerated -docbook.update", path)
	}
	if _, err := LoadProfile(bytes.NewReader(want)); err != nil {
		t.Fatal(err)
	}
}
