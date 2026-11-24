package gofastxml

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/AOShei/go-fast-core"
)

func feedFixture(t *testing.T, name string, child bool) (*gofastcore.Document, []handed) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "profiles", "feed", name))
	if err != nil {
		t.Fatal(err)
	}
	var got []handed
	opts := &gofastcore.Options{}
	if child {
		opts.Child = fakeChild(&got, nil)
	}
	doc, err := ExtractBytes(context.Background(), data, opts)
	if err != nil {
		t.Fatal(err)
	}
	if err := doc.Validate(); err != nil {
		t.Fatal(err)
	}
	if bad := safeCite(data, doc); bad != "" {
		t.Errorf("cite: %s", bad)
	}
	checkChildren(t, data, doc)
	return doc, got
}

func TestFeedRSS2(t *testing.T) {
	doc, got := feedFixture(t, "rss2.xml", true)
	if doc.Metadata.Extra[ExtraProfile] != "feed" || doc.Metadata.Title != "Example feed" || doc.Metadata.Language != "en" ||
		!doc.Metadata.Modified.Equal(time.Date(2026, 3, 2, 10, 0, 0, 0, time.UTC)) || doc.Metadata.Extra["feed.link"] != "https://example.org/" {
		t.Errorf("metadata %+v", doc.Metadata)
	}
	wantBrief(t, doc, "heading1:Example feed\nparagraph:Plain channel text.\nheading2:One\nparagraph:Ann\nparagraph:news\nnote:https://example.org/1.mp3\nheading2:Two\nparagraph:Only text, a < b.\n")
	if len(got) != 2 || got[0].data != "<p>First <b>item</b></p>" || doc.Children[0].Source.Name != "item/description#id-1" || got[1].data != "<p>Full text</p>" {
		t.Errorf("handed %q", got)
	}
	bs := blocks(doc)
	if h := bs[2]; h.Attrs[AttrChildren] != "0" || h.Attrs["feed.link"] != "https://example.org/1" || h.Attrs[AttrAnchorParent] != "id-1" || h.Attrs["feed.guid_is_permalink"] != "false" {
		t.Errorf("heading One %v", h.Attrs)
	}
	if bs[5].Attrs["feed.type"] != "audio/mpeg" || bs[5].Attrs["feed.length"] != "12" || bs[6].Attrs[AttrChildren] != "1" {
		t.Errorf("enclosure %v, heading Two %v", bs[5].Attrs, bs[6].Attrs)
	}
	if doc.Children[0].Metadata.Extra[ExtraParentAnchor] != "id-1" || doc.Children[1].Metadata.Extra[ExtraParentElement] != "rss/channel/item/content:encoded" {
		t.Errorf("children extra %v %v", doc.Children[0].Metadata.Extra, doc.Children[1].Metadata.Extra)
	}
	// Without a child extractor the text is in the body.
	doc, _ = feedFixture(t, "rss2.xml", false)
	wantBrief(t, doc, "heading1:Example feed\nparagraph:Plain channel text.\nheading2:One\nparagraph:<p>First <b>item</b></p>\nparagraph:Ann\nparagraph:news\nnote:https://example.org/1.mp3\nheading2:Two\nparagraph:Only text, a < b.\nparagraph:<p>Full text</p>\n")
}

func TestFeedAtom(t *testing.T) {
	doc, got := feedFixture(t, "atom.xml", true)
	wantBrief(t, doc, "heading1:Atom example\nheading2:A <i>title</i>\nnote:https://example.org/e1\nparagraph:Text <not markup>\n")
	if len(got) != 1 || got[0].mime != "application/xhtml+xml; charset=utf-8" || got[0].data != `<div xmlns="http://www.w3.org/1999/xhtml"><p>Body</p></div>` || doc.Children[0].Source.Name != "entry/content#urn:e1" {
		t.Errorf("handed %q", got)
	}
	if h := blocks(doc)[1]; h.Attrs[AttrMediaType] != "text/html" || h.Attrs[AttrChildren] != "0" || h.Attrs[AttrAnchorParent] != "urn:e1" {
		t.Errorf("heading %v", h.Attrs)
	}
}

func TestFeedRDFNotFeed(t *testing.T) {
	doc, _ := feedFixture(t, "rdf-not-feed.xml", false)
	if p := doc.Metadata.Extra[ExtraProfile]; p != "" {
		t.Errorf("read with %q", p)
	}
}
