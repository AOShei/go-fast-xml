package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AOShei/go-fast-core"
)

const htmlFeed = `<?xml version="1.0"?>
<rss version="2.0"><channel><title>The feed</title><link>https://example.org/</link><description>A plain description.</description>
<item><title>First</title><guid>g1</guid><description>&lt;p&gt;Hello &lt;a href="https://example.org/x"&gt;world&lt;/a&gt;.&lt;/p&gt;&lt;p&gt;Second paragraph.&lt;/p&gt;</description></item>
<item><title>Second</title><guid>g2</guid><description>Just text.</description></item>
</channel></rss>`

// TestFeedChildren reads a feed whose items hold escaped HTML: the
// children are go-fast-html documents with real paragraphs and links.
func TestFeedChildren(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "feed.xml")
	os.WriteFile(a, []byte(htmlFeed), 0o644)
	var out, errs bytes.Buffer
	if code := run(context.Background(), []string{"-spans", a}, &out, &errs); code != 0 {
		t.Fatalf("exit %d: %s", code, errs.String())
	}
	var doc gofastcore.Document
	if err := json.Unmarshal(out.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	if err := doc.Validate(); err != nil {
		t.Fatal(err)
	}
	if len(doc.Children) != 1 {
		t.Fatalf("children %d", len(doc.Children))
	}
	c := doc.Children[0]
	if c.Source.Format != "html" || c.Source.Name != "item/description#g1" || c.Metadata.Extra["xml.parent_anchor"] != "g1" {
		t.Errorf("child source %+v extra %v", c.Source, c.Metadata.Extra)
	}
	var paras []string
	link := ""
	for _, p := range c.Parts {
		for _, b := range p.Blocks {
			if b.Kind == gofastcore.KindParagraph {
				paras = append(paras, b.Text)
			}
			for _, s := range b.Spans {
				if s.Href != "" {
					link = s.Href
				}
			}
		}
	}
	if strings.Join(paras, "|") != "Hello world.|Second paragraph." || link != "https://example.org/x" {
		t.Errorf("child paragraphs %q, link %q", paras, link)
	}
	// Markdown with and without the children.
	for _, c := range []struct {
		flag string
		want bool
	}{{"-children=true", true}, {"-children=false", false}} {
		out.Reset()
		if code := run(context.Background(), []string{"-format", "markdown", c.flag, a}, &out, &errs); code != 0 {
			t.Fatalf("exit %d: %s", code, errs.String())
		}
		md := out.String()
		if strings.Contains(md, "Second paragraph.") != c.want || !strings.Contains(md, "## First") || !strings.Contains(md, "Just text.") {
			t.Errorf("%s:\n%s", c.flag, md)
		}
		out.Reset()
		if code := run(context.Background(), []string{"-format", "text", c.flag, a}, &out, &errs); code != 0 {
			t.Fatalf("exit %d: %s", code, errs.String())
		}
		if strings.Contains(out.String(), "Second paragraph.") != c.want {
			t.Errorf("text %s:\n%s", c.flag, out.String())
		}
	}
}
