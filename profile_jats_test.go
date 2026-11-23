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

// The helpers named jats* below that take a profile name are shared by the
// group-A profile tests (jats, docbook, tei, dita).

// jatsCorpusDir returns the directory of a corpus set and format, or "".
func jatsCorpusDir(set, format string) string {
	roots := []string{os.Getenv("GOFAST_CORPUS")}
	if roots[0] == "" {
		// The repository's sibling, or, from a worktree one level down,
		// its parent's sibling.
		roots = []string{filepath.Join("..", ".testdata", "corpus"), filepath.Join("..", "..", ".testdata", "corpus")}
	}
	for _, root := range roots {
		dir := filepath.Join(root, set, format)
		if _, err := os.Stat(dir); err == nil {
			return dir
		}
	}
	return ""
}

func jatsCorpusFiles(dir string) []string {
	var files []string
	filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			files = append(files, p)
		}
		return nil
	})
	sort.Strings(files)
	return files
}

// jatsCensus counts, over the dev files of a format, elements by
// parent/name and attributes by name@attr, to map from the most frequent
// down. It only logs.
func jatsCensus(t *testing.T, format string) {
	dir := jatsCorpusDir("dev", format)
	if dir == "" {
		t.Skip("no dev corpus")
	}
	elems, attrs := map[string]int{}, map[string]int{}
	for _, path := range jatsCorpusFiles(dir) {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		d := xml.NewDecoder(bytes.NewReader(data))
		d.Strict = false
		var stack []string
		for {
			tok, err := d.Token()
			if err != nil {
				break
			}
			switch tk := tok.(type) {
			case xml.StartElement:
				parent := ""
				if len(stack) > 0 {
					parent = stack[len(stack)-1]
				}
				elems[parent+"/"+tk.Name.Local]++
				for _, a := range tk.Attr {
					attrs[tk.Name.Local+"@"+a.Name.Local]++
				}
				stack = append(stack, tk.Name.Local)
			case xml.EndElement:
				if len(stack) > 0 {
					stack = stack[:len(stack)-1]
				}
			}
		}
	}
	t.Logf("elements: %s", byCount(elems))
	t.Logf("attributes: %s", byCount(attrs))
}

// jatsOracleCase configures the corpus test and the two oracles for one
// profile.
type jatsOracleCase struct {
	profile string
	format  string // corpus directory, e.g. "xml-jats"
	// skip lists the elements whose character data is not block or cell
	// text of the body, by design, as a path of local names matched at
	// the end of the element's path ("a/b/name", "name"); a leading "/"
	// anchors it at the root. Each must be met on dev.
	skip map[string]string
	// gap is an Attrs key and value marking blocks whose text is in their
	// span in order but with gaps (see jatsCiteCheck).
	gap []string
	// prep, if set, adjusts a document before the oracles see it (blocks
	// whose text or Marker the profile takes from an attribute).
	prep func(doc *gofastcore.Document)
}

func (c *jatsOracleCase) skipped(stack []string) (string, bool) {
	best := ""
	for k := range c.skip {
		steps := strings.Split(strings.TrimPrefix(k, "/"), "/")
		n := len(steps)
		if n > len(stack) || strings.HasPrefix(k, "/") && n != len(stack) {
			continue
		}
		ok := true
		for i := range n {
			if s := steps[n-1-i]; s != "*" && s != stack[len(stack)-1-i] {
				ok = false
				break
			}
		}
		if ok && (best == "" || k < best) {
			best = k
		}
	}
	return best, best != ""
}

// sourceText is the character data of the file in order, without the
// skipped elements, white space removed. Entities the reader does not
// define are left as written.
func (c *jatsOracleCase) sourceText(data []byte, used map[string]bool) (string, error) {
	d := xml.NewDecoder(bytes.NewReader(data))
	d.Strict = false
	d.Entity = xml.HTMLEntity
	d.CharsetReader = func(label string, r io.Reader) (io.Reader, error) {
		// ISO-8859-1 only, the one other encoding in the dev sets.
		if l := strings.ToLower(label); l != "iso-8859-1" && l != "latin1" {
			return nil, fmt.Errorf("charset %s", label)
		}
		b, err := io.ReadAll(r)
		if err != nil {
			return nil, err
		}
		u := make([]rune, len(b))
		for i, c := range b {
			u[i] = rune(c)
		}
		return strings.NewReader(string(u)), nil
	}
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
			stack = append(stack, tk.Name.Local)
			if skip > 0 {
				skip++
			} else if k, ok := c.skipped(stack); ok {
				skip = 1
				used[k] = true
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

// jatsDocText is the text of the body's blocks and cells in order, white
// space removed, as spDocText: not stored data, not text from an
// attribute, not an image's alt, not the numbers of ordered items, which
// the file does not hold. A table's Marker (its label) comes before its
// cells.
func jatsDocText(doc *gofastcore.Document) string {
	var b strings.Builder
	for _, p := range doc.Parts {
		if p.Kind == gofastcore.PartStored {
			continue
		}
		for _, x := range p.Blocks {
			switch {
			case x.Attrs[gofastcore.AttrStored] != "", x.Attrs[AttrAttribute] != "", x.Kind == gofastcore.KindImage:
				continue
			}
			if x.Marker != "" && !(x.Kind == gofastcore.KindListItem && x.Ordered && strings.HasSuffix(x.Marker, ".") && x.Marker[0] >= '0' && x.Marker[0] <= '9') {
				b.WriteString(noSpace(x.Marker))
			}
			if x.Kind == gofastcore.KindTable {
				for _, row := range x.Table.Rows {
					for _, c := range row {
						b.WriteString(noSpace(cellMarkerRE.ReplaceAllString(c, "$1")))
					}
				}
				continue
			}
			b.WriteString(noSpace(x.Text))
		}
	}
	return b.String()
}

// run is the corpus test: every file of dev and held-out read with the
// profile, aggregate counts logged (names only on dev).
func (c *jatsOracleCase) run(t *testing.T) {
	for _, set := range []string{"dev", "heldout"} {
		dir := jatsCorpusDir(set, c.format)
		if dir == "" {
			t.Logf("%s: no %s corpus", set, c.format)
			continue
		}
		files := jatsCorpusFiles(dir)
		tally := newTally()
		unmapped := map[string]map[string]int{WarnUnmappedElements: {}, WarnUnmappedAttributes: {}}
		unmappedTotal := map[string]int{}
		start := time.Now()
		for _, path := range files {
			res := corpusOne(path)
			tally.add(c.format, res)
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
			case res.doc.Metadata.Extra[ExtraProfile] != c.profile:
				t.Errorf("%s: read with profile %q, not %s", label, res.doc.Metadata.Extra[ExtraProfile], c.profile)
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

// oracles checks, over every dev file, (a) the body's block and cell text
// in order is the file's character data in order, white space aside,
// apart from the listed exceptions, and (b) every block's text is in the
// source at its offsets. Every exception must be met.
func (c *jatsOracleCase) oracles(t *testing.T) {
	dir := jatsCorpusDir("dev", c.format)
	if dir == "" {
		t.Skip("no dev corpus")
	}
	used := map[string]bool{}
	fails := 0
	for _, path := range jatsCorpusFiles(dir) {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		doc, err := ExtractBytes(context.Background(), data, nil)
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		src, err := c.sourceText(data, used)
		if err != nil {
			t.Fatalf("%s: encoding/xml: %v", path, err)
		}
		if c.prep != nil {
			c.prep(doc)
		}
		got := jatsDocText(doc)
		if got != src {
			// The numbers the engine writes for the items of an ordered
			// list joined into a block or cell are not in the file: compare
			// both sides without numbers followed by a full stop.
			got, src = jatsNumberRE.ReplaceAllString(got, ""), jatsNumberRE.ReplaceAllString(src, "")
		}
		if got != src && jatsEntityRE.MatchString(src) && jatsInOrder(got, jatsEntityRE.Split(src, -1)) {
			// Entities the oracle's reader cannot expand (declared in the
			// DTD or the internal subset): the text between them is in the
			// document in order.
			got = src
		}
		if got != src && fails < 10 {
			fails++
			i := 0
			for i < len(got) && i < len(src) && got[i] == src[i] {
				i++
			}
			t.Errorf("%s: (a) text differs at byte %d:\n doc:  %q\n file: %q", filepath.Base(path), i, around(got, i), around(src, i))
		}
		if bad := c.cite(data, doc); bad != "" && fails < 10 {
			fails++
			t.Errorf("%s: (b) %s", filepath.Base(path), around(bad, 0))
		}
	}
	for k := range c.skip {
		if !used[k] {
			t.Errorf("exception %q is met nowhere in the dev set", k)
		}
	}
}

var jatsNumberRE = regexp.MustCompile(`\d+\.`)

// jatsCiteCheck is citeCheck (A6) with two exceptions: the numbers the
// engine writes in front of the items of an ordered list joined into a
// block ("1. "), which the file does not hold, are not looked for; and a
// block marked with Attrs[gapKey] (a contributor whose name went to
// Metadata.Authors) must be in its span in order, with gaps.
func jatsCiteCheck(src []byte, doc *gofastcore.Document, gapKey ...string) string {
	c := *doc
	c.Parts = make([]gofastcore.Part, len(doc.Parts))
	for i, p := range doc.Parts {
		p.Blocks = append([]gofastcore.Block(nil), p.Blocks...)
		for j := range p.Blocks {
			b := &p.Blocks[j]
			if len(gapKey) > 0 && b.Attrs[gapKey[0]] == gapKey[1] && citable(b) {
				l := b.Loc
				if l.Offset < 0 || l.Offset+l.Length > int64(len(src)) ||
					!jatsSubsequence(noSpace(b.Text), noSpace(stripMarkup(string(src[l.Offset:l.Offset+l.Length])))) {
					return "block " + strconv.Itoa(j) + " text " + strconv.Quote(b.Text) + " not in order in its span"
				}
				b.Attrs = map[string]string{AttrAttribute: "checked"}
				continue
			}
			// A generated item number at a line's start is not in the
			// source; a number the file wrote there is (text after a
			// list nested in a paragraph starts a line): strip only
			// when the text as it stands is not in its span.
			l := b.Loc
			asIs := l.Offset >= 0 && l.Length > 0 && l.Offset+l.Length <= int64(len(src)) &&
				strings.Contains(noSpace(stripMarkup(string(src[l.Offset:l.Offset+l.Length]))), noSpace(gofastcore.ExpandLigatures(b.Text)))
			if t := strings.TrimSpace(cellMarkerRE.ReplaceAllString(b.Text, "$1")); t != "" && !asIs {
				b.Text = t
			}
		}
		c.Parts[i] = p
	}
	return citeCheck(src, &c)
}

// jatsEntityRE is a reference to an entity other than XML's five.
var jatsEntityRE = regexp.MustCompile(`&(?:[A-Za-z_][\w.-]*);`)

// jatsInOrder reports whether the pieces are in s in order.
func jatsInOrder(s string, pieces []string) bool {
	for _, p := range pieces {
		if strings.Contains(p, "&amp;") || strings.Contains(p, "&lt;") {
			p = strings.NewReplacer("&amp;", "&", "&lt;", "<", "&gt;", ">", "&quot;", `"`, "&apos;", "'").Replace(p)
		}
		i := strings.Index(s, p)
		if i < 0 {
			return false
		}
		s = s[i+len(p):]
	}
	return true
}

// cite is jatsCiteCheck, and for a block it rejects, two further readings
// of its span: without the elements the case's exceptions name (an index
// term inside a paragraph), and, when the span refers to entities the
// file declares, with the text between the references in the block in
// order.
func (c *jatsOracleCase) cite(src []byte, doc *gofastcore.Document) string {
	for {
		bad := jatsCiteCheck(src, doc, c.gap...)
		if bad == "" {
			return ""
		}
		var pi, bi int
		if _, err := fmt.Sscanf(bad, "block %d", &bi); err != nil {
			return bad
		}
		// Find the part: citeCheck numbers blocks within their part.
		found := false
		for p := range doc.Parts {
			if bi < len(doc.Parts[p].Blocks) {
				b := &doc.Parts[p].Blocks[bi]
				if l := b.Loc; citable(b) && l.Offset >= 0 && l.Offset+l.Length <= int64(len(src)) && strings.Contains(bad, strconv.Quote(b.Text)) {
					pi, found = p, true
					break
				}
			}
		}
		if !found {
			return bad
		}
		b := &doc.Parts[pi].Blocks[bi]
		span := src[b.Loc.Offset : b.Loc.Offset+b.Loc.Length]
		text := noSpace(gofastcore.ExpandLigatures(strings.TrimSpace(cellMarkerRE.ReplaceAllString(b.Text, "$1"))))
		ok := false
		if s, err := c.sourceText(span, map[string]bool{}); err == nil {
			s, text := jatsNumberRE.ReplaceAllString(s, ""), jatsNumberRE.ReplaceAllString(text, "")
			ok = strings.Contains(s, text) || jatsEntityRE.MatchString(s) && jatsInOrder(text, jatsEntityRE.Split(s, -1))
		}
		// An undeclared entity is kept as written: the raw span holds it.
		ok = ok || strings.Contains(noSpace(stripMarkup(strings.ReplaceAll(string(span), "&", "&amp;"))), text)
		if !ok {
			return bad
		}
		// Accepted: check the rest of the document without it.
		cp := *doc
		cp.Parts = append([]gofastcore.Part(nil), doc.Parts...)
		cp.Parts[pi].Blocks = append([]gofastcore.Block(nil), doc.Parts[pi].Blocks...)
		cp.Parts[pi].Blocks[bi].Attrs = map[string]string{AttrAttribute: "checked"}
		doc = &cp
	}
}

// jatsSubsequence reports whether s is a subsequence of t, byte by byte.
func jatsSubsequence(s, t string) bool {
	for i := 0; i < len(t) && len(s) > 0; i++ {
		if t[i] == s[0] {
			s = s[1:]
		}
	}
	return s == ""
}

// jatsExtractFixture reads a fixture of testdata/profiles with the
// built-in profiles, checks Validate, the profile chosen and A6, and
// returns the document and the source.
func jatsExtractFixture(t *testing.T, name, profile string) (*gofastcore.Document, []byte) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "profiles", name))
	if err != nil {
		t.Fatal(err)
	}
	doc, err := ExtractBytes(context.Background(), data, &gofastcore.Options{Spans: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := doc.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if got := doc.Metadata.Extra[ExtraProfile]; got != profile {
		t.Fatalf("profile %q, want %s", got, profile)
	}
	return doc, data
}

// jatsBriefAll writes the blocks of every part, stored included, with
// the part's kind and name before them and anchors after the text.
func jatsBriefAll(doc *gofastcore.Document) string {
	var b strings.Builder
	for _, p := range doc.Parts {
		b.WriteString(strings.TrimSpace(fmt.Sprintf("# %s %s", p.Kind, p.Name)) + "\n")
		for _, x := range p.Blocks {
			b.WriteString(strings.TrimSuffix(brief([]gofastcore.Block{x}), "\n"))
			if x.Loc.Anchor != "" {
				b.WriteString(" #" + x.Loc.Anchor)
			}
			if a := x.Attrs[AttrAnchorParent]; a != "" {
				b.WriteString(" ^" + a)
			}
			b.WriteString("\n")
		}
	}
	return b.String()
}

func TestJatsFixture(t *testing.T) {
	doc, data := jatsExtractFixture(t, "jats/article.xml", "jats")
	want := `# body front
heading1:A study of tests
paragraph:1 #c1
paragraph(1):University of Tests , UK #aff1
heading2:Abstract
paragraph:We test.
# body body
heading2(1.):Introduction #s1
paragraph:See [1] and the site, bold H2O. #p1
footnote(a):A footnote. #fn1
list_item1(1.):First ^s1
list_item1(2.):Second ^s1
list_item1:API ^s1
paragraph:Interface. ^s1
formula((1)):x=2 #e1
heading3:Sub #s1.1
paragraph:Inline y math. ^s1.1
caption(Figure 1.):Fig title.
Fig text. #f1
image: ^f1
caption:Table 1. #t1
caption:Tab cap. #t1
table: ^t1
footnote:Table note. #tfn1
caption:Key table #t2
table: ^t2
caption(Data S1):Raw data. #sm1
# body back
heading2:Acknowledgements
paragraph:Thanks.
heading2:References
footnote(1):Smith J 2010 On tests Nature 1 2 10.1/x #r1
`
	if got := jatsBriefAll(doc); got != want {
		t.Errorf("blocks:\n%s\nwant:\n%s", got, want)
	}
	m := doc.Metadata
	for k, v := range map[string]string{"jats.doi": "10.1234/test.1", "jats.pmid": "999", "jats.journal": "Journal of Tests",
		"jats.journal_id": "J Test", "jats.journal_id_type": "nlm-ta", "jats.issn": "1234-5678", "jats.publisher": "Test Press",
		"jats.subject": "Biology", "jats.pub_date": "18 12 2012", "jats.pub_date_type": "epub", "jats.volume": "2", "jats.fpage": "993",
		"jats.copyright": "© 2012 The Authors", "jats.license": "CC BY 4.0", "jats.license_href": "https://creativecommons.org/licenses/by/4.0/",
		"jats.article_type": "research-article", "jats.dtd_version": "1.2", "jats.lang": "en"} {
		if m.Extra[k] != v {
			t.Errorf("Extra[%s] = %q, want %q", k, m.Extra[k], v)
		}
	}
	if m.Title != "A study of tests" || strings.Join(m.Authors, "|") != "Krusche Adele" || strings.Join(m.Keywords, "|") != "testing|profiles" {
		t.Errorf("metadata %+v", m)
	}
	bs := blocks(doc)
	find := func(text string) *gofastcore.Block {
		for i := range bs {
			if strings.HasPrefix(bs[i].Text, text) {
				return &bs[i]
			}
		}
		t.Fatalf("no block %q", text)
		return nil
	}
	if b := find("1"); b.Attrs["jats.contrib_type"] != "author" || b.Attrs["jats.field"] != "contrib" {
		t.Errorf("contrib %v", b.Attrs)
	}
	p := find("See [1]")
	var hrefs []string
	for _, s := range p.Spans {
		if s.Href != "" {
			hrefs = append(hrefs, s.Href)
		}
	}
	if strings.Join(hrefs, " ") != "r1 https://example.org" {
		t.Errorf("links %+v", p.Spans)
	}
	if f := find("x=2"); f.Kind != gofastcore.KindFormula || f.Attrs["notation"] != "plain" || f.Attrs["jats.math"] != "mathml" || !doc.Parts[1].Has(gofastcore.WarnInputUnreadContent) {
		t.Errorf("formula %+v", f)
	}
	if im := bs[slicesIndexKind(bs, gofastcore.KindImage)]; im.Image.ID != "fig1.tif" {
		t.Errorf("image %+v", im.Image)
	}
	if tb := find("").Table; tb != nil {
		t.Errorf("first block is a table")
	}
	for _, b := range bs {
		if b.Kind == gofastcore.KindTable && b.Attrs[AttrAnchorParent] == "t1" {
			if mustJSON(b.Table.Rows) != `[["H1","H2"],["wide",""]]` || b.Table.HeaderRows != 1 || len(b.Table.Merges) != 1 {
				t.Errorf("table %+v", b.Table)
			}
		}
	}
	if sm := find("Raw data."); sm.Attrs["jats.href"] != "data.xlsx" || sm.Attrs["jats.box"] != "supplementary-material" {
		t.Errorf("supplementary %v", sm.Attrs)
	}
	if bad := jatsCiteCheck(data, doc, "jats.field", "contrib"); bad != "" {
		t.Error(bad)
	}
	for _, w := range doc.AllWarnings() {
		if w.Code == WarnUnmappedElements {
			t.Errorf("unmapped: %s", w.Message)
		}
	}
}

func slicesIndexKind(bs []gofastcore.Block, k gofastcore.Kind) int {
	for i := range bs {
		if bs[i].Kind == k {
			return i
		}
	}
	return 0
}

func TestJatsCensus(t *testing.T) { jatsCensus(t, "xml-jats") }

var jatsCase = &jatsOracleCase{profile: "jats", format: "xml-jats", skip: map[string]string{
	// Metadata only (Metadata fields and Extra jats.*).
	"journal-meta": "metadata", "processing-meta": "metadata",
	"article-meta/article-id": "metadata", "article-meta/article-version": "metadata",
	"article-categories": "metadata", "title-group/alt-title": "metadata",
	"article-meta/contrib-group/contrib/name":   "Metadata.Authors",
	"article-meta/contrib-group/contrib/collab": "Metadata.Authors",
	"article-meta/page-range":                   "metadata",
	"article-meta/pub-date":                     "metadata", "article-meta/volume": "metadata", "article-meta/issue": "metadata",
	"article-meta/fpage": "metadata", "article-meta/lpage": "metadata", "article-meta/elocation-id": "metadata",
	"article-meta/issue-id": "metadata", "article-meta/history": "metadata", "article-meta/pub-history": "metadata",
	"article-meta/permissions": "metadata", "article-meta/self-uri": "metadata", "article-meta/funding-group": "metadata",
	"article-meta/custom-meta-group": "metadata", "article-meta/counts": "metadata",
	"kwd-group/kwd": "Metadata.Keywords", "kwd-group/title": "metadata", "kwd-group/label": "metadata",
}, gap: []string{"jats.field", "contrib"}}

func TestJatsCorpus(t *testing.T) { jatsCase.run(t) }

func TestJatsOracles(t *testing.T) { jatsCase.oracles(t) }
