package gofastxml

import (
	"context"
	"strings"
	"testing"
)

func TestNewsmlCensus(t *testing.T) { jatsCensus(t, "xml-newsml") }

var newsmlCase = &jatsOracleCase{profile: "newsml-g2", format: "xml-newsml", prep: pubmedPrep}

func TestNewsmlCorpus(t *testing.T) { newsmlCase.run(t) }

func TestNewsmlOracles(t *testing.T) { newsOracles(t, newsmlCase, 1) }

func TestNewsmlFixtureItem(t *testing.T) { newsFixture(t, newsmlCase, "item") }

// The profile is chosen by the item roots, newsMessage and catalog in
// the NAR namespace; the same names in no namespace or another stay
// generic.
func TestNewsmlSelection(t *testing.T) {
	const nar = `xmlns="http://iptc.org/std/nar/2006-10-01/"`
	for _, c := range []struct{ src, want string }{
		{`<newsItem ` + nar + ` guid="a"/>`, "newsml-g2"},
		{`<packageItem ` + nar + ` guid="a"/>`, "newsml-g2"},
		{`<newsMessage ` + nar + `><header/></newsMessage>`, "newsml-g2"},
		{`<knowledgeItem ` + nar + `/>`, "newsml-g2"},
		{`<conceptItem ` + nar + `/>`, "newsml-g2"},
		{`<planningItem ` + nar + `/>`, "newsml-g2"},
		{`<catalogItem ` + nar + `/>`, "newsml-g2"},
		{`<catalog ` + nar + `/>`, "newsml-g2"},
		{`<newsItem guid="a"/>`, ""},
		{`<newsItem xmlns="urn:example" guid="a"/>`, ""},
		{`<contentMeta ` + nar + `/>`, ""},
	} {
		doc, err := ExtractBytes(context.Background(), []byte(c.src), nil)
		if err != nil {
			t.Fatal(err)
		}
		if got := doc.Metadata.Extra[ExtraProfile]; got != c.want {
			t.Errorf("%s: profile %q, want %q", c.src, got, c.want)
		}
	}
}

// TestNewsmlUnmappedPaths logs, over dev, the paths of the elements read
// generically (an inferred block's Attrs["element"]), with counts.
func TestNewsmlUnmappedPaths(t *testing.T) {
	dir := jatsCorpusDir("dev", "xml-newsml")
	if dir == "" {
		t.Skip("no dev corpus")
	}
	paths := map[string]int{}
	for _, path := range jatsCorpusFiles(dir) {
		res := corpusOne(path)
		if res.doc == nil {
			continue
		}
		for _, p := range res.doc.Parts {
			for _, b := range p.Blocks {
				if e := b.Attrs["element"]; e != "" {
					if i := strings.Index(e, "/"); i >= 0 {
						e = e[i+1:]
					}
					paths[e]++
				}
			}
		}
	}
	t.Logf("generic paths: %s", byCount(paths))
}
