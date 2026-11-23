package profile

import (
	"strings"
	"testing"
)

type testPath []struct {
	name  QName
	attrs map[string]string
}

func (p testPath) Len() int         { return len(p) }
func (p testPath) Name(i int) QName { return p[len(p)-1-i].name }
func (p testPath) Attr(i int, q QName) (string, bool) {
	v, ok := p[len(p)-1-i].attrs[q.Local]
	return v, ok
}

// path builds a path from the root: "a/b[x=1]/c".
func path(s string) testPath {
	var p testPath
	for _, st := range strings.Split(s, "/") {
		name, rest, _ := strings.Cut(st, "[")
		attrs := map[string]string{}
		for _, kv := range strings.Split(strings.TrimSuffix(rest, "]"), ",") {
			if k, v, ok := strings.Cut(kv, "="); ok {
				attrs[k] = v
			}
		}
		p = append(p, struct {
			name  QName
			attrs map[string]string
		}{QName{Local: name}, attrs})
	}
	return p
}

func TestSelector(t *testing.T) {
	cases := []struct {
		sel, path string
		want      bool
	}{
		{"Titre", "a/Titre", true},
		{"Chapitre/Titre", "a/Chapitre/Titre", true},
		{"Chapitre/Titre", "a/Cas/Titre", false},
		{"/a/Titre", "a/Titre", true},
		{"/Titre", "a/Titre", false},
		{"*/Titre", "a/Titre", true},
		{"Liste[@type='numero']/Item", "Liste[type=numero]/Item", true},
		{"Liste[@type='numero']/Item", "Liste[type=puce]/Item", false},
		{"p[@class~='b']", "p[class=a b c]", true},
		{"p[@class~='x']", "p[class=a b c]", false},
		{"p[@id]", "p[id=]", true},
		{"p[@id]", "p", false},
	}
	for _, c := range cases {
		s, err := ParseSelector(c.sel, nil, "")
		if err != nil {
			t.Fatalf("%s: %v", c.sel, err)
		}
		if got := s.Match(path(c.path)); got != c.want {
			t.Errorf("%s on %s: %v, want %v", c.sel, c.path, got, c.want)
		}
	}
	for _, bad := range []string{"", "/", "a//b", "a/", "a[@b", "a[@b=c]", "a[@b~='x y']", "x:a", "a b", "a[1]", "a/..", "a|b"} {
		if _, err := ParseSelector(bad, nil, ""); err == nil {
			t.Errorf("%q: no error", bad)
		}
	}
	s, err := ParseSelector("dc:title", map[string]string{"dc": "urn:dc"}, "")
	if err != nil || s.Steps[0].Name != (QName{"urn:dc", "title"}) {
		t.Errorf("prefix: %v %v", s, err)
	}
}

func load(t *testing.T, rules string) (*Profile, error) {
	t.Helper()
	src := `{"profile": "t", "format": 1, "namespaces": {"dc": "urn:dc"}, "match": [{"root": "r"}], "rules": [` + rules + `]}`
	return Load(strings.NewReader(src), false)
}

func TestLoadErrors(t *testing.T) {
	cases := []struct{ rules, want string }{
		{`{"match": "a", "as": "frob"}`, `rule 1 ("a"): unknown "as" "frob"`},
		{`{"match": "a", "as": "block", "kind": "table"}`, `"kind" "table" is not one of`},
		{`{"match": "a", "as": "block", "kind": "paragraph", "colour": 1}`, `unknown key "colour"`},
		{`{"match": "a", "as": "section", "level": 1}`, `key "level" is not allowed on a "section" rule`},
		{`{"match": "x:a", "as": "section"}`, `prefix "x" is not declared`},
		{`{"match": "a", "as": "note", "type": "gossip"}`, `note "type" "gossip"`},
		{`{"match": "a", "as": "stored", "kind": "secret"}`, `stored "kind" "secret"`},
		{`{"match": "a", "as": "heading"}`, `needs "level"`},
		{`{"match": "a", "as": "heading", "level": "deep"}`, `"level" "deep"`},
		{`{"match": "a", "as": "metadata", "field": "colour"}`, `"field" "colour"`},
		{`{"match": "a", "as": "metadata", "field": "extra"}`, `"key" goes with`},
		{`{"match": "a", "as": "item", "marker": {"attr": "n", "child": "b", "of": "c"}}`, `exactly one of`},
		{`{"match": "a", "as": "item", "marker": {"child": "b", "structure": "c"}}`, `exactly one of`},
		{`{"match": "a", "as": "row", "cells": [{"attr": "n", "child": "b"}]}`, `attribute of the row's element`},
		{`{"match": "a", "as": "item", "marker": {"atr": "n"}}`, `unknown field "atr"`},
		{`{"match": "a", "as": "section", "attrs": {"stored": "x"}}`, `core's`},
		{`{"match": "a", "as": "list", "ordered": {"attr": "t"}}`, `needs "attr" and "in"`},
		{`{"match": "a", "as": "section", "display": {"attr": "h", "in": ["1"], "reason": "white"}}`, `reason "white"`},
		{`{"match": "a", "as": "inline", "title": {"attr": "t"}}`, `needs "link"`},
		{`{"match": "a", "as": "heading", "level": 2, "duplicate_of": "b"}`, `needs "key"`},
		{`{"as": "section"}`, `"match" is missing`},
	}
	for _, c := range cases {
		_, err := load(t, c.rules)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: error %v, want %q", c.rules, err, c.want)
		}
	}
	if _, err := Load(strings.NewReader(`{"profile": "t", "format": 1, "match": [{"root": "r"}], "rules": [], "extra_key": 1}`), false); err == nil {
		t.Error("unknown top-level key accepted")
	}
	if _, err := Load(strings.NewReader(`{"profile": "t", "format": 2, "match": [{"root": "r"}]}`), false); err == nil {
		t.Error("format 2 accepted")
	}
	if _, err := Load(strings.NewReader(`{"profile": "t", "format": 1, "match": [{"root": "r"}], "rules": [{"match": "a", "as": "skip"}]}`), true); err == nil {
		t.Error("skip accepted in a built-in profile")
	}
	if _, err := load(t, `{"match": "a", "as": "skip"}`); err != nil {
		t.Errorf("skip refused in a user profile: %v", err)
	}
}

func TestLookupPrecedence(t *testing.T) {
	p, err := load(t, `{"match": "Titre", "as": "block", "kind": "paragraph"},
		{"match": "*/Titre", "as": "block", "kind": "caption"},
		{"match": "Chapitre/Titre", "as": "heading", "level": "nesting"},
		{"match": "Liste[@type='numero']/Item", "as": "item"},
		{"match": "Liste/Item", "as": "item", "marker": "x"},
		{"match": "Item", "as": "transparent"},
		{"match": "Item", "as": "section"}`)
	if err != nil {
		t.Fatal(err)
	}
	check := func(pth string, want int) {
		t.Helper()
		r := p.Lookup(path(pth))
		got := -1
		if r != nil {
			got = r.Index
		}
		if got != want {
			t.Errorf("%s: rule %d, want %d", pth, got, want)
		}
	}
	check("a/Chapitre/Titre", 2) // equal steps and tests: the later rule
	check("a/Cas/Titre", 1)      // two steps beat one
	check("Liste[type=numero]/Item", 3)
	check("Liste[type=puce]/Item", 4)
	check("a/Item", 6) // later wins
	check("a/Other", -1)
	if got := p.KeptAttrs(); len(got) != 1 || got[0].Local != "type" {
		t.Errorf("kept %v", got)
	}
}

func TestMatchesRoot(t *testing.T) {
	src := `{"profile": "t", "format": 1, "match": [{"root": "r", "attr": "[@v='2']"}, {"root": "s", "namespace": "urn:s", "doctype_public": "-//S"}], "rules": []}`
	p, err := Load(strings.NewReader(src), true)
	if err != nil {
		t.Fatal(err)
	}
	if !p.MatchesRoot(path("r[v=2]"), "") || p.MatchesRoot(path("r[v=1]"), "") {
		t.Error("attribute test on the root")
	}
	sp := testPath{{name: QName{"urn:s", "s"}}}
	if !p.MatchesRoot(sp, "-//S//DTD 1") || p.MatchesRoot(sp, "-//T") {
		t.Error("doctype_public")
	}
}

func FuzzSelector(f *testing.F) {
	for _, s := range []string{"a", "/a/b", "a[@b='c']/d", "*[@x~='y']", "dc:title", "a[@b][@c=\"d\"]"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		sel, err := ParseSelector(s, map[string]string{"dc": "urn:dc"}, "")
		if err != nil {
			return
		}
		if len(sel.Steps) == 0 {
			t.Fatal("no steps")
		}
		sel.Match(path("a/b/c"))
	})
}

func FuzzLoad(f *testing.F) {
	f.Add(`{"profile": "t", "format": 1, "match": [{"root": "r"}], "rules": [{"match": "a", "as": "block", "kind": "paragraph"}]}`)
	f.Add(`{"profile": "t", "format": 1, "match": [{"root": "r", "attr": "[@a]"}], "heading_base": 2, "rules": [{"match": "a/b", "as": "heading", "level": {"attr": "n"}, "attrs": {"k": {"child": "c"}}}]}`)
	f.Fuzz(func(t *testing.T, s string) {
		p, err := Load(strings.NewReader(s), false)
		if err != nil {
			return
		}
		if p.Name == "" || len(p.Matches) == 0 {
			t.Fatal("loaded without a name or match")
		}
		p.Lookup(path("r/a/b"))
	})
}
