package gofastxml

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"testing"

	"github.com/AOShei/go-fast-core"
)

// extractWith reads src with the profile, checks the document and that
// every block can be cited (A6), and returns it.
func extractWith(t *testing.T, prof, src string, opts *gofastcore.Options) *gofastcore.Document {
	t.Helper()
	p, err := LoadProfile(strings.NewReader(prof))
	if err != nil {
		t.Fatal(err)
	}
	doc, err := (&Extractor{Profiles: []*Profile{p}}).Extract(context.Background(), strings.NewReader(src), int64(len(src)), opts)
	if err != nil {
		t.Fatal(err)
	}
	if err := doc.Validate(); err != nil {
		t.Fatalf("Validate: %v\n%s", err, dump(doc))
	}
	if got := doc.Metadata.Extra[ExtraProfile]; got != p.Name() {
		t.Errorf("xml.profile %q, want %q", got, p.Name())
	}
	if bad := citeCheck([]byte(src), doc); bad != "" {
		t.Errorf("A6: %s\n%s", bad, dump(doc))
	}
	return doc
}

func dump(doc *gofastcore.Document) string {
	b, _ := json.MarshalIndent(doc.Parts, "", " ")
	return string(b)
}

// citable reports whether a block's text comes from its place in the
// source: not stored data, not an attribute's value.
func citable(b *gofastcore.Block) bool {
	if b.Attrs[gofastcore.AttrStored] != "" || b.Attrs[AttrAttribute] != "" || b.Kind == gofastcore.KindTable {
		return false
	}
	return b.Kind != gofastcore.KindImage
}

// citeCheck checks A6: the source at a block's offsets, stripped of
// markup, contains the block's text, white space aside.
func citeCheck(src []byte, doc *gofastcore.Document) string {
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
			text := noSpace(gofastcore.ExpandLigatures(b.Text))
			if !strings.Contains(noSpace(stripMarkup(string(src[l.Offset:l.Offset+l.Length]))), text) {
				return "block " + strconv.Itoa(bi) + " text " + strconv.Quote(b.Text) + " not in its span " + strconv.Quote(string(src[l.Offset:l.Offset+l.Length]))
			}
		}
	}
	return ""
}

func blocks(doc *gofastcore.Document) []gofastcore.Block {
	var out []gofastcore.Block
	for _, p := range doc.Parts {
		if p.Kind != gofastcore.PartStored {
			out = append(out, p.Blocks...)
		}
	}
	return out
}

// brief writes blocks as "kind[level]:text" lines.
func brief(bs []gofastcore.Block) string {
	var b strings.Builder
	for _, x := range bs {
		b.WriteString(x.Kind.String())
		if x.Level > 0 {
			b.WriteString(strconv.Itoa(int(x.Level)))
		}
		if x.Marker != "" {
			b.WriteString("(" + x.Marker + ")")
		}
		if x.Origin == gofastcore.OriginInferred {
			b.WriteString("~")
		}
		b.WriteString(":" + x.Text + "\n")
	}
	return b.String()
}

func wantBrief(t *testing.T, doc *gofastcore.Document, want string) {
	t.Helper()
	if got := brief(blocks(doc)); got != want {
		t.Errorf("blocks:\n%s\nwant:\n%s", got, want)
	}
}

func head(name, match string) string {
	return `{"profile": "` + name + `", "format": 1, "match": [` + match + `], `
}

func TestProfileNestingHeadings(t *testing.T) {
	prof := head("nest", `{"root": "doc"}`) + `"heading_base": 2, "rules": [
		{"match": "title", "as": "metadata", "field": "title", "also_heading": 1},
		{"match": "sec", "as": "section", "anchor": {"attr": "id"}},
		{"match": "sec/title", "as": "heading", "level": "nesting"},
		{"match": "p", "as": "block", "kind": "paragraph"},
		{"match": "body", "as": "transparent"}, {"match": "/doc", "as": "transparent"}]}`
	src := `<doc><title>The title</title><body><sec id="s1"><title>One</title><p>First.</p>
<sec><title>One.one</title><p>Deep.</p>stray text<sec><sec><sec><sec><title>Six deep</title></sec></sec></sec></sec></sec></sec></body></doc>`
	doc := extractWith(t, prof, src, nil)
	wantBrief(t, doc, "heading1:The title\nheading2:One\nparagraph:First.\nheading3:One.one\nparagraph:Deep.\nparagraph~:stray text\nheading6:Six deep\n")
	if doc.Metadata.Title != "The title" {
		t.Errorf("title %q", doc.Metadata.Title)
	}
	bs := blocks(doc)
	if bs[2].Attrs[AttrAnchorParent] != "s1" || bs[4].Attrs[AttrAnchorParent] != "s1" {
		t.Errorf("anchor_parent: %v %v", bs[2].Attrs, bs[4].Attrs)
	}
	if bs[6].Attrs["level"] != "7" || !doc.Parts[0].Has(gofastcore.WarnHeadingLevelClamped) {
		t.Errorf("clamped: %v %v", bs[6].Attrs, doc.Parts[0].Warnings)
	}
	if len(doc.Parts[0].Warnings) != 1 {
		t.Errorf("warnings %v", doc.Parts[0].Warnings)
	}
}

func TestProfileAttributeDispatch(t *testing.T) {
	prof := head("dispatch", `{"root": "topic", "attr": "[@class~='topic/topic']"}`) + `"rules": [
		{"match": "*[@class~='topic/p']", "as": "block", "kind": "paragraph"},
		{"match": "*[@class~='topic/title']", "as": "heading", "level": 1},
		{"match": "hi[@rend='italic']", "as": "inline", "style": "italic"},
		{"match": "hi", "as": "inline", "style": "bold"},
		{"match": "list[@list-type='order']", "as": "list", "ordered": true},
		{"match": "list", "as": "list", "ordered": {"attr": "type", "in": ["numero"]}},
		{"match": "item", "as": "item"},
		{"match": "*[@class~='topic/body']", "as": "transparent"},
		{"match": "*[@class~='topic/topic']", "as": "transparent"}]}`
	src := `<topic class="- topic/topic "><title class="- topic/title ">T</title><body class="- topic/body ">` +
		`<p class="- topic/p ">An <hi rend="italic">it</hi> and <hi>bold</hi>.</p>` +
		`<list list-type="order"><item><p class="- topic/p ">one</p></item><item>two<list type="puce"><item>a</item></list></item></list>` +
		`<list type="numero"><item>x</item></list></body></topic>`
	doc := extractWith(t, prof, src, &gofastcore.Options{Spans: true})
	wantBrief(t, doc, "heading1:T\nparagraph:An it and bold.\nlist_item1(1.):one\nlist_item1(2.):two\nlist_item2:a\nlist_item1(1.):x\n")
	p := blocks(doc)[1]
	if len(p.Spans) != 2 || p.Spans[0] != (gofastcore.Span{Start: 3, End: 5, Style: gofastcore.StyleItalic}) ||
		p.Spans[1] != (gofastcore.Span{Start: 10, End: 14, Style: gofastcore.StyleBold}) {
		t.Errorf("spans %+v", p.Spans)
	}
	if !blocks(doc)[2].Ordered || blocks(doc)[4].Ordered {
		t.Error("ordered")
	}
	// The other root does not match: generic reading.
	other := strings.Replace(src, `class="- topic/topic "`, `class="- map/map "`, 1)
	d2, err := (&Extractor{Profiles: []*Profile{mustLoad(t, prof)}}).Extract(context.Background(), strings.NewReader(other), int64(len(other)), nil)
	if err != nil || d2.Metadata.Extra[ExtraProfile] != "" {
		t.Errorf("profile used for a root it does not match: %v %v", err, d2.Metadata.Extra)
	}
}

func mustLoad(t *testing.T, s string) *Profile {
	t.Helper()
	p, err := LoadProfile(strings.NewReader(s))
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestProfileCALSTable(t *testing.T) {
	prof := head("cals", `{"root": "doc"}`) + `"rules": [
		{"match": "table", "as": "table", "model": "cals"},
		{"match": "table/title", "as": "block", "kind": "caption"},
		{"match": "para", "as": "block", "kind": "paragraph"}]}`
	src := `<doc><table><title>Cap</title><tgroup cols="3"><colspec colname="c1"/><colspec colname="c2"/><colspec colname="c3"/>
<spanspec spanname="all" namest="c1" nameend="c3"/>
<thead><row><entry namest="c1" nameend="c2">AB</entry><entry>C</entry></row></thead>
<tbody><row><entry morerows="1">x</entry><entry><para>p1</para><para>p2</para></entry><entry>z</entry></row>
<row><entry colname="c2">y2</entry><entry>z2</entry></row>
<row><entry spanname="all">wide</entry></row></tbody></tgroup></table></doc>`
	doc := extractWith(t, prof, src, nil)
	bs := blocks(doc)
	if len(bs) != 2 || bs[0].Kind != gofastcore.KindCaption || bs[1].Table == nil {
		t.Fatalf("blocks:\n%s", brief(bs))
	}
	tb := bs[1].Table
	want := [][]string{{"AB", "", "C"}, {"x", "p1\np2", "z"}, {"", "y2", "z2"}, {"wide", "", ""}}
	if got, _ := json.Marshal(tb.Rows); string(got) != mustJSON(want) {
		t.Errorf("rows %s", got)
	}
	if tb.HeaderRows != 1 {
		t.Errorf("header rows %d", tb.HeaderRows)
	}
	wantM := []gofastcore.Merge{{Row: 0, Col: 0, RowSpan: 1, ColSpan: 2}, {Row: 1, Col: 0, RowSpan: 2, ColSpan: 1}, {Row: 3, Col: 0, RowSpan: 1, ColSpan: 3}}
	if got, _ := json.Marshal(tb.Merges); string(got) != mustJSON(wantM) {
		t.Errorf("merges %s", got)
	}
}

func mustJSON(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func TestProfileRowsTable(t *testing.T) {
	prof := head("rows", `{"root": "doc"}`) + `"rules": [
		{"match": "Tableau", "as": "table"},
		{"match": "Tableau/TitreRiche", "as": "block", "kind": "caption"},
		{"match": "Rangee", "as": "row", "header": {"attr": "type", "in": ["header"]}},
		{"match": "Cellule", "as": "cell", "colspan": "fusionHorizontale", "rowspan": "fusionVerticale"},
		{"match": "Paragraphe", "as": "block", "kind": "paragraph"},
		{"match": "Liste", "as": "list", "ordered": {"attr": "type", "in": ["numero"]}},
		{"match": "Item", "as": "item"},
		{"match": "Lien", "as": "inline", "link": {"attr": "URL"}}]}`
	src := `<doc><Tableau><TitreRiche><Paragraphe>Caption</Paragraphe></TitreRiche>
<Rangee type="header"><Cellule fusionHorizontale="2"><Paragraphe>H</Paragraphe></Cellule></Rangee>
<Rangee type="normal"><Cellule fusionVerticale="2"><Paragraphe>a</Paragraphe></Cellule><Cellule><Paragraphe>b <Lien URL="http://x">link</Lien></Paragraphe><Paragraphe>b2</Paragraphe></Cellule></Rangee>
<Rangee type="normal"><Cellule><Liste type="numero"><Item><Paragraphe>i1</Paragraphe></Item><Item><Paragraphe>i2</Paragraphe></Item></Liste></Cellule></Rangee>
</Tableau></doc>`
	doc := extractWith(t, prof, src, &gofastcore.Options{Spans: true})
	bs := blocks(doc)
	if len(bs) != 2 || bs[0].Text != "Caption" {
		t.Fatalf("blocks:\n%s", brief(bs))
	}
	tb := bs[1].Table
	if got := mustJSON(tb.Rows); got != mustJSON([][]string{{"H", ""}, {"a", "b link\nb2"}, {"", "1. i1\n2. i2"}}) {
		t.Errorf("rows %s", got)
	}
	if tb.HeaderRows != 1 || len(tb.Merges) != 2 || len(tb.Cells) != 1 || tb.Cells[0].Href != "http://x" {
		t.Errorf("table %+v", tb)
	}
}

func TestProfileLabels(t *testing.T) {
	prof := head("labels", `{"root": "act"}`) + `"rules": [
		{"match": "section", "as": "section"},
		{"match": "section/num", "as": "label"},
		{"match": "section/heading", "as": "heading", "level": "nesting"},
		{"match": "paragraph", "as": "block", "kind": "paragraph"},
		{"match": "paragraph/num", "as": "label"},
		{"match": "box", "as": "transparent", "mark": {"sp.box": "box"}},
		{"match": "box/title", "as": "label"},
		{"match": "p", "as": "block", "kind": "paragraph"}]}`
	src := `<act><section><num>4.2</num><heading>Scope</heading><paragraph><num>(a)</num> Text a.</paragraph></section>` +
		`<box><title>Note</title><p>Boxed.</p><p>Second.</p></box><box><title>Alone</title></box></act>`
	doc := extractWith(t, prof, src, nil)
	wantBrief(t, doc, "heading1(4.2):Scope\nparagraph((a)):Text a.\nparagraph(Note):Boxed.\nparagraph:Second.\nparagraph:Alone\n")
	if bs := blocks(doc); bs[2].Attrs["sp.box"] != "box" || bs[3].Attrs["sp.box"] != "box" {
		t.Errorf("mark: %v", bs[2].Attrs)
	}
}

func TestProfileFootnotes(t *testing.T) {
	prof := head("fn", `{"root": "article"}`) + `"rules": [
		{"match": "p", "as": "block", "kind": "paragraph"},
		{"match": "xref", "as": "inline", "style": "superscript", "link": {"attr": "rid"}},
		{"match": "fn-group", "as": "transparent"},
		{"match": "fn", "as": "block", "kind": "footnote", "anchor": {"attr": "id"}},
		{"match": "fn/label", "as": "label"}]}`
	src := `<article><p>Claim<xref rid="fn1">1</xref>.</p><fn-group><fn id="fn1"><label>1</label><p>The note.</p></fn></fn-group></article>`
	doc := extractWith(t, prof, src, &gofastcore.Options{Spans: true})
	wantBrief(t, doc, "paragraph:Claim1.\nfootnote(1):The note.\n")
	bs := blocks(doc)
	if bs[1].Loc.Anchor != "fn1" || len(bs[0].Spans) != 1 || bs[0].Spans[0].Href != "fn1" {
		t.Errorf("fn %+v %+v", bs[1].Loc, bs[0].Spans)
	}
}

func TestProfileMarksAndValues(t *testing.T) {
	prof := head("marks", `{"root": "Publication"}`) + `"heading_base": 2, "rules": [
		{"match": "BlocCas", "as": "transparent", "mark": {"sp.case_group": {"attr": "affichage"}}},
		{"match": "Cas", "as": "section", "mark": {"sp.condition": {"structure": "Condition"}}},
		{"match": "Cas/Titre", "as": "heading", "level": "nesting", "duplicate_of": "TitreRiche", "key": "sp.title_plain"},
		{"match": "Cas/TitreRiche", "as": "heading", "level": "nesting"},
		{"match": "Paragraphe", "as": "block", "kind": "paragraph"},
		{"match": "Exposant", "as": "inline", "style": "superscript"},
		{"match": "Reference", "as": "item", "anchor": {"attr": "ID"}, "attrs": {"sp.group": "Reference", "sp.source": {"child": "Source"}, "sp.url": {"attr": "URL"}}},
		{"match": "Reference/Titre", "as": "inline", "link": {"attr": "URL", "of": "Reference"}},
		{"match": "Reference/Complement", "as": "block", "kind": "paragraph"}]}`
	src := `<Publication><BlocCas affichage="radio"><Cas><Titre>Same</Titre><TitreRiche><Paragraphe>Same</Paragraphe></TitreRiche>` +
		`<Condition><ou><estVrai var="T1"/><estIndefini var="T2"/></ou></Condition><Paragraphe>In case.</Paragraphe></Cas>` +
		`<Cas><Titre>1er</Titre><TitreRiche><Paragraphe>1<Exposant>er</Exposant> cas</Paragraphe></TitreRiche><Paragraphe>Other.</Paragraphe></Cas></BlocCas>` +
		`<Reference ID="R1" URL="http://r"><Titre>Code</Titre><Complement>articles 1 to 3</Complement><Source>Legifrance</Source></Reference></Publication>`
	doc := extractWith(t, prof, src, nil)
	wantBrief(t, doc, "heading2:Same\nparagraph:In case.\nheading2:1er cas\nparagraph:Other.\nlist_item1:Code\nparagraph:articles 1 to 3\n")
	bs := blocks(doc)
	for _, i := range []int{0, 1} {
		if bs[i].Attrs["sp.condition"] != "ou(estVrai(var=T1), estIndefini(var=T2))" || bs[i].Attrs["sp.case_group"] != "radio" {
			t.Errorf("block %d attrs %v", i, bs[i].Attrs)
		}
	}
	if _, ok := bs[0].Attrs["sp.title_plain"]; ok {
		t.Error("equal titles kept twice")
	}
	if bs[2].Attrs["sp.title_plain"] != "1er" || bs[2].Attrs["sp.condition"] != "" {
		t.Errorf("block 2 attrs %v", bs[2].Attrs)
	}
	ref := bs[4]
	if ref.Loc.Anchor != "R1" || ref.Attrs["sp.source"] != "Legifrance" || ref.Attrs["sp.url"] != "http://r" || ref.Attrs["sp.group"] != "Reference" {
		t.Errorf("reference %+v", ref)
	}
	if bs[5].Attrs[AttrAnchorParent] != "R1" {
		t.Errorf("complement %v", bs[5].Attrs)
	}
}

func TestProfileParts(t *testing.T) {
	prof := head("corpus", `{"root": "teiCorpus"}`) + `"rules": [
		{"match": "TEI", "as": "part", "kind": "chapter", "name": {"child": "teiHeader/title"}},
		{"match": "p", "as": "block", "kind": "paragraph"},
		{"match": "text", "as": "transparent"}]}`
	src := `<teiCorpus><TEI><teiHeader><title>First</title></teiHeader><text><p>one</p></text></TEI>` +
		`<TEI><teiHeader><title>Second</title></teiHeader><text><p>two</p></text></TEI></teiCorpus>`
	doc := extractWith(t, prof, src, nil)
	if len(doc.Parts) != 2 || doc.Parts[0].Name != "First" || doc.Parts[1].Name != "Second" || doc.Parts[1].Number != 2 ||
		doc.Parts[0].Kind != gofastcore.PartChapter || doc.Parts[1].Blocks[0].Text != "two" {
		t.Fatalf("parts %s", dump(doc))
	}
	// teiHeader has no rule: read generically and counted.
	if !doc.Parts[0].Has(WarnUnmappedElements) {
		t.Errorf("warnings %v", doc.Parts[0].Warnings)
	}
	d2, err := (&Extractor{Profiles: []*Profile{mustLoad(t, prof)}}).Extract(context.Background(), strings.NewReader(src), int64(len(src)),
		&gofastcore.Options{Parts: gofastcore.PartRange{First: 2}})
	if err != nil || len(d2.Parts) != 1 || d2.Parts[0].Number != 2 {
		t.Errorf("part range: %v %v", err, d2)
	}
}

func TestProfileMetadataAndUnmapped(t *testing.T) {
	prof := head("meta", `{"root": "Publication"}`) + `"namespaces": {"dc": "http://purl.org/dc/elements/1.1/"}, "rules": [
		{"match": "/Publication", "as": "transparent", "extra": {"sp.id": {"attr": "ID"}}},
		{"match": "dc:creator", "as": "metadata", "field": "author"},
		{"match": "dc:date", "as": "metadata", "field": "extra", "key": "dc:date"},
		{"match": "dc:subject", "as": "metadata", "field": "keyword"},
		{"match": "Niveau", "as": "metadata", "field": "extra", "key": "sp.breadcrumb", "join": " > ", "extra": {"sp.breadcrumb_ids": {"attr": "ID"}}},
		{"match": "FilDAriane", "as": "transparent"},
		{"match": "Paragraphe", "as": "block", "kind": "paragraph"}]}`
	src := `<Publication xmlns:dc="http://purl.org/dc/elements/1.1/" ID="F1" lost="x"><dc:creator>DILA</dc:creator><dc:date>modified 2026-11-03</dc:date>` +
		`<dc:subject>Famille</dc:subject><FilDAriane><Niveau ID="A">Accueil</Niveau><Niveau ID="B">Famille</Niveau></FilDAriane>` +
		`<Inconnu code="7">Unknown text</Inconnu><Paragraphe>P</Paragraphe></Publication>`
	doc := extractWith(t, prof, src, nil)
	m := doc.Metadata
	if m.Extra["sp.id"] != "F1" || m.Extra["dc:date"] != "modified 2026-11-03" || !m.Modified.IsZero() ||
		m.Extra["sp.breadcrumb"] != "Accueil > Famille" || m.Extra["sp.breadcrumb_ids"] != "A > B" ||
		len(m.Authors) != 1 || m.Authors[0] != "DILA" || len(m.Keywords) != 1 {
		t.Errorf("metadata %+v", m)
	}
	wantBrief(t, doc, "note~:7\nparagraph~:Unknown text\nparagraph:P\n")
	var codes []string
	for _, w := range doc.Parts[0].Warnings {
		codes = append(codes, w.Code+": "+w.Message)
	}
	got := strings.Join(codes, "\n")
	if !strings.Contains(got, "xml.unmapped_elements: elements with no rule in the profile, read generically: 1 in all: Inconnu (1)") ||
		!strings.Contains(got, "Publication@lost (1)") || !strings.Contains(got, "Inconnu@code (1)") {
		t.Errorf("warnings:\n%s", got)
	}
}

func TestProfileLinksImagesValues(t *testing.T) {
	prof := head("links", `{"root": "doc"}`) + `"rules": [
		{"match": "p", "as": "block", "kind": "paragraph"},
		{"match": "LienInterne", "as": "inline", "link": {"attr": "LienPublication"}, "title": {"attr": "commentaireLien"}, "attrs": {"sp.type": {"attr": "type"}}},
		{"match": "Image", "as": "image", "id": {"attr": "LienPublication"}, "alt": {"child": "TexteDeRemplacement"}},
		{"match": "Legende", "as": "block", "kind": "caption"},
		{"match": "RessourceWeb", "as": "block", "kind": "paragraph", "value": {"attr": "URL"}},
		{"match": "hidden", "as": "transparent", "display": {"attr": "h", "in": ["yes"], "reason": "hidden"}},
		{"match": "data", "as": "stored", "kind": "custom_xml"}, {"match": "/doc", "as": "transparent"}]}`
	src := `<doc><p>See <LienInterne LienPublication="F1" type="Fiche" commentaireLien="Title of F1">this</LienInterne>.</p>` +
		`<Image LienPublication="I1"><Legende>Cap</Legende><TexteDeRemplacement>Alt text</TexteDeRemplacement></Image>` +
		`<RessourceWeb URL="https://x.example"/><hidden h="yes"><p>Hid</p></hidden><data>raw</data></doc>`
	doc := extractWith(t, prof, src, &gofastcore.Options{Spans: true})
	wantBrief(t, doc, "paragraph:See this.\nimage:Alt text\ncaption:Cap\nparagraph:https://x.example\nparagraph:Hid\n")
	bs := blocks(doc)
	if bs[0].Spans[0].Href != "F1" || bs[1].Image.ID != "I1" || bs[3].Attrs[AttrAttribute] != "URL" || bs[4].Display != gofastcore.DisplayHidden {
		t.Errorf("blocks %s", dump(doc))
	}
	st := doc.Parts[len(doc.Parts)-1]
	if st.Kind != gofastcore.PartStored || len(st.Blocks) != 2 {
		t.Fatalf("stored part %s", dump(doc))
	}
	rec := st.Blocks[0]
	if rec.Text != "this" || rec.Attrs["title"] != "Title of F1" || rec.Attrs["href"] != "F1" || rec.Attrs["sp.type"] != "Fiche" || rec.Attrs[gofastcore.AttrNoteType] != "" ||
		rec.Attrs[gofastcore.AttrStored] != gofastcore.StoredCustomProperty || st.Blocks[1].Text != "raw" || st.Blocks[1].Attrs["stored"] != "custom_xml" {
		t.Errorf("stored %s", dump(doc))
	}
	if doc.Parts[0].Warnings != nil {
		t.Errorf("warnings %v", doc.Parts[0].Warnings)
	}
}

func TestProfileForced(t *testing.T) {
	prof := mustLoad(t, head("forced", `{"root": "nomatch"}`)+`"rules": [{"match": "p", "as": "heading", "level": 2}]}`)
	src := `<doc><p>x</p></doc>`
	doc, err := (&Extractor{Profiles: []*Profile{prof}, Profile: "forced"}).Extract(context.Background(), strings.NewReader(src), int64(len(src)), nil)
	if err != nil || doc.Parts[0].Blocks[0].Kind != gofastcore.KindHeading {
		t.Fatalf("%v %s", err, dump(doc))
	}
	if _, err := (&Extractor{Profile: "nosuch"}).Extract(context.Background(), strings.NewReader(src), int64(len(src)), nil); err == nil {
		t.Error("unknown profile name accepted")
	}
	doc, err = (&Extractor{Profiles: []*Profile{prof}, Profile: ProfileNone}).Extract(context.Background(), strings.NewReader(src), int64(len(src)), nil)
	if err != nil || doc.Metadata.Extra[ExtraProfile] != "" {
		t.Errorf("none: %v %v", err, doc.Metadata.Extra)
	}
}
