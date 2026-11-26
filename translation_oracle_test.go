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
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/AOShei/go-fast-core"
)

// transCase is the oracle setting of a translation profile (tmx, xliff):
// the dev files are read as they are, many not well-formed or in another
// encoding, so a file encoding/xml stops in is compared up to there.
type transCase struct {
	profile, format string
	// skip: elements whose character data is not body text by design
	// (path of names as jatsOracleCase.skip; each must be met on dev).
	skip map[string]string
	// with: elements whose rule writes attributes after their text
	// ("with"), path to the attributes, space-separated.
	with map[string]string
	// skipEl: elements left out by an attribute test (an embedded
	// document handed to the child extractor).
	skipEl func(xml.StartElement) bool
	name   func(xml.Name) string
	// keyed: tables whose cells are in column order, not file order (TMX
	// languages, XLIFF source, target, notes, other): when (a) finds
	// another order, the text must be the same characters, and (b)
	// places each row.
	keyed bool
	// codepoint: elements whose rule writes the character an attribute
	// holds in hexadecimal (XLIFF 2 cp@hex), path to the attribute; a
	// control character (Cc) is not written.
	codepoint map[string]string
	// ancestorCols: the leading columns of a row taken from an
	// ancestor's attributes (XLIFF 2 unit@id, name, type), outside the
	// row's span; attrCols, when not 0, the number of leading columns
	// made from attributes.
	ancestorCols, attrCols int
}

// cpRE is an XLIFF 2 cp, whose hex code point is written as its
// character (hexChar, profile_akn_test.go).
var cpRE = regexp.MustCompile(`<(?:[\w-]+:)?cp\b[^>]*?\bhex="([^"]*)"[^>]*/>`)

// sameChars reports whether a and b hold the same characters, in any
// order.
func sameChars(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	ra, rb := []rune(a), []rune(b)
	slices.Sort(ra)
	slices.Sort(rb)
	return slices.Equal(ra, rb)
}

// sourceText is the file's character data in order, white space removed,
// without skip's elements, with with's attributes after their element's
// text. err is encoding/xml's, the text being what it read before.
func (c *transCase) sourceText(data []byte, used map[string]bool) (string, error) {
	d := xml.NewDecoder(bytes.NewReader(data))
	d.Strict = false
	d.Entity = xml.HTMLEntity
	d.CharsetReader = func(label string, r io.Reader) (io.Reader, error) {
		return nil, fmt.Errorf("charset %s", label)
	}
	var stack, after []string
	skip := 0
	var b strings.Builder
	for {
		tok, err := d.Token()
		if err == io.EOF {
			return b.String(), nil
		}
		if err != nil {
			return b.String(), err
		}
		switch tk := tok.(type) {
		case xml.StartElement:
			n := tk.Name.Local
			if c.name != nil {
				n = c.name(tk.Name)
			}
			stack = append(stack, n)
			a := ""
			if skip > 0 {
				skip++
			} else if k, ok := pathMatch(c.skip, stack); ok {
				skip, used[k] = 1, true
			} else if c.skipEl != nil && c.skipEl(tk) {
				skip, used["skipEl"] = 1, true
			} else if k, ok := pathMatch(c.codepoint, stack); ok {
				for _, at := range tk.Attr {
					if at.Name.Local == c.codepoint[k] {
						a, used["@"+k] = noSpace(hexChar(at.Value)), true
					}
				}
			} else if k, ok := pathMatch(c.with, stack); ok {
				for _, name := range strings.Fields(c.with[k]) {
					for _, at := range tk.Attr {
						if at.Name.Local == name {
							a, used["@"+k] = a+noSpace(at.Value), true
						}
					}
				}
			}
			after = append(after, a)
		case xml.EndElement:
			if len(stack) == 0 {
				continue
			}
			b.WriteString(after[len(after)-1])
			stack, after = stack[:len(stack)-1], after[:len(after)-1]
			if skip > 0 {
				skip--
			}
		case xml.CharData:
			if skip == 0 && len(stack) > 0 {
				b.WriteString(noSpace(gofastcore.ExpandLigatures(string(tk))))
			}
		}
	}
}

// docText is jatsDocText without the header rows of tables: the
// profile's words or attribute values (TMX languages), not character data.
// No list is ever in a cell of these profiles, so a cell's text is taken
// as it is (jatsDocText drops "1. " at a line's start as a list number).
func transDocText(doc *gofastcore.Document) string { return transDocTextN(doc, 0) }

// transDocTextN is transDocText with n leading attribute columns, when n
// is not 0.
func transDocTextN(doc *gofastcore.Document, n int) string {
	var b strings.Builder
	for _, p := range doc.Parts {
		if p.Kind == gofastcore.PartStored {
			continue
		}
		for _, x := range p.Blocks {
			if x.Table != nil {
				// Header rows that are the profile's or attribute values,
				// and the leading columns made from attributes ("cells"),
				// are not character data.
				from := 0
				if x.Attrs[AttrHeaderSource] != "" {
					from = x.Table.HeaderRows
				}
				// A child's attribute (target@state, bin-target@state)
				// is one column whichever child gives it.
				cols := map[string]bool{}
				for _, n := range strings.Fields(x.Attrs[AttrAttribute]) {
					cols[n[strings.LastIndex(n, "@")+1:]] = true
				}
				skip := len(cols)
				if n > 0 {
					skip = n
				}
				for _, r := range x.Table.Rows[from:] {
					for _, c := range r[min(skip, len(r)):] {
						b.WriteString(noSpace(c))
					}
				}
				continue
			}
			one := *doc
			one.Parts = []gofastcore.Part{{Kind: p.Kind, Blocks: []gofastcore.Block{x}}}
			b.WriteString(jatsDocText(&one))
		}
	}
	return b.String()
}

// rowCite checks that every table row's cells are in the source at the
// row's location: each line of a cell in the row's span with markup
// stripped, or, for a line with text written from attributes ("with"),
// each of its words in the span as written.
func rowCite(src []byte, doc *gofastcore.Document) string { return rowCiteFrom(src, doc, 0) }

// rowCiteFrom is rowCite without the first columns of each row, an
// ancestor's attributes outside the row's span.
func rowCiteFrom(src []byte, doc *gofastcore.Document, first int) string {
	for _, x := range blocks(doc) {
		if x.Table == nil {
			continue
		}
		for i, r := range x.Table.Rows {
			if i < x.Table.HeaderRows {
				continue
			}
			if i >= len(x.Table.RowLocs) {
				return fmt.Sprintf("table at %d: row %d has no location", x.Loc.Offset, i)
			}
			l := x.Table.RowLocs[i]
			if l.Offset < 0 || l.Length <= 0 || l.Offset+l.Length > int64(len(src)) {
				return fmt.Sprintf("table at %d: row %d has no span", x.Loc.Offset, i)
			}
			raw := string(src[l.Offset : l.Offset+l.Length])
			if !utf8.ValidString(raw) {
				continue // another encoding: the bytes are not the text
			}
			span := noSpace(stripMarkup(raw))
			data := noSpace(charData([]byte(raw)))
			// An empty placeholder's equiv-text is written where it stands.
			data += "\x00" + noSpace(charData([]byte(equivRE.ReplaceAllString(raw, "$2"))))
			// A cp's character (XLIFF 2) is written where it stands.
			data += "\x00" + noSpace(charData([]byte(cpRE.ReplaceAllStringFunc(raw, func(m string) string {
				return hexChar(cpRE.FindStringSubmatch(m)[1])
			}))))
			for _, cell := range r[min(first, len(r)):] {
				for line := range strings.SplitSeq(cell, "\n") {
					if l := noSpace(gofastcore.ExpandLigatures(line)); strings.Contains(span, l) || strings.Contains(data, l) {
						continue
					}
					for _, w := range strings.Fields(line) {
						if !strings.Contains(span, noSpace(w)) && !strings.Contains(raw, w) && !strings.Contains(xmlUnescape.Replace(raw), w) {
							return fmt.Sprintf("table at %d: row %d: %q not in its span %q", x.Loc.Offset, i, line, around(raw, 0))
						}
					}
				}
			}
		}
	}
	return ""
}

// corpus reads every file of dev and held-out and logs aggregate counts
// (names only on dev): files by profile (a set may hold files of another
// version, read generically), errors, Validate failures, panics, blocks,
// warnings, unmapped names.
func (c *transCase) corpus(t *testing.T) {
	for _, set := range []string{"dev", "heldout"} {
		dir := jatsCorpusDir(set, c.format)
		if dir == "" {
			t.Logf("%s: no %s corpus", set, c.format)
			continue
		}
		files := jatsCorpusFiles(dir)
		tally := newTally()
		profiles := map[string]int{}
		unmapped := map[string]map[string]int{WarnUnmappedElements: {}, WarnUnmappedAttributes: {}}
		total := map[string]int{}
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
			}
			if res.doc == nil {
				continue
			}
			p := res.doc.Metadata.Extra[ExtraProfile]
			profiles[p]++
			if p != c.profile {
				continue
			}
			for _, w := range res.doc.AllWarnings() {
				m, ok := unmapped[w.Code]
				if !ok {
					continue
				}
				for _, sm := range countRE.FindAllStringSubmatch(w.Message, -1) {
					n, _ := strconv.Atoi(sm[2])
					total[w.Code] += n
					if set == "dev" {
						m[strings.TrimSpace(sm[1])] += n
					}
				}
			}
		}
		t.Logf("%s: %d files in %v, by profile %s\n%s", set, len(files), time.Since(start).Round(time.Millisecond), sorted(profiles), tally)
		for _, code := range []string{WarnUnmappedElements, WarnUnmappedAttributes} {
			line := fmt.Sprintf("%s: %s (files read with %s): %d in all", set, code, c.profile, total[code])
			if set == "dev" {
				line += ": " + byCount(unmapped[code])
			}
			t.Log(line)
		}
	}
}

var equivRE = regexp.MustCompile(`<((?:[\w-]+:)?(?:x|bx|ex))\b[^>]*?\bequiv-text="([^"]*)"[^>]*/>`)

var xmlUnescape = strings.NewReplacer("&lt;", "<", "&gt;", ">", "&amp;", "&", "&quot;", `"`, "&apos;", "'")

// oracles runs both oracles over every dev file of the set.
func (c *transCase) oracles(t *testing.T) {
	dir := jatsCorpusDir("dev", c.format)
	if dir == "" {
		t.Skip("no dev corpus")
	}
	used := map[string]bool{}
	files, stopped, fails, reordered, wide := 0, 0, 0, 0, 0
	for _, path := range jatsCorpusFiles(dir) {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		doc, err := ExtractBytes(ctx, data, nil)
		cancel()
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		if doc.Metadata.Extra[ExtraProfile] != c.profile {
			continue
		}
		files++
		name := filepath.Base(path)
		src, serr := c.sourceText(data, used)
		got := transDocTextN(doc, c.attrCols)
		ok := got == src || jatsEntityRE.MatchString(src) && jatsInOrder(got, jatsEntityRE.Split(src, -1))
		if serr != nil {
			// encoding/xml stops (not well-formed, another encoding): the
			// text it read is the start of the document's.
			stopped++
			ok = strings.HasPrefix(got, src)
		}
		if !ok && c.keyed && sameChars(got, src) {
			// The tuv of a tu are in the columns of their languages, not
			// in file order: the same characters, each row in its span (b).
			ok = true
			reordered++
		}
		if !ok && fails < 10 {
			fails++
			i := 0
			for i < len(got) && i < len(src) && got[i] == src[i] {
				i++
			}
			t.Errorf("%s: (a) text differs at byte %d (encoding/xml: %v):\n doc:  %q\n file: %q", name, i, serr, around(got, i), around(src, i))
		}
		bad := ""
		if !utf8.Valid(data) {
			// UTF-16, windows-1252: offsets are bytes of the file, whose
			// text the oracle cannot compare as written.
			wide++
		} else if bad = jatsCiteCheck(data, doc); bad == "" {
			bad = rowCiteFrom(data, doc, c.ancestorCols)
		}
		if bad != "" && fails < 10 {
			fails++
			t.Errorf("%s: (b) %s", name, around(bad, 0))
		}
	}
	t.Logf("%d files read with %s: %d compared up to where encoding/xml stops, %d with the same characters in another order, %d not UTF-8 not cited", files, c.profile, stopped, reordered, wide)
	for k := range c.skip {
		if !used[k] {
			t.Errorf("exception %q is met nowhere in the dev set", k)
		}
	}
	for k := range c.codepoint {
		if !used["@"+k] {
			t.Errorf("code point attribute %q is met nowhere in the dev set", k)
		}
	}
	for k := range c.with {
		if !used["@"+k] {
			t.Errorf("attribute %q is met nowhere in the dev set", k)
		}
	}
}
