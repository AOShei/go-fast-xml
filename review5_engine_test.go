package gofastxml

// Phase 5 review: one test per finding. Each fails today and passes when
// the defect it names is fixed.

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/AOShei/go-fast-core"
)

// r5Extract reads src with the profile prof ("" for generic reading).
func r5Extract(t *testing.T, prof, src string, opts *gofastcore.Options) *gofastcore.Document {
	t.Helper()
	ex := &Extractor{}
	if prof != "" {
		p, err := LoadProfile(strings.NewReader(prof))
		if err != nil {
			t.Fatal(err)
		}
		ex.Profiles = []*Profile{p}
	}
	doc, err := ex.Extract(context.Background(), strings.NewReader(src), int64(len(src)), opts)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if err := doc.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	return doc
}

// r5Alloc returns the bytes allocated while reading src with prof.
func r5Alloc(t *testing.T, prof, src string, opts *gofastcore.Options) uint64 {
	t.Helper()
	var a, b runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&a)
	r5Extract(t, prof, src, opts)
	runtime.ReadMemStats(&b)
	return b.TotalAlloc - a.TotalAlloc
}

// r5Time returns the fastest of three readings of src with prof.
func r5Time(t *testing.T, prof, src string) time.Duration {
	t.Helper()
	best := time.Duration(1 << 62)
	for range 3 {
		start := time.Now()
		r5Extract(t, prof, src, nil)
		best = min(best, time.Since(start))
	}
	return best
}

const r5Prof = `{"profile":"review5","format":1,"match":[{"root":"r"}],"rules":[
 {"match":"r","as":"transparent"},
 {"match":"p","as":"block","kind":"paragraph"},
 {"match":"ul","as":"list"},
 {"match":"ol","as":"list","ordered":true},
 {"match":"li","as":"item"},
 {"match":"num","as":"label"},
 {"match":"b","as":"inline","style":"bold"},
 {"match":"k","as":"metadata","field":"extra","key":"k"},
 {"match":"t","as":"table","model":"cals"},
 {"match":"rt","as":"table"},
 {"match":"tr","as":"row"},
 {"match":"td","as":"cell","colspan":"cs","rowspan":"rs"}
]}`

// Finding: a list (or any block) read as lines of the paragraph it stands
// in is followed by the paragraph's tail text with no separator, so the
// last item's last word and the tail's first word become one word
// (JATS <p>...<list>...</list>tail</p>, DocBook para/itemizedlist).
func TestReview5NestedBlockTailFused(t *testing.T) {
	doc := r5Extract(t, r5Prof, `<r><p>lead<ul><li>one</li><li>two</li></ul>tail</p></r>`, nil)
	b := doc.Parts[0].Blocks
	if len(b) != 1 {
		t.Fatalf("%d blocks", len(b))
	}
	if strings.Contains(b[0].Text, "twotail") {
		t.Errorf("text %q: the item \"two\" and the tail \"tail\" are fused into one word; want a line break between them", b[0].Text)
	}
	doc = r5Extract(t, r5Prof, `<r><p>a<p>inner</p>b</p></r>`, nil)
	if got := doc.Parts[0].Blocks[0].Text; got != "a\ninner\nb" {
		t.Errorf("nested paragraph: text %q, want %q", got, "a\ninner\nb")
	}
}

// Finding: PROFILE_FORMAT.md says a label as the first thing in a block is
// its marker and that "an ordered list's number gives way to the file's
// label"; in an ordered list the item keeps its counted number and the
// label is pushed into the text.
func TestReview5OrderedItemLabelIsMarker(t *testing.T) {
	doc := r5Extract(t, r5Prof, `<r><ol><li><num>(b)</num>bee</li></ol></r>`, nil)
	b := doc.Parts[0].Blocks[0]
	if b.Marker != "(b)" || b.Text != "bee" {
		t.Errorf("marker %q text %q, want marker \"(b)\" text \"bee\"", b.Marker, b.Text)
	}
}

// Finding (known open item a): an unclosed processing instruction or
// comment inside the internal subset of a DOCTYPE swallows the rest of the
// file; the whole document is ErrMalformed ("no XML element").
func TestReview5DoctypeUnclosedPI(t *testing.T) {
	for _, src := range []string{
		`<!DOCTYPE r [ <?pi ]><r><p>Hello</p></r>`,
		`<!DOCTYPE r [ <!-- ]><r><p>Hello</p></r>`,
	} {
		doc, err := Extract(context.Background(), strings.NewReader(src), int64(len(src)), nil)
		if err != nil {
			t.Errorf("%s: %v; want the element r/p read as a paragraph", src, err)
			continue
		}
		found := false
		for _, b := range doc.Parts[0].Blocks {
			found = found || b.Text == "Hello" && b.Attrs[AttrElement] == "r/p"
		}
		if !found {
			t.Errorf("%s: no paragraph \"Hello\" of element r/p", src)
		}
	}
}

// Finding (known open item b): a paragraph over MaxBlockBytes is cut where
// a text chunk would pass the bound, which may be at an inline tag's edge
// in the middle of a word ("abc<b>def</b>" gives pieces "...abc" and
// "def ..."), although white space is available just before.
func TestReview5LongParagraphCutMidWord(t *testing.T) {
	for _, prof := range []string{"", r5Prof} {
		for delta := 0; delta < 6; delta++ {
			pre := strings.Repeat("wxyz ", (MaxBlockBytes-6)/5) + strings.Repeat("q", delta) + " "
			src := "<r><p>" + pre + "abc<b>def</b> tail</p></r>"
			doc := r5Extract(t, prof, src, nil)
			whole := false
			for _, b := range doc.Parts[0].Blocks {
				whole = whole || strings.Contains(b.Text, "abcdef")
			}
			if !whole {
				t.Errorf("profile %v, delta %d: the word \"abcdef\" is cut between two Continued pieces", prof != "", delta)
			}
		}
	}
}

// Finding: repeated values of one Metadata.Extra key are joined by
// concatenating the whole value so far each time (engine.setExtra), so N
// short values cost O(N^2) bytes up to MaxValueBytes: 500,000 two-byte
// values (4 MB of input) copy about 250 GB.
func TestReview5ExtraJoinQuadratic(t *testing.T) {
	doc := func(n int) string { return "<r>" + strings.Repeat("<k>a</k>", n) + "</r>" }
	small := r5Alloc(t, r5Prof, doc(4000), nil)
	large := r5Alloc(t, r5Prof, doc(16000), nil)
	if ratio := float64(large) / float64(small); ratio > 8 {
		t.Errorf("4x the input allocates %.1fx the bytes (%d vs %d); want about 4x", ratio, large, small)
	}
}

// Finding: a table's rows are padded to its widest row when its block is
// made (table.block), before the text budget sees them, and empty rows cost
// nothing while the table is read: "<tr/>" (5 bytes) becomes 1,024 cells
// (16 KiB) once one row spans 1,024 columns. With MaxTextBytes at 1 MiB,
// 20,000 empty rows (100 KB of input) allocate 1.2 GB before the table is
// refused; this test uses 2,000.
func TestReview5TablePaddingBeforeBudget(t *testing.T) {
	const n = 2000
	wide := `<r><rt><tr><td cs="1024">x</td></tr>` + strings.Repeat("<tr/>", n) + `</rt></r>`
	narrow := `<r><rt><tr><td cs="1">x</td></tr>` + strings.Repeat("<tr/>", n) + `</rt></r>`
	opts := &gofastcore.Options{Limits: gofastcore.Limits{MaxTextBytes: 1 << 20}}
	w := r5Alloc(t, r5Prof, wide, opts)
	nw := r5Alloc(t, r5Prof, narrow, opts)
	if w > 16*nw {
		t.Errorf("a 1,024-column table of %d empty rows allocates %d bytes, %.0fx the 1-column one (%d), under a 1 MiB text budget", n, w, float64(w)/float64(nw), nw)
	}
}

// Finding: the rule cache key (engine.lookup) copies the value of every
// tested attribute that ancestors keep, for every element; a value over
// 128 bytes also disables the cache. A profile with an ancestor attribute
// test ("d[@c]/p") makes N children of one element with a long @c cost
// N x len(@c): quadratic in the input.
func TestReview5LookupKeyQuadratic(t *testing.T) {
	prof := `{"profile":"review5k","format":1,"match":[{"root":"r"}],"rules":[
 {"match":"r","as":"transparent"},{"match":"d","as":"transparent"},
 {"match":"d[@c]/p","as":"block","kind":"paragraph"}]}`
	doc := func(n int) string {
		return `<r><d c="` + strings.Repeat("v", 32*n) + `">` + strings.Repeat("<p/>", n) + `</d></r>`
	}
	small := r5Time(t, prof, doc(2000))
	large := r5Time(t, prof, doc(8000))
	if ratio := float64(large) / float64(small); ratio > 8 {
		t.Errorf("4x the input takes %.1fx the time (%v vs %v); want about 4x", ratio, large, small)
	}
}

// Finding: CALS spans that overlap a cell spanning rows from above
// (namest/nameend over a column a morerows entry occupies) give Merges
// that cover the same cell twice; the same holds in the rows model with
// colspan over a rowspan. Validate accepts it.
func TestReview5OverlappingMerges(t *testing.T) {
	for _, src := range []string{
		`<r><t><tgroup cols="2"><colspec colname="c1"/><colspec colname="c2"/><tbody>` +
			`<row><entry colname="c2" morerows="1">A</entry></row>` +
			`<row><entry namest="c1" nameend="c2">B</entry></row></tbody></tgroup></t></r>`,
		`<r><rt><tr><td>1</td><td rs="2">2</td></tr><tr><td cs="2">3</td></tr></rt></r>`,
	} {
		doc := r5Extract(t, r5Prof, src, nil)
		tb := doc.Parts[0].Blocks[0].Table
		seen := map[[2]int]bool{}
		for _, m := range tb.Merges {
			for r := m.Row; r < m.Row+m.RowSpan; r++ {
				for c := m.Col; c < m.Col+m.ColSpan; c++ {
					if seen[[2]int{r, c}] {
						t.Errorf("%s: cell (%d,%d) is in two merges: %+v", src, r, c, tb.Merges)
					}
					seen[[2]int{r, c}] = true
				}
			}
		}
	}
}

// Finding: an xs:include naming a FIFO (a named pipe) blocks DraftProfile
// for ever in os.Open, before the regular-file check, and the context is
// not consulted: a hostile schema hangs -draft-profile.
func TestReview5DraftIncludeFIFOHangs(t *testing.T) {
	dir := t.TempDir()
	if err := syscall.Mkfifo(filepath.Join(dir, "pipe.xsd"), 0o600); err != nil {
		t.Skip("no FIFO:", err)
	}
	schema := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:include schemaLocation="pipe.xsd"/><xs:element name="a" type="xs:string"/></xs:schema>`
	main := filepath.Join(dir, "main.xsd")
	if err := os.WriteFile(main, []byte(schema), 0o600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	done := make(chan struct{})
	go func() {
		DraftProfile(ctx, main)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		// Unblock the reader so the test process can end.
		if f, err := os.OpenFile(filepath.Join(dir, "pipe.xsd"), os.O_WRONLY, 0); err == nil {
			f.Close()
		}
		t.Error("DraftProfile still blocked 2s after a 500ms context: an include naming a FIFO is opened and waited on")
	}
}
