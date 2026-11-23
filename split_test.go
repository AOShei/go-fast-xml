package gofastxml

import (
	"strings"
	"testing"

	"github.com/AOShei/go-fast-core"
)

// TestProfilePartInsideBlock: a part met inside an open block finishes
// the block first; text after the part starts a new block.
func TestProfilePartInsideBlock(t *testing.T) {
	prof := head("partin", `{"root": "doc"}`) + `"rules": [
		{"match": "p", "as": "block", "kind": "paragraph"},
		{"match": "b", "as": "inline", "style": "bold"},
		{"match": "part", "as": "part", "kind": "chapter", "name": {"attr": "n"}}]}`
	src := `<doc><p>before <b>bold</b> <part n="P"><p>inside</p></part> after <b>more</b></p><p>next</p></doc>`
	doc := extractWith(t, prof, src, &gofastcore.Options{Spans: true})
	if len(doc.Parts) != 2 || doc.Parts[1].Name != "P" {
		t.Fatalf("parts %s", dump(doc))
	}
	p1, p2 := doc.Parts[0].Blocks, doc.Parts[1].Blocks
	if len(p1) != 1 || p1[0].Text != "before bold" || p1[0].Continued || len(p1[0].Spans) != 1 {
		t.Fatalf("part 1 %s", dump(doc))
	}
	if l := p1[0].Loc; src[l.Offset:l.Offset+l.Length] != `<p>before <b>bold</b> ` {
		t.Errorf("first block's Loc %q", src[l.Offset:l.Offset+l.Length])
	}
	if len(p2) != 3 || p2[0].Text != "inside" || p2[1].Text != "after more" || p2[1].Continued || p2[2].Text != "next" {
		t.Fatalf("part 2 %s", dump(doc))
	}
	if sp := p2[1].Spans; len(sp) != 1 || p2[1].Text[sp[0].Start:sp[0].End] != "more" {
		t.Errorf("spans %+v", sp)
	}
	if l := p2[1].Loc; src[l.Offset:l.Offset+l.Length] != ` after <b>more</b></p>` {
		t.Errorf("block after the part: Loc %q", src[l.Offset:l.Offset+l.Length])
	}
}

// TestProfileSpansSurviveSplit: a paragraph over the block bound keeps its
// spans, each on the piece that holds it; one across the cut is in two.
func TestProfileSpansSurviveSplit(t *testing.T) {
	if testing.Short() {
		t.Skip("builds 4 MiB of text")
	}
	prof := head("split", `{"root": "doc"}`) + `"rules": [
		{"match": "p", "as": "block", "kind": "paragraph"},
		{"match": "b", "as": "inline", "style": "bold"},
		{"match": "a", "as": "inline", "link": {"attr": "href"}}]}`
	filler := strings.Repeat("x ", (1<<20-200)/2)
	big := strings.Repeat("y", 300)
	huge := strings.Repeat("z ", 3<<18) // 1.5 MiB in one text node
	src := `<doc><p><a href="h1">first</a> ` + filler + `<a href="h2">across <b>` + big + `</b> end</a> tail <a href="h3">last</a></p>` +
		`<p>before <a href="h4">` + huge + `</a> after <a href="h5">five</a></p></doc>`
	doc := extractWith(t, prof, src, &gofastcore.Options{Spans: true})
	type got struct{ text, href string }
	var spans []got
	var bold []string
	pieces := 0
	for _, b := range blocks(doc) {
		pieces++
		for _, sp := range b.Spans {
			if sp.Start < 0 || sp.End > len(b.Text) || sp.Start >= sp.End {
				t.Fatalf("span %+v out of a piece of %d bytes", sp, len(b.Text))
			}
			s := b.Text[sp.Start:sp.End]
			if sp.Style == gofastcore.StyleBold {
				bold = append(bold, s)
				continue
			}
			if len(s) > 20 {
				s = s[:10] + "..." + s[len(s)-10:]
			}
			spans = append(spans, got{s, sp.Href})
		}
	}
	if pieces < 4 {
		t.Fatalf("%d pieces", pieces)
	}
	want := []got{{"first", "h1"}, {"across", "h2"}, {big + " end", "h2"}, {"last", "h3"},
		{"z z z z z ... z z z z z", "h4"}, {"z z z z z ... z z z z z", "h4"}, {"five", "h5"}}
	want[2].text = want[2].text[:10] + "..." + want[2].text[len(want[2].text)-10:]
	if len(spans) != len(want) {
		t.Fatalf("spans %v, want %v", spans, want)
	}
	for i := range want {
		if spans[i] != want[i] {
			t.Errorf("span %d: %v, want %v", i, spans[i], want[i])
		}
	}
	if len(bold) != 1 || bold[0] != big {
		t.Errorf("bold %d spans", len(bold))
	}
}
