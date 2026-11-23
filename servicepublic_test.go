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
// space removed: not stored data, not text from an attribute, not the
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
