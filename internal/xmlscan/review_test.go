package xmlscan

// Tests of the phase 1 review's findings, by number.

import (
	"strconv"
	"strings"
	"testing"
)

// Finding 2: the prefix table holds the prefixes in scope, not every
// prefix ever declared.
func TestPrefixTableShrinks(t *testing.T) {
	var b strings.Builder
	b.WriteString(`<r xmlns:q="urn:q">`)
	for i := range 2000 {
		b.WriteString(`<a xmlns:p` + strconv.Itoa(i) + `="u"><q:b/></a>`)
	}
	b.WriteString(`<q:c/></r>`)
	s := NewBytes([]byte(b.String()), nil)
	for k := s.Next(); k != EOF; k = s.Next() {
		if k == Start && string(s.Local()) == "c" && string(s.URI()) != "urn:q" {
			t.Errorf("q:c in %q", s.URI())
		}
	}
	if len(s.slots) > 2 || len(s.heads) > 2 {
		t.Errorf("%d prefixes, %d slots", len(s.slots), len(s.heads))
	}
}
