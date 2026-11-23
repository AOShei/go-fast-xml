package gofastxml

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/AOShei/go-fast-core"
)

// timed extracts src with opts, checks the document, and fails if it
// took longer than limit.
func timed(t *testing.T, src []byte, opts *gofastcore.Options, limit time.Duration) *gofastcore.Document {
	t.Helper()
	start := time.Now()
	doc, err := Extract(context.Background(), bytes.NewReader(src), int64(len(src)), opts)
	if err != nil {
		t.Fatal(err)
	}
	if d := time.Since(start); d > limit {
		t.Errorf("took %v, more than %v", d, limit)
	}
	if err := doc.Validate(); err != nil {
		t.Fatal(err)
	}
	return doc
}

func hasLimit(doc *gofastcore.Document, substr string) bool {
	for _, w := range doc.AllWarnings() {
		if w.Code == gofastcore.WarnLimitExceeded && w.Severity == gofastcore.SeverityError && strings.Contains(w.Message, substr) {
			return true
		}
	}
	return false
}

func TestDeepNesting(t *testing.T) {
	const n = 100_000
	src := strings.Repeat("<a>", n) + "deep" + strings.Repeat("</a>", n)
	doc := timed(t, []byte(src), nil, 5*time.Second)
	if !hasLimit(doc, "nested deeper than") {
		t.Errorf("warnings %+v", doc.Parts[0].Warnings)
	}
	if bs := doc.Parts[0].Blocks; len(bs) != 1 || bs[0].Text != "deep" || len(bs[0].Attrs[AttrElement]) > MaxPathBytes {
		t.Errorf("blocks %d", len(bs))
	}
}

func TestMillionAttributes(t *testing.T) {
	if testing.Short() {
		t.Skip("13 MB tag")
	}
	var b bytes.Buffer
	b.WriteString("<a")
	for i := range 1_000_000 {
		fmt.Fprintf(&b, ` x%d="v"`, i)
	}
	b.WriteString(">text</a>")
	doc := timed(t, b.Bytes(), nil, 10*time.Second)
	if !hasLimit(doc, "attributes past") {
		t.Errorf("warnings %+v", doc.Parts[0].Warnings)
	}
	if n := len(doc.Parts[0].Blocks); n != MaxAttributes+1 {
		t.Errorf("%d blocks", n)
	}
}

func lolDoc(levels, fan int) []byte {
	var b strings.Builder
	b.WriteString("<!DOCTYPE lolz [\n<!ENTITY lol0 \"lol\">\n")
	for i := 1; i <= levels; i++ {
		fmt.Fprintf(&b, "<!ENTITY lol%d \"%s\">\n", i, strings.Repeat(fmt.Sprintf("&lol%d;", i-1), fan))
	}
	fmt.Fprintf(&b, "]>\n<lolz a=\"&lol%d;\">&lol%d;</lolz>", levels, levels)
	return []byte(b.String())
}

func TestBillionLaughs(t *testing.T) {
	var before runtime.MemStats
	runtime.ReadMemStats(&before)
	doc := timed(t, lolDoc(9, 10), nil, 2*time.Second)
	if !hasLimit(doc, "entity references kept as written") {
		t.Errorf("warnings %+v", doc.Parts[0].Warnings)
	}
	text := 0
	for _, b := range doc.Parts[0].Blocks {
		text += len(b.Text)
	}
	if text > 4*EntityExpansionBase {
		t.Errorf("%d bytes of text", text)
	}
	// The decode budget bounds expansion too.
	doc = timed(t, lolDoc(9, 10), &gofastcore.Options{Limits: gofastcore.Limits{MaxDecodedBytes: 1000}}, time.Second)
	text = 0
	for _, b := range doc.Parts[0].Blocks {
		text += len(b.Text)
	}
	if text > 4000 {
		t.Errorf("decode budget: %d bytes of text", text)
	}
}

func TestQuadraticBlowup(t *testing.T) {
	// One entity of 64 KiB, referenced 50,000 times: 3.2 GB if expanded.
	var b strings.Builder
	fmt.Fprintf(&b, "<!DOCTYPE r [<!ENTITY a \"%s\">]><r>", strings.Repeat("x", 64<<10))
	for range 50_000 {
		b.WriteString("&a;")
	}
	b.WriteString("</r>")
	doc := timed(t, []byte(b.String()), nil, 3*time.Second)
	if !hasLimit(doc, "entity references kept as written") {
		t.Errorf("warnings %+v", doc.Parts[0].Warnings)
	}
	text := 0
	for _, bl := range doc.Parts[0].Blocks {
		text += len(bl.Text)
	}
	if limit := EntityExpansionBase + EntityExpansionRatio*b.Len() + 4*b.Len(); text > limit {
		t.Errorf("%d bytes of text, more than %d", text, limit)
	}
}

func TestHugeTextNode(t *testing.T) {
	if testing.Short() {
		t.Skip("64 MiB text")
	}
	word := []byte("lorem ipsum dolor sit amet ")
	src := make([]byte, 0, 64<<20+32)
	src = append(src, "<doc><p>"...)
	for len(src) < 64<<20 {
		src = append(src, word...)
	}
	src = append(src, "</p></doc>"...)
	doc := timed(t, src, nil, 20*time.Second)
	bs := doc.Parts[0].Blocks
	if len(bs) < 64 {
		t.Fatalf("%d blocks", len(bs))
	}
	for i, b := range bs {
		if len(b.Text) > MaxBlockBytes || b.Continued != (i > 0) {
			t.Fatalf("block %d: %d bytes, continued %v", i, len(b.Text), b.Continued)
		}
		if !strings.HasPrefix(b.Text, "lorem") && i > 0 && !strings.HasPrefix(b.Text, "ipsum") && !strings.HasPrefix(b.Text, "dolor") &&
			!strings.HasPrefix(b.Text, "sit") && !strings.HasPrefix(b.Text, "amet") {
			t.Fatalf("block %d not cut at white space: %q", i, b.Text[:20])
		}
	}
}

// genStream is an io.ReaderAt of a document of small elements: a root,
// then records, each a container with white space and an empty element
// in it, a paragraph every 65,536 records, then the end of the root.
type genStream struct{ size int64 }

const (
	streamHead   = "<root>\n"
	streamTail   = "</root>\n"
	streamRecord = "<rec>\n  <f/>\n</rec>\n"
	streamText   = "<p>marker</p>\n      " // as long as a record
)

func (g genStream) ReadAt(p []byte, off int64) (int, error) {
	n := 0
	for n < len(p) && off < g.size {
		var c byte
		switch body := g.size - int64(len(streamTail)); {
		case off < int64(len(streamHead)):
			c = streamHead[off]
		case off >= body:
			c = streamTail[off-body]
		default:
			rel := (off - int64(len(streamHead))) % int64(len(streamRecord))
			rec := (off - int64(len(streamHead))) / int64(len(streamRecord))
			if rec%65536 == 0 {
				c = streamText[rel]
			} else {
				c = streamRecord[rel]
			}
		}
		p[n] = c
		n++
		off++
	}
	if n < len(p) {
		return n, io.EOF
	}
	return n, nil
}

func TestLargeStream(t *testing.T) {
	if testing.Short() {
		t.Skip("200 MiB stream")
	}
	records := int64(200<<20) / int64(len(streamRecord))
	size := int64(len(streamHead)+len(streamTail)) + records*int64(len(streamRecord))
	// The body must end on a whole record for the tail to follow one.
	var peak atomic.Uint64
	done := make(chan struct{})
	go func() {
		var m runtime.MemStats
		for {
			select {
			case <-done:
				return
			case <-time.After(20 * time.Millisecond):
				runtime.ReadMemStats(&m)
				if m.HeapAlloc > peak.Load() {
					peak.Store(m.HeapAlloc)
				}
			}
		}
	}()
	runtime.GC()
	start := time.Now()
	doc, err := (&Extractor{}).Extract(context.Background(), genStream{size}, size, nil)
	close(done)
	if err != nil {
		t.Fatal(err)
	}
	if err := doc.Validate(); err != nil {
		t.Fatal(err)
	}
	want := int((records + 65535) / 65536)
	if n := len(doc.Parts[0].Blocks); n != want {
		t.Errorf("%d blocks, want %d", n, want)
	}
	t.Logf("%d MiB in %v, peak heap %d MiB", size>>20, time.Since(start), peak.Load()>>20)
	if peak.Load() > 64<<20 {
		t.Errorf("peak heap %d MiB for %d MiB of input", peak.Load()>>20, size>>20)
	}
}

func TestTextBudget(t *testing.T) {
	src := "<doc>" + strings.Repeat("<p>some text here</p>", 1000) + "</doc>"
	doc := timed(t, []byte(src), &gofastcore.Options{Limits: gofastcore.Limits{MaxTextBytes: 10_000}}, time.Second)
	if !hasLimit(doc, "bytes of input left unread") {
		t.Errorf("warnings %+v", doc.Parts[0].Warnings)
	}
	if n := len(doc.Parts[0].Blocks); n == 0 || n > 10_000/gofastcore.BlockCost {
		t.Errorf("%d blocks", n)
	}
}

func TestInputLimit(t *testing.T) {
	_, err := Extract(context.Background(), strings.NewReader("<a/>"), 4, &gofastcore.Options{Limits: gofastcore.Limits{MaxInputBytes: 3}})
	if !errors.Is(err, gofastcore.ErrLimitExceeded) {
		t.Errorf("err %v", err)
	}
}

func TestNameAndValueBounds(t *testing.T) {
	long := strings.Repeat("n", MaxNameBytes+10)
	src := "<doc><" + long + " " + long + "=\"v\" big=\"" + strings.Repeat("v ", MaxValueBytes/2+10) + "\">t</" + long + "></doc>"
	doc := timed(t, []byte(src), nil, 2*time.Second)
	for _, want := range []string{"names longer than", "1 attribute value longer than"} {
		if !hasLimit(doc, want) {
			t.Errorf("no %q in %+v", want, doc.Parts[0].Warnings)
		}
	}
	name := strings.Repeat("m", 200)
	src = strings.Repeat("<"+name+">", 30) + "t" + strings.Repeat("</"+name+">", 30)
	doc = timed(t, []byte(src), nil, time.Second)
	if p := doc.Parts[0].Blocks[0].Attrs[AttrElement]; len(p) > MaxPathBytes || !hasLimit(doc, "paths of element names") {
		t.Errorf("path of %d bytes, warnings %+v", len(p), doc.Parts[0].Warnings)
	}
}

func TestTagTooLarge(t *testing.T) {
	if testing.Short() {
		t.Skip("17 MiB tag")
	}
	src := "<doc><p>before</p><x a=\"" + strings.Repeat("v", MaxTagBytes+1) + "\"/><p>after</p></doc>"
	doc := timed(t, []byte(src), nil, 10*time.Second)
	if !hasLimit(doc, "larger than") || doc.Parts[0].Blocks[0].Text != "before" {
		t.Errorf("warnings %+v", doc.Parts[0].Warnings)
	}
}

func TestCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Extract(ctx, strings.NewReader("<a/>"), 4, nil); !errors.Is(err, context.Canceled) {
		t.Errorf("err %v", err)
	}
}

func TestDeterministic(t *testing.T) {
	src := lolDoc(5, 10)
	var first []byte
	for i := range 3 {
		doc := timed(t, src, &gofastcore.Options{Workers: i + 1}, time.Second)
		js, _ := json.Marshal(doc)
		if i == 0 {
			first = js
		} else if !bytes.Equal(js, first) {
			t.Fatal("output differs between runs")
		}
	}
}
