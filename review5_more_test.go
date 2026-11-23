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
