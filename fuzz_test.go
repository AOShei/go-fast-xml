package gofastxml

import (
	"bytes"
	"context"
	"errors"
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
		if err != nil {
			if !errors.Is(err, gofastcore.ErrMalformed) && !errors.Is(err, gofastcore.ErrUnsupported) {
				t.Fatalf("error %v", err)
			}
			return
		}
		if err := doc.Validate(); err != nil {
			t.Fatalf("invalid: %v", err)
		}
		if len(doc.Parts) != 1 || doc.Parts[0].Number != 1 {
			t.Fatalf("parts %d", len(doc.Parts))
		}
	})
}
