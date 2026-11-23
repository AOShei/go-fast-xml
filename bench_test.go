package gofastxml

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"testing"
)

var benchDoc []byte

// benchXML is a synthetic 10 MiB document: articles of sections with
// titles, mixed-content paragraphs, attributes and a comment now and then.
func benchXML() []byte {
	if benchDoc != nil {
		return benchDoc
	}
	var b bytes.Buffer
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n<corpus xmlns=\"urn:bench\" xml:lang=\"en\">\n")
	for i := 0; b.Len() < 10<<20; i++ {
		fmt.Fprintf(&b, "<article id=\"a%d\" type=\"report\">\n  <title>Article number %d</title>\n", i, i)
		for s := range 4 {
			fmt.Fprintf(&b, "  <section n=\"%d\">\n    <heading>Section %d</heading>\n", s, s)
			for range 3 {
				b.WriteString("    <para>The quick brown fox <em>jumps</em> over the lazy dog &amp; runs away; " +
					"café crème, naïve résumé <link href=\"http://example.com/x\">here</link>.</para>\n")
			}
			if s == 2 {
				b.WriteString("    <!-- reviewed -->\n")
			}
			b.WriteString("  </section>\n")
		}
		b.WriteString("</article>\n")
	}
	b.WriteString("</corpus>\n")
	benchDoc = b.Bytes()
	return benchDoc
}

func BenchmarkExtract(b *testing.B) {
	src := benchXML()
	b.SetBytes(int64(len(src)))
	b.ReportAllocs()
	for b.Loop() {
		if _, err := Extract(context.Background(), bytes.NewReader(src), int64(len(src)), nil); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkExtractNoAttributes(b *testing.B) {
	src := benchXML()
	x := &Extractor{Attributes: AttributesNone}
	b.SetBytes(int64(len(src)))
	b.ReportAllocs()
	for b.Loop() {
		if _, err := x.Extract(context.Background(), bytes.NewReader(src), int64(len(src)), nil); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkEncodingXML tokenizes the same bytes with encoding/xml, for
// comparison: tokens only, no document built.
func BenchmarkEncodingXML(b *testing.B) {
	src := benchXML()
	b.SetBytes(int64(len(src)))
	b.ReportAllocs()
	for b.Loop() {
		d := xml.NewDecoder(bytes.NewReader(src))
		for {
			if _, err := d.RawToken(); err == io.EOF {
				break
			} else if err != nil {
				b.Fatal(err)
			}
		}
	}
}
