package gofastxml

import (
	"encoding/xml"
	"fmt"
	"math/rand/v2"
	"strings"
	"testing"
	"unicode"

	"github.com/AOShei/go-fast-core"
)

// noSpace removes all white space, the normalisation both sides of the
// oracle comparison get.
func noSpace(s string) string {
	return strings.Map(func(r rune) rune {
		// Controls are removed from text by core's convention
		// (text.control_characters_removed): C1 controls stand for
		// windows-1252 characters in some DILA decisions.
		if unicode.IsSpace(r) || unicode.IsControl(r) {
			return -1
		}
		return r
	}, s)
}

// oracleText is the character data encoding/xml finds inside the root.
func oracleText(t *testing.T, src string) string {
	d := xml.NewDecoder(strings.NewReader(src))
	var b strings.Builder
	depth := 0
	for {
		tok, err := d.Token()
		if err != nil {
			break
		}
		switch v := tok.(type) {
		case xml.StartElement:
			depth++
		case xml.EndElement:
			depth--
		case xml.CharData:
			if depth > 0 {
				b.Write(v)
			}
		}
	}
	return noSpace(b.String())
}

func ourText(t *testing.T, src string) string {
	doc := extract(t, src, &Extractor{Attributes: AttributesNone})
	var b strings.Builder
	for _, bl := range doc.Parts[0].Blocks {
		if bl.Kind == gofastcore.KindParagraph {
			b.WriteString(bl.Text)
		}
	}
	return noSpace(b.String())
}

var words = []string{"alpha", "bêta", "γάμμα", "&amp;", "&lt;tag&gt;", "&#x263A;", "&#233;", "&quot;q&apos;", "中文", "x"}

// genXML writes a random well-formed element.
func genXML(r *rand.Rand, b *strings.Builder, depth int) {
	name := fmt.Sprintf("e%d", r.IntN(5))
	if r.IntN(4) == 0 {
		name = "p:" + name
	}
	b.WriteString("<" + name)
	for i := range r.IntN(3) {
		fmt.Fprintf(b, ` a%d="%s"`, i, words[r.IntN(len(words))])
	}
	if depth > 6 || r.IntN(6) == 0 {
		b.WriteString("/>")
		return
	}
	b.WriteString(">")
	for range r.IntN(5) {
		switch r.IntN(6) {
		case 0, 1:
			genXML(r, b, depth+1)
		case 2:
			b.WriteString(words[r.IntN(len(words))])
		case 3:
			b.WriteString([]string{" ", "\n  ", "\t", ""}[r.IntN(4)])
		case 4:
			b.WriteString("<![CDATA[" + []string{"<raw> & ", "x]y", ""}[r.IntN(3)] + "]]>")
		case 5:
			b.WriteString("<!-- c -->")
		}
	}
	b.WriteString("</" + name + ">")
}

func TestOracle(t *testing.T) {
	fixtures := []string{
		`<doc><p>Hello <b>world</b></p><p>Two</p></doc>`,
		`<doc xmlns:p="urn:p"><p:a>x<p:b>y</p:b>z</p:a><c><d>e</d></c></doc>`,
		"<doc>\n  <a>1</a>\n  <b>2 <i>3</i></b>\n</doc>",
	}
	r := rand.New(rand.NewPCG(1, 2))
	for range 500 {
		var b strings.Builder
		b.WriteString(`<root xmlns:p="urn:p">`)
		genXML(r, &b, 0)
		b.WriteString("</root>")
		fixtures = append(fixtures, b.String())
	}
	for _, src := range fixtures {
		if want, got := oracleText(t, src), ourText(t, src); got != want {
			t.Fatalf("%s\nencoding/xml: %q\ngofastxml:    %q", src, want, got)
		}
	}
}
