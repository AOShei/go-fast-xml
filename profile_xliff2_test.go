package gofastxml

import (
	"context"
	"testing"
)

func TestXLIFF2Fixture(t *testing.T) {
	doc, data := readFixture(t, "testdata/profiles/xliff2/basic.xlf")
	if p := doc.Metadata.Extra[ExtraProfile]; p != "xliff2" {
		t.Fatalf("profile %q", p)
	}
	got := tableBrief(doc) + "version=" + doc.Metadata.Extra["xliff2.version"] + "\n"
	want := `heading1:weather.html
  attribute=original
  xliff2.original=weather.html
  xliff2.source_language=en
  xliff2.target_language=fr
  #f1
paragraph:Written for the tests.
  anchor_parent=f1
  xliff2.category=context
  xliff2.field=note
  xliff2.note_id=n1
  xliff2.original=weather.html
  xliff2.source_language=en
  xliff2.target_language=fr
paragraph:Keep the brace.
  anchor_parent=u1
  xliff2.field=note
  xliff2.original=weather.html
  xliff2.priority=2
  xliff2.source_language=en
  xliff2.target_language=fr
  xliff2.unit_name=hello
paragraph:<b>
  anchor_parent=u1
  xliff2.data_id=d1
  xliff2.field=data
  xliff2.original=weather.html
  xliff2.source_language=en
  xliff2.target_language=fr
  xliff2.unit_name=hello
paragraph:</b>
  anchor_parent=u1
  xliff2.data_id=d2
  xliff2.field=data
  xliff2.original=weather.html
  xliff2.source_language=en
  xliff2.target_language=fr
  xliff2.unit_name=hello
paragraph:Hello world!
  anchor_parent=u1
  xliff2.field=match
  xliff2.match_ref=#s1
  xliff2.original=weather.html
  xliff2.part=source
  xliff2.similarity=80
  xliff2.source_language=en
  xliff2.target_language=fr
  xliff2.unit_name=hello
paragraph:Salut le monde !
  anchor_parent=u1
  xliff2.field=match
  xliff2.match_ref=#s1
  xliff2.original=weather.html
  xliff2.part=target
  xliff2.similarity=80
  xliff2.source_language=en
  xliff2.target_language=fr
  xliff2.unit_name=hello
paragraph:world
  anchor_parent=u1
  xliff2.field=glossary
  xliff2.gls_source=tb
  xliff2.original=weather.html
  xliff2.part=term
  xliff2.source_language=en
  xliff2.target_language=fr
  xliff2.unit_name=hello
paragraph:monde
  anchor_parent=u1
  xliff2.field=glossary
  xliff2.original=weather.html
  xliff2.part=translation
  xliff2.source_language=en
  xliff2.target_language=fr
  xliff2.unit_name=hello
table:
  anchor_parent=f1
  attribute=id name type unit@id unit@name unit@type state
  header_source=profile
  xliff2.original=weather.html
  xliff2.source_language=en
  xliff2.target_language=fr
table @233+1148 header=1
  ["unit" "name" "type" "segment" "state" "source" "target" "other"] @0+0#
  ["g1" "greetings" "" "" "" "" "" ""] @354+32#g1
  ["u1" "hello" "" "s1" "translated" "Hello world!" "Bonjour le monde !" ""] @578+222#u1
  ["u1" "hello" "" "" "" "" "" ""] @805+41#u1
  ["u1" "hello" "" "s2" "" "Bye` + "—" + `now." "" ""] @851+83#u1
  ["u2" "" "" "" "" "It is x cold." "" ""] @1261+104#u2
version=2.1
`
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
	if bad := citeCheck(data, doc); bad != "" {
		t.Error(bad)
	}
	if bad := rowCiteFrom(data, doc, xliff2Case.ancestorCols); bad != "" {
		t.Error(bad)
	}
}

var xliff2Case = &transCase{profile: "xliff2", format: "xml-xliff", keyed: true,
	skip:      map[string]string{},
	with:      map[string]string{"mrk": "value", "sm": "value"},
	codepoint: map[string]string{"cp": "hex"},
	// unit@id, name, type on each row (outside the row's span), then
	// segment@id and state: five columns from attributes.
	ancestorCols: 3, attrCols: 5,
}

// TestXLIFF2Oracles: (a) the body's text in order is the file's character
// data in order, apart from xliff2Case's exceptions; (b) every block's
// and every row's text is in the source at its offsets.
func TestXLIFF2Oracles(t *testing.T) { xliff2Case.oracles(t) }

// TestXLIFF2Corpus reads every XLIFF of dev and held-out and logs
// aggregate counts (names only on dev).
func TestXLIFF2Corpus(t *testing.T) { xliff2Case.corpus(t) }

// TestXLIFF2Selection: XLIFF 2.x by its namespaces only; XLIFF 1.x stays
// with xliff, and an xliff root in another namespace or none with
// version 2.x stays generic.
func TestXLIFF2Selection(t *testing.T) {
	for src, want := range map[string]string{
		`<xliff version="2.0" xmlns="urn:oasis:names:tc:xliff:document:2.0"><file id="f"/></xliff>`: "xliff2",
		`<xliff version="2.1" xmlns="urn:oasis:names:tc:xliff:document:2.0"><file id="f"/></xliff>`: "xliff2",
		`<xliff version="2.2" xmlns="urn:oasis:names:tc:xliff:document:2.2"><file id="f"/></xliff>`: "xliff2",
		`<xliff version="1.2" xmlns="urn:oasis:names:tc:xliff:document:1.2"><file/></xliff>`:        "xliff",
		`<xliff version="1.2"><file/></xliff>`:                                                      "xliff",
		`<xliff version="2.0"><file id="f"/></xliff>`:                                               "",
		`<xliff version="2.0" xmlns="urn:x"><file id="f"/></xliff>`:                                 "",
		`<x:xliff version="2.0" xmlns:x="urn:oasis:names:tc:xliff:document:2.1"/>`:                  "",
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
