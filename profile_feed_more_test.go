package gofastxml

import (
	"context"
	"strings"
	"testing"

	"github.com/AOShei/go-fast-core"
)

// feedString reads src with the profile its root selects, with the fake
// child when got is not nil, and checks what every feed test checks.
func feedString(t *testing.T, src string, got *[]handed) *gofastcore.Document {
	t.Helper()
	opts := &gofastcore.Options{}
	if got != nil {
		opts.Child = fakeChild(got, nil)
	}
	doc, err := ExtractBytes(context.Background(), []byte(src), opts)
	if err != nil {
		t.Fatal(err)
	}
	if err := doc.Validate(); err != nil {
		t.Fatal(err)
	}
	if bad := safeCite([]byte(src), doc); bad != "" {
		t.Errorf("cite: %s", bad)
	}
	checkChildren(t, []byte(src), doc)
	return doc
}

// TestStripMarkupCDATA: the oracle's stripper keeps a CDATA section's
// content as text and an ampersand that starts no reference as itself.
func TestStripMarkupCDATA(t *testing.T) {
	for in, want := range map[string]string{
		`<t><![CDATA[<b>I</b>t & co]]> &amp; x</t>`: `<b>I</b>t & co & x`,
		`a & b; <x>c</x> &#65;&lt;`:                 `a & b; c A<`,
		`<![CDATA[cut <i>`:                          `cut <i>`,
		`end & `:                                    `end & `,
	} {
		if got := stripMarkup(in); got != want {
			t.Errorf("stripMarkup(%q) = %q, want %q", in, got, want)
		}
	}
	if got := unescapeMarkup(`<d>&lt;p&gt;x<!--c--><![CDATA[<b>y</b>]]></d>`); got != `<d><p>x<b>y</b></d>` {
		t.Errorf("unescapeMarkup: %q", got)
	}
}

// TestDocumentComments: a comment inside a document rule's element is the
// child's (in the bytes handed over) when the child is extracted, and a
// note of the parent, as anywhere else, when its text stays in the body.
func TestDocumentComments(t *testing.T) {
	const src = `<doc><sec id="s"><t>T</t>` +
		`<d><p>One<!-- c1 --></p></d>` +
		`<content type="xhtml"><div xmlns="http://www.w3.org/1999/xhtml">Two<!-- c2 --></div></content>` +
		`<m>plain <!-- c3 --> text</m><m><!-- c4 --></m><p>P<!-- c5 --></p></sec></doc>`
	read := func(got *[]handed) *gofastcore.Document {
		p, err := LoadProfile(strings.NewReader(docProf))
		if err != nil {
			t.Fatal(err)
		}
		opts := &gofastcore.Options{}
		if got != nil {
			opts.Child = fakeChild(got, nil)
		}
		doc, err := (&Extractor{Profiles: []*Profile{p}}).Extract(context.Background(), strings.NewReader(src), int64(len(src)), opts)
		if err != nil {
			t.Fatal(err)
		}
		if err := doc.Validate(); err != nil {
			t.Fatal(err)
		}
		if bad := safeCite([]byte(src), doc); bad != "" {
			t.Errorf("cite: %s", bad)
		}
		checkChildren(t, []byte(src), doc)
		return doc
	}
	var got []handed
	doc := read(&got)
	wantBrief(t, doc, "heading1:T\nparagraph:plain text\nnote:c3\nnote:c4\nparagraph:P\nnote:c5\n")
	if len(got) != 2 || got[0].data != `<p>One<!-- c1 --></p>` || got[1].data != `<div xmlns="http://www.w3.org/1999/xhtml">Two<!-- c2 --></div>` {
		t.Errorf("handed %q", got)
	}
	doc = read(nil)
	wantBrief(t, doc, "heading1:T\nparagraph:<p>One</p>\nnote:c1\nparagraph:Two\nnote:c2\nparagraph:plain text\nnote:c3\nnote:c4\nparagraph:P\nnote:c5\n")
}

// TestFeedItemWithNoBlock: what an item reads for its first block or its
// anchor comes out when the item makes no block.
func TestFeedItemWithNoBlock(t *testing.T) {
	const rss = `<rss version="2.0"><channel><title>T</title>` +
		`<item><link>https://example.org/only-link</link></item>` +
		`<item><guid isPermaLink="false">only-guid</guid></item>` +
		`<item><enclosure url="https://example.org/a.mp3" type="audio/mpeg" length="3"/></item>` +
		`<item></item>` +
		`<item><pubDate>Mon, 02 Mar 2026 10:00:00 GMT</pubDate><link>https://example.org/l</link><guid>g-5</guid></item>` +
		`</channel></rss>`
	doc := feedString(t, rss, nil)
	wantBrief(t, doc, "heading1:T\nparagraph:https://example.org/only-link\nnote:false\nparagraph:only-guid\nnote:https://example.org/a.mp3\n"+
		"paragraph:Mon, 02 Mar 2026 10:00:00 GMT\nparagraph:https://example.org/l\nparagraph:g-5\n")
	bs := blocks(doc)
	if a := bs[1].Attrs; a["feed.field"] != "link" || a["feed.link"] != "https://example.org/only-link" || a[AttrAnchorParent] != "" {
		t.Errorf("only a link: %v", a)
	}
	if a := bs[3].Attrs; a["feed.field"] != "guid" || a[AttrAnchorParent] != "only-guid" {
		t.Errorf("only a guid: %v", a)
	}
	if a := bs[2].Attrs; a[gofastcore.AttrNoteType] != gofastcore.NoteAttribute || a[AttrAttribute] != "isPermaLink" || a[AttrElement] != "rss/channel/item/guid" ||
		a[AttrAnchorParent] != "only-guid" || a["feed.guid_is_permalink"] != "false" {
		t.Errorf("isPermaLink: %v", a)
	}
	if a := bs[4].Attrs; a["feed.field"] != "enclosure" || a["feed.type"] != "audio/mpeg" {
		t.Errorf("only an enclosure: %v", a)
	}
	if a := bs[5].Attrs; a["feed.field"] != "pubDate" || a["feed.published"] != "Mon, 02 Mar 2026 10:00:00 GMT" || a["feed.link"] != "https://example.org/l" || a[AttrAnchorParent] != "g-5" {
		t.Errorf("first of three: %v", a)
	}
	// RSS 1.0: an item with only its rdf:about.
	const rdf = `<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#" xmlns="http://purl.org/rss/1.0/">` +
		`<channel rdf:about="https://example.org/"><title>R</title></channel><item rdf:about="https://example.org/i1"></item></rdf:RDF>`
	doc = feedString(t, rdf, nil)
	wantBrief(t, doc, "heading1:R\nnote:https://example.org/i1\n")
	if a := blocks(doc)[1].Attrs; a[AttrAttribute] != "rdf:about" || a[AttrAnchorParent] != "https://example.org/i1" {
		t.Errorf("rdf:about: %v", a)
	}
}
