package gofastxml

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
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

// BenchmarkServicePublic reads a fixed list of 200 Service-Public dev
// files (every n-th of the sorted list, the index files left out) with
// the service-public profile and with generic reading. One op is the 200
// files; allocs/file is allocs/op divided by 200.
func BenchmarkServicePublic(b *testing.B) {
	dir := spDir("dev")
	if dir == "" {
		b.Skip("no Service-Public dev corpus")
	}
	var all []string
	for _, f := range spFiles(dir) {
		if base := filepath.Base(f); base[0] >= 'A' && base[0] <= 'Z' {
			all = append(all, f)
		}
	}
	const n = 200
	if len(all) < n {
		b.Skip("fewer than 200 files")
	}
	var data [][]byte
	total := 0
	for i := range n {
		d, err := os.ReadFile(all[i*len(all)/n])
		if err != nil {
			b.Fatal(err)
		}
		data = append(data, d)
		total += len(d)
	}
	for _, c := range []struct {
		name    string
		profile string
	}{{"profile", ""}, {"generic", ProfileNone}} {
		b.Run(c.name, func(b *testing.B) {
			x := &Extractor{Profile: c.profile}
			b.SetBytes(int64(total))
			b.ReportAllocs()
			var m runtime.MemStats
			runtime.ReadMemStats(&m)
			before := m.Mallocs
			ops := 0
			for b.Loop() {
				for _, d := range data {
					if _, err := x.Extract(context.Background(), bytes.NewReader(d), int64(len(d)), nil); err != nil {
						b.Fatal(err)
					}
				}
				ops++
			}
			runtime.ReadMemStats(&m)
			b.ReportMetric(float64(m.Mallocs-before)/float64(ops*n), "allocs/file")
		})
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

// batchFiles lists the Service-Public dev batch: every file of
// dev/xml-servicepublic (4,194 files, 106 MB, one of 25 MB), sorted.
func batchFiles(b *testing.B) ([]string, int64) {
	dir := spDir("dev")
	if dir == "" {
		b.Skip("no Service-Public dev corpus")
	}
	files := spFiles(dir)
	var total int64
	for _, f := range files {
		info, err := os.Stat(f)
		if err != nil {
			b.Fatal(err)
		}
		total += info.Size()
	}
	return files, total
}

// BenchmarkBatch reads the whole Service-Public dev batch as a consumer
// would, one document at a time, the result dropped. One op is the
// batch; MB/s is of the files' bytes.
//
//   - file: ExtractFile, the profile chosen by the root element;
//   - bytes: ExtractBytes, each file read into one reused buffer first;
//   - generic: ExtractFile with generic reading (ProfileNone);
//   - encodingxml: encoding/xml's RawToken over each file (tokens only,
//     no document), for comparison.
func BenchmarkBatch(b *testing.B) {
	files, total := batchFiles(b)
	ctx := context.Background()
	b.Run("file", func(b *testing.B) {
		b.SetBytes(total)
		b.ReportAllocs()
		defer userCPU(b)()
		for b.Loop() {
			for _, f := range files {
				if _, err := ExtractFile(ctx, f, nil); err != nil {
					b.Fatal(err)
				}
			}
		}
	})
	b.Run("bytes", func(b *testing.B) {
		b.SetBytes(total)
		b.ReportAllocs()
		defer userCPU(b)()
		var buf bytes.Buffer
		for b.Loop() {
			for _, f := range files {
				buf.Reset()
				fh, err := os.Open(f)
				if err != nil {
					b.Fatal(err)
				}
				_, err = buf.ReadFrom(fh)
				fh.Close()
				if err != nil {
					b.Fatal(err)
				}
				if _, err := ExtractBytes(ctx, buf.Bytes(), nil); err != nil {
					b.Fatal(err)
				}
			}
		}
	})
	b.Run("generic", func(b *testing.B) {
		x := &Extractor{Profile: ProfileNone}
		b.SetBytes(total)
		b.ReportAllocs()
		defer userCPU(b)()
		for b.Loop() {
			for _, f := range files {
				if _, err := x.ExtractFile(ctx, f, nil); err != nil {
					b.Fatal(err)
				}
			}
		}
	})
	b.Run("encodingxml", func(b *testing.B) {
		b.SetBytes(total)
		b.ReportAllocs()
		defer userCPU(b)()
		for b.Loop() {
			for _, f := range files {
				fh, err := os.Open(f)
				if err != nil {
					b.Fatal(err)
				}
				d := xml.NewDecoder(fh)
				for {
					if _, err := d.RawToken(); err == io.EOF {
						break
					} else if err != nil {
						b.Fatal(err)
					}
				}
				fh.Close()
			}
		}
	})
}
