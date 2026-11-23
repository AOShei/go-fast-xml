package gofastxml

// Phase 5 review: a second case of our own for each finding whose review
// test is narrow.

import (
	"strings"
	"testing"

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
