package gofastxml

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/AOShei/go-fast-core"
)

// FuzzExtract runs whole extraction on arbitrary input, with small
// budgets so that a bomb the fuzzer finds costs little: every document
// must be valid, and every error one of core's.
func FuzzExtract(f *testing.F) {
	for _, s := range []string{
		`<?xml version="1.0"?><doc a="1"><p>Hello <b>bold</b></p><!-- c --><![CDATA[x]]><?pi d?></doc>`,
		`<!DOCTYPE d SYSTEM "d.dtd" [<!ENTITY e "<i>&f;</i>"><!ENTITY f "x&#1;">]><d>&e; &eacute; &nope;</d>`,
		"<?xml version=\"1.0\" encoding=\"windows-1252\"?><d>\x93q\x94</d>",
		"\xFF\xFE<\x00d\x00>\x00x\x00<\x00/\x00d\x00>\x00",
		`<d xml:space="preserve"> a  b <e xml:space="default"> c  d </e></d>`,
		`<r xmlns:x="http://www.w3.org/2001/XInclude"><x:include href="a"/>t<p>u</p>v</r>`,
		`<a><b>x</a> < & </c>`,
		// canada-laws: a section's label before a numbered subsection,
		// a marginal note holding a historical note, a CALS table.
		`<Statute xmlns:lims="http://justice.gc.ca/lims" lims:id="1" xml:lang="en"><Body><Section lims:id="2"><MarginalNote>M<HistoricalNote>h</HistoricalNote></MarginalNote><Label>1</Label><Subsection in-force="no"><Label>(1)</Label><Text>t<FootnoteRef idref="f">a</FootnoteRef></Text><TableGroup><table><tgroup cols="1"><tbody><row><entry>c</entry></row></tbody></tgroup></table></TableGroup></Subsection></Section></Body></Statute>`,
	} {
		f.Add([]byte(s), false)
	}
	f.Fuzz(func(t *testing.T, data []byte, noAttrs bool) {
		x := &Extractor{}
		if noAttrs {
			x.Attributes = AttributesNone
		}
		opts := &gofastcore.Options{Limits: gofastcore.Limits{MaxTextBytes: 1 << 20, MaxDecodedBytes: 1 << 16}}
		doc, err := x.Extract(context.Background(), bytes.NewReader(data), int64(len(data)), opts)
		if !noAttrs {
			// ExtractBytes reads the bytes in place: it gives the same
			// document and leaves them as they were.
			kept := bytes.Clone(data)
			doc2, err2 := ExtractBytes(context.Background(), data, opts)
			if !bytes.Equal(kept, data) {
				t.Fatal("ExtractBytes changed its input")
			}
			j1, _ := json.Marshal(doc)
			j2, _ := json.Marshal(doc2)
			if (err == nil) != (err2 == nil) || !bytes.Equal(j1, j2) {
				t.Fatalf("ExtractBytes differs from Extract: %v / %v", err, err2)
			}
		}
		if err != nil {
			if !errors.Is(err, gofastcore.ErrMalformed) && !errors.Is(err, gofastcore.ErrUnsupported) {
				t.Fatalf("error %v", err)
			}
			return
		}
		if err := doc.Validate(); err != nil {
			t.Fatalf("invalid: %v", err)
		}
		if len(doc.Parts) == 0 || doc.Parts[0].Number != 1 {
			t.Fatalf("parts %d", len(doc.Parts))
		}
	})
}

// fuzzProfile has a rule of every kind, both table models, every value
// source and the keys of A3.
const fuzzProfile = `{"profile": "fuzz", "format": 1, "match": [{"root": "r"}], "heading_base": 2,
 "namespaces": {"m": "http://www.w3.org/1998/Math/MathML"},
 "rules": [
  {"match": "/r", "as": "transparent", "extra": {"x.id": {"attr": "id"}}},
  {"match": "t", "as": "metadata", "field": "title", "also_heading": 1},
  {"match": "k", "as": "metadata", "field": "extra", "key": "k", "join": ", ", "value": {"child": "v"}},
  {"match": "s", "as": "section", "anchor": {"attr": "id"}, "mark": {"x.c": {"structure": "c"}}, "attrs": {"x.a": {"attr": "a"}}},
  {"match": "s/h", "as": "heading", "level": "nesting", "duplicate_of": "hh", "key": "x.plain"},
  {"match": "hh", "as": "heading", "level": {"attr": "n"}},
  {"match": "p", "as": "block", "kind": "paragraph", "marker": {"attr": "n"}},
  {"match": "q", "as": "block", "kind": "quote"},
  {"match": "code", "as": "block", "kind": "code", "language": "go"},
  {"match": "m:math", "as": "block", "kind": "formula", "notation": "plain"},
  {"match": "fn", "as": "block", "kind": "footnote"},
  {"match": "u", "as": "block", "kind": "paragraph", "value": {"attr": "href"}},
  {"match": "l", "as": "list", "ordered": {"attr": "t", "in": ["1"]}},
  {"match": "i", "as": "item", "attrs": {"x.v": {"child": "v"}}},
  {"match": "lab", "as": "label"},
  {"match": "tb", "as": "table"},
  {"match": "atb", "as": "table", "header": ["from", "to"]},
  {"match": "ar", "as": "row", "cells": [{"attr": "from"}, {"attr": "to"}]},
  {"match": "cals", "as": "table", "model": "cals"},
  {"match": "tr", "as": "row", "header": {"attr": "h", "in": ["1"]}},
  {"match": "td", "as": "cell", "colspan": "cs", "rowspan": "rs"},
  {"match": "b", "as": "inline", "style": "bold"},
  {"match": "a", "as": "inline", "link": {"attr": "href"}, "title": {"attr": "title"}, "attrs": {"x.t": {"attr": "t"}}},
  {"match": "img", "as": "image", "id": {"attr": "src"}, "alt": {"child": "alt"}},
  {"match": "n", "as": "note", "type": "comment"},
  {"match": "part", "as": "part", "kind": "chapter", "name": {"child": "pn"}},
  {"match": "st", "as": "stored", "kind": "custom_xml"},
  {"match": "hide", "as": "transparent", "display": {"attr": "h", "in": ["1"], "reason": "hidden"}},
  {"match": "sk", "as": "skip"},
  {"match": "br", "as": "break"},
  {"match": "sf", "as": "block", "kind": "footnote", "split": true},
  {"match": "fg", "as": "transparent", "split": true, "attrs": {"x.c": {"attr": "c", "child": "cc"}}},
  {"match": "oo", "as": "block", "kind": "footnote", "out_of_line": true},
  {"match": "ti", "as": "item", "value": {"attr": "href"}},
  {"match": "qs", "as": "inline", "value": {"attr": "c", "codepoint": "hex"}},
  {"match": "pg", "as": "inline", "value": {"attr": "n"}, "space": true, "attrs": {"x.m": "pb"}},
  {"match": "dd", "as": "document", "media_type": "text/html", "anchor": {"attr": "id"}},
  {"match": "dm", "as": "document", "media_type": "text/html", "when": "markup"},
  {"match": "dx", "as": "document", "media_type": {"attr": "type", "map": {"html": "text/html", "xhtml": "application/xhtml+xml"}}},
  {"match": "dx", "as": "block", "kind": "paragraph"},
  {"match": "ja", "as": "section", "anchor": {"child": "jm/ji"}, "extra": {"x.ji": {"child": "jm/ji"}}},
  {"match": "jm", "as": "transparent"},
  {"match": "ji", "as": "metadata", "field": "extra", "key": "x.j"}]}`

// fuzzChild is a child extractor for the fuzz tests: a document of one
// paragraph holding what it is given, or ErrUnsupported for an odd size.
func fuzzChild(ctx context.Context, name, mime string, r io.ReaderAt, size int64, opts *gofastcore.Options) (*gofastcore.Document, error) {
	if size%2 == 1 {
		return nil, gofastcore.ErrUnsupported
	}
	b := make([]byte, size)
	r.ReadAt(b, 0)
	return oneParagraph(b), nil
}

// oneParagraph is the document of a fake child extractor: one paragraph
// holding b, cleaned as an extractor cleans text (valid UTF-8, no control
// characters, no white space at its ends); no block when that is empty.
func oneParagraph(b []byte) *gofastcore.Document {
	text := strings.TrimSpace(strings.Map(func(r rune) rune {
		if r < 0x20 && r != '\t' && r != '\n' || r >= 0x7f && r <= 0x9f {
			return -1
		}
		return r
	}, strings.ToValidUTF8(string(b), "�")))
	d := gofastcore.New("fake")
	d.Parts = []gofastcore.Part{{Kind: gofastcore.PartBody, Number: 1, Blocks: []gofastcore.Block{}}}
	if text != "" {
		d.Parts[0].Blocks = []gofastcore.Block{{Kind: gofastcore.KindParagraph, Origin: gofastcore.OriginNative, Text: text}}
	}
	return d
}

// FuzzProfile runs the rule engine with a profile of every rule kind on
// arbitrary input.
func FuzzProfile(f *testing.F) {
	for _, s := range []string{
		`<r id="1"><t>T</t><k><v>a</v></k><s id="s"><c><x y="1"/></c><h>H</h><hh n="3">H</hh><p n="1.">P <b>b</b> <a href="h" title="T" t="x">a</a></p>` +
			`<l t="1"><i>one<v>v</v><p>p</p><l><i>two</i></l></i></l><lab>L</lab><q>Q</q></s><tb><tr h="1"><td cs="2">a</td></tr><tr><td rs="3"><p>x</p><l><i>y</i></l></td><td>z</td></tr></tb>` +
			`<cals><tgroup><colspec colname="a"/><colspec colname="b"/><thead><row><entry namest="a" nameend="b">h</entry></row></thead><tbody><row><entry morerows="2">x</entry></row></tbody></tgroup></cals>` +
			`<img src="i"><alt>A</alt></img><u href="http://x"/><n>note</n><st>stored</st><hide h="1"><p>hid</p></hide><sk>gone</sk>stray<zz q="1">unmapped<b>in</b></zz>` +
			`<part><pn>P2</pn><p>in part</p></part><fn><lab>1</lab><p>fn</p></fn><code>x  y</code>` +
			`<atb><ar from="a" to="b"/><ar from="c"/><tr><td>t</td></tr><ar to="d">text</ar></atb></r>`,
		`<r><s><s><s><s><s><s><s><h>deep</h></s></s></s></s></s></s></s></r>`,
		`<r><p>a<br/>b<sf>f<br/>n</sf>c<fg><cc c="1"/><lab>L</lab><p>in</p><tb><tr><td>x<sf>y</sf></td></tr></tb></fg>d<oo>o</oo>e</p>` +
			`<p><lab>2</lab><tb><tr><td>t</td></tr></tb></p><l t="1"><lab>(a)</lab><ti href="h"><ti href="g"/></ti></l><t>T2<br/>x</t></r>`,
		`<r><p><qs c="2018"/>q<qs c="D800"/>x<pg n="12"/>y</p><t><qs c="22"/>T<pg n="3"/></t><qs c="110000"/><pg n="4"/><td><qs c="A"/></td></r>`,
		`<r><s id="s"><h>H</h><dd id="d">&lt;p&gt;one&lt;/p&gt;</dd><dm>plain</dm><dm><![CDATA[<b>b</b>]]></dm><dm>a &amp;amp; b</dm>` +
			`<dx type="xhtml"><div xmlns="http://www.w3.org/1999/xhtml"><p>x<br/>y</p></div></dx><dx type="html">&lt;i&gt;</dx><dx type="text">&lt;t&gt;</dx>` +
			`<p>in <dd>&lt;q&gt;</dd> p</p><dd><x a="1">t<!-- c --></x></dd></s></r>`,
		`<r><ja><p>before</p><jm><ji>I1</ji><ji>I2</ji></jm><p>after</p><s id="in"><p>x</p><ja><jm><ji>I3</ji></jm><p>y</p></ja></s><p>z</p></ja></r>`,
	} {
		f.Add([]byte(s), true)
	}
	p, err := LoadProfile(strings.NewReader(fuzzProfile))
	if err != nil {
		f.Fatal(err)
	}
	f.Fuzz(func(t *testing.T, data []byte, spans bool) {
		x := &Extractor{Profiles: []*Profile{p}, Profile: "fuzz"}
		opts := &gofastcore.Options{Spans: spans, Limits: gofastcore.Limits{MaxTextBytes: 1 << 20, MaxDecodedBytes: 1 << 16}}
		if spans {
			opts.Child = fuzzChild
		}
		doc, err := x.Extract(context.Background(), bytes.NewReader(data), int64(len(data)), opts)
		if err != nil {
			if !errors.Is(err, gofastcore.ErrMalformed) && !errors.Is(err, gofastcore.ErrUnsupported) {
				t.Fatalf("error %v", err)
			}
			return
		}
		if err := doc.Validate(); err != nil {
			t.Fatalf("invalid: %v", err)
		}
		if bad := sharedMergeCell(doc); bad != "" {
			t.Fatal(bad)
		}
	})
}

// TestFuzzProfileSeed reads the first seed of FuzzProfile and checks its
// output and that every block can be cited.
func TestFuzzProfileSeed(t *testing.T) {
	src := `<r id="1"><t>T</t><k><v>a</v></k><s id="s"><c><x y="1"/></c><h>H</h><hh n="3">H</hh><p n="1.">P <b>b</b> <a href="h" title="T" t="x">a</a></p>` +
		`<l t="1"><i>one<v>v</v><p>p</p><l><i>two</i></l></i></l><lab>L</lab><q>Q</q></s><tb><tr h="1"><td cs="2">a</td></tr><tr><td rs="3"><p>x</p><l><i>y</i></l></td><td>z</td></tr></tb>` +
		`<img src="i"><alt>A</alt></img><u href="http://x"/><n>note</n><st>stored</st><hide h="1"><p>hid</p></hide>stray<zz q="1">unmapped<b>in</b></zz>` +
		`<part><pn>P2</pn><p>in part</p></part><fn><lab>1</lab><p>fn</p></fn><code>x  y</code></r>`
	doc := extractWith(t, fuzzProfile, src, &gofastcore.Options{Spans: true})
	got := brief(blocks(doc))
	want := "heading1:T\nheading3:H\nparagraph(1.):P b a\nlist_item1(1.):one\nparagraph:p\nlist_item2:two\nquote1(L):Q\ntable:\nimage:A\nparagraph:http://x\nnote:note\nparagraph:hid\nparagraph~:strayunmappedin\nnote~:1\nparagraph:in part\nfootnote(1):fn\ncode:x  y\n"
	if got != want {
		t.Errorf("blocks:\n%s\nwant:\n%s", got, want)
	}
	if doc.Metadata.Extra["k"] != "a" || doc.Metadata.Extra["x.id"] != "1" || doc.Parts[1].Name != "P2" {
		t.Errorf("metadata %v, part name %q", doc.Metadata.Extra, doc.Parts[1].Name)
	}
	bs := blocks(doc)
	if bs[1].Attrs["x.c"] != "x(y=1)" || bs[1].Attrs["x.a"] != "" && bs[1].Attrs["x.a"] != "x" {
		t.Errorf("heading attrs %v", bs[1].Attrs)
	}
}
