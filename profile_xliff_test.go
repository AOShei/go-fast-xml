package gofastxml

import (
	"context"
	"encoding/xml"
	"testing"
)

func TestXLIFFFixture(t *testing.T) {
	doc, data := readFixture(t, "testdata/profiles/xliff/basic.xlf")
	if p := doc.Metadata.Extra[ExtraProfile]; p != "xliff" {
		t.Fatalf("profile %q", p)
	}
	got := tableBrief(doc) + "version=" + doc.Metadata.Extra["xliff.version"] + "\n"
	want := `heading1:weather.properties
  attribute=original
  xliff.datatype=javapropertyresourcebundle
  xliff.original=weather.properties
  xliff.source_language=en
  xliff.target_language=fr-FR
paragraph:Hand
  attribute=tool-name
  xliff.datatype=javapropertyresourcebundle
  xliff.field=tool
  xliff.original=weather.properties
  xliff.source_language=en
  xliff.target_language=fr-FR
  xliff.tool_id=hand
paragraph:Written for the tests.
  xliff.datatype=javapropertyresourcebundle
  xliff.field=note
  xliff.from=editor
  xliff.original=weather.properties
  xliff.source_language=en
  xliff.target_language=fr-FR
table:
  attribute=id resname restype approved translate target@state
  header_source=profile
  xliff.datatype=javapropertyresourcebundle
  xliff.original=weather.properties
  xliff.source_language=en
  xliff.target_language=fr-FR
table @336+566 header=1
  ["id" "resname" "restype" "approved" "translate" "state" "source" "target" "notes" "other"] @0+0#
  ["g1" "greetings" "" "" "" "" "" "" "" ""] @343+35#g1
  ["1" "hello" "" "yes" "" "translated" "Hello world{0}!" "Bonjour le monde{0} !" "Keep the brace. dev" "Hello\nSalut\n80% tm"] @379+373#1
  ["2" "" "" "" "no" "" "It is <b>cold</b>." "" "" ""] @762+132#2
version=1.2
`
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
	if bad := citeCheck(data, doc); bad != "" {
		t.Error(bad)
	}
	if bad := rowCite(data, doc); bad != "" {
		t.Error(bad)
	}
}

// TestXLIFFSelection: XLIFF 1.x by namespace, or with no namespace by
// version; an xliff root in another namespace, or with no namespace and
// another version, is not read with xliff.
func TestXLIFFSelection(t *testing.T) {
	for src, want := range map[string]string{
		`<xliff version="1.2" xmlns="urn:oasis:names:tc:xliff:document:1.2"><file/></xliff>`: "xliff",
		`<xliff xmlns="urn:oasis:names:tc:xliff:document:1.1"><file/></xliff>`:               "xliff",
		`<xliff version="1.0"><file/></xliff>`:                                               "xliff",
		`<xliff version="2.0"><file/></xliff>`:                                               "",
		`<xliff><file/></xliff>`:                                                             "",
		`<xliff version="1.2" xmlns="urn:x"><file/></xliff>`:                                 "",
	} {
		doc, err := ExtractBytes(context.Background(), []byte(src), nil)
		if err != nil {
			t.Fatal(err)
		}
		if got := doc.Metadata.Extra[ExtraProfile]; got != want {
			t.Errorf("%s: profile %q, want %q", src, got, want)
		}
	}
}

var xliffCase = &transCase{profile: "xliff", format: "xml-xliff", keyed: true,
	skip: map[string]string{},
	with: map[string]string{"alt-trans": "match-quality origin tool", "trans-unit/context-group/context": "context-type", "trans-unit/note": "from", "x": "equiv-text", "bx": "equiv-text", "ex": "equiv-text", "trans-unit/count-group/count": "count-type unit"},
	// internal-file form="base64" is an embedded document: its base64 is
	// handed to the child extractor, not written in the body.
	skipEl: func(el xml.StartElement) bool {
		if el.Name.Local != "internal-file" {
			return false
		}
		for _, a := range el.Attr {
			if a.Name.Local == "form" && a.Value == "base64" {
				return true
			}
		}
		return false
	},
}

// TestXLIFFOracles: (a) the body's text in order is the file's character
// data in order, apart from xliffCase's exceptions; (b) every block's and
// every row's text is in the source at its offsets.
func TestXLIFFOracles(t *testing.T) { xliffCase.oracles(t) }

// TestXLIFFCorpus reads every XLIFF of dev and held-out and logs
// aggregate counts (names only on dev).
func TestXLIFFCorpus(t *testing.T) {
	xliffCase.corpus(t)
	largest(t, "xml-xliff")
}
