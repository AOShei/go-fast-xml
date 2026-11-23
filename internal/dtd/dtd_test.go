package dtd

import (
	"testing"
)

func TestParse(t *testing.T) {
	d := Parse([]byte(` article PUBLIC "-//NLM//DTD JATS//EN" "JATS.dtd" [
  <!-- a comment with > and ]] -->
  <!ENTITY product "Widget &#x2122;">
  <!ENTITY product "second">
  <!ENTITY lt2 "&#60;b>x&#60;/b>">
  <!ENTITY chap SYSTEM "chap.xml">
  <!ENTITY logo SYSTEM "logo.png" NDATA png>
  <!ENTITY % ext SYSTEM "more.dtd">
  %ext;
  <!ELEMENT p (#PCDATA)>
  <!ATTLIST p id ID #IMPLIED note CDATA "a > b">
  <?pi in the subset?>
]`))
	if d.Name != "article" || d.PublicID != "-//NLM//DTD JATS//EN" || d.SystemID != "JATS.dtd" || !d.HasExternal {
		t.Errorf("ids %+v", d)
	}
	if e := d.Entities["product"]; e == nil || string(e.Value) != "Widget ™" {
		t.Errorf("product %+v", e)
	}
	if e := d.Entities["lt2"]; e == nil || string(e.Value) != "<b>x</b>" {
		t.Errorf("lt2 %+v", e)
	}
	if e := d.Entities["chap"]; e == nil || !e.External || e.Unparsed {
		t.Errorf("chap %+v", e)
	}
	if e := d.Entities["logo"]; e == nil || !e.Unparsed {
		t.Errorf("logo %+v", e)
	}
	if d.ParamRefs != 1 || d.ExternalParamRefs != 1 || d.Damaged != 0 {
		t.Errorf("params %d %d damaged %d", d.ParamRefs, d.ExternalParamRefs, d.Damaged)
	}
}

func FuzzParse(f *testing.F) {
	f.Add([]byte(` a SYSTEM "a.dtd" [<!ENTITY x "y&#65;"><!ENTITY % p "q">%p;<!-- c -->]`))
	f.Add([]byte(` a [<!ENTITY x 'unterminated`))
	f.Fuzz(func(t *testing.T, b []byte) {
		d := Parse(b)
		for _, e := range d.Entities {
			if len(e.Value) > 4*len(b)+4 {
				t.Fatalf("value of %d bytes from %d", len(e.Value), len(b))
			}
		}
	})
}
