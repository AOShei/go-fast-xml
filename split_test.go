package gofastxml

import (
	"strings"
	"testing"

	"github.com/AOShei/go-fast-core"
)

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
