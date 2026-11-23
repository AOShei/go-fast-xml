package xmlscan

// Tests of the phase 1 review's findings, by number.

import (
	"errors"
	"io"
	"strconv"
	"strings"
	"testing"
	"time"
)

// smallReads returns at most n bytes per Read, as a decoder does.
type smallReads struct {
	r     io.Reader
	n     int
	reads int
}

func (s *smallReads) Read(p []byte) (int, error) {
	s.reads++
	return s.r.Read(p[:min(len(p), s.n)])
}

// Finding 4: a long tag read in small pieces is not parsed again for
// every piece.
func TestLongTagSmallReads(t *testing.T) {
	// A long attribute name is scanned byte by byte.
	src := `<r a` + strings.Repeat("x", 4<<20) + `="v"/>`
	r := &smallReads{r: strings.NewReader(src), n: 512}
	s := New(r, nil)
	begin := time.Now()
	for s.Next() != EOF {
	}
	if s.Err() != nil {
		t.Fatal(s.Err())
	}
	if d := time.Since(begin); d > 2*time.Second {
		t.Errorf("took %v", d)
	}
}

// Finding 5: constructs passed over without a token still give the
// caller the chance to stop.
func TestInterruptWhileSkipping(t *testing.T) {
	stop := errors.New("stop")
	for _, src := range []string{
		"<r>" + strings.Repeat("</x>", 100000) + "</r>",
		strings.Repeat("<a>", 600) + strings.Repeat("<b>", 100000),
	} {
		calls := 0
		s := NewBytes([]byte(src), &Options{Interrupt: func() error {
			calls++
			if calls == 2 {
				return stop
			}
			return nil
		}})
		for s.Next() != EOF {
		}
		if !errors.Is(s.Err(), stop) || s.Offset() > int64(len(src))/10 {
			t.Errorf("err %v at %d of %d", s.Err(), s.Offset(), len(src))
		}
	}
}

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
