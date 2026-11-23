package gofastxml

import (
	"context"
	"fmt"
	"math/rand/v2"
	"strconv"
	"strings"
	"testing"
	"unicode/utf16"

	"github.com/AOShei/go-fast-core"
	"github.com/AOShei/go-fast-xml/internal/charset"
)

// testEntities are the internal entities the generated documents declare.
var testEntities = map[string]string{"lt": "<", "gt": ">", "amp": "&", "quot": `"`, "apos": "'", "ent": "entity text"}

// stripMarkup returns the character data of a piece of source: tags,
// comments and processing instructions removed, CDATA unwrapped,
// references decoded, line ends normalized. A piece may start or end
// inside a CDATA section.
func stripMarkup(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); {
		switch {
		case strings.HasPrefix(s[i:], "<!--"):
			j := strings.Index(s[i:], "-->")
			if j < 0 {
				return b.String()
			}
			i += j + 3
		case strings.HasPrefix(s[i:], "<![CDATA["):
			i += 9
		case strings.HasPrefix(s[i:], "]]>"):
			i += 3
		case s[i] == '<':
			j := strings.IndexByte(s[i:], '>')
			if j < 0 {
				return b.String()
			}
			i += j + 1
		case s[i] == '&':
			j := strings.IndexByte(s[i:], ';')
			name := s[i+1 : i+j]
			switch {
			case strings.HasPrefix(name, "#x"):
				n, _ := strconv.ParseUint(name[2:], 16, 32)
				b.WriteRune(rune(n))
			case strings.HasPrefix(name, "#"):
				n, _ := strconv.ParseUint(name[1:], 10, 32)
				b.WriteRune(rune(n))
			default:
				b.WriteString(testEntities[name])
			}
			i += j + 1
		default:
			b.WriteByte(s[i])
			i++
		}
	}
	return b.String()
}

func collapse(s string) string { return strings.Join(strings.Fields(s), " ") }

// checkLocations checks that each block's location, cut out of src and
// stripped of markup, holds its text, and that the pieces of a split
// block are in order and do not overlap.
func checkLocations(t *testing.T, name string, src []byte, doc *gofastcore.Document, decode func([]byte) string) {
	t.Helper()
	var last [2]*gofastcore.Block // the last paragraph, the last comment
	bad := 0
	for i := range doc.Parts[0].Blocks {
		b := &doc.Parts[0].Blocks[i]
		lo, hi := b.Loc.Offset, b.Loc.Offset+b.Loc.Length
		if lo < 0 || hi > int64(len(src)) || lo > hi {
			t.Errorf("%s: block %d at %d+%d outside %d bytes", name, i, b.Loc.Offset, b.Loc.Length, len(src))
			return
		}
		piece := strings.ReplaceAll(decode(src[lo:hi]), "\r\n", "\n")
		kind := 0
		switch {
		case b.Attrs[gofastcore.AttrStored] != "":
			kind = 1
			piece = strings.NewReplacer("<!--", "", "-->", "").Replace(piece)
		default:
			piece = stripMarkup(piece)
		}
		if b.Kind == gofastcore.KindNote && kind == 0 {
			// An attribute note: its value is in the start tag.
			piece = strings.NewReplacer("&amp;", "&", "&lt;", "<", "&quot;", `"`).Replace(decode(src[lo:hi]))
		}
		if !strings.Contains(collapse(piece), collapse(b.Text)) && bad < 3 {
			bad++
			t.Errorf("%s: block %d (%s, continued %v) at %d+%d: %q not in %q", name, i, b.Kind, b.Continued, lo, b.Loc.Length, clip(b.Text), clip(collapse(piece)))
		}
		if b.Kind == gofastcore.KindParagraph || kind == 1 {
			if p := last[kind]; b.Continued && p != nil && p.Loc.Offset+p.Loc.Length > lo {
				t.Errorf("%s: piece %d at %d overlaps the one before, at %d+%d", name, i, lo, p.Loc.Offset, p.Loc.Length)
			}
			last[kind] = b
		}
	}
}

// genDoc writes a random document with the constructs whose locations are
// easy to get wrong. big makes texts, comments and CDATA long enough to
// split.
func genDoc(r *rand.Rand, big bool) []byte {
	var b strings.Builder
	nl := "\n"
	if r.IntN(2) == 0 {
		nl = "\r\n"
	}
	if r.IntN(3) == 0 {
		b.WriteString("\xEF\xBB\xBF") // a byte order mark
	}
	b.WriteString(`<?xml version="1.0"?>` + nl + `<!DOCTYPE r [<!ENTITY ent "entity text">]>` + nl)
	words := []string{"alpha", "beta", "&amp;", "&lt;", "&#233;", "&#x263A;", "&ent;", "gamma" + nl, "delta\t", "é", "x"}
	text := func(n int) {
		for range n {
			b.WriteString(words[r.IntN(len(words))])
			b.WriteByte(' ')
		}
	}
	long := 1
	if big {
		long = 30000
	}
	b.WriteString(`<r a="v&amp;w">`)
	for range 3 + r.IntN(6) {
		switch r.IntN(6) {
		case 0:
			b.WriteString("<p>")
			text(r.IntN(20) * long)
			b.WriteString("</p>" + nl)
		case 1:
			b.WriteString("<p><b>")
			text(1 + r.IntN(5))
			b.WriteString("</b>")
			for range r.IntN(3) * long {
				b.WriteString(" <i n=\"1\">in</i> ")
				text(2)
			}
			b.WriteString(" tail</p>" + nl)
		case 2:
			b.WriteString("<!--")
			for range r.IntN(10) * long {
				b.WriteString("comment" + nl + " words ")
			}
			b.WriteString("-->" + nl)
		case 3:
			b.WriteString("<p>before <![CDATA[")
			for range r.IntN(10) * long {
				b.WriteString("cdata text" + nl)
			}
			b.WriteString("]]> after</p>")
		case 4:
			b.WriteString(`<x big="` + strings.Repeat("v", r.IntN(3)*long*4) + `"/><p>`)
			text(r.IntN(10) * long)
			b.WriteString("</p>")
		case 5:
			b.WriteString("<p>")
			text(3)
			b.WriteString("<!-- inner -->")
			text(3)
			b.WriteString("</p>")
		}
	}
	b.WriteString("</r>" + nl)
	return []byte(b.String())
}

// encodeAs rewrites a generated UTF-8 document in another encoding: it
// names the encoding in the declaration and writes "é" as the encoding
// does (Shift_JIS has no é: it writes "あ" there).
func encodeAs(src []byte, enc charset.Encoding) []byte {
	s := strings.TrimPrefix(string(src), "\xEF\xBB\xBF")
	switch enc {
	case charset.UTF16LE, charset.UTF16BE:
		s = strings.Replace(s, `version="1.0"?>`, `version="1.0" encoding="UTF-16"?>`, 1)
		out := []byte{0xFE, 0xFF}
		if enc == charset.UTF16LE {
			out = []byte{0xFF, 0xFE}
		}
		for _, u := range utf16.Encode([]rune(s)) {
			if enc == charset.UTF16LE {
				out = append(out, byte(u), byte(u>>8))
			} else {
				out = append(out, byte(u>>8), byte(u))
			}
		}
		return out
	case charset.Windows1252:
		s = strings.Replace(s, `version="1.0"?>`, `version="1.0" encoding="windows-1252"?>`, 1)
		return []byte(strings.ReplaceAll(s, "é", "\xE9"))
	case charset.ShiftJIS:
		s = strings.Replace(s, `version="1.0"?>`, `version="1.0" encoding="Shift_JIS"?>`, 1)
		return []byte(strings.ReplaceAll(s, "é", "\x82\xA0"))
	}
	panic("encoding")
}

// Item A: locations are offsets in the file in every encoding: the
// bytes at a block's location, decoded, hold its text.
func TestLocationsInEncodings(t *testing.T) {
	for _, enc := range []charset.Encoding{charset.UTF16LE, charset.UTF16BE, charset.Windows1252, charset.ShiftJIS} {
		r := rand.New(rand.NewPCG(3, uint64(enc)))
		for i := range 40 {
			big := i == 0 && !testing.Short()
			src := encodeAs(genDoc(r, big), enc)
			doc, err := ExtractBytes(context.Background(), src, nil)
			if err != nil {
				t.Fatalf("%s doc %d: %v", enc, i, err)
			}
			if got := doc.Metadata.Extra[ExtraEncoding]; got != enc.String() {
				t.Fatalf("%s doc %d read as %s", enc, i, got)
			}
			checkLocations(t, fmt.Sprintf("%s doc %d", enc, i), src, doc, func(b []byte) string {
				out, _, _ := charset.DecodeChunk(nil, b, enc, true)
				return string(out)
			})
		}
	}
}

// Item B: every block's location holds its text, in documents with
// entities, CDATA, CR LF, a byte order mark, comments and long texts.
func TestLocationsHoldText(t *testing.T) {
	r := rand.New(rand.NewPCG(1, 2))
	n, continued := 300, 0
	defer func() {
		if continued == 0 {
			t.Error("no block was split")
		}
	}()
	for i := range n {
		big := i%60 == 0
		if big && testing.Short() && i > 0 {
			continue
		}
		src := genDoc(r, big)
		doc, err := ExtractBytes(context.Background(), src, nil)
		if err != nil {
			t.Fatalf("doc %d: %v", i, err)
		}
		checkLocations(t, fmt.Sprintf("doc %d", i), src, doc, func(b []byte) string { return string(b) })
		for _, b := range doc.Parts[0].Blocks {
			if b.Continued {
				continued++
			}
		}
	}
}
