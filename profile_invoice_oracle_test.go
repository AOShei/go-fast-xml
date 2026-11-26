package gofastxml

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/AOShei/go-fast-core"
)

// invoiceMeta lists the elements whose character data goes to Metadata
// (Title, Created, Language, Extra inv.*), not to the body, by design,
// as paths of local names matched at the end of the element's path
// (pathMatch). The oracle checks that their text is a metadata value.
// Each must be met on dev.
var invoiceMeta = map[string]string{
	"ExchangedDocumentContext/GuidelineSpecifiedDocumentContextParameter/ID":       "",
	"ExchangedDocumentContext/BusinessProcessSpecifiedDocumentContextParameter/ID": "",
	"ExchangedDocument/ID":                                "",
	"ExchangedDocument/TypeCode":                          "",
	"ExchangedDocument/Name":                              "",
	"ExchangedDocument/IssueDateTime/DateTimeString":      "",
	"ApplicableHeaderTradeSettlement/InvoiceCurrencyCode": "",
	"ApplicableHeaderTradeSettlement/TaxCurrencyCode":     "",
	"ExchangedDocumentContext/TestIndicator/Indicator":    "",
	"/CreditNote/UBLVersionID":                            "", "/CreditNote/TaxCurrencyCode": "",
	"/Invoice/CustomizationID": "", "/CreditNote/CustomizationID": "",
	"/Invoice/ProfileID": "", "/CreditNote/ProfileID": "",
	"/Invoice/ID": "", "/CreditNote/ID": "",
	"/Invoice/IssueDate": "", "/CreditNote/IssueDate": "",
	"/Invoice/InvoiceTypeCode": "", "/CreditNote/CreditNoteTypeCode": "",
	"/Invoice/DocumentCurrencyCode": "", "/CreditNote/DocumentCurrencyCode": "",
	"/Invoice/TaxCurrencyCode": "",
}

// invoiceInAttrs lists the elements whose character data a rule reads
// into the Attrs of blocks (mark, attrs), not into their text. The
// oracle checks that their text is such a value. Each must be met on
// dev.
var invoiceInAttrs = map[string]string{
	"IncludedNote/SubjectCode": "", "IncludedNote/ContentCode": "",
}

// invoiceAttachments lists the elements whose content is an attachment
// (base64): it is handed to Options.ExtractChild, not written in the
// body. Each must be met on dev.
var invoiceAttachments = map[string]string{
	"AttachmentBinaryObject": "", "EmbeddedDocumentBinaryObject": "",
}

// invoiceHanded reports whether the attachment of run r is a child of
// doc, its element at the child's parent offset.
func invoiceHanded(doc *gofastcore.Document, r invoiceRun) bool {
	for _, c := range doc.Children {
		off, _ := strconv.ParseInt(c.Metadata.Extra[ExtraParentOffset], 10, 64)
		n, _ := strconv.ParseInt(c.Metadata.Extra[ExtraParentLength], 10, 64)
		if off <= r.off && r.off < off+n {
			return true
		}
	}
	return false
}

// invoiceRun is a run of character data of the file, white space
// removed, with the attribute values of its element, which a rule may
// write after it ("with").
type invoiceRun struct {
	text  string
	path  []string
	attrs []string
	off   int64 // where the run starts in the file
}

// invoiceRuns reads the file's character data runs in order.
func invoiceRuns(data []byte) ([]invoiceRun, error) {
	d := xml.NewDecoder(bytes.NewReader(data))
	d.Strict = false
	d.CharsetReader = func(label string, r io.Reader) (io.Reader, error) { return r, nil }
	var stack []string
	var attrs [][]string
	var runs []invoiceRun
	for {
		off := d.InputOffset()
		tok, err := d.Token()
		if err == io.EOF {
			return runs, nil
		}
		if err != nil {
			return runs, err
		}
		switch tk := tok.(type) {
		case xml.StartElement:
			stack = append(stack, tk.Name.Local)
			var as []string
			for _, a := range tk.Attr {
				if a.Name.Space != "xmlns" && a.Name.Local != "xmlns" && oneSpace(a.Value) != "" {
					as = append(as, oneSpace(a.Value))
				}
			}
			attrs = append(attrs, as)
		case xml.EndElement:
			stack, attrs = stack[:len(stack)-1], attrs[:len(attrs)-1]
		case xml.CharData:
			if t := oneSpace(string(tk)); t != "" && len(stack) > 0 {
				runs = append(runs, invoiceRun{t, append([]string(nil), stack...), attrs[len(attrs)-1], off})
			}
		}
	}
}

// oneSpace trims s and writes each run of white space in it as one
// space.
func oneSpace(s string) string { return strings.Join(strings.Fields(s), " ") }

// invoiceBody is the body's text in order, white space as one space,
// each piece (block text, Marker, table cell) followed by a NUL: not
// stored data and not attribute notes (checked by oracle (b)), without
// the profile's header rows. inTable tells, for each byte, whether it is
// a table's.
func invoiceBody(doc *gofastcore.Document) (text string, inTable []bool, rows []invoiceRow) {
	var b strings.Builder
	add := func(s string, tab bool) {
		s = oneSpace(s)
		if s == "" {
			return
		}
		b.WriteString(s + "\x00")
		for range len(s) + 1 {
			inTable = append(inTable, tab)
		}
	}
	for _, p := range doc.Parts {
		if p.Kind == gofastcore.PartStored {
			continue
		}
		for _, x := range p.Blocks {
			if x.Attrs[gofastcore.AttrStored] != "" || x.Attrs[AttrAttribute] != "" || x.Kind == gofastcore.KindImage {
				continue
			}
			add(x.Marker, false)
			if x.Table == nil {
				add(x.Text, false)
				continue
			}
			nameCol, idCol, pairs := pairCols(x)
			for r, row := range x.Table.Rows {
				if r == 0 && x.Attrs[AttrHeaderSource] == "profile" {
					continue
				}
				from := b.Len()
				for ci, c := range row {
					// A table of names and values: the names (and a
					// line's id) are not the file's character data
					// there, nor an attribute's value; invoiceNames
					// checks them.
					if pairs && (ci == nameCol || ci == idCol || ci == nameCol+1 && strings.Contains(row[nameCol], "/@")) {
						continue
					}
					// A cell's values on lines of their own are pieces.
					for _, l := range strings.Split(c, "\n") {
						add(l, true)
					}
				}
				if r < len(x.Table.RowLocs) {
					rows = append(rows, invoiceRow{x.Table.RowLocs[r], from, b.Len()})
				}
			}
		}
	}
	return b.String(), inTable, rows
}

// pairCols reports whether x is a table of names and values the profile
// makes ("pairs", "rest_table": a profile header row ending in name,
// value), with the columns of its names and of its ids (-1: none).
func pairCols(x gofastcore.Block) (name, id int, ok bool) {
	if x.Table == nil || x.Attrs[AttrHeaderSource] != "profile" || len(x.Table.Rows) == 0 {
		return -1, -1, false
	}
	h := x.Table.Rows[0]
	switch n := len(h); {
	case n == 2 && h[0] == "name" && h[1] == "value":
		return 0, -1, true
	case n == 3 && h[1] == "name" && h[2] == "value":
		return 1, 0, true
	}
	return -1, -1, false
}

// invoiceNames checks the exception of oracle (a): the names a table of
// names and values writes. Each is the name of the element its row cites
// (the end of that element's path of local names), and a name ending in
// /@attribute names an attribute of that element whose value is the
// row's value. A line's id is the text of a cell of the row (of another
// table) whose element holds the row's element. It returns the number of
// names checked.
func invoiceNames(data []byte, doc *gofastcore.Document) (string, int) {
	paths := map[int64][]string{}
	attrs := map[int64]map[string]string{}
	d := xml.NewDecoder(bytes.NewReader(data))
	d.Strict = false
	d.CharsetReader = func(label string, r io.Reader) (io.Reader, error) { return r, nil }
	var stack []string
	for {
		off := d.InputOffset()
		tok, err := d.Token()
		if err != nil {
			break
		}
		switch tk := tok.(type) {
		case xml.StartElement:
			stack = append(stack, tk.Name.Local)
			paths[off] = append([]string(nil), stack...)
			m := map[string]string{}
			for _, a := range tk.Attr {
				m[a.Name.Local] = oneSpace(a.Value)
			}
			attrs[off] = m
		case xml.EndElement:
			stack = stack[:len(stack)-1]
		}
	}
	type lineRow struct {
		loc   gofastcore.Location
		cells []string
	}
	var lines []lineRow
	for _, p := range doc.Parts {
		for _, x := range p.Blocks {
			if _, _, ok := pairCols(x); ok || x.Table == nil {
				continue
			}
			for r, l := range x.Table.RowLocs {
				lines = append(lines, lineRow{l, x.Table.Rows[r]})
			}
		}
	}
	n := 0
	for _, p := range doc.Parts {
		for bi, x := range p.Blocks {
			nameCol, idCol, ok := pairCols(x)
			if !ok {
				continue
			}
			for r, row := range x.Table.Rows[1:] {
				l := x.Table.RowLocs[r+1]
				name, attr, _ := strings.Cut(row[nameCol], "/@")
				path, ok := paths[l.Offset]
				steps := strings.Split(name, "/")
				if !ok || len(steps) > len(path) || strings.Join(path[len(path)-len(steps):], "/") != name {
					return fmt.Sprintf("block %d row %d: %q is not the name of the element there (%v)", bi, r+1, row[nameCol], path), n
				}
				if attr != "" && attrs[l.Offset][attr] != oneSpace(row[nameCol+1]) {
					return fmt.Sprintf("block %d row %d: %q is not an attribute of the element there with value %q", bi, r+1, row[nameCol], row[nameCol+1]), n
				}
				n++
				if idCol < 0 || row[idCol] == "" {
					continue
				}
				found := false
				for _, lr := range lines {
					if lr.loc.Offset <= l.Offset && l.Offset+l.Length <= lr.loc.Offset+lr.loc.Length {
						found = found || slices.Contains(lr.cells, row[idCol])
					}
				}
				if !found {
					return fmt.Sprintf("block %d row %d: id %q is no cell of a row holding its element", bi, r+1, row[idCol]), n
				}
			}
		}
	}
	return "", n
}

// invoiceRow is a row's location in the file and its text's in the body.
type invoiceRow struct {
	loc      gofastcore.Location
	from, to int
}

// rowOf is the body range of the innermost row whose location holds the
// file offset off; ok is false when no row does.
func rowOf(rows []invoiceRow, off int64) (from, to int, ok bool) {
	best := int64(-1)
	for _, r := range rows {
		if r.loc.Length > 0 && off >= r.loc.Offset && off < r.loc.Offset+r.loc.Length && (best < 0 || r.loc.Length < best) {
			best, from, to, ok = r.loc.Length, r.from, r.to, true
		}
	}
	return
}

// invoiceMetaValues are the document's metadata values, white space
// removed, the dates also as YYYYMMDD and YYYY-MM-DD.
func invoiceMetaValues(doc *gofastcore.Document) map[string]bool {
	m := map[string]bool{oneSpace(doc.Metadata.Title): true, oneSpace(doc.Metadata.Language): true}
	for _, v := range doc.Metadata.Extra {
		m[oneSpace(v)] = true
	}
	if !doc.Metadata.Created.IsZero() {
		m[doc.Metadata.Created.Format("20060102")] = true
		m[doc.Metadata.Created.Format("2006-01-02")] = true
	}
	return m
}

// invoiceOracleA is oracle (a) for the invoice profiles. Every character
// data run of the file is found exactly once: in the body, or (the
// elements of invoiceMeta) among the metadata values. Each run is looked
// for at the first place not yet used at or after the end of the last
// run found outside a table, else at any place not yet used; the
// attribute values of its element may follow it ("with"). Runs found
// outside tables must be in file order; table cells may be anywhere (a
// line's values are placed by "column", and CII's lines table is made
// when the transaction ends). At the end every byte of the body must be
// used: the body holds nothing that is not the file's, apart from the
// profile's header rows, which invoiceBody leaves out.
func invoiceOracleA(data []byte, doc *gofastcore.Document, used map[string]bool) string {
	runs, err := invoiceRuns(data)
	if err != nil {
		return "encoding/xml: " + err.Error()
	}
	body, inTable, rows := invoiceBody(doc)
	meta := invoiceMetaValues(doc)
	taken := make([]bool, len(body))
	free := func(i, n int) bool {
		for k := i; k < i+n; k++ {
			if taken[k] {
				return false
			}
		}
		return true
	}
	edge := func(i int) bool { return i <= 0 || i >= len(body) || body[i] == ' ' || body[i] == 0 }
	// find looks for s in body[from:to], not taken, as whole words when
	// words is set, in a table's text only when tables is set.
	piece := func(i int) bool { return i < 0 || i >= len(body) || body[i] == 0 }
	findPiece := func(s string, from, to int) int {
		for i := from; i <= to-len(s); {
			j := strings.Index(body[i:to], s)
			if j < 0 {
				return -1
			}
			if at := i + j; free(at, len(s)) && piece(at-1) && piece(at+len(s)) {
				return at
			}
			i += j + 1
		}
		return -1
	}
	find := func(s string, from, to int, words, tables bool) int {
		for i := from; i <= to-len(s); {
			j := strings.Index(body[i:to], s)
			if j < 0 {
				return -1
			}
			at := i + j
			if free(at, len(s)) && (!words || edge(at-1) && edge(at+len(s))) && (!tables || inTable[at]) {
				return at
			}
			i = at + 1
		}
		return -1
	}
	// next is the end of the first piece outside tables, at or after the
	// cursor, with text not yet taken: the place the next run outside a
	// table must be in.
	next := func(cursor int) int {
		for i := cursor; i < len(body); i++ {
			if !taken[i] && !inTable[i] && body[i] != ' ' && body[i] != 0 {
				if j := strings.IndexByte(body[i:], 0); j >= 0 {
					return i + j
				}
				return len(body)
			}
		}
		return len(body)
	}
	attrVals := map[string]bool{}
	for _, p := range doc.Parts {
		for _, b := range p.Blocks {
			for _, v := range b.Attrs {
				attrVals[oneSpace(v)] = true
			}
		}
	}
	cursor := 0
	for _, r := range runs {
		if k, ok := pathMatch(invoiceMeta, r.path); ok {
			used[k] = true
			if !meta[r.text] {
				return fmt.Sprintf("%s: %q is not a metadata value", strings.Join(r.path, "/"), r.text)
			}
			continue
		}
		if k, ok := pathMatch(invoiceAttachments, r.path); ok {
			used[k] = true
			if !invoiceHanded(doc, r) {
				return fmt.Sprintf("%s: an attachment is not handed to a child", strings.Join(r.path, "/"))
			}
			continue
		}
		if k, ok := pathMatch(invoiceInAttrs, r.path); ok {
			used[k] = true
			if !attrVals[r.text] {
				return fmt.Sprintf("%s: %q is not in a block's Attrs", strings.Join(r.path, "/"), r.text)
			}
			continue
		}
		// A run inside a row's element is looked for in that row's
		// cells only: whole words followed by an attribute value of its
		// element, then whole words, then a part of a word (character
		// data split by an inline element). Any other run outside
		// tables: in the next piece, then after the cursor, then
		// anywhere.
		i := -1
		var tries []func() int
		if from, to, ok := rowOf(rows, r.off); ok {
			for _, a := range r.attrs {
				tries = append(tries, func() int { return findPiece(r.text+" "+a, from, to) })
			}
			tries = append(tries, func() int { return findPiece(r.text, from, to) })
			for _, a := range r.attrs {
				tries = append(tries, func() int { return find(r.text+" "+a, from, to, true, true) })
			}
			tries = append(tries,
				func() int { return find(r.text, from, to, true, true) },
				func() int { return find(r.text, from, to, false, true) })
		} else {
			n := next(cursor)
			tries = append(tries,
				func() int { return find(r.text, cursor, n, true, false) },
				func() int { return find(r.text, cursor, len(body), true, false) },
				func() int { return find(r.text, 0, len(body), true, false) },
				func() int { return find(r.text, cursor, len(body), false, false) },
				func() int { return find(r.text, 0, len(body), false, false) })
		}
		for _, try := range tries {
			if i = try(); i >= 0 {
				break
			}
		}
		if i < 0 {
			return fmt.Sprintf("%s: %q is not in the body (or found twice)", strings.Join(r.path, "/"), around(r.text, 0))
		}
		end := i + len(r.text)
		if _, _, inRow := rowOf(rows, r.off); !inRow && inTable[i] {
			return fmt.Sprintf("%s: %q is in a table's cell, not in a row of its own", strings.Join(r.path, "/"), around(r.text, 0))
		}
		if !inTable[i] {
			if i < cursor {
				return fmt.Sprintf("%s: %q is out of file order outside a table", strings.Join(r.path, "/"), around(r.text, 0))
			}
			cursor = end
		}
		for k := i; k < end; k++ {
			taken[k] = true
		}
		for _, a := range r.attrs {
			if strings.HasPrefix(body[end:], " "+a) && free(end+1, len(a)) && edge(end+1+len(a)) {
				for k := end + 1; k < end+1+len(a); k++ {
					taken[k] = true
				}
				end += 1 + len(a)
				if !inTable[i] {
					cursor = end
				}
			}
		}
	}
	for i := range taken {
		if !taken[i] && body[i] != ' ' && body[i] != 0 {
			j := i
			for j < len(taken) && !taken[j] && body[j] != 0 {
				j++
			}
			return fmt.Sprintf("the body holds %q, which is no character data of the file", around(body[i:j], 0))
		}
	}
	return ""
}

// invoiceRowText is oracle (b) for rows: the source at each row's
// RowLocs holds every word of each of its cells, as character data or
// as an attribute value.
func invoiceRowText(data []byte, doc *gofastcore.Document) string {
	for _, p := range doc.Parts {
		for bi, b := range p.Blocks {
			if b.Table == nil {
				continue
			}
			for r, l := range b.Table.RowLocs {
				if l == (gofastcore.Location{}) {
					continue
				}
				if l.Offset < 0 || l.Offset+l.Length > int64(len(data)) {
					return fmt.Sprintf("block %d row %d: %+v outside the file", bi, r, l)
				}
				span := data[l.Offset : l.Offset+l.Length]
				hay := noSpace(charData(span))
				runs, _ := invoiceRuns(span)
				for _, x := range runs {
					hay += " " + strings.Join(x.attrs, " ")
				}
				hay += " " + attrValues(span)
				nameCol, idCol, pairs := pairCols(b)
				for ci, c := range b.Table.Rows[r] {
					if pairs && (ci == nameCol || ci == idCol) {
						continue // checked by invoiceNames
					}
					for _, w := range strings.Fields(c) {
						if !strings.Contains(hay, w) {
							return fmt.Sprintf("block %d row %d: %q is not in the row's source", bi, r, w)
						}
					}
				}
			}
		}
	}
	return ""
}

// attrValues is the attribute values of every start tag of a piece of
// markup, as written.
func attrValues(span []byte) string {
	d := xml.NewDecoder(bytes.NewReader(span))
	d.Strict = false
	var b strings.Builder
	for {
		tok, err := d.Token()
		if err != nil {
			return b.String()
		}
		if s, ok := tok.(xml.StartElement); ok {
			for _, a := range s.Attr {
				b.WriteString(" " + a.Value)
			}
		}
	}
}

// TestInvoiceAttachments: a CII AttachmentBinaryObject is decoded and
// handed to the child extractor under its filename with its mimeCode
// (application/octet-stream when none); its base64 is not in the body,
// whether the child was extracted or not; text that is not base64 stays
// in the body with media_type and input.stream_damaged; past MaxChildren
// an attachment is limit.exceeded and not in the body.
func TestInvoiceAttachments(t *testing.T) {
	var got []handed
	_, doc := invoiceFixture(t, "cii/attachment.xml", "cii", &gofastcore.Options{Child: fakeChild(&got, nil)})
	want := []handed{{"note.txt", "text/plain", "hello world"}, {"ram:AdditionalReferencedDocument/ram:AttachmentBinaryObject", "application/octet-stream", "%PDF"}}
	if len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("handed %q, want %q", got, want)
	}
	if len(doc.Children) != 2 || doc.Children[0].Metadata.Extra[ExtraParentElement] == "" {
		t.Fatalf("children %d", len(doc.Children))
	}
	var texts []string
	links := 0
	for _, b := range doc.Parts[0].Blocks {
		texts = append(texts, b.Text)
		if b.Attrs[AttrChildren] != "" {
			links++
		}
		if b.Text == "not*base64!" && b.Attrs[AttrMediaType] != "application/pdf" {
			t.Errorf("bad base64 block attrs %v", b.Attrs)
		}
	}
	body := strings.Join(texts, "|")
	if strings.Contains(body, "aGVsbG8") || strings.Contains(body, "JVBERg") || !strings.Contains(body, "not*base64!") || links != 2 {
		t.Errorf("body %q, %d blocks link a child", body, links)
	}
	if !docHas(&gofastcore.Document{Warnings: doc.Parts[0].Warnings}, gofastcore.WarnInputStreamDamaged) {
		t.Errorf("no %s: %v", gofastcore.WarnInputStreamDamaged, doc.AllWarnings())
	}
	// No child extractor: both listed, not extracted, base64 not in the body.
	_, doc = invoiceFixture(t, "cii/attachment.xml", "cii", nil)
	if len(doc.Children) != 2 || !docHas(doc.Children[0], gofastcore.WarnChildNotExtracted) || doc.Children[0].Source.Name != "note.txt" {
		t.Errorf("children %d", len(doc.Children))
	}
	for _, b := range doc.Parts[0].Blocks {
		if strings.Contains(b.Text, "aGVsbG8") || strings.Contains(b.Text, "JVBERg") {
			t.Errorf("base64 in the body: %q", b.Text)
		}
	}
	// Past MaxChildren.
	defer func(n int) { maxChildren = n }(maxChildren)
	maxChildren = 1
	got = nil
	_, doc = invoiceFixture(t, "cii/attachment.xml", "cii", &gofastcore.Options{Child: fakeChild(&got, nil)})
	cut := false
	for _, w := range doc.Parts[0].Warnings {
		cut = cut || w.Code == gofastcore.WarnLimitExceeded && w.Severity == gofastcore.SeverityError && strings.Contains(w.Message, "1 attachment ")
	}
	if len(got) != 1 || len(doc.Children) != 1 || !cut {
		t.Errorf("past MaxChildren: handed %d, children %d, warnings %v", len(got), len(doc.Children), doc.Parts[0].Warnings)
	}
	for _, b := range doc.Parts[0].Blocks {
		if strings.Contains(b.Text, "JVBERg") {
			t.Errorf("base64 in the body: %q", b.Text)
		}
	}
}

// TestInvoiceOracles runs the two oracles over every dev file read with
// cii or ubl.
func TestInvoiceOracles(t *testing.T) {
	dir := patentCorpusDir("dev", "xml-invoice")
	if dir == "" {
		t.Skip("no dev corpus")
	}
	used := map[string]bool{}
	files := 0
	depths := map[int]int{}
	kids, names := 0, 0
	for _, path := range patentCorpusFiles(dir) {
		for _, child := range []bool{false, true} {
			doc, data, err := feedRead(path, child)
			if err != nil {
				break // the one file with an unknown encoding label
			}
			if p := doc.Metadata.Extra[ExtraProfile]; p != "cii" && p != "ubl" {
				break
			}
			name := filepath.Base(path)
			if !child {
				files++
				runs, _ := invoiceRuns(data)
				for _, r := range runs {
					for i, s := range r.path {
						if s == "IncludedSupplyChainTradeLineItem" || s == "InvoiceLine" || s == "CreditNoteLine" {
							depths[len(r.path)-1-i]++
							break
						}
					}
				}
				for _, w := range doc.AllWarnings() {
					if w.Code == WarnUnmappedElements || w.Code == WarnUnmappedAttributes {
						t.Logf("%s: %s", name, w.Message)
					}
				}
				kids += len(doc.Children)
			}
			if bad := invoiceOracleA(data, doc, used); bad != "" {
				t.Errorf("%s (child %v): (a) %s", name, child, bad)
			}
			if bad := safeCite(data, doc); bad != "" {
				t.Errorf("%s (child %v): (b) %s", name, child, around(bad, 0))
			}
			if bad, _ := rowLocCheck(data, doc); bad != "" {
				t.Errorf("%s (child %v): (b) RowLocs: %s", name, child, bad)
			}
			if bad := invoiceRowText(data, doc); bad != "" {
				t.Errorf("%s (child %v): (b) RowLocs: %s", name, child, bad)
			}
			bad, n := invoiceNames(data, doc)
			if bad != "" {
				t.Errorf("%s (child %v): (a) names: %s", name, child, bad)
			}
			if !child {
				names += n
			}
			for i, k := range doc.Children {
				off, _ := strconv.ParseInt(k.Metadata.Extra[ExtraParentOffset], 10, 64)
				n, _ := strconv.ParseInt(k.Metadata.Extra[ExtraParentLength], 10, 64)
				el := k.Metadata.Extra[ExtraParentElement]
				local := el[strings.LastIndexByte(el, '/')+1:]
				if off < 0 || n <= 0 || off+n > int64(len(data)) || !bytes.HasPrefix(data[off:], []byte("<"+local)) || data[off+n-1] != '>' {
					t.Errorf("%s: child %d: its element %s is not at its parent offsets", name, i, el)
				}
			}
		}
	}
	t.Logf("%d files read with cii or ubl, %d attachments, %d names of elements and attributes checked; character data below a line, by depth: %v", files, kids, names, depths)
	if names == 0 {
		t.Error("no name of a table of names and values met on dev")
	}
	for _, m := range []map[string]string{invoiceMeta, invoiceInAttrs, invoiceAttachments} {
		for k := range m {
			if !used[k] {
				t.Errorf("exception %q is met nowhere in the dev set", k)
			}
		}
	}
}
