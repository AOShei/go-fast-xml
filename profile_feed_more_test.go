package gofastxml

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"

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

// fullBrief is brief with each block's attrs, sorted, and its anchor.
func fullBrief(doc *gofastcore.Document) string {
	var b strings.Builder
	for _, x := range blocks(doc) {
		b.WriteString(brief([]gofastcore.Block{x}))
		keys := make([]string, 0, len(x.Attrs))
		for k := range x.Attrs {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			fmt.Fprintf(&b, "  %s=%s\n", k, x.Attrs[k])
		}
		if x.Loc.Anchor != "" {
			fmt.Fprintf(&b, "  #%s\n", x.Loc.Anchor)
		}
		if x.Image != nil {
			fmt.Fprintf(&b, "  image=%s\n", x.Image.ID)
		}
	}
	return b.String()
}

func wantFull(t *testing.T, doc *gofastcore.Document, want string) {
	t.Helper()
	if got := fullBrief(doc); got != want {
		t.Errorf("blocks:\n%s\nwant:\n%s", got, want)
	}
}

func TestFeedRSS091(t *testing.T) {
	doc, got := feedFixture(t, "rss091.xml", true)
	wantFull(t, doc, `heading1:Old feed
paragraph:A feed of 0.91.
image:Old feed logo
  feed.height=31
  feed.link=http://example.org/
  feed.width=88
  image=http://example.org/logo.gif
heading2:Café opens
  feed.link=http://example.org/cafe
paragraph:Coffee & cake.
paragraph:Search
  feed.description=Search the site
  feed.field=textInput
  feed.link=http://example.org/search
  feed.name=q
`)
	m := doc.Metadata
	if len(got) != 0 || m.Title != "Old feed" || m.Language != "en-gb" || m.Extra["feed.version"] != "0.91" ||
		m.Extra["feed.rights"] != "Copyright 2001" || m.Extra["feed.link"] != "http://example.org/" {
		t.Errorf("metadata %+v, handed %q", m, got)
	}
}

func TestFeedRSS1(t *testing.T) {
	doc, got := feedFixture(t, "rss1.xml", true)
	wantFull(t, doc, `heading1:RDF feed
paragraph:Site news
note:https://example.org/a
  attribute=resource
  feed.field=items
  type=attribute
note:https://example.org/b
  attribute=resource
  feed.field=items
  type=attribute
note:https://example.org/search
  attribute=resource
  feed.field=textinput
  type=attribute
heading2:First
  anchor_parent=https://example.org/a
  children=0
  feed.link=https://example.org/a
paragraph:Ann
  anchor_parent=https://example.org/a
  feed.field=dc:creator
paragraph:Plain words.
  anchor_parent=https://example.org/a
note:https://example.org/b
  anchor_parent=https://example.org/b
  attribute=rdf:about
  element=rdf:RDF/item
  feed.link=https://example.org/b
  type=attribute
paragraph:https://example.org/b
  anchor_parent=https://example.org/b
  feed.field=link
paragraph:Search
  feed.description=Search the site
  feed.field=textInput
  feed.link=https://example.org/search
  feed.name=q
  #https://example.org/search
`)
	m := doc.Metadata
	if len(got) != 1 || got[0].name != "item/content:encoded#https://example.org/a" || got[0].data != "<p>Rich <em>words</em></p>" ||
		m.Title != "RDF feed" || m.Language != "fr" || !m.Modified.Equal(time.Date(2026, 3, 2, 10, 0, 0, 0, time.UTC)) ||
		m.Extra["feed.about"] != "https://example.org/rss" || m.Extra["feed.link"] != "https://example.org/" {
		t.Errorf("metadata %+v, handed %q", m, got)
	}
}

func TestFeedPodcast(t *testing.T) {
	doc, got := feedFixture(t, "podcast.xml", true)
	wantFull(t, doc, `heading1:Pod
paragraph:Talk about things.
image:
  image=https://pod.example/art.jpg
paragraph:Support us
  feed.field=podcast:funding
  feed.url=https://pod.example/give
heading2:Episode 1
  anchor_parent=pod-ep-1
  feed.guid_is_permalink=false
  feed.itunes_duration=00:42:10
  feed.itunes_episode=1
  feed.itunes_episode_type=full
  feed.published=Tue, 03 Mar 2026 08:00:00 +0000
paragraph:The first one
  anchor_parent=pod-ep-1
  feed.field=itunes:subtitle
paragraph:Show notes.
  anchor_parent=pod-ep-1
note:https://pod.example/1.mp3
  anchor_parent=pod-ep-1
  attribute=url
  feed.field=enclosure
  feed.length=1000
  feed.type=audio/mpeg
  type=attribute
note:https://pod.example/1.vtt
  anchor_parent=pod-ep-1
  attribute=url
  feed.field=podcast:transcript
  feed.language=en
  feed.type=text/vtt
  type=attribute
note:https://pod.example/1.json
  anchor_parent=pod-ep-1
  attribute=url
  feed.field=podcast:chapters
  feed.type=application/json+chapters
  type=attribute
paragraph:Host Name
  anchor_parent=pod-ep-1
  feed.field=podcast:person
  feed.href=https://pod.example/host
  feed.role=host
`)
	m := doc.Metadata
	if len(got) != 1 || got[0].name != "channel/itunes:summary" || got[0].data != "A summary <b>in bold</b>." ||
		m.Title != "Pod" || strings.Join(m.Authors, "|") != "Host Name" || strings.Join(m.Keywords, "|") != "Podcasting|Technology" ||
		m.Extra["feed.itunes_explicit"] != "false" || m.Extra["feed.itunes_owner_email"] != "owner@pod.example" ||
		m.Extra["feed.itunes_owner_name"] != "Owner" || m.Extra["feed.itunes_type"] != "episodic" {
		t.Errorf("metadata %+v, handed %q", m, got)
	}
}

// TestFeedDecodeBudgetSpent: once the decode budget is spent no child is
// handed over; the text stays in the body and limit.exceeded says so.
func TestFeedDecodeBudgetSpent(t *testing.T) {
	var b strings.Builder
	b.WriteString(`<rss version="2.0"><channel><title>T</title>`)
	for i := range 40 {
		fmt.Fprintf(&b, `<item><title>I%d</title><description>&lt;p&gt;%s&lt;/p&gt;</description></item>`, i, strings.Repeat("x", 1000))
	}
	b.WriteString(`</channel></rss>`)
	src := b.String()
	var got []handed
	opts := &gofastcore.Options{Child: fakeChild(&got, nil), Limits: gofastcore.Limits{MaxDecodedBytes: 20000}}
	doc, err := ExtractBytes(context.Background(), []byte(src), opts)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) == 0 || len(got) >= 40 {
		t.Fatalf("%d children handed over", len(got))
	}
	kept := 0
	for _, x := range blocks(doc) {
		if x.Attrs[AttrMediaType] == "text/html" {
			kept++
			if !strings.Contains(x.Text, strings.Repeat("x", 1000)) {
				t.Errorf("kept text cut: %q", around(x.Text, 0))
			}
		}
	}
	found := false
	for _, w := range doc.AllWarnings() {
		if w.Code == gofastcore.WarnLimitExceeded && w.Severity == gofastcore.SeverityError && strings.Contains(w.Message, "21 embedded documents were not handed") {
			found = true
		}
	}
	if len(got)+kept != 40 || len(doc.Children) != 40 || !found {
		t.Errorf("%d handed, %d kept, %d children; warnings %v", len(got), kept, len(doc.Children), doc.AllWarnings())
	}
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
