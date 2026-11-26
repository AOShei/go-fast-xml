package gofastxml

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/AOShei/go-fast-core"
)

// tableBrief writes every block as fullBrief does, and a table's rows
// one per line, cells separated by " | ", with each row's location.
func tableBrief(doc *gofastcore.Document) string {
	var b strings.Builder
	b.WriteString(fullBrief(doc))
	for _, x := range blocks(doc) {
		if x.Table == nil {
			continue
		}
		fmt.Fprintf(&b, "table @%d+%d header=%d\n", x.Loc.Offset, x.Loc.Length, x.Table.HeaderRows)
		for i, r := range x.Table.Rows {
			loc := ""
			if i < len(x.Table.RowLocs) {
				l := x.Table.RowLocs[i]
				loc = fmt.Sprintf(" @%d+%d#%s", l.Offset, l.Length, l.Anchor)
			}
			fmt.Fprintf(&b, "  %q%s\n", r, loc)
		}
	}
	return b.String()
}

func readFixture(t *testing.T, path string) (*gofastcore.Document, []byte) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	doc, err := ExtractBytes(context.Background(), data, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := doc.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	return doc, data
}

func TestTMXFixture(t *testing.T) {
	doc, data := readFixture(t, "testdata/profiles/tmx/basic.tmx")
	if p := doc.Metadata.Extra[ExtraProfile]; p != "tmx" {
		t.Fatalf("profile %q", p)
	}
	got := tableBrief(doc)
	extra := []string{}
	for _, k := range []string{"tmx.version", "tmx.srclang", "tmx.adminlang", "tmx.creationtool", "tmx.segtype", "tmx.o-tmf", "tmx.datatype"} {
		extra = append(extra, k+"="+doc.Metadata.Extra[k])
	}
	got += strings.Join(extra, " ") + "\n"
	want := `paragraph:Written for the tests.
  tmx.field=note
paragraph:Weather
  tmx.field=prop
  tmx.type=x-domain
table:
  attribute=unicode code ent subst
  tmx.field=ude
  tmx.ude_base=Macintosh
  tmx.ude_name=MacRoman
table:
  header_source=attribute
  tmx.field=body
table @266+112 header=0
  ["#xF8FF" "#xF0" "Apple_logo" "[Apple]"] @304+68#
table @389+511 header=1
  ["" "en" "fr-FR" "de" "EN"] @0+0#
  ["Météo x-client" "It is <b>cold</b> today." "Checked.\nIl fait froid aujourd'hui." "" ""] @396+269#t1
  ["" "" "Bonjour{0}" "Hallo\nGuten Tag" ""] @666+173#t2
  ["" "" "" "" "Old style"] @840+52#
tmx.version=1.4 tmx.srclang=en tmx.adminlang=en tmx.creationtool=Hand tmx.segtype=sentence tmx.o-tmf=none tmx.datatype=plaintext
`
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
	if bad := citeCheck(data, doc); bad != "" {
		t.Error(bad)
	}
	for i, l := range doc.Parts[0].Blocks[3].Table.RowLocs[1:] {
		if s := string(data[l.Offset : l.Offset+l.Length]); !strings.HasPrefix(s, "<tu") || !strings.HasSuffix(s, "</tu>") {
			t.Errorf("row %d: %q is not its tu", i+1, s)
		}
	}
}

var tmxCase = &transCase{profile: "tmx", format: "xml-tmx", keyed: true,
	skip: map[string]string{},
	with: map[string]string{"tu/prop": "type", "tuv/prop": "type"},
}

// TestTMXOracles: (a) the body's text in order is the file's character
// data in order, the props' types after their values; (b) every block's
// and every row's text is in the source at its offsets.
func TestTMXOracles(t *testing.T) { tmxCase.oracles(t) }

// TestTMXCorpus reads every TMX of dev and held-out and logs aggregate
// counts (names only on dev).
func TestTMXCorpus(t *testing.T) {
	c := &jatsOracleCase{profile: "tmx", format: "xml-tmx"}
	c.run(t)
	largest(t, "xml-tmx")
}

// largest reads the largest dev file of a set and logs its time, the
// heap allocated while reading, the rows of its tables and its warnings.
func largest(t *testing.T, set string) {
	dir := jatsCorpusDir("dev", set)
	if dir == "" {
		return
	}
	var big string
	var size int64
	for _, p := range jatsCorpusFiles(dir) {
		if fi, err := os.Stat(p); err == nil && fi.Size() > size {
			big, size = p, fi.Size()
		}
	}
	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	start := time.Now()
	doc, _ := readFixture(t, big)
	took := time.Since(start)
	runtime.ReadMemStats(&after)
	rows := 0
	for _, b := range blocks(doc) {
		if b.Table != nil {
			rows += len(b.Table.Rows)
		}
	}
	warns := map[string]int{}
	for _, w := range doc.AllWarnings() {
		warns[w.Code]++
	}
	t.Logf("largest dev file: %d bytes in %v, %d MiB allocated, heap in use %d MiB after, %d table rows, warnings %s",
		size, took.Round(time.Millisecond), (after.TotalAlloc-before.TotalAlloc)>>20, after.HeapInuse>>20, rows, sorted(warns))
}

// TestTMXSelection: a tmx root with no version, or in a namespace, stays
// generic.
func TestTMXSelection(t *testing.T) {
	for src, want := range map[string]string{
		`<tmx version="1.4"><body/></tmx>`:                                       "tmx",
		`<tmx><body><tu><tuv xml:lang="en"><seg>a</seg></tuv></tu></body></tmx>`: "",
		`<tmx xmlns="urn:x" version="1.4"><body/></tmx>`:                         "",
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

// TestColumnKeyLoad: "column_key" needs "column" and an attribute of the
// element's own.
func TestColumnKeyLoad(t *testing.T) {
	for _, rule := range []string{
		`{"match": "c", "as": "cell", "column_key": {"attr": "lang"}}`,
		`{"match": "c", "as": "cell", "column": 2, "column_key": "en"}`,
		`{"match": "c", "as": "cell", "column": 2, "column_key": {"attr": "lang", "of": "r"}}`,
	} {
		src := `{"profile": "x", "title": "x", "format": 1, "match": [{"root": "t"}], "rules": [` + rule + `]}`
		if _, err := LoadProfile(strings.NewReader(src)); err == nil {
			t.Errorf("%s: loaded", rule)
		}
	}
}

// TestColumnKeyBound: more languages than a table has columns are cut
// with limit.exceeded.
func TestColumnKeyBound(t *testing.T) {
	var b strings.Builder
	b.WriteString(`<tmx version="1.4"><body><tu>`)
	for i := range 1100 {
		fmt.Fprintf(&b, `<tuv xml:lang="l%d"><seg>s</seg></tuv>`, i)
	}
	b.WriteString(`</tu></body></tmx>`)
	doc, err := ExtractBytes(context.Background(), []byte(b.String()), nil)
	if err != nil {
		t.Fatal(err)
	}
	cut := false
	for _, w := range doc.AllWarnings() {
		cut = cut || w.Code == gofastcore.WarnLimitExceeded
	}
	tb := blocks(doc)[0].Table
	if !cut || len(tb.Rows[0]) != 1024 {
		t.Errorf("cut %v, %d columns", cut, len(tb.Rows[0]))
	}
}
