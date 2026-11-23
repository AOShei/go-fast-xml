package gofastxml

// Phase 5 review: a second case of our own for each finding whose review
// test is narrow.

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/AOShei/go-fast-core"
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
