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
	"unicode"

	"github.com/AOShei/go-fast-core"
)

// The harness below (aknCorpusDir, aknCorpus, aknOracleSpec, aknOracles,
// aknCiteCheck) is shared by the four law profiles' tests.

// aknCorpusDir returns the directory of a corpus set and format, or "":
// under GOFAST_CORPUS, else ../.testdata/corpus, else (from a worktree
// one level deeper) ../../.testdata/corpus.
func aknCorpusDir(set, format string) string {
	roots := []string{filepath.Join("..", ".testdata", "corpus"), filepath.Join("..", "..", ".testdata", "corpus")}
	if root := os.Getenv("GOFAST_CORPUS"); root != "" {
		roots = []string{root}
	}
	for _, root := range roots {
		dir := filepath.Join(root, set, format)
		if _, err := os.Stat(dir); err == nil {
			return dir
		}
	}
	return ""
}

// aknCorpus reads every file of the dev and held-out sets of format and
// reports aggregate counts; unmapped names on dev only. It fails on a
// panic, an error, a Validate failure, a timeout or a file not read with
// the profile.
func aknCorpus(t *testing.T, format, profile string) {
	for _, set := range []string{"dev", "heldout"} {
		dir := aknCorpusDir(set, format)
		if dir == "" {
			t.Logf("%s: no %s corpus", set, format)
			continue
		}
		files := spFiles(dir)
		tally := newTally()
		unmapped := map[string]map[string]int{WarnUnmappedElements: {}, WarnUnmappedAttributes: {}}
		unmappedTotal := map[string]int{}
		start := time.Now()
		for _, path := range files {
			res := corpusOne(path)
			tally.add(format, res)
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
			case res.doc.Metadata.Extra[ExtraProfile] != profile:
				t.Errorf("%s: not read with the %s profile", label, profile)
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

// aknOracleSpec says how a schema's file text relates to its blocks.
type aknOracleSpec struct {
	// prefixes names the namespaces whose elements are written prefix:name.
	prefixes map[string]string
	// skip: elements whose text is not block text in the body, by design,
	// as "name", "parent/name" or "/name" (a child of the root's child,
	// i.e. of the document element under the root when rootDepth is 2).
	skip map[string]string
	// outOfLine: elements read with "out_of_line": met inside a block,
	// their text comes before that block's.
	outOfLine map[string]bool
	// blocks: elements that open a block (the outermost one is where an
	// out-of-line note's text goes); cells: inside them notes stay inline.
	blocks, cells map[string]bool
	// labels: elements read as "label"; items: elements read as "item";
	// closers: lists and tables, which end the block of an item.
	labels, items, closers map[string]bool
	// splits: tables and images, which met inside a paragraph-like block
	// (not an item's, not in a cell) end it; text after them, still in
	// the block's element, starts a new block.
	splits map[string]bool
	// cellNumbers: cells may hold ordered lists, whose numbers the file
	// does not hold.
	cellNumbers bool
	// attrMarker: block kinds whose Marker comes from an attribute.
	attrMarker map[gofastcore.Kind]bool
	// attrText: elements whose marker comes from an attribute, which is
	// written in the text when they stand inside a block.
	attrText map[string]string
	// attrChar: elements an inline rule's "value" writes in the text as
	// the character whose code point their attribute holds in hex
	// (Formex QUOT.START@CODE), wherever they stand. Each must be met on
	// dev; codepointTagRE must name the same elements.
	attrChar map[string]string
	// only, when set, names the dev files the oracles read (one corpus
	// set holding files of several profiles, xml-dila).
	only func(path string) bool
}

// codepointTagRE is a tag of an attrChar element, for the cite check,
// whose span text is stripped of markup.
var codepointTagRE = regexp.MustCompile(`<(?:QUOT\.START|QUOT\.END)\b[^>]*\bCODE="([0-9A-Fa-f]{1,6})"[^>]*>`)

// expandCodepoints writes the character an attrChar element names in
// place of its tag.
func expandCodepoints(s string) string {
	if !strings.Contains(s, "CODE=") {
		return s
	}
	return codepointTagRE.ReplaceAllStringFunc(s, func(tag string) string {
		return hexChar(codepointTagRE.FindStringSubmatch(tag)[1])
	})
}

// hexChar is the character of a hex code point, or "" for one the
// engine refuses (surrogates, past U+10FFFF, controls).
func hexChar(h string) string {
	n, err := strconv.ParseUint(strings.TrimSpace(h), 16, 32)
	if err != nil || n > unicode.MaxRune || n >= 0xD800 && n <= 0xDFFF || unicode.IsControl(rune(n)) {
		return ""
	}
	return string(rune(n))
}

func (s *aknOracleSpec) name(n xml.Name) string {
	if p, ok := s.prefixes[n.Space]; ok {
		return p + ":" + n.Local
	}
	return n.Local
}

func (s *aknOracleSpec) skipped(stack []string) (string, bool) {
	n := len(stack)
	name := stack[n-1]
	if _, ok := s.skip[name]; ok {
		return name, true
	}
	if n >= 2 {
		if k := stack[n-2] + "/" + name; s.skip[k] != "" {
			return k, true
		}
	}
	if n >= 2 && s.skip["/"+name] != "" && n <= 3 {
		return "/" + name, true
	}
	return "", false
}

// sourceText is the file's character data in order, white space removed,
// skipped elements left out, out-of-line notes moved before the block
// they stand in.
func (s *aknOracleSpec) sourceText(data []byte, used map[string]bool) (string, error) {
	d := xml.NewDecoder(bytes.NewReader(data))
	d.Strict = false
	d.Entity = xml.HTMLEntity
	var stack []string
	skip, cellDepth := 0, 0
	blockAt, blockDepth, labelAt, labelCont, labelDepth := -1, 0, -1, 0, 0
	inItem, itemText, anyLabel := false, false, 0
	noteDepth := 0
	splitDepth, splitBlock := 0, 0
	var b, note []byte
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
			name := s.name(tk.Name)
			stack = append(stack, name)
			switch {
			case skip > 0:
				skip++
				continue
			case noteDepth > 0:
				noteDepth++
				if a, ok := s.attrChar[name]; ok {
					for _, at := range tk.Attr {
						if at.Name.Local == a {
							note = append(note, noSpace(hexChar(at.Value))...)
							used["@"+name] = true
						}
					}
				}
				continue
			}
			if k, ok := s.skipped(stack); ok {
				skip = 1
				used[k] = true
				continue
			}
			if s.cells[name] {
				cellDepth++
			}
			if s.outOfLine[name] && (blockAt >= 0 || labelDepth > 0) && cellDepth == 0 {
				noteDepth = 1
				note = note[:0]
				continue
			}
			if a, ok := s.attrChar[name]; ok {
				for _, at := range tk.Attr {
					if at.Name.Local == a {
						b = append(b, noSpace(hexChar(at.Value))...)
						used["@"+name] = true
					}
				}
			}
			if a, ok := s.attrText[name]; ok && (blockAt >= 0 || cellDepth > 0) {
				// An item inside a block or cell: its marker, from an
				// attribute, is a line of the block's text.
				for _, at := range tk.Attr {
					if at.Name.Local == a {
						b = append(b, noSpace(at.Value)...)
					}
				}
			}
			if s.labels[name] && anyLabel == 0 {
				anyLabel = len(stack)
			}
			if inItem && blockAt >= 0 && cellDepth == 0 && len(stack) > blockDepth {
				// An item's block ends at a list or table in it, or at a
				// second block once it has text.
				switch {
				case s.closers[name]:
					blockAt, inItem = -1, false
				case s.blocks[name] && itemText:
					blockAt, blockDepth, inItem = len(b), len(stack), false
				}
			}
			if s.splits[name] && blockAt >= 0 && cellDepth == 0 && !inItem && splitDepth == 0 {
				// The block ends here; what follows the table in its
				// element starts another.
				splitDepth, splitBlock, blockAt = len(stack), blockDepth, -1
			}
			if s.labels[name] && blockAt < 0 && cellDepth == 0 {
				// A label waits for the block it marks, before which
				// the notes of that block come.
				labelAt, labelCont, labelDepth = len(b), len(stack)-1, len(stack)
			} else if s.blocks[name] && blockAt < 0 && cellDepth == 0 {
				blockAt, blockDepth = len(b), len(stack)
				if labelAt >= 0 {
					blockAt = labelAt
				}
				labelAt = -1
				inItem, itemText = s.items[name], false
			}
		case xml.EndElement:
			name := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			switch {
			case skip > 0:
				skip--
				continue
			case noteDepth > 0:
				noteDepth--
				if noteDepth == 0 {
					at := &blockAt
					if blockAt < 0 {
						at = &labelAt
					}
					b = append(b[:*at], append(append([]byte(nil), note...), b[*at:]...)...)
					*at += len(note)
				}
				continue
			}
			if s.cells[name] {
				cellDepth--
			}
			if splitDepth > 0 && len(stack)+1 == splitDepth {
				splitDepth = 0
				if len(stack) >= splitBlock {
					blockAt, blockDepth, labelAt = len(b), splitBlock, -1
				}
			}
			if blockAt >= 0 && len(stack)+1 == blockDepth {
				blockAt = -1
			}
			if labelDepth > 0 && len(stack)+1 == labelDepth {
				labelDepth = 0
			}
			if anyLabel > 0 && len(stack)+1 == anyLabel {
				anyLabel = 0
			}
			if labelAt >= 0 && len(stack) < labelCont {
				labelAt = -1
			}
		case xml.CharData:
			if skip > 0 || len(stack) == 0 {
				continue
			}
			t := noSpace(gofastcore.ExpandLigatures(string(tk)))
			if inItem && anyLabel == 0 && noteDepth == 0 && t != "" {
				itemText = true
			}
			if noteDepth > 0 {
				note = append(note, t...)
			} else {
				b = append(b, t...)
			}
		}
	}
	return string(b), nil
}

// docText is the text of the body's blocks and cells in order, white
// space removed, as spDocText, leaving out markers taken from attributes.
func (s *aknOracleSpec) docText(doc *gofastcore.Document) string {
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
						if s.cellNumbers {
							c = cellMarkerRE.ReplaceAllString(c, "$1")
						}
						b.WriteString(noSpace(c))
					}
				}
				continue
			}
			if x.Marker != "" && !(x.Kind == gofastcore.KindListItem && x.Ordered) && !s.attrMarker[x.Kind] {
				b.WriteString(noSpace(x.Marker))
			}
			b.WriteString(noSpace(x.Text))
		}
	}
	return b.String()
}

// aknCiteCheck is citeCheck with the spans of out-of-line notes (blocks
// of kind footnote inside another block's span) cut out of that span.
func aknCiteCheck(src []byte, doc *gofastcore.Document) string {
	for pi := range doc.Parts {
		bs := doc.Parts[pi].Blocks
		var notes [][2]int64
		for i := range bs {
			if bs[i].Kind == gofastcore.KindFootnote {
				notes = append(notes, [2]int64{bs[i].Loc.Offset, bs[i].Loc.Offset + bs[i].Loc.Length})
			}
		}
		for bi := range bs {
			b := &bs[bi]
			if !citable(b) {
				continue
			}
			l := b.Loc
			if l.Offset < 0 || l.Length <= 0 || l.Offset+l.Length > int64(len(src)) {
				return "block " + strconv.Itoa(bi) + " has no span in the source"
			}
			var piece []byte
			at := l.Offset
			for _, n := range notes {
				if n[0] > at && n[1] <= l.Offset+l.Length && !(n[0] == l.Offset && n[1] == l.Offset+l.Length) {
					piece = append(piece, src[at:n[0]]...)
					at = n[1]
				}
			}
			piece = append(piece, src[at:l.Offset+l.Length]...)
			text := noSpace(gofastcore.ExpandLigatures(b.Text))
			if !strings.Contains(noSpace(stripMarkup(expandCodepoints(string(piece)))), text) {
				return "block " + strconv.Itoa(bi) + " text " + strconv.Quote(b.Text) + " not in its span " + strconv.Quote(string(src[l.Offset:l.Offset+l.Length]))
			}
		}
	}
	return ""
}

// aknOracles checks the two oracles over every dev file of format, and
// that every exception of the spec is met.
func aknOracles(t *testing.T, format string, spec *aknOracleSpec) {
	dir := aknCorpusDir("dev", format)
	if dir == "" {
		t.Skipf("no %s dev corpus", format)
	}
	used := map[string]bool{}
	fails := 0
	for _, path := range spFiles(dir) {
		if spec.only != nil && !spec.only(path) {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		doc, err := ExtractBytes(context.Background(), data, nil)
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		src, err := spec.sourceText(data, used)
		if err != nil {
			t.Fatalf("%s: encoding/xml: %v", path, err)
		}
		if got := spec.docText(doc); got != src && fails < 10 {
			fails++
			i := 0
			for i < len(got) && i < len(src) && got[i] == src[i] {
				i++
			}
			t.Errorf("%s: (a) text differs at byte %d:\n doc:  %q\n file: %q", path, i, around(got, i), around(src, i))
		}
		if bad := aknCiteCheck(data, doc); bad != "" && fails < 10 {
			fails++
			t.Errorf("%s: (b) %s", path, around(bad, 0))
		}
	}
	var unmet []string
	for k := range spec.skip {
		if !used[k] {
			unmet = append(unmet, k)
		}
	}
	for k := range spec.attrChar {
		if !used["@"+k] {
			unmet = append(unmet, "@"+k)
		}
	}
	sort.Strings(unmet)
	if len(unmet) > 0 {
		t.Errorf("exceptions met nowhere in the dev set: %v", unmet)
	}
}

// aknSpec is the akn oracle: the metadata block of meta is not body text
// (its tables are cells of attributes), apart from meta/notes.
var aknSpec = &aknOracleSpec{
	prefixes: map[string]string{"http://purl.org/dc/elements/1.1/": "dc", "http://www.w3.org/1999/xhtml": "html",
		"http://www.legislation.gov.uk/namespaces/metadata": "ukm", "http://www.w3.org/1998/Math/MathML": "mml"},
	skip: map[string]string{
		"identification": "metadata", "publication": "metadata", "lifecycle": "table of attributes",
		"workflow": "table of attributes", "references": "table of attributes", "classification": "table of attributes",
		"dc:title": "metadata", "dc:identifier": "metadata", "dc:language": "metadata", "dc:publisher": "metadata",
		"dc:modified": "metadata", "dc:subject": "metadata", "dc:description": "metadata",
	},
	outOfLine: map[string]bool{"authorialNote": true},
	blocks: map[string]bool{"p": true, "heading": true, "block": true, "listIntroduction": true, "listWrapUp": true,
		"subheading": true, "crossHeading": true, "narrative": true, "summary": true, "scene": true, "other": true, "item": true,
		"li": true, "tocItem": true, "caption": true, "html:p": true},
	labels:     map[string]bool{"num": true, "from": true},
	items:      map[string]bool{"item": true, "li": true, "tocItem": true},
	closers:    map[string]bool{"blockList": true, "ul": true, "ol": true, "toc": true, "table": true, "html:table": true},
	splits:     map[string]bool{"table": true, "html:table": true, "img": true, "html:img": true},
	cells:      map[string]bool{"td": true, "th": true, "html:td": true, "html:th": true},
	attrMarker: map[gofastcore.Kind]bool{gofastcore.KindFootnote: true},
}

func TestAknCorpus(t *testing.T) { aknCorpus(t, "xml-akn", "akn") }

func TestAknOracles(t *testing.T) { aknOracles(t, "xml-akn", aknSpec) }

// TestAknSample checks every rule kind the akn profile uses on a
// synthetic act, with exact blocks.
func TestAknSample(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "profiles", "akn", "sample.xml"))
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
	if m.Extra[ExtraProfile] != "akn" || m.Title != "The Sample Act 2026" || m.Language != "eng" ||
		m.Extra["akn.work.this"] != "/akn/xx/act/2026/1/!main" || m.Extra["akn.work.date"] != "2026-01-02" ||
		m.Extra["akn.work.date_name"] != "enacted" || m.Extra["akn.doc_type"] != "act" || m.Extra["akn.doc_name"] != "sample" ||
		m.Extra["ukm:Year"] != "2026" || m.Extra["akn.manifestation.date"] != "2026-01-03" {
		t.Errorf("metadata %+v", m)
	}
	var b strings.Builder
	for _, p := range doc.Parts {
		fmt.Fprintf(&b, "[%s %s]\n%s", p.Kind, p.Name, brief(p.Blocks))
	}
	want := `[body ]
table:
[chapter preface]
paragraph:The Sample Act 2026
[chapter preamble]
paragraph:Be it enacted by Parliament:
[body body]
heading2(Part 1):General
heading3(1.):Definitions
footnote(1):As amended in 2026.
paragraph((1)):In this Act the following terms apply.
paragraph((2)):The terms are:
list_item1((a)):apple, and
list_item1((b)):pear.
heading3(2.):Rates
table:
paragraph:The sum is:
formula:x+1
image:A figure
[chapter conclusions]
paragraph:Signed by the Editor.
[chapter attachment]
paragraph:Schedule text.
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
			if x.Text == text {
				return x
			}
		}
		t.Fatalf("no block %q", text)
		return gofastcore.Block{}
	}
	if x := find("General"); x.Attrs[AttrAnchorParent] != "part_1" || x.Attrs["akn.element"] != "part" {
		t.Errorf("part heading %+v", x)
	}
	if x := find("As amended in 2026."); x.Loc.Anchor != "art_1__note_1" || x.Attrs["akn.element"] != "authorialNote" {
		t.Errorf("note %+v", x)
	}
	if x := find("In this Act the following terms apply."); x.Loc.Anchor != "" || x.Attrs[AttrAnchorParent] != "art_1__para_1" {
		t.Errorf("paragraph %+v", x)
	}
	if x := find("apple, and"); x.Loc.Anchor != "art_1__para_2__list_1__item_a" {
		t.Errorf("item %+v", x)
	}
	if x := find("Be it enacted by Parliament:"); len(x.Spans) != 1 || x.Spans[0].Href != "/ontology/organization/xx/parliament" {
		t.Errorf("link %+v", x)
	}
	if x := find("x+1"); x.Attrs["akn.formula_markup"] != "MathML" || x.Attrs["notation"] != "plain" {
		t.Errorf("formula %+v", x)
	}
	if x := find("Schedule text."); x.Attrs["akn.doc_name"] != "schedule" || x.Attrs[AttrAnchorParent] != "att_1__para_1" {
		t.Errorf("attachment %+v", x)
	}
	refs := doc.Parts[0].Blocks[0]
	if got := mustJSON(refs.Table.Rows); got != `[["eId","id","href","showAs","name","shortForm"],["parl","","/ontology/organization/xx/parliament","Parliament","",""],["ed","","/ontology/person/xx/editor","Editor","",""]]` ||
		refs.Attrs["akn.meta"] != "references" {
		t.Errorf("references %s %v", got, refs.Attrs)
	}
	for _, x := range all {
		if x.Kind == gofastcore.KindTable && x.Loc.Anchor == "art_2__table_1" {
			// th marks a cell, not its row: HeaderRows stays 0 (no rule
			// can test a row's children).
			if got := mustJSON(x.Table.Rows); got != `[["Rate","Value"],["none",""]]` || len(x.Table.Merges) != 1 {
				t.Errorf("table %s", got)
			}
		}
	}
	if bad := aknCiteCheck(data, doc); bad != "" {
		t.Error(bad)
	}
	src, _ := aknSpec.sourceText(data, map[string]bool{})
	if got := aknSpec.docText(doc); got != src {
		t.Errorf("oracle (a):\n doc:  %s\n file: %s", got, src)
	}
}
