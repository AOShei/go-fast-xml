package gofastxml

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/AOShei/go-fast-core"
)

// The helpers of this file are shared by the tests of the patent, s1000d
// and cda profiles (profile_s1000d_test.go, profile_cda_test.go).

// patentCorpusDir returns the directory of one corpus format of a set
// ("dev" or "heldout"), or "".
func patentCorpusDir(set, format string) string {
	roots := []string{os.Getenv("GOFAST_CORPUS")}
	if roots[0] == "" {
		// The repository's sibling, or the sibling of a worktree's parent.
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

// patentCorpusFiles lists the .xml files of dir, whatever the case of
// the extension (S1000D names its files .XML).
func patentCorpusFiles(dir string) []string {
	var files []string
	filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.EqualFold(filepath.Ext(p), ".xml") {
			files = append(files, p)
		}
		return nil
	})
	sort.Strings(files)
	return files
}

// patentRunCorpus reads every file of the format in the dev and held-out
// sets and logs aggregate counts: files, errors, Validate failures,
// blocks by kind, warnings by code, and the unmapped elements and
// attributes (by name on dev only; held-out files are counted, never
// looked at). It fails on a panic, an error, a Validate failure, a
// timeout, or a dev file not read with the profile, except the dev file
// whose name contains notSelected (when not empty), which the profile
// is known not to select.
func patentRunCorpus(t *testing.T, format, profile, notSelected string) {
	if notSelected == "" {
		notSelected = "\x00"
	}
	for _, set := range []string{"dev", "heldout"} {
		dir := patentCorpusDir(set, format)
		if dir == "" {
			t.Logf("%s: no %s corpus", set, format)
			continue
		}
		files := patentCorpusFiles(dir)
		tally := newTally()
		unmapped := map[string]map[string]int{WarnUnmappedElements: {}, WarnUnmappedAttributes: {}}
		unmappedTotal := map[string]int{}
		profiles := map[string]int{}
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
			}
			if res.doc == nil {
				continue
			}
			profiles[res.doc.Metadata.Extra[ExtraProfile]]++
			if res.doc.Metadata.Extra[ExtraProfile] != profile && set == "dev" && !strings.Contains(path, notSelected) {
				t.Errorf("%s: read with profile %q, not %s", label, res.doc.Metadata.Extra[ExtraProfile], profile)
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
		t.Logf("%s: %d files in %v; profiles used: %s\n%s", set, len(files), time.Since(start).Round(time.Millisecond), byCount(profiles), tally)
		for _, code := range []string{WarnUnmappedElements, WarnUnmappedAttributes} {
			line := fmt.Sprintf("%s: %s: %d in all", set, code, unmappedTotal[code])
			if set == "dev" {
				line += ": " + byCount(unmapped[code])
			}
			t.Log(line)
		}
	}
}

// patentSkipped reports whether the element at the end of stack is in
// skip, written as the end of its path ("name", "parent/name",
// "a/b/name", up to four steps) or "/name" (a child of the root).
func patentSkipped(skip map[string]string, stack []string) (string, bool) {
	n := len(stack)
	for k := 1; k <= min(n, 4); k++ {
		key := strings.Join(stack[n-k:], "/")
		if _, ok := skip[key]; ok {
			return key, true
		}
	}
	if n == 2 {
		if _, ok := skip["/"+stack[1]]; ok {
			return "/" + stack[1], true
		}
	}
	return "", false
}

// patentSourceText is the character data of the file in order (local
// names only), without the elements skip lists, white space removed.
// The exceptions met are recorded in used.
func patentSourceText(data []byte, skip map[string]string, used map[string]bool) (string, error) {
	d := xml.NewDecoder(bytes.NewReader(data))
	d.Strict = false
	d.Entity = xml.HTMLEntity
	var stack []string
	depth := 0
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
			if depth > 0 {
				depth++
			} else if k, ok := patentSkipped(skip, stack); ok {
				depth = 1
				used[k] = true
			}
		case xml.EndElement:
			stack = stack[:len(stack)-1]
			if depth > 0 {
				depth--
			}
		case xml.CharData:
			if depth == 0 && len(stack) > 0 {
				b.WriteString(noSpace(gofastcore.ExpandLigatures(string(tk))))
			}
		}
	}
	return b.String(), nil
}

// patentRunOracles checks, for every dev file of the format, the two
// oracles: (a) the body's block and cell text in order (spDocText) is the
// file's character data in order, white space aside, apart from the
// elements skip lists; (b) every block's text is in the source at its
// offsets. Every exception must be met at least once in the dev set.
func patentRunOracles(t *testing.T, format string, skip map[string]string) {
	dir := patentCorpusDir("dev", format)
	if dir == "" {
		t.Skipf("no %s dev corpus", format)
	}
	used := map[string]bool{}
	fails := 0
	for _, path := range patentCorpusFiles(dir) {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		doc, err := ExtractBytes(context.Background(), data, nil)
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		src, err := patentSourceText(data, skip, used)
		if err != nil {
			t.Fatalf("%s: encoding/xml: %v", path, err)
		}
		if got := spDocText(doc); got != src && fails < 10 {
			fails++
			i := 0
			for i < len(got) && i < len(src) && got[i] == src[i] {
				i++
			}
			t.Errorf("%s: (a) text differs at byte %d:\n doc:  %q\n file: %q", filepath.Base(path), i, around(got, i), around(src, i))
		}
		if bad := citeCheck(data, doc); bad != "" && fails < 10 {
			fails++
			t.Errorf("%s: (b) %s", filepath.Base(path), around(bad, 0))
		}
	}
	for k := range skip {
		if !used[k] {
			t.Errorf("exception %q is met nowhere in the dev set", k)
		}
	}
}

// patentFixture reads a fixture of testdata/profiles with the built-in
// profiles, checks Validate, the profile used and that every block can be
// cited, and returns the source and the document.
func patentFixture(t *testing.T, name, profile string) ([]byte, *gofastcore.Document) {
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
		t.Fatalf("Validate: %v\n%s", err, dump(doc))
	}
	if got := doc.Metadata.Extra[ExtraProfile]; got != profile {
		t.Fatalf("profile %q, want %s", got, profile)
	}
	if bad := citeCheck(data, doc); bad != "" {
		t.Errorf("cite: %s", bad)
	}
	for _, w := range doc.AllWarnings() {
		if w.Code == WarnUnmappedElements {
			t.Errorf("fixture warning %s: %s", w.Code, w.Message)
		}
	}
	return data, doc
}

// patentExtra checks Metadata.Extra against want.
func patentExtra(t *testing.T, doc *gofastcore.Document, want map[string]string) {
	t.Helper()
	for k, v := range want {
		if got := doc.Metadata.Extra[k]; got != v {
			t.Errorf("Extra[%q] = %q, want %q", k, got, v)
		}
	}
}

// patentAttrs writes the blocks with their anchors and Attrs, one line
// each, for exact comparison.
func patentAttrs(bs []gofastcore.Block) string {
	var b strings.Builder
	for _, x := range bs {
		keys := make([]string, 0, len(x.Attrs))
		for k := range x.Attrs {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		fmt.Fprintf(&b, "%s #%s", x.Kind, x.Loc.Anchor)
		for _, k := range keys {
			fmt.Fprintf(&b, " %s=%s", k, x.Attrs[k])
		}
		b.WriteString("\n")
	}
	return b.String()
}

// patentCheck compares the fixture's blocks (brief) and their anchors and
// Attrs with the expected text.
func patentCheck(t *testing.T, doc *gofastcore.Document, wantBrief, wantAttrs string) {
	t.Helper()
	bs := blocks(doc)
	if got := brief(bs); got != wantBrief {
		t.Errorf("blocks:\n%s\nwant:\n%s", got, wantBrief)
	}
	if got := patentAttrs(bs); got != wantAttrs {
		t.Errorf("attrs:\n%s\nwant:\n%s", got, wantAttrs)
	}
}

func TestPatentUSGrant(t *testing.T) {
	_, doc := patentFixture(t, "patent/us-grant.xml", "patent")
	patentExtra(t, doc, map[string]string{
		"pat.doc_type": "us-patent-grant", "pat.pub_country": "US", "pat.pub_number": "09999999", "pat.pub_kind": "B2",
		"pat.pub_date": "20080101", "pat.app_type": "utility", "pat.app_number": "11000001", "pat.app_date": "20050623",
		"pat.priority": "JP 2004-000001 20041224; JP 2004-000002 20041225", "pat.priority_kind": "national; national", "pat.priority_sequence": "01; 02",
		"pat.class_national": "US 123/456", "pat.title_id": "d0e71", "pat.number_of_claims": "2",
		"pat.applicant": "Doe Jane Austin US", "pat.applicant_type": "applicant-inventor", "pat.date_publ": "20080101",
	})
	if doc.Metadata.Title != "Widget holder" {
		t.Errorf("title %q", doc.Metadata.Title)
	}
	patentCheck(t, doc, `heading1:Widget holder
list_item1:US 1234567 A Smith 19900400 cited by examiner
paragraph:A widget holder.
image:embedded image
heading2:BACKGROUND
paragraph:See FIG. 1; H2O at 103 Pa.
paragraph:Results:
caption:Table 1
table:
paragraph:as shown.
paragraph:Energy:
E=m2
paragraph:Compound:
image:embedded image
list_item1:first item
paragraph:What is claimed is:
list_item1:1. A holder comprising: a base; and a lid.
list_item1:2. The holder of claim 1, made of steel.
image:
`, `heading #
list_item #cit1 pat.section=references-cited
paragraph #0000 pat.id=p-0001 pat.section=abstract
image #EMI-D00001 pat.height=100mm pat.img_content=drawing pat.img_format=tif pat.num=00001 pat.section=drawings pat.width=80mm
heading #h-0001 pat.level=1 pat.section=description
paragraph #0001 pat.id=p-0002 pat.section=description
paragraph #0002 pat.id=p-0003 pat.section=description
caption # anchor_parent=TABLE-US-00001 pat.section=description
table #TABLE-US-00001 pat.frame=none pat.num=00001 pat.section=description
paragraph #0002 pat.id=p-0003 pat.section=description
paragraph #0003 pat.id=p-0004 pat.section=description
paragraph #0004 pat.id=p-0005 pat.section=description
image #EMI-C00001 pat.height=20mm pat.img_content=chem pat.img_format=tif pat.kind=chemistry pat.num=00001 pat.parent_id=CHEM-US-00001 pat.section=description pat.width=30mm
list_item #0000 pat.id=ul0001-0001 pat.list_style=none pat.section=description
paragraph # pat.part=claim-statement
list_item #00001 pat.id=CLM-00001 pat.section=claims
list_item #00002 pat.id=CLM-00002 pat.section=claims
image # pat.idref=CHEM-US-00001 pat.kind=chemistry pat.mol_file=US09999999-C00001.MOL
`)
	bs := blocks(doc)
	if tb := bs[8].Table; tb == nil || tb.HeaderRows != 1 || len(tb.Rows) != 2 || tb.Rows[1][1] != "90%" {
		t.Errorf("table %+v", bs[8].Table)
	}
	if sp := bs[16].Spans; len(sp) != 1 || sp[0].Href != "CLM-00001" {
		t.Errorf("claim-ref spans %+v", sp)
	}
	if bs[17].Image == nil || bs[17].Image.ID != "US09999999-C00001.CDX" {
		t.Errorf("us-chemistry %+v", bs[17].Image)
	}
	if bs[5].Spans == nil || bs[5].Spans[0].Href != "DRAWINGS" {
		t.Errorf("figref spans %+v", bs[5].Spans)
	}
}

func TestPatentEP(t *testing.T) {
	_, doc := patentFixture(t, "patent/ep.xml", "patent")
	patentExtra(t, doc, map[string]string{
		"pat.doc_type": "ep-patent-document", "pat.pub_number": "1000001", "pat.pub_kind": "B1", "pat.pub_date": "20030212",
		"pat.pub_kind_text": "EUROPEAN PATENT SPECIFICATION", "pat.app_number": "99000001.1", "pat.app_date": "19991108",
		"pat.filing_lang": "nl", "pat.pub_lang": "en", "pat.grant_date": "20030212", "pat.grant_bulletin": "200307",
		"pat.class_ipcr": "B28B 5/02 20060101AFI20051220BHEP", "pat.title": "Vorrichtung\nApparatus", "pat.title_lang": "de; en",
		"pat.inventor": "Kosman, Wim; Jansen, Piet", "pat.inventor_city": "Groesbeek; Nijmegen", "pat.designated_states": "AT BE",
		"pat.bib_lang": "en", "pat.kind": "B1",
	})
	patentCheck(t, doc, `list_item1:EP-A- 0 680 812
list_item1:SMITH: "Bricks", 1993
note:EPO <DP n="1">
heading2:FIELD
paragraph:The invention comprises:
(i) a frame;
(ii) a stop.
list_item1:Apparatus characterized in that it has a frame.
image:
`, `list_item # pat.cit_type=patent pat.section=references-cited
list_item # pat.cit_type=non-patent pat.section=references-cited
note # stored=markup_comment
heading #h0001 pat.lang=en pat.section=description
paragraph #0001 pat.id=p0001 pat.lang=en pat.section=description
list_item #0001 pat.id=c-en-01-0001 pat.lang=en pat.section=claims
image #if0001 pat.height=196 pat.img_content=drawing pat.img_format=tif pat.lang=en pat.num=1 pat.section=drawings pat.width=161
`)
	st := doc.Parts[len(doc.Parts)-1]
	found := false
	for _, b := range st.Blocks {
		if b.Attrs[gofastcore.AttrStored] == gofastcore.StoredCustomXML && b.Text == "DIM350 (Ver 2.1 Jan 2001)" {
			found = true
		}
	}
	if !found {
		t.Errorf("eptags not stored: %s", dump(doc))
	}
}

func TestPatentCorpus(t *testing.T) {
	patentRunCorpus(t, "xml-patent", "patent", "")
}

// patentOracleSkip lists the elements whose character data is metadata
// (or attrs, or stored), not block text, by design.
var patentOracleSkip = map[string]string{
	// USPTO bibliographic data, to Metadata.Extra.
	"publication-reference": "metadata", "application-reference": "metadata", "us-application-series-code": "metadata",
	"priority-claims": "metadata", "us-term-of-grant": "metadata", "classification-locarno": "metadata",
	"us-bibliographic-data-grant/classification-national": "metadata", "us-bibliographic-data-application/classification-national": "metadata",
	"classifications-ipcr": "metadata", "us-bibliographic-data-grant/classification-ipc": "metadata", "assignees": "metadata",
	"us-field-of-classification-search": "metadata", "us-bibliographic-data-grant/field-of-search": "metadata",
	"number-of-claims": "metadata", "us-exemplary-claim": "metadata", "us-bibliographic-data-grant/figures": "metadata",
	"us-related-documents": "metadata", "parties": "metadata", "us-parties": "metadata", "examiners": "metadata",
	// EPO bibliographic data (SDOBI), to Metadata.Extra; eptags stored.
	"B000": "stored", "B100": "metadata", "B200": "metadata", "B300": "metadata", "B400": "metadata",
	"B510": "metadata", "B510EP": "metadata", "B520EP": "metadata", "B540": "metadata", "B590": "metadata", "B565EP": "metadata",
	"B600": "metadata", "B700": "metadata", "B800": "metadata",
	// pap-v16 bibliographic data.
	"subdoc-bibliographic-information/document-id": "metadata", "domestic-filing-data": "metadata",
	"technical-information/classification-ipc": "metadata", "classification-us": "metadata",
	"inventors": "metadata", "correspondence-address": "metadata", "continuity-data": "metadata",
	"publication-filing-type": "metadata", "representative-figure": "metadata",
	"pct-or-regional-filing-data": "metadata", "pct-or-regional-publishing-data": "metadata",
}

func TestPatentOracles(t *testing.T) {
	patentRunOracles(t, "xml-patent", patentOracleSkip)
}
