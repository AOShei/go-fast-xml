package gofastxml

import (
	"strings"
	"testing"
)

// TestProfileNamespaceAliases: elements of an older namespace of a schema
// are matched by the rules written for the current one.
func TestProfileNamespaceAliases(t *testing.T) {
	prof := head("alias", `{"root": "doc", "namespace": "urn:new"}, {"root": "doc", "namespace": "urn:old"}`) +
		`"default_namespace": "urn:new", "namespace_aliases": {"urn:old": "urn:new"}, "rules": [
		{"match": "/doc", "as": "transparent"},
		{"match": "h", "as": "heading", "level": 1},
		{"match": "p", "as": "block", "kind": "paragraph"}]}`
	for _, ns := range []string{"urn:new", "urn:old"} {
		src := `<doc xmlns="` + ns + `"><h>T</h><p>Para.</p><x:p xmlns:x="urn:other">Other.</x:p></doc>`
		doc := extractWith(t, prof, src, nil)
		wantBrief(t, doc, "heading1:T\nparagraph:Para.\nparagraph~:Other.\n")
	}
	for _, bad := range []string{`{"": "urn:new"}`, `{"urn:a": "urn:a"}`, `{"urn:a": "urn:b", "urn:b": "urn:c"}`} {
		p := head("alias", `{"root": "doc"}`) + `"namespace_aliases": ` + bad + `, "rules": []}`
		if _, err := LoadProfile(strings.NewReader(p)); err == nil {
			t.Errorf("%s loaded", bad)
		}
	}
}
