package gofastxml

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

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
	// with: elements whose rule writes an attribute after their text
	// ("with"), path to the attribute.
	with map[string]string
	name func(xml.Name) string
	// keyed: tables with columns by attribute value (TMX), whose cells
	// are in column order, not file order.
	keyed bool
}

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
			} else if k, ok := pathMatch(c.with, stack); ok {
				for _, at := range tk.Attr {
					if at.Name.Local == c.with[k] {
						a, used["@"+k] = noSpace(at.Value), true
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
func transDocText(doc *gofastcore.Document) string {
	var b strings.Builder
	for _, p := range doc.Parts {
		if p.Kind == gofastcore.PartStored {
			continue
		}
		for _, x := range p.Blocks {
			if x.Table != nil && x.Attrs[AttrAttribute] == "" {
				from := 0
				if x.Attrs[AttrHeaderSource] != "" {
					from = x.Table.HeaderRows
				}
				for _, r := range x.Table.Rows[from:] {
					for _, c := range r {
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
func rowCite(src []byte, doc *gofastcore.Document) string {
	for _, x := range blocks(doc) {
		if x.Table == nil || x.Attrs[AttrAttribute] != "" {
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
			span := noSpace(stripMarkup(raw))
			for _, cell := range r {
				for line := range strings.SplitSeq(cell, "\n") {
					if strings.Contains(span, noSpace(gofastcore.ExpandLigatures(line))) {
						continue
					}
					for _, w := range strings.Fields(line) {
						if !strings.Contains(span, noSpace(w)) && !strings.Contains(raw, w) && !strings.Contains(unescapeMarkup(raw), w) {
							return fmt.Sprintf("table at %d: row %d: %q not in its span %q", x.Loc.Offset, i, line, around(raw, 0))
						}
					}
				}
			}
		}
	}
	return ""
}

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
		got := transDocText(doc)
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
		if len(data) > 3 && bytes.IndexByte(data[:4], 0) >= 0 {
			// UTF-16: offsets are bytes of the file, whose text the
			// oracle cannot compare as written.
			wide++
		} else if bad = jatsCiteCheck(data, doc); bad == "" {
			bad = rowCite(data, doc)
		}
		if bad != "" && fails < 10 {
			fails++
			t.Errorf("%s: (b) %s", name, around(bad, 0))
		}
	}
	t.Logf("%d files read with %s: %d compared up to where encoding/xml stops, %d with the same characters in another order, %d UTF-16 not cited", files, c.profile, stopped, reordered, wide)
	for k := range c.skip {
		if !used[k] {
			t.Errorf("exception %q is met nowhere in the dev set", k)
		}
	}
	for k := range c.with {
		if !used["@"+k] {
			t.Errorf("attribute %q is met nowhere in the dev set", k)
		}
	}
}
