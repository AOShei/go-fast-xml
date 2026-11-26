package gofastxml

import (
	"context"
	"flag"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/AOShei/go-fast-core"
)

func TestNitfCensus(t *testing.T) { jatsCensus(t, "xml-nitf") }

// nitfCase: pubmedPrep removes the name column and attribute rows of the
// name and value tables (the head), which are not character data.
var nitfCase = &jatsOracleCase{profile: "nitf", format: "xml-nitf", prep: pubmedPrep}

func TestNitfCorpus(t *testing.T) { nitfCase.run(t) }

func TestNitfOracles(t *testing.T) { newsOracles(t, nitfCase) }

// latin1RE finds an XML declaration naming ISO-8859-1.
var latin1RE = regexp.MustCompile(`^(?:\xef\xbb\xbf)?<\?xml[^>]*encoding=["'](?i:iso-8859-1|latin1)["']`)

// latin1ToUTF8 decodes ISO-8859-1 bytes.
func latin1ToUTF8(b []byte) string {
	u := make([]rune, len(b))
	for i, c := range b {
		u[i] = rune(c)
	}
	return string(u)
}

// newsCite is the case's cite (oracle b); for a file declared ISO-8859-1,
// whose spans are not UTF-8, each block is looked for in its span decoded
// from ISO-8859-1.
func newsCite(c *jatsOracleCase, src []byte, doc *gofastcore.Document) string {
	if !latin1RE.Match(src) {
		return c.cite(src, doc)
	}
	for pi := range doc.Parts {
		for bi := range doc.Parts[pi].Blocks {
			b := &doc.Parts[pi].Blocks[bi]
			if !citable(b) {
				continue
			}
			l := b.Loc
			if l.Offset < 0 || l.Length <= 0 || l.Offset+l.Length > int64(len(src)) {
				return "block " + strconv.Itoa(bi) + " has no span in the source"
			}
			span := latin1ToUTF8(src[l.Offset : l.Offset+l.Length])
			text := noSpace(gofastcore.ExpandLigatures(b.Text))
			s, _ := c.sourceText([]byte(span), map[string]bool{})
			if !strings.Contains(noSpace(stripMarkup(span)), text) && !strings.Contains(s, text) {
				return "block " + strconv.Itoa(bi) + " text " + strconv.Quote(b.Text) + " not in its span " + strconv.Quote(span)
			}
		}
	}
	return ""
}

// newsOracles is jatsOracleCase.oracles with newsCite for oracle (b).
func newsOracles(t *testing.T, c *jatsOracleCase) {
	dir := jatsCorpusDir("dev", c.format)
	if dir == "" {
		t.Skip("no dev corpus")
	}
	used := map[string]bool{}
	fails := 0
	for _, path := range jatsCorpusFiles(dir) {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		doc, err := ExtractBytes(context.Background(), data, nil)
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		src, err := c.sourceText(data, used)
		if err != nil {
			t.Fatalf("%s: encoding/xml: %v", path, err)
		}
		if c.prep != nil {
			c.prep(doc)
		}
		got := jatsDocText(doc)
		if got != src {
			got, src = jatsNumberRE.ReplaceAllString(got, ""), jatsNumberRE.ReplaceAllString(src, "")
		}
		if got != src && jatsEntityRE.MatchString(src) && jatsInOrder(got, jatsEntityRE.Split(src, -1)) {
			got = src
		}
		if got != src && fails < 10 {
			fails++
			i := 0
			for i < len(got) && i < len(src) && got[i] == src[i] {
				i++
			}
			t.Errorf("%s: (a) text differs at byte %d:\n doc:  %q\n file: %q", filepath.Base(path), i, around(got, i), around(src, i))
		}
		if bad := newsCite(c, data, doc); bad != "" && fails < 10 {
			fails++
			t.Errorf("%s: (b) %s", filepath.Base(path), around(bad, 0))
		}
	}
	for k := range c.skip {
		if !used[k] {
			t.Errorf("exception %q is met nowhere in the dev set", k)
		}
	}
}

var nitfUpdate = flag.Bool("nitf.update", false, "rewrite testdata/profiles/nitf/*.want")

// newsFixture reads a fixture with the built-in profiles and compares
// its dump (pubmedDump), its warnings and its title with the .want file
// beside it, written by hand from the fixture and checked line by line
// (-nitf.update writes it from the output, to be reviewed before it is
// committed).
func newsFixture(t *testing.T, c *jatsOracleCase, name string) *gofastcore.Document {
	t.Helper()
	doc, data := jatsExtractFixture(t, c.profile+"/"+name+".xml", c.profile)
	got := "title: " + doc.Metadata.Title + "\n" + pubmedDump(doc, data)
	for _, w := range doc.AllWarnings() {
		got += "warning " + w.Code + ": " + w.Message + "\n"
	}
	wantPath := filepath.Join("testdata", "profiles", c.profile, name+".want")
	if *nitfUpdate {
		if err := os.WriteFile(wantPath, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(wantPath)
	if err != nil {
		t.Fatal(err)
	}
	if got != string(want) {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
	if bad := newsCite(c, data, doc); bad != "" {
		t.Errorf("cite: %s", bad)
	}
	return doc
}

func TestNitfFixtureStory(t *testing.T) { newsFixture(t, nitfCase, "story") }

func TestNitfFixtureNamespace(t *testing.T) { newsFixture(t, nitfCase, "namespace") }

// The profile is chosen by the root nitf in no namespace or in the NITF
// namespace NewsML-G2 embeds; in another namespace the file stays
// generic, as does a fragment of NITF without its root.
func TestNitfSelection(t *testing.T) {
	for _, c := range []struct{ src, want string }{
		{`<nitf><body><body.content><p>x</p></body.content></body></nitf>`, "nitf"},
		{`<nitf xmlns="http://iptc.org/std/NITF/2006-10-18/"><body/></nitf>`, "nitf"},
		{`<n:nitf xmlns:n="http://iptc.org/std/NITF/2006-10-18/"><n:body/></n:nitf>`, "nitf"},
		{`<nitf xmlns="urn:example"><body/></nitf>`, ""},
		{`<nitf xmlns="http://iptc.org/std/NITF/2006-10-18"><body/></nitf>`, ""},
		{`<body><body.content><p>x</p></body.content></body>`, ""},
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
