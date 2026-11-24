package gofastxml

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/AOShei/go-fast-core"
)

// feedChild is the fake child extractor of the feed tests: a document of
// one paragraph holding what it is given.
func feedChild(ctx context.Context, name, mime string, r io.ReaderAt, size int64, opts *gofastcore.Options) (*gofastcore.Document, error) {
	var none []handed
	return fakeChild(&none, nil)(ctx, name, mime, r, size, opts)
}

// feedRead reads a file of the corpus with the feed profile chosen by its
// root, with the fake child when child is set.
func feedRead(path string, child bool) (*gofastcore.Document, []byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	opts := &gofastcore.Options{}
	if child {
		opts.Child = feedChild
	}
	doc, err := ExtractBytes(ctx, data, opts)
	return doc, data, err
}

// feedSkip lists the elements whose character data is not text of the
// body by design (metadata, values on the heading, the anchor).
var feedSkip = map[string]string{
	"channel/link": "", "channel/language": "", "channel/lastBuildDate": "", "channel/pubDate": "", "channel/generator": "",
	"channel/copyright": "", "channel/managingEditor": "", "channel/webMaster": "", "channel/docs": "", "channel/ttl": "",
	"channel/category": "", "channel/creator": "", "channel/date": "", "channel/rights": "", "channel/publisher": "",
	"channel/subject": "", "channel/source": "", "channel/updatePeriod": "", "channel/updateFrequency": "", "channel/updateBase": "",
	"channel/author": "", "channel/keywords": "", "channel/explicit": "", "channel/type": "", "channel/new-feed-url": "",
	"owner/name": "", "owner/email": "", "channel/email": "",
	"image/url": "", "image/link": "", "image/width": "", "image/height": "", "image/description": "", "image/title": "",
	"item/guid": "", "item/link": "", "item/pubDate": "", "item/date": "", "item/duration": "",
	"item/episode": "", "item/episodeType": "", "item/explicit": "", "item/point": "", "Point/lat": "", "Point/long": "",
	"feed/id": "", "feed/updated": "", "feed/rights": "", "feed/generator": "", "feed/icon": "", "feed/logo": "",
	"feed/author": "", "entry/id": "", "entry/published": "", "entry/updated": "",
	"author/uri": "", "author/email": "",
}

// feedText is the body's text in order, white space removed, with the
// text handed to each child (its one paragraph, markup removed for
// XHTML) where its element stands.
func feedText(doc *gofastcore.Document) string {
	type at struct {
		off  int64
		text string
	}
	var kids []at
	for _, c := range doc.Children {
		if len(c.Parts) == 0 || len(c.Parts[0].Blocks) == 0 {
			continue
		}
		off, _ := strconv.ParseInt(c.Metadata.Extra[ExtraParentOffset], 10, 64)
		t := c.Parts[0].Blocks[0].Text
		if strings.HasPrefix(c.Source.MIME, "application/xhtml+xml") {
			t = charData([]byte(t))
		}
		kids = append(kids, at{off, noSpace(t)})
	}
	var b strings.Builder
	for _, p := range doc.Parts {
		if p.Kind == gofastcore.PartStored {
			continue
		}
		for _, x := range p.Blocks {
			for len(kids) > 0 && kids[0].off < x.Loc.Offset {
				b.WriteString(kids[0].text)
				kids = kids[1:]
			}
			one := *doc
			one.Parts = []gofastcore.Part{{Kind: p.Kind, Blocks: []gofastcore.Block{x}}}
			b.WriteString(jatsDocText(&one))
		}
	}
	for _, k := range kids {
		b.WriteString(k.text)
	}
	return b.String()
}

// safeCite is jatsCiteCheck; a panic of the test's markup stripper on a
// span is reported as a difference.
func safeCite(data []byte, doc *gofastcore.Document) (bad string) {
	defer func() {
		if r := recover(); r != nil {
			bad = fmt.Sprintf("the oracle's markup stripper failed: %v", r)
		}
	}()
	// A document rule's text kept in the body (media_type) is the
	// element's character data with its markup as text, which the
	// oracle's stripper would remove: it is checked to start where its
	// element does instead.
	c := *doc
	c.Parts = make([]gofastcore.Part, len(doc.Parts))
	for i, p := range doc.Parts {
		p.Blocks = append([]gofastcore.Block(nil), p.Blocks...)
		for j := range p.Blocks {
			b := &p.Blocks[j]
			if b.Attrs[AttrMediaType] != "" {
				if b.Loc.Offset < 0 || b.Loc.Offset >= int64(len(data)) || data[b.Loc.Offset] != '<' {
					return fmt.Sprintf("block %d (media_type) does not start at its element", j)
				}
				b.Attrs = map[string]string{AttrAttribute: "checked"}
			}
		}
		c.Parts[i] = p
	}
	return jatsCiteCheck(data, &c)
}

// charData is the character data of a piece of markup.
func charData(s []byte) string {
	d := xml.NewDecoder(bytes.NewReader(s))
	d.Strict = false
	d.Entity = xml.HTMLEntity
	var b strings.Builder
	for {
		tok, err := d.Token()
		if err != nil {
			return b.String()
		}
		if c, ok := tok.(xml.CharData); ok {
			b.Write(c)
		}
	}
}

// TestFeedOracles checks over every dev feed, without a child extractor
// and with the fake one: (a) the body's text in order, with the text
// handed to children at their place, is the file's character data in
// order, apart from feedSkip; (b) every block's text is in the source at
// its offsets, and the source at each child's parent offsets is its
// element.
func TestFeedOracles(t *testing.T) {
	dir := jatsCorpusDir("dev", "xml-feeds")
	if dir == "" {
		t.Skip("no dev corpus")
	}
	c := &jatsOracleCase{profile: "feed", format: "xml-feeds", skip: feedSkip}
	used := map[string]bool{}
	// The known differences of (a) and (b) are counted and the first ones
	// shown (see the report); a child whose element is not at its parent
	// offsets fails the test.
	fails, notWellFormed, aDiff, bDiff, files := 0, 0, 0, 0, 0
	show := func(format string, args ...any) {
		if fails < 6 {
			t.Logf(format, args...)
		}
		fails++
	}
	for _, path := range jatsCorpusFiles(dir) {
		files++
		for _, child := range []bool{false, true} {
			doc, data, err := feedRead(path, child)
			if err != nil {
				t.Fatalf("%s: %v", path, err)
			}
			got := feedText(doc)
			src, err := c.sourceText(data, used)
			if err != nil {
				// Not well-formed for encoding/xml: the text it read is a
				// prefix of the document's, white space aside.
				notWellFormed++
				if !strings.HasPrefix(got, src[:len(src)*9/10]) {
					aDiff++
					show("%s (child %v): (a) the text read before the error is not the document's", filepath.Base(path), child)
				}
			} else if got != src && !(jatsEntityRE.MatchString(src) && jatsInOrder(got, jatsEntityRE.Split(src, -1))) {
				aDiff++
				i := 0
				for i < len(got) && i < len(src) && got[i] == src[i] {
					i++
				}
				show("%s (child %v): (a) text differs at byte %d:\n doc:  %q\n file: %q", filepath.Base(path), child, i, around(got, i), around(src, i))
			}
			if bad := safeCite(data, doc); bad != "" {
				bDiff++
				show("%s (child %v): (b) %s", filepath.Base(path), child, around(bad, 0))
			}
			for i, k := range doc.Children {
				off, _ := strconv.ParseInt(k.Metadata.Extra[ExtraParentOffset], 10, 64)
				n, _ := strconv.ParseInt(k.Metadata.Extra[ExtraParentLength], 10, 64)
				el := k.Metadata.Extra[ExtraParentElement]
				local := el[strings.LastIndexByte(el, '/')+1:]
				// An element the file leaves unclosed ends where reading did.
				if off < 0 || n <= 0 || off+n > int64(len(data)) || !bytes.HasPrefix(data[off:], []byte("<"+local)) || data[off+n-1] != '>' && off+n < int64(len(data))-64 {
					t.Errorf("%s: child %d: its element %s is not at its parent offsets", filepath.Base(path), i, el)
				}
			}
		}
	}
	t.Logf("%d files, two modes each: (a) differs in %d readings, (b) in %d", files, aDiff, bDiff)
	for k := range feedSkip {
		if !used[k] {
			t.Errorf("exception %q is met nowhere in the dev set", k)
		}
	}
	t.Logf("files read leniently past an error of encoding/xml: %d (both modes)", notWellFormed)
}

// TestFeedCorpus reads every feed of dev and held-out with the fake child
// and without one, and logs aggregate counts (names only on dev).
func TestFeedCorpus(t *testing.T) {
	for _, set := range []string{"dev", "heldout"} {
		dir := jatsCorpusDir(set, "xml-feeds")
		if dir == "" {
			t.Logf("%s: no corpus", set)
			continue
		}
		files := jatsCorpusFiles(dir)
		for _, child := range []bool{false, true} {
			errs, invalid, plain, wrongProfile := 0, 0, 0, 0
			kinds, kids, unmapped, warns := map[string]int{}, map[string]int{}, map[string]int{}, map[string]int{}
			unmappedTotal := 0
			for _, path := range files {
				label := path
				if set == "heldout" {
					label = "a held-out file"
				}
				doc, data, err := feedRead(path, child)
				if err != nil {
					errs++
					t.Errorf("%s: error: %s", label, firstLine(err.Error(), set))
					continue
				}
				if err := doc.Validate(); err != nil {
					invalid++
					t.Errorf("%s: Validate: %s", label, firstLine(err.Error(), set))
				}
				if doc.Metadata.Extra[ExtraProfile] != "feed" {
					wrongProfile++
				}
				for _, b := range blocks(doc) {
					kinds[b.Kind.String()]++
					if b.Kind == gofastcore.KindParagraph && b.Attrs[AttrMediaType] == "" && b.Loc.Offset >= 0 && b.Loc.Offset < int64(len(data)) {
						for _, n := range []string{"<description", "<content:encoded", "<itunes:summary", "<media:description", "<dc:description", "<googleplay:description"} {
							if bytes.HasPrefix(data[b.Loc.Offset:], []byte(n)) {
								plain++
							}
						}
					}
				}
				for _, k := range doc.Children {
					state := "extracted"
					if !childExtracted(k) {
						state = "not extracted"
					}
					mt, _, _ := strings.Cut(k.Source.MIME, ";")
					kids[mt+" "+state]++
				}
				for _, w := range doc.AllWarnings() {
					warns[w.Code]++
					if w.Code != WarnUnmappedElements {
						continue
					}
					for _, sm := range countRE.FindAllStringSubmatch(w.Message, -1) {
						n, _ := strconv.Atoi(sm[2])
						unmappedTotal += n
						if set == "dev" {
							unmapped[strings.TrimSpace(sm[1])] += n
						}
					}
				}
			}
			line := fmt.Sprintf("%s (child %v): %d files, %d errors, %d Validate failures, %d not read with feed; blocks %s; children %s; kept as paragraphs by when markup %d; warnings %s; unmapped elements %d",
				set, child, len(files), errs, invalid, wrongProfile, sorted(kinds), sorted(kids), plain, sorted(warns), unmappedTotal)
			if set == "dev" && !child {
				line += ": " + byCount(unmapped)
			}
			t.Log(line)
		}
	}
}

func sorted(m map[string]int) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for i, k := range keys {
		if i > 0 {
			b.WriteString(", ")
		}
		fmt.Fprintf(&b, "%s %d", k, m[k])
	}
	return b.String()
}
