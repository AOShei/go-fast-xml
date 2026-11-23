package gofastxml

// Phase 5 review: a second case of our own for each finding whose review
// test is narrow.

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/AOShei/go-fast-core"
	"github.com/AOShei/go-fast-xml/internal/generic"
)

func r5LimitHit(doc *gofastcore.Document) bool {
	for _, w := range doc.AllWarnings() {
		if w.Code == gofastcore.WarnLimitExceeded && w.Severity == gofastcore.SeverityError {
			return true
		}
	}
	return false
}

// The CALS model, with the width growing after the empty rows were read:
// the rows already held are charged their padding when the table widens,
// and the table stops with limit.exceeded before any grid is padded.
func TestReview5TableWidthGrowsLate(t *testing.T) {
	const n = 2000
	src := `<r><t><tgroup cols="1"><tbody>` + strings.Repeat("<row/>", n) +
		`<row><entry namest="a" nameend="b">x</entry>` + strings.Repeat("<entry>y</entry>", 1023) + `</row></tbody></tgroup></t><p>after</p></r>`
	opts := &gofastcore.Options{Limits: gofastcore.Limits{MaxTextBytes: 1 << 20}}
	r5Extract(t, r5Prof, src, opts) // warm up
	if a := r5Alloc(t, r5Prof, src, opts); a > 8<<20 {
		t.Errorf("allocates %d bytes under a 1 MiB text budget", a)
	}
	doc := r5Extract(t, r5Prof, src, opts)
	if !r5LimitHit(doc) {
		t.Errorf("no limit.exceeded: %+v", doc.Parts[0].Warnings)
	}
	for _, b := range doc.Parts[0].Blocks {
		if b.Kind == gofastcore.KindTable {
			t.Errorf("a table of %d rows by 1024 columns is kept under a 1 MiB budget", len(b.Table.Rows))
		}
	}
	// A table the budget can take is kept whole.
	doc = r5Extract(t, r5Prof, `<r><rt><tr><td cs="3">x</td></tr><tr/><tr><td>y</td></tr></rt></r>`, nil)
	tb := doc.Parts[0].Blocks[0].Table
	if len(tb.Rows) != 3 || len(tb.Rows[1]) != 3 {
		t.Errorf("rows %q", tb.Rows)
	}
}

// Text after a nested block starts a line, an inline element's included;
// a nested block at the end of its paragraph leaves no break behind, and
// white space between two nested lists gives one break.
func TestReview5NestedBlockTailMore(t *testing.T) {
	for src, want := range map[string]string{
		`<r><p>lead<ul><li>one</li></ul><b>bold</b> tail</p></r>`:            "lead\none\nbold tail",
		`<r><p>lead<ul><li>one</li></ul> </p></r>`:                           "lead\none",
		`<r><p>a<ul><li>x</li></ul> <ul><li>y</li></ul> b</p></r>`:           "a\nx\ny\nb",
		`<r><ul><li><p>inner</p>after</li></ul></r>`:                         "inner\nafter",
		`<r><rt><tr><td>c<p>para</p>tail</td></tr></rt></r>`:                 "c\npara\ntail",
		`<r><p>lead<ol><li>one</li><li>two</li></ol>tail</p><p>next</p></r>`: "lead\n1. one\n2. two\ntail",
	} {
		doc := r5Extract(t, r5Prof, src, nil)
		b := doc.Parts[0].Blocks[0]
		got := b.Text
		if b.Table != nil {
			got = b.Table.Rows[0][0]
		}
		if got != want {
			t.Errorf("%s: text %q, want %q", src, got, want)
		}
	}
}

// An unclosed comment in the internal subset ends at the subset's "]>":
// input.repaired says so, and the document after it is read.
func TestReview5DoctypeUnclosedRepaired(t *testing.T) {
	src := `<!DOCTYPE r [ <!ENTITY e "Ee"> <!-- open ]><r><p>Hello &e;</p></r>`
	doc := r5Extract(t, "", src, nil)
	repaired := false
	for _, w := range doc.AllWarnings() {
		repaired = repaired || w.Code == gofastcore.WarnInputRepaired
	}
	if !repaired {
		t.Errorf("no input.repaired: %+v", doc.AllWarnings())
	}
	text := ""
	for _, b := range doc.Parts[0].Blocks {
		text += b.Text
	}
	if !strings.Contains(text, "Hello") {
		t.Errorf("text %q", text)
	}
}

// A paragraph cut while reading: each piece's text is in its own source
// span (the word kept whole moves the next piece's start back), and a run
// with no white space at all is cut where it stands, nothing lost.
func TestReview5LongParagraphPieces(t *testing.T) {
	for _, prof := range []string{"", r5Prof} {
		pre := strings.Repeat("wxyz ", (MaxBlockBytes-6)/5) + "q "
		src := "<r><p>" + pre + "abc<b>def</b> tail</p></r>"
		doc := r5Extract(t, prof, src, nil)
		for i, b := range doc.Parts[0].Blocks {
			span := noSpace(stripMarkup(src[b.Loc.Offset : b.Loc.Offset+b.Loc.Length]))
			if !strings.Contains(span, noSpace(b.Text)) {
				t.Errorf("profile %v: piece %d (%d bytes) is not in its span", prof != "", i, len(b.Text))
			}
		}
		run := strings.Repeat("a", MaxBlockBytes-2)
		doc = r5Extract(t, prof, "<r><p>"+run+"<b>bbbb</b>cc</p></r>", nil)
		all := ""
		for _, b := range doc.Parts[0].Blocks {
			all += b.Text
		}
		if all != run+"bbbbcc" || len(doc.Parts[0].Blocks) != 2 {
			t.Errorf("profile %v: %d pieces, %d bytes, want 2 pieces, %d bytes", prof != "", len(doc.Parts[0].Blocks), len(all), len(run)+6)
		}
	}
}

// Rules are still chosen right under ancestors whose tested values are
// long (looked up afresh) and short (cached), in either order.
func TestReview5LookupLongValueRight(t *testing.T) {
	prof := `{"profile":"review5v","format":1,"match":[{"root":"r"}],"rules":[
 {"match":"r","as":"transparent"},{"match":"d","as":"transparent"},
 {"match":"d[@c='x']/p","as":"heading","level":2},
 {"match":"p","as":"block","kind":"paragraph"}]}`
	long := strings.Repeat("x", 200)
	src := `<r><d c="x"><p>h1</p></d><d c="` + long + `"><p>p1</p><p>p2</p></d><d c="x"><p>h2</p></d><d c="` + long + `x"><p>p3</p></d></r>`
	doc := r5Extract(t, prof, src, nil)
	got := ""
	for _, b := range doc.Parts[0].Blocks {
		got += b.Kind.String() + ":" + b.Text + " "
	}
	if want := "heading:h1 paragraph:p1 paragraph:p2 heading:h2 paragraph:p3 "; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// An include through a symbolic link to a regular file is read (links are
// followed); one through a link to a FIFO is not opened; a context ended
// before reading stops the draft.
func TestReview5DraftIncludeLinks(t *testing.T) {
	dir := t.TempDir()
	write := func(name, s string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(s), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	const xs = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">`
	write("real.xsd", xs+`<xs:element name="inc" type="xs:string"/></xs:schema>`)
	if err := os.Symlink("real.xsd", filepath.Join(dir, "link.xsd")); err != nil {
		t.Skip("no symbolic link:", err)
	}
	fifo := syscall.Mkfifo(filepath.Join(dir, "pipe"), 0o600) == nil
	if fifo {
		if err := os.Symlink("pipe", filepath.Join(dir, "pipe.xsd")); err != nil {
			t.Fatal(err)
		}
	}
	write("main.xsd", xs+`<xs:include schemaLocation="link.xsd"/><xs:include schemaLocation="pipe.xsd"/><xs:element name="a" type="xs:string"/></xs:schema>`)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	d, err := DraftProfile(ctx, filepath.Join(dir, "main.xsd"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(d.JSON), `"inc"`) {
		t.Errorf("the element of the linked include is not in the draft:\n%s", d.JSON)
	}
	if fifo && slices.ContainsFunc(d.Files, func(f string) bool { return strings.HasSuffix(f, "pipe.xsd") }) {
		t.Errorf("the FIFO is listed as read: %v", d.Files)
	}
	done, stop := context.WithCancel(context.Background())
	stop()
	if _, err := DraftProfile(done, filepath.Join(dir, "main.xsd")); err == nil {
		t.Error("a cancelled context gives a draft")
	}
}

// In an ordered list, a label first in an item is its marker; one after
// text stays in the text and the count stands; items with no label keep
// the list's count.
func TestReview5OrderedItemLabelMore(t *testing.T) {
	doc := r5Extract(t, r5Prof, `<r><ol><li><num>(a)</num>ay</li><li>bee <num>(x)</num> more</li><li>sea</li></ol></r>`, nil)
	got := ""
	for _, b := range doc.Parts[0].Blocks {
		got += b.Marker + "|" + b.Text + ";"
	}
	if want := "(a)|ay;2.|bee (x) more;3.|sea;"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// sharedMergeCell describes a cell of a table that two merges share, or
// returns "".
func sharedMergeCell(doc *gofastcore.Document) string {
	for _, p := range doc.Parts {
		for _, b := range p.Blocks {
			if b.Table == nil {
				continue
			}
			seen := map[[2]int]bool{}
			for _, m := range b.Table.Merges {
				for r := m.Row; r < m.Row+m.RowSpan; r++ {
					for c := m.Col; c < m.Col+m.ColSpan; c++ {
						if seen[[2]int{r, c}] {
							return fmt.Sprintf("cell (%d,%d) is in two merges: %+v", r, c, b.Table.Merges)
						}
						seen[[2]int{r, c}] = true
					}
				}
			}
		}
	}
	return ""
}

// Random tables in both models, with spans that run over cells spanning
// rows from above: no two merges share a cell.
func TestReview5MergesDisjointRandom(t *testing.T) {
	r := rand.New(rand.NewPCG(5, 9))
	for i := range 400 {
		var sb strings.Builder
		if i%2 == 0 {
			sb.WriteString(`<r><rt>`)
			for range 1 + r.IntN(5) {
				sb.WriteString("<tr>")
				for range 1 + r.IntN(4) {
					fmt.Fprintf(&sb, `<td cs="%d" rs="%d">x</td>`, 1+r.IntN(3), 1+r.IntN(3))
				}
				sb.WriteString("</tr>")
			}
			sb.WriteString(`</rt></r>`)
		} else {
			sb.WriteString(`<r><t><tgroup cols="4"><colspec colname="c1"/><colspec colname="c2"/><colspec colname="c3"/><colspec colname="c4"/><tbody>`)
			for range 1 + r.IntN(5) {
				sb.WriteString("<row>")
				for range 1 + r.IntN(4) {
					a := 1 + r.IntN(4)
					fmt.Fprintf(&sb, `<entry namest="c%d" nameend="c%d" morerows="%d">x</entry>`, a, a+r.IntN(3), r.IntN(3))
				}
				sb.WriteString("</row>")
			}
			sb.WriteString(`</tbody></tgroup></t></r>`)
		}
		doc := r5Extract(t, r5Prof, sb.String(), nil)
		if bad := sharedMergeCell(doc); bad != "" {
			t.Fatalf("%s: %s", sb.String(), bad)
		}
	}
}

// Finding 10 (suspected by the review): the attribute notes of unmapped
// elements inside a block waited in the block's queue without a bound.
// They are bounded at MaxPendingBlocks as in generic reading: the block
// is given as a piece, the notes follow, limit.exceeded says so, and
// nothing is lost.
func TestReview5PendingNotesBounded(t *testing.T) {
	n := generic.MaxPendingBlocks + 100
	src := "<r><p>lead " + strings.Repeat(`<x a="v">w </x>`, n) + "end</p></r>"
	doc := r5Extract(t, r5Prof, src, nil)
	if !r5LimitHit(doc) {
		t.Error("no limit.exceeded")
	}
	notes, words, firstNote := 0, 0, -1
	for i, b := range doc.Parts[0].Blocks {
		if b.Kind == gofastcore.KindNote {
			notes++
			if firstNote < 0 {
				firstNote = i
			}
		} else {
			words += strings.Count(b.Text, "w")
		}
	}
	if notes != n || words != n {
		t.Errorf("%d notes and %d words, want %d each", notes, words, n)
	}
	if firstNote != 1 {
		t.Errorf("the first note is block %d; want 1, after the first piece", firstNote)
	}
}

// Token tests split an element's value once, however many rules test it.
func TestReview5TokenTestOnce(t *testing.T) {
	var rules strings.Builder
	for i := range 50 {
		fmt.Fprintf(&rules, `{"match":"d[@c~='t%d']/p","as":"heading","level":2},`, i)
	}
	prof := `{"profile":"review5t","format":1,"match":[{"root":"r"}],"rules":[` + rules.String() + `
 {"match":"r","as":"transparent"},{"match":"d","as":"transparent"},
 {"match":"p","as":"block","kind":"paragraph"}]}`
	doc := func(n int) string {
		return `<r><d c="` + strings.Repeat("v ", 16*n) + ` t49">` + strings.Repeat("<p/>", n) + `</d></r>`
	}
	small := r5Time(t, prof, doc(500))
	large := r5Time(t, prof, doc(2000))
	if ratio := float64(large) / float64(small); ratio > 8 {
		t.Errorf("4x the input takes %.1fx the time (%v vs %v); want about 4x", ratio, large, small)
	}
	d := r5Extract(t, prof, `<r><d c="a t49 b"><p>h</p></d><d c="t4"><p>h2</p></d><d c="t499"><p>p</p></d></r>`, nil)
	got := ""
	for _, b := range d.Parts[0].Blocks {
		got += b.Kind.String() + " "
	}
	if got != "heading heading paragraph " {
		t.Errorf("kinds %q", got)
	}
}

// Extractor.Charset is used whatever the declaration names or the bytes
// suggest; a byte order mark still wins; an unknown label is an error
// before reading.
func TestCharsetOverride(t *testing.T) {
	read := func(cs, src string) (*gofastcore.Document, error) {
		return (&Extractor{Charset: cs}).Extract(context.Background(), strings.NewReader(src), int64(len(src)), nil)
	}
	text := func(doc *gofastcore.Document) string {
		s := ""
		for _, b := range doc.Parts[0].Blocks {
			s += b.Text
		}
		return s
	}
	// Declared UTF-8, and valid UTF-8 ("\xd0\x94" is "Д"), read as
	// windows-1251 as told.
	doc, err := read("windows-1251", "<?xml version=\"1.0\" encoding=\"UTF-8\"?><r>\xc4\xe0</r>")
	if err != nil {
		t.Fatal(err)
	}
	if got := text(doc); got != "Да" || doc.Metadata.Extra[ExtraEncoding] != "windows-1251" {
		t.Errorf("text %q, encoding %q", got, doc.Metadata.Extra[ExtraEncoding])
	}
	doc, err = read("windows-1251", "<r>\xd0\x94</r>")
	if err != nil {
		t.Fatal(err)
	}
	if got := text(doc); got != "Р”" {
		t.Errorf("valid UTF-8 read as windows-1251: %q", got)
	}
	doc, err = read("Windows-1251", "\xef\xbb\xbf<r>\xd0\x94</r>")
	if err != nil {
		t.Fatal(err)
	}
	if got := text(doc); got != "Д" {
		t.Errorf("with a UTF-8 byte order mark: %q", got)
	}
	if _, err := read("no-such-encoding", "<r/>"); !errors.Is(err, gofastcore.ErrUnsupported) {
		t.Errorf("unknown label: %v", err)
	}
	if _, err := (&Extractor{Charset: "x-unknown"}).Extract(context.Background(), nil, 0, nil); !errors.Is(err, gofastcore.ErrUnsupported) {
		t.Errorf("unknown label is not refused before reading: %v", err)
	}
}

// Values of one Extra key are joined in order, and the joined value stops
// growing at MaxValueBytes.
func TestReview5ExtraJoinOrder(t *testing.T) {
	doc := r5Extract(t, r5Prof, `<r><k>a</k><k>b</k><k>c</k></r>`, nil)
	if got := doc.Metadata.Extra["k"]; got != "a\nb\nc" {
		t.Errorf("extra k %q", got)
	}
	n := 2*MaxValueBytes/len("<k>abcdefgh</k>") + 10
	doc = r5Extract(t, r5Prof, "<r>"+strings.Repeat("<k>abcdefgh</k>", n)+"</r>", nil)
	if got := len(doc.Metadata.Extra["k"]); got > MaxValueBytes || got < MaxValueBytes-16 {
		t.Errorf("joined value of %d bytes, bound %d", got, MaxValueBytes)
	}
}
