package draft

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/AOShei/go-fast-xml/internal/xsd"
)

// FuzzDraft drafts arbitrary schemas: the draft is always JSON.
func FuzzDraft(f *testing.F) {
	f.Add([]byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:t" xmlns:t="urn:t"><xs:element name="s"><xs:complexType><xs:sequence><xs:element name="title" type="xs:string"/><xs:element ref="t:s" maxOccurs="unbounded"/><xs:element name="l"><xs:complexType><xs:sequence><xs:element name="i" maxOccurs="unbounded" type="xs:string"/></xs:sequence></xs:complexType></xs:element></xs:sequence><xs:attribute name="id" type="xs:ID"/></xs:complexType></xs:element></xs:schema>`))
	f.Add([]byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="table"><xs:complexType><xs:sequence><xs:element name="tr" maxOccurs="unbounded"><xs:complexType><xs:sequence><xs:element name="td" maxOccurs="unbounded" type="xs:string"/></xs:sequence></xs:complexType></xs:element></xs:sequence></xs:complexType></xs:element></xs:schema>`))
	f.Fuzz(func(t *testing.T, data []byte) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		s, err := xsd.LoadBytes(ctx, "t.xsd", data, xsd.Limits{MaxNodes: 20000})
		if err != nil {
			return
		}
		out := Write(Input{Entry: "t.xsd", Set: s, Model: s.Analyze()})
		if !json.Valid(out) {
			t.Fatalf("not JSON:\n%s", out)
		}
	})
}
