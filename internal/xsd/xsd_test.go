package xsd

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const head = `<?xml version="1.0"?><xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" `

func load(t *testing.T, src string) (*Set, *Model) {
	t.Helper()
	s, err := LoadBytes(context.Background(), "t.xsd", []byte(src), Limits{})
	if err != nil {
		t.Fatal(err)
	}
	return s, s.Analyze()
}

func find(m *Model, local string) *Decl {
	for _, d := range m.Decls {
		if d.Name.Local == local {
			return d
		}
	}
	return nil
}

func childNames(d *Decl) string {
	var b []string
	for _, c := range d.Children {
		s := c.Decl.Name.Local
		if c.Repeats {
			s += "*"
		}
		b = append(b, s)
	}
	return strings.Join(b, " ")
}

func TestConstructs(t *testing.T) {
	_, m := load(t, head+`xmlns:t="urn:t" targetNamespace="urn:t" elementFormDefault="qualified">
  <xs:element name="doc">
    <xs:complexType><xs:sequence>
      <xs:element name="title" type="xs:string"/>
      <xs:group ref="t:body" maxOccurs="unbounded"/>
      <xs:any namespace="##other" processContents="lax" minOccurs="0"/>
    </xs:sequence>
    <xs:attributeGroup ref="t:common"/>
    <xs:attribute ref="xml:lang"/>
    </xs:complexType>
  </xs:element>
  <xs:group name="body"><xs:choice>
    <xs:element ref="t:p"/>
    <xs:element ref="t:section"/>
    <xs:element ref="t:block"/>
  </xs:choice></xs:group>
  <xs:attributeGroup name="common">
    <xs:attribute name="id" type="xs:ID"/>
    <xs:attribute name="ref" type="t:Ref"/>
  </xs:attributeGroup>
  <xs:simpleType name="Ref"><xs:restriction base="xs:IDREF"/></xs:simpleType>
  <xs:element name="p" type="t:Inline"/>
  <xs:complexType name="Inline" mixed="true">
    <xs:choice minOccurs="0" maxOccurs="unbounded"><xs:element name="b" type="t:Inline"/><xs:element ref="t:a"/></xs:choice>
  </xs:complexType>
  <xs:element name="a"><xs:complexType mixed="true"><xs:attribute name="href" type="xs:anyURI"/></xs:complexType></xs:element>
  <xs:element name="section" type="t:Section"/>
  <xs:complexType name="Section"><xs:sequence>
    <xs:element name="title" type="xs:string"/>
    <xs:group ref="t:body" maxOccurs="unbounded"/>
  </xs:sequence></xs:complexType>
  <xs:element name="block" abstract="true"/>
  <xs:element name="note" substitutionGroup="t:block" type="t:Inline"/>
  <xs:element name="img"><xs:complexType><xs:attribute name="src" type="xs:anyURI" use="required"/></xs:complexType></xs:element>
  <xs:element name="big" type="t:Big"/>
  <xs:complexType name="Big"><xs:complexContent><xs:extension base="t:Section">
    <xs:sequence><xs:element ref="t:img"/></xs:sequence>
    <xs:attribute name="kind"/>
  </xs:extension></xs:complexContent></xs:complexType>
  <xs:element name="small"><xs:complexType><xs:complexContent><xs:restriction base="t:Section">
    <xs:sequence><xs:element name="title" type="xs:string"/></xs:sequence>
  </xs:restriction></xs:complexContent></xs:complexType></xs:element>
  <xs:element name="price"><xs:complexType><xs:simpleContent><xs:extension base="xs:decimal">
    <xs:attribute name="currency" type="xs:string"/>
  </xs:extension></xs:simpleContent></xs:complexType></xs:element>
  <xs:element name="local" type="xs:string" form="unqualified"/>
</xs:schema>`)
	doc := find(m, "doc")
	if doc == nil || doc.Name.URI != "urn:t" || doc.Content != Elements || !doc.Any {
		t.Fatalf("doc: %+v", doc)
	}
	if got := childNames(doc); got != "title p* section* note*" {
		t.Errorf("doc children %q (the abstract head is replaced by its substitution group)", got)
	}
	var types []string
	for _, a := range doc.Attrs {
		types = append(types, a.Name.Local+":"+a.Type.String())
	}
	if got := strings.Join(types, " "); got != "id:ID ref:IDREF lang:language" {
		t.Errorf("doc attributes %q", got)
	}
	if doc.Children[0].Decl.Name.URI != "urn:t" {
		t.Error("elementFormDefault qualified: a local element is in the target namespace")
	}
	if p := find(m, "p"); p.Content != Mixed || childNames(p) != "b* a*" {
		t.Errorf("p: %s %q", p.Content, childNames(p))
	}
	if b := find(m, "b"); !b.Nests || len(b.Parents) != 3 {
		t.Errorf("b nests in itself and has parents p, b, note: %v %d", b.Nests, len(b.Parents))
	}
	if s := find(m, "section"); !s.Nests || s.Content != Elements {
		t.Errorf("section: %+v", s)
	}
	if a := find(m, "a"); a.Content != Mixed || len(a.Attrs) != 1 || a.Attrs[0].Type != AnyURI {
		t.Errorf("a: %s", a.Describe())
	}
	if img := find(m, "img"); img.Content != Empty || img.Attrs[0].Type != AnyURI {
		t.Errorf("img: %s", img.Describe())
	}
	if big := find(m, "big"); childNames(big) != "title p* section* note* img" || len(big.Attrs) != 1 {
		t.Errorf("big (extension adds to the base): %q %d", childNames(big), len(big.Attrs))
	}
	if small := find(m, "small"); childNames(small) != "title" {
		t.Errorf("small (restriction restates the content): %q", childNames(small))
	}
	if pr := find(m, "price"); pr.Content != Text || len(pr.Attrs) != 1 {
		t.Errorf("price: %s", pr.Describe())
	}
	if l := find(m, "local"); l == nil || l.Name.URI != "urn:t" {
		t.Errorf("a global element is in the target namespace whatever its form: %v", l)
	}
	var roots []string
	for _, r := range m.Roots {
		roots = append(roots, r.Name.Local)
	}
	if got := strings.Join(roots, " "); got != "doc img big small price local" && got != "doc big small price local" {
		t.Errorf("roots %q", got)
	}
}

func TestUnqualifiedAndChameleon(t *testing.T) {
	dir := t.TempDir()
	write := func(name, s string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("main.xsd", head+`targetNamespace="urn:m" xmlns="urn:m">
  <xs:include schemaLocation="part.xsd"/>
  <xs:include schemaLocation="gone.xsd"/>
  <xs:import namespace="urn:x" schemaLocation="https://example.org/x.xsd"/>
  <xs:import namespace="urn:y"/>
  <xs:import namespace="urn:z" schemaLocation="/etc/z.xsd"/>
  <xs:element name="root"><xs:complexType><xs:sequence>
    <xs:element name="kid" type="Kid"/><xs:element ref="other"/>
  </xs:sequence></xs:complexType></xs:element>
</xs:schema>`)
	write("part.xsd", head+`>
  <xs:complexType name="Kid"><xs:sequence><xs:element ref="leaf"/></xs:sequence></xs:complexType>
  <xs:element name="leaf" type="xs:string"/>
  <xs:element name="other" type="Missing"/>
</xs:schema>`)
	s, err := Load(context.Background(), filepath.Join(dir, "main.xsd"), Limits{})
	if err != nil {
		t.Fatal(err)
	}
	m := s.Analyze()
	if len(s.Docs) != 2 {
		t.Errorf("%d documents", len(s.Docs))
	}
	kid := find(m, "kid")
	if kid == nil || kid.Name.URI != "" {
		t.Fatalf("kid is unqualified (no elementFormDefault): %+v", kid)
	}
	if leaf := find(m, "leaf"); leaf == nil || leaf.Name.URI != "urn:m" || len(leaf.Parents) != 1 {
		t.Errorf("leaf, from a chameleon include, is in urn:m: %+v", leaf)
	}
	if o := find(m, "other"); o.Content != Unknown {
		t.Errorf("other's type is missing: %s", o.Content)
	}
	kinds := map[ProblemKind]int{}
	for _, p := range s.Problems {
		kinds[p.Kind]++
	}
	if kinds[NotFound] != 1 || kinds[NotFetched] != 2 || kinds[NoLocation] != 1 {
		t.Errorf("problems %+v", s.Problems)
	}
}

func TestRedefineAndCycles(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.xsd"), []byte(head+`>
  <xs:redefine schemaLocation="b.xsd">
    <xs:complexType name="T"><xs:complexContent><xs:extension base="T">
      <xs:sequence><xs:element name="extra" type="xs:string"/></xs:sequence>
    </xs:extension></xs:complexContent></xs:complexType>
  </xs:redefine>
  <xs:include schemaLocation="a.xsd"/>
  <xs:element name="r" type="T"/>
  <xs:group name="g1"><xs:sequence><xs:group ref="g2"/><xs:element name="x" type="xs:string"/></xs:sequence></xs:group>
  <xs:group name="g2"><xs:sequence><xs:group ref="g1"/></xs:sequence></xs:group>
  <xs:element name="cyc"><xs:complexType><xs:group ref="g1"/></xs:complexType></xs:element>
  <xs:complexType name="L1"><xs:complexContent><xs:extension base="L2"/></xs:complexContent></xs:complexType>
  <xs:complexType name="L2"><xs:complexContent><xs:extension base="L1"/></xs:complexContent></xs:complexType>
  <xs:element name="loop" type="L1"/>
</xs:schema>`), 0o644)
	os.WriteFile(filepath.Join(dir, "b.xsd"), []byte(head+`>
  <xs:complexType name="T"><xs:sequence><xs:element name="base" type="xs:string"/></xs:sequence></xs:complexType>
</xs:schema>`), 0o644)
	s, err := Load(context.Background(), filepath.Join(dir, "a.xsd"), Limits{})
	if err != nil {
		t.Fatal(err)
	}
	m := s.Analyze()
	if got := childNames(find(m, "r")); got != "base extra" {
		t.Errorf("redefined T: %q", got)
	}
	if got := childNames(find(m, "cyc")); got != "x" {
		t.Errorf("group cycle: %q", got)
	}
	if find(m, "loop") == nil {
		t.Error("derivation cycle")
	}
}

func TestLimits(t *testing.T) {
	dir := t.TempDir()
	// Each file includes the next: a chain longer than MaxDepth.
	for i := 0; i < 10; i++ {
		os.WriteFile(filepath.Join(dir, "f"+string(rune('0'+i))+".xsd"), []byte(head+`><xs:include schemaLocation="f`+string(rune('1'+i))+`.xsd"/><xs:element name="e`+string(rune('0'+i))+`"/></xs:schema>`), 0o644)
	}
	s, err := Load(context.Background(), filepath.Join(dir, "f0.xsd"), Limits{MaxDepth: 3})
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Docs) != 4 || len(s.Problems) != 1 || s.Problems[0].Kind != LimitCut {
		t.Errorf("%d docs, problems %+v", len(s.Docs), s.Problems)
	}
	s, _ = Load(context.Background(), filepath.Join(dir, "f0.xsd"), Limits{MaxFiles: 2})
	if len(s.Docs) != 2 || s.Problems[0].Kind != LimitCut {
		t.Errorf("MaxFiles: %d docs, %+v", len(s.Docs), s.Problems)
	}
	big := head + `>` + strings.Repeat(`<xs:element name="e"/>`, 100) + `</xs:schema>`
	s, err = LoadBytes(context.Background(), "b.xsd", []byte(big), Limits{MaxNodes: 10})
	if err != nil || len(s.Problems) != 1 || s.Problems[0].Kind != LimitCut {
		t.Errorf("MaxNodes: %v %+v", err, s.Problems)
	}
	if _, err = LoadBytes(context.Background(), "b.xsd", []byte(big), Limits{MaxBytes: 100}); err == nil {
		t.Error("MaxBytes on the entry document is an error")
	}
	if _, err := LoadBytes(context.Background(), "n.xsd", []byte(`<doc/>`), Limits{}); err == nil {
		t.Error("not a schema")
	}
}

// A schema whose types refer to each other in a long chain and a wide
// fan-out is read quickly.
func TestWideSchema(t *testing.T) {
	var b strings.Builder
	b.WriteString(head + `>`)
	for i := 0; i < 2000; i++ {
		n := strings.Repeat("x", i%7+1) + itoa(i)
		b.WriteString(`<xs:element name="` + n + `"><xs:complexType><xs:choice maxOccurs="unbounded"><xs:group ref="all"/></xs:choice></xs:complexType></xs:element>`)
	}
	b.WriteString(`<xs:group name="all"><xs:choice>`)
	for i := 0; i < 2000; i++ {
		b.WriteString(`<xs:element ref="` + strings.Repeat("x", i%7+1) + itoa(i) + `"/>`)
	}
	b.WriteString(`</xs:choice></xs:group></xs:schema>`)
	start := time.Now()
	_, m := load(t, b.String())
	if len(m.Decls) != 2000 || !m.Decls[0].Nests {
		t.Errorf("%d decls", len(m.Decls))
	}
	if d := time.Since(start); d > 10*time.Second {
		t.Errorf("took %v", d)
	}
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b []byte
	for ; i > 0; i /= 10 {
		b = append([]byte{byte('0' + i%10)}, b...)
	}
	return string(b)
}

func FuzzLoad(f *testing.F) {
	f.Add([]byte(head + `targetNamespace="urn:t" xmlns:t="urn:t"><xs:element name="a" type="t:T"/><xs:complexType name="T" mixed="true"><xs:sequence maxOccurs="unbounded"><xs:element ref="t:a"/><xs:group ref="t:g"/></xs:sequence><xs:attributeGroup ref="t:ag"/></xs:complexType><xs:group name="g"><xs:choice><xs:element name="b"><xs:complexType><xs:complexContent><xs:extension base="t:T"/></xs:complexContent></xs:complexType></xs:element></xs:choice></xs:group><xs:attributeGroup name="ag"><xs:attribute name="id" type="xs:ID"/><xs:attributeGroup ref="t:ag"/></xs:attributeGroup></xs:schema>`))
	f.Add([]byte(head + `><xs:include schemaLocation="x.xsd"/><xs:redefine schemaLocation="t.xsd"><xs:group name="g"><xs:sequence><xs:group ref="g"/></xs:sequence></xs:group></xs:redefine><xs:element name="e" substitutionGroup="e"/></xs:schema>`))
	f.Add([]byte(`<!DOCTYPE s [<!ENTITY a "<xs:element name='q'/>">]>` + head + `>&a;&a;</xs:schema>`))
	f.Fuzz(func(t *testing.T, data []byte) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		s, err := LoadBytes(ctx, "t.xsd", data, Limits{MaxNodes: 20000})
		if err != nil {
			return
		}
		m := s.Analyze()
		for _, d := range m.Decls {
			_ = d.Describe()
		}
	})
}
