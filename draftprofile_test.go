package gofastxml

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

// stripReview removes the review marks, as a person finishing a draft
// would once each rule is checked.
func stripReview(b []byte) []byte { return bytes.ReplaceAll(b, []byte(`, "review": true`), nil) }

const smallXSD = `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:doc" xmlns="urn:doc" elementFormDefault="qualified">
  <xs:import namespace="urn:remote" schemaLocation="http://example.org/remote.xsd"/>
  <xs:include schemaLocation="missing.xsd"/>
  <xs:element name="doc"><xs:complexType><xs:sequence>
    <xs:element name="meta" type="xs:string"/>
    <xs:element ref="chapter" maxOccurs="unbounded"/>
  </xs:sequence></xs:complexType></xs:element>
  <xs:element name="chapter"><xs:complexType><xs:sequence>
    <xs:element name="heading" type="Inline"/>
    <xs:choice maxOccurs="unbounded">
      <xs:element ref="para"/><xs:element ref="chapter"/><xs:element ref="bullets"/><xs:element ref="grid"/><xs:element ref="pic"/>
    </xs:choice>
  </xs:sequence><xs:attribute name="id" type="xs:ID"/></xs:complexType></xs:element>
  <xs:element name="para" type="Inline"/>
  <xs:complexType name="Inline" mixed="true"><xs:choice minOccurs="0" maxOccurs="unbounded">
    <xs:element name="emph" type="Inline"/>
    <xs:element name="ref"><xs:complexType mixed="true"><xs:attribute name="to" type="xs:anyURI"/></xs:complexType></xs:element>
  </xs:choice></xs:complexType>
  <xs:element name="bullets"><xs:complexType><xs:sequence>
    <xs:element name="entry" type="Inline" maxOccurs="unbounded"/>
  </xs:sequence></xs:complexType></xs:element>
  <xs:element name="grid"><xs:complexType><xs:sequence>
    <xs:element name="caption" type="xs:string" minOccurs="0"/>
    <xs:element name="r" maxOccurs="unbounded"><xs:complexType><xs:sequence>
      <xs:element name="c" type="Inline" maxOccurs="unbounded"/>
    </xs:sequence></xs:complexType></xs:element>
  </xs:sequence></xs:complexType></xs:element>
  <xs:element name="pic"><xs:complexType><xs:attribute name="src" type="xs:anyURI"/></xs:complexType></xs:element>
  <xs:element name="what"><xs:complexType><xs:attribute name="v"/></xs:complexType></xs:element>
</xs:schema>`

func writeTemp(t *testing.T, name, s string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

type draftJSON struct {
	Profile string `json:"profile"`
	Notes   struct {
		Files     []string          `json:"schema_files"`
		NotRead   []string          `json:"not_read"`
		Undecided []json.RawMessage `json:"undecided"`
	} `json:"notes"`
	Match []map[string]string `json:"match"`
	Rules []map[string]any    `json:"rules"`
}

func draftKinds(t *testing.T, b []byte) (draftJSON, map[string]string) {
	t.Helper()
	var d draftJSON
	if err := json.Unmarshal(b, &d); err != nil {
		t.Fatalf("draft is not JSON: %v\n%s", err, b)
	}
	kinds := map[string]string{}
	for _, r := range d.Rules {
		kinds[r["match"].(string)] = r["as"].(string)
		if r["review"] != true {
			t.Errorf("rule %v has no review mark", r)
		}
	}
	return d, kinds
}

func TestDraftProfileSmall(t *testing.T) {
	path := writeTemp(t, "Small Doc.xsd", smallXSD)
	d, err := DraftProfile(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	dj, kinds := draftKinds(t, d.JSON)
	if testing.Verbose() {
		t.Logf("%s", d.JSON)
	}
	want := map[string]string{
		"doc": "transparent", "chapter": "section", "chapter/heading": "heading", "para": "block",
		"emph": "inline", "ref": "inline", "bullets": "list", "bullets/entry": "item",
		"grid": "table", "grid/r": "row", "r/c": "cell", "pic": "image",
	}
	for sel, as := range want {
		if kinds[sel] != as {
			t.Errorf("%s: drafted %q, want %q", sel, kinds[sel], as)
		}
	}
	if _, ok := kinds["what"]; ok {
		t.Error("an empty element with no URI attribute is undecided")
	}
	if dj.Profile != "draft-small-doc" || len(dj.Match) != 2 || dj.Match[0]["root"] != "doc" || dj.Match[0]["namespace"] != "urn:doc" {
		t.Errorf("profile %q, match %v", dj.Profile, dj.Match)
	}
	if len(dj.Notes.NotRead) != 2 || len(dj.Notes.Undecided) == 0 {
		t.Errorf("notes: %+v", dj.Notes)
	}
	s := string(d.JSON)
	for _, want := range []string{`"anchor": {"attr": "id"}`, `"link": {"attr": "to"}`, `"id": {"attr": "src"}`, `"level": "nesting"`, `"default_namespace": "urn:doc"`} {
		if !strings.Contains(s, want) {
			t.Errorf("draft lacks %s", want)
		}
	}
	codes := map[string]int{}
	for _, w := range d.Warnings {
		codes[w.Code]++
	}
	if codes[gofastcore.WarnInputExternalReference] != 1 {
		t.Errorf("warnings %+v", d.Warnings)
	}
	_, err = LoadProfile(bytes.NewReader(d.JSON))
	if err == nil || !strings.Contains(err.Error(), "draft") {
		t.Errorf("a draft with review marks loads: %v", err)
	}
	p, err := LoadProfile(bytes.NewReader(stripReview(d.JSON)))
	if err != nil {
		t.Fatalf("the stripped draft does not load: %v", err)
	}
	// Read a document with it.
	x := &Extractor{Profiles: []*Profile{p}}
	doc, err := x.Extract(context.Background(), strings.NewReader(""), 0, nil)
	_ = doc
	_ = err
	src := `<doc xmlns="urn:doc"><meta>m</meta><chapter id="c1"><heading>One</heading><para>Hello <emph>you</emph>.</para>` +
		`<bullets><entry>a</entry><entry>b</entry></bullets><grid><r><c>1</c><c>2</c></r></grid><pic src="x.png"/></chapter></doc>`
	got, err := x.Extract(context.Background(), strings.NewReader(src), int64(len(src)), nil)
	if err != nil {
		t.Fatal(err)
	}
	if got.Metadata.Extra[ExtraProfile] != "draft-small-doc" {
		t.Errorf("profile used: %q", got.Metadata.Extra[ExtraProfile])
	}
	if err := got.Validate(); err != nil {
		t.Error(err)
	}
	kindsSeen := map[gofastcore.Kind]int{}
	for _, b := range got.Parts[0].Blocks {
		kindsSeen[b.Kind]++
	}
	if kindsSeen[gofastcore.KindHeading] != 1 || kindsSeen[gofastcore.KindListItem] != 2 || kindsSeen[gofastcore.KindTable] != 1 || kindsSeen[gofastcore.KindImage] != 1 {
		t.Errorf("blocks by kind %v", kindsSeen)
	}
	// Deterministic.
	d2, _ := DraftProfile(context.Background(), path)
	if !bytes.Equal(d.JSON, d2.JSON) {
		t.Error("two drafts of one schema differ")
	}
}

// A local element with two meanings gets one rule per parent.
func TestDraftProfileTwoMeanings(t *testing.T) {
	path := writeTemp(t, "two.xsd", `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root"><xs:complexType><xs:sequence>
    <xs:element name="a"><xs:complexType><xs:sequence><xs:element name="x" type="M" maxOccurs="unbounded"/><xs:element name="y" type="M"/></xs:sequence></xs:complexType></xs:element>
    <xs:element name="b"><xs:complexType><xs:sequence><xs:element name="y" type="M"/><xs:element name="x"><xs:complexType><xs:attribute name="href" type="xs:anyURI"/></xs:complexType></xs:element></xs:sequence></xs:complexType></xs:element>
  </xs:sequence></xs:complexType></xs:element>
  <xs:complexType name="M" mixed="true"/>
</xs:schema>`)
	d, err := DraftProfile(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	_, kinds := draftKinds(t, d.JSON)
	if kinds["a/x"] != "block" || kinds["b/x"] != "image" || kinds["y"] != "block" {
		t.Errorf("kinds %v", kinds)
	}
	if _, err := LoadProfile(bytes.NewReader(stripReview(d.JSON))); err != nil {
		t.Error(err)
	}
}

func TestDraftProfileErrors(t *testing.T) {
	if _, err := DraftProfile(context.Background(), filepath.Join(t.TempDir(), "none.xsd")); err == nil {
		t.Error("a missing file")
	}
	if _, err := DraftProfile(context.Background(), writeTemp(t, "a.xsd", "<doc/>")); err == nil {
		t.Error("not a schema")
	}
}
