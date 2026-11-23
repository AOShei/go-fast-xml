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

// Finding 13: an attribute value is decoded once, and its expansion is
// held only to the bound.
func TestAttrDecodedOnce(t *testing.T) {
	calls := 0
	res := func(name []byte) ([]byte, Resolution) {
		calls++
		if string(name) == "e" {
			return []byte(strings.Repeat("y", 1000)), Expand
		}
		return nil, Keep
	}
	src := `<r xmlns:p="&u;" a="` + strings.Repeat("&e;", 3000) + `"><p:x/></r>`
	s := NewBytes([]byte(src), &Options{Resolve: res, ExpansionLimit: -1, MaxValueBytes: 100000})
	if s.Next() != Start {
		t.Fatal("no start")
	}
	for i := range s.NumAttr() {
		s.AttrAt(i)
		s.AttrAt(i)
	}
	_, _, v := s.AttrAt(1)
	if len(v) != 100000 || len(s.attrBuf) > 101000 || s.Stats.ValuesCut != 1 {
		t.Errorf("value %d, buffer %d, cut %d", len(v), len(s.attrBuf), s.Stats.ValuesCut)
	}
	// One lookup of u, and of e only as far as the bound.
	if calls > 2+101 {
		t.Errorf("%d lookups", calls)
	}
}

// Finding 3: open elements keep bounded names, and offsets stay valid.
func TestLongNamesKept(t *testing.T) {
	long := strings.Repeat("n", 5000)
	src := "<" + long + "1><" + long + "2><" + long + "2>t</" + long + "2></" + long + "2></" + long + "1>"
	s := NewBytes([]byte(src), nil)
	ends := 0
	for k := s.Next(); k != EOF; k = s.Next() {
		if len(s.names) > 3*MaxNameKept {
			t.Fatalf("%d bytes of names", len(s.names))
		}
		if k == End {
			ends++
		}
	}
	if ends != 3 || s.Stats.Repairs != 0 || s.Stats.NamesCut != 3 {
		t.Errorf("%d ends, %d repairs, %d cut", ends, s.Stats.Repairs, s.Stats.NamesCut)
	}
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

// Phase 5: an unclosed comment in the internal subset with no "]>" after
// it ends at the end of the DOCTYPE's bounded window (the token limit),
// counted as a repair, and the element after the window is read.
func TestDoctypeUnclosedCommentAtWindow(t *testing.T) {
	src := `<!DOCTYPE r [ <!-- ` + strings.Repeat("x", 8000) + `<r>Hello</r>`
	s := New(strings.NewReader(src), &Options{MaxTokenBytes: 4096})
	var doctype, start bool
	for k := s.Next(); k != EOF; k = s.Next() {
		doctype = doctype || k == Doctype
		start = start || k == Start && string(s.Local()) == "r"
	}
	if s.Err() != nil {
		t.Fatal(s.Err())
	}
	if !doctype || !start || s.Stats.Repairs == 0 {
		t.Errorf("doctype %v, element r %v, repairs %d", doctype, start, s.Stats.Repairs)
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
