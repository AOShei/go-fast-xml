package gofastxml

import (
	"context"
	"strings"
	"testing"

	"github.com/AOShei/go-fast-core"
	"github.com/AOShei/go-fast-xml/internal/generic"
)

// pendingWarning returns the limit.exceeded message about the bound on
// waiting blocks, or "".
func pendingWarning(doc *gofastcore.Document) string {
	for _, w := range doc.AllWarnings() {
		if w.Code == gofastcore.WarnLimitExceeded && w.Severity == gofastcore.SeverityError && strings.Contains(w.Message, "what waits for a block") {
			return w.Message
		}
	}
	return ""
}

// One cut, one behaviour: past MaxPendingBlocks, generic reading raises
// limit.exceeded as the engine does, with the same message.
func TestPendingCutGeneric(t *testing.T) {
	n := generic.MaxPendingBlocks + 100
	cases := map[string]string{
		// More attribute notes wait for a paragraph than the bound.
		"notes": "<r><p>lead " + strings.Repeat(`<x a="v">w </x>`, n) + "end</p></r>",
		// More comments inside a paragraph than the bound.
		"comments": "<r><p>lead " + strings.Repeat(`<!--c-->w `, n) + "end</p></r>",
		// More blocks wait for an element not yet known to be a block,
		// which then has text: it gives more than one block.
		"queue": "<r><d>" + strings.Repeat(`<p>w</p>`, n) + "tail</d></r>",
	}
	for name, src := range cases {
		doc, err := ExtractBytes(context.Background(), []byte(src), nil)
		if err != nil {
			t.Fatal(err)
		}
		if err := doc.Validate(); err != nil {
			t.Fatal(err)
		}
		if doc.Metadata.Extra[ExtraProfile] != "" {
			t.Fatalf("%s: read with profile %q", name, doc.Metadata.Extra[ExtraProfile])
		}
		if pendingWarning(doc) == "" {
			t.Errorf("%s: no limit.exceeded for the cut", name)
		}
		words := 0
		for _, p := range doc.Parts {
			for _, b := range p.Blocks {
				words += strings.Count(b.Text, "w")
			}
		}
		if words < n {
			t.Errorf("%s: %d words, want at least %d", name, words, n)
		}
	}
	// Under the bound nothing is said.
	doc, err := ExtractBytes(context.Background(), []byte("<r><p>lead "+strings.Repeat(`<x a="v">w </x>`, 100)+"end</p></r>"), nil)
	if err != nil {
		t.Fatal(err)
	}
	if w := pendingWarning(doc); w != "" {
		t.Errorf("under the bound: %s", w)
	}
	// A row span past MaxRowSpan is cut, and limit.exceeded says so.
	prof := head("rs", `{"root": "d"}`) + `"rules": [
		{"match": "T", "as": "table"}, {"match": "R", "as": "row"},
		{"match": "C", "as": "cell", "rowspan": "rs"}]}`
	rs := extractWith(t, prof, `<d><T><R><C rs="100000">a</C><C>b</C></R><R><C>c</C></R></T></d>`, nil)
	found := false
	for _, w := range rs.AllWarnings() {
		found = found || w.Code == gofastcore.WarnLimitExceeded && strings.Contains(w.Message, "spanning more than 65535 rows")
	}
	if !found {
		t.Errorf("row span: no limit.exceeded: %+v", rs.AllWarnings())
	}
	// The engine's cut gives the same message.
	eng := r5Extract(t, r5Prof, "<r><p>lead "+strings.Repeat(`<x a="v">w </x>`, n)+"end</p></r>", nil)
	if pendingWarning(eng) == "" {
		t.Error("engine: no limit.exceeded for the cut")
	}
}
