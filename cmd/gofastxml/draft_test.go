package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDraftProfileFlag(t *testing.T) {
	dir := t.TempDir()
	xsd := filepath.Join(dir, "s.xsd")
	os.WriteFile(xsd, []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:import namespace="urn:x" schemaLocation="https://example.org/x.xsd"/><xs:element name="doc"><xs:complexType><xs:sequence><xs:element name="p" type="xs:string" maxOccurs="unbounded"/></xs:sequence></xs:complexType></xs:element></xs:schema>`), 0o644)
	var out, errs bytes.Buffer
	if code := run(context.Background(), []string{"-draft-profile", xsd}, &out, &errs); code != 0 {
		t.Fatalf("exit %d: %s", code, errs.String())
	}
	if !strings.Contains(out.String(), `"review": true`) || !strings.Contains(errs.String(), "input.external_reference") {
		t.Errorf("out %s\nerr %s", out.String(), errs.String())
	}
	draft := filepath.Join(dir, "d.json")
	out.Reset()
	if code := run(context.Background(), []string{"-draft-profile", xsd, "-o", draft, "-quiet"}, &out, &errs); code != 0 || out.Len() != 0 {
		t.Fatalf("exit %d", code)
	}
	// The draft is refused as a profile.
	doc := filepath.Join(dir, "a.xml")
	os.WriteFile(doc, []byte(`<doc><p>x</p></doc>`), 0o644)
	errs.Reset()
	if code := run(context.Background(), []string{"-profile", draft, doc}, &out, &errs); code != 2 || !strings.Contains(errs.String(), "review") {
		t.Errorf("exit %d: %s", code, errs.String())
	}
	if code := run(context.Background(), []string{"-draft-profile", filepath.Join(dir, "none.xsd")}, &out, &errs); code != 1 {
		t.Errorf("missing schema: exit %d", code)
	}
}
