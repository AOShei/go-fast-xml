package gofastxml

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"
)

// schemaDir is the directory of public schemas next to the corpus, or "".
func schemaDir(t *testing.T) string {
	root := os.Getenv("GOFAST_SCHEMAS")
	if root == "" {
		root = filepath.Join("..", ".testdata", "schemas", "xml")
	}
	if _, err := os.Stat(root); err != nil {
		t.Skip("no schema directory (", root, ")")
	}
	return root
}

// draftOf drafts a schema and reports the time and memory it took.
func draftOf(t *testing.T, path string) *Draft {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		t.Skip("schema absent: ", path)
	}
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	start := time.Now()
	d, err := DraftProfile(context.Background(), path)
	took := time.Since(start)
	runtime.ReadMemStats(&after)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%s: %d schema documents, %d bytes of draft, %v, %.1f MiB allocated, heap %.1f MiB after",
		filepath.Base(path), len(d.Files), len(d.JSON), took.Round(time.Millisecond),
		float64(after.TotalAlloc-before.TotalAlloc)/(1<<20), float64(after.HeapAlloc)/(1<<20))
	if took > 20*time.Second {
		t.Errorf("drafting took %v", took)
	}
	for _, w := range d.Warnings {
		t.Logf("warning %s: %s", w.Code, w.Message)
	}
	if _, err := LoadProfile(bytes.NewReader(d.JSON)); err == nil {
		t.Error("the draft loads with its review marks")
	}
	if _, err := LoadProfile(bytes.NewReader(stripReview(d.JSON))); err != nil {
		t.Fatalf("the stripped draft does not load: %v", err)
	}
	return d
}

var attrTestRE = regexp.MustCompile(`\[[^\]]*\]`)

// lastLocal is the local name of a selector's last step.
func lastLocal(sel string) string {
	sel = attrTestRE.ReplaceAllString(sel, "")
	if i := strings.LastIndexByte(sel, '/'); i >= 0 {
		sel = sel[i+1:]
	}
	if i := strings.IndexByte(sel, ':'); i >= 0 {
		sel = sel[i+1:]
	}
	return sel
}

type kindsByName map[string]map[string]bool

func ruleKinds(t *testing.T, data []byte) kindsByName {
	var f struct {
		Rules []struct {
			Match string `json:"match"`
			As    string `json:"as"`
		} `json:"rules"`
	}
	if err := json.Unmarshal(data, &f); err != nil {
		t.Fatal(err)
	}
	k := kindsByName{}
	for _, r := range f.Rules {
		n := lastLocal(r.Match)
		if n == "*" {
			continue
		}
		if k[n] == nil {
			k[n] = map[string]bool{}
		}
		k[n][r.As] = true
	}
	return k
}

// agreement compares the draft's rule kinds with a hand-written profile's,
// element by local name, over the elements the schema declares: the same
// kind (one in common), a different kind, or no rule in the draft.
func agreement(t *testing.T, d *Draft, profile string) {
	hand, err := os.ReadFile(filepath.Join("profiles", profile))
	if err != nil {
		t.Fatal(err)
	}
	hk := ruleKinds(t, hand)
	dk := ruleKinds(t, d.JSON)
	var notes struct {
		Notes struct {
			Undecided []struct {
				Element string `json:"element"`
			} `json:"undecided"`
		} `json:"notes"`
	}
	json.Unmarshal(d.JSON, &notes)
	declared := map[string]bool{}
	for n := range dk {
		declared[n] = true
	}
	for _, u := range notes.Notes.Undecided {
		declared[lastLocal(u.Element)] = true
	}
	same, diff, none, unprofiled := 0, 0, 0, 0
	pairs := map[string]int{}
	for n := range declared {
		h := hk[n]
		if len(h) == 0 {
			unprofiled++
			continue
		}
		dd := dk[n]
		switch {
		case len(dd) == 0:
			none++
		case common(h, dd):
			same++
		default:
			diff++
			pairs[keys(dd)+" for "+keys(h)]++
		}
	}
	t.Logf("%s: %d elements declared; of the %d the profile has rules for: same kind %d, different kind %d, no rule in the draft %d; %d declared elements the profile has no rule for",
		profile, len(declared), same+diff+none, same, diff, none, unprofiled)
	var ps []string
	for p, n := range pairs {
		ps = append(ps, fmt.Sprintf("%s=%d", p, n))
	}
	sort.Strings(ps)
	t.Logf("%s: different (draft for profile): %s", profile, strings.Join(ps, "; "))
}

func common(a, b map[string]bool) bool {
	for k := range a {
		if b[k] {
			return true
		}
	}
	return false
}

func keys(m map[string]bool) string {
	var ks []string
	for k := range m {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return strings.Join(ks, "+")
}

func TestDraftServicePublic(t *testing.T) {
	dir := schemaDir(t)
	d := draftOf(t, filepath.Join(dir, "servicepublic", "schema-3.5", "Publication.xsd"))
	agreement(t, d, "service-public.json")

	// The stripped draft as the profile of the Service-Public dev corpus.
	cdir := spDir("dev")
	if cdir == "" {
		t.Log("no Service-Public dev corpus")
		return
	}
	p, err := LoadProfile(bytes.NewReader(stripReview(d.JSON)))
	if err != nil {
		t.Fatal(err)
	}
	files := spFiles(cdir)
	if testing.Short() {
		files = files[:min(len(files), 300)]
	}
	x := &Extractor{Profiles: []*Profile{p}, Profile: p.Name()}
	unmapped := map[string]int{}
	total, read := 0, 0
	start := time.Now()
	for _, path := range files {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		doc, err := x.ExtractFile(ctx, path, nil)
		cancel()
		if err != nil {
			t.Errorf("%s: %v", path, err)
			continue
		}
		if err := doc.Validate(); err != nil {
			t.Errorf("%s: Validate: %v", path, err)
		}
		read++
		for _, w := range doc.AllWarnings() {
			if w.Code != WarnUnmappedElements {
				continue
			}
			for _, sm := range countRE.FindAllStringSubmatch(w.Message, -1) {
				n, _ := strconv.Atoi(sm[2])
				unmapped[strings.TrimSpace(sm[1])] += n
				total += n
			}
		}
		if time.Since(start) > 90*time.Second {
			t.Logf("stopped after %d files (time)", read)
			break
		}
	}
	t.Logf("Service-Public dev with the drafted profile: %d files read in %v, %d unmapped elements (%d names): %s",
		read, time.Since(start).Round(time.Millisecond), total, len(unmapped), firstN(byCount(unmapped), 400))
}

func firstN(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

func TestDraftAkomaNtoso(t *testing.T) {
	dir := schemaDir(t)
	d := draftOf(t, filepath.Join(dir, "akn", "akomantoso30.xsd"))
	agreement(t, d, "akn.json")
}

func TestDraftJATS(t *testing.T) {
	dir := schemaDir(t)
	d := draftOf(t, filepath.Join(dir, "jats", "JATS-Publishing-1-3-MathML3-XSD", "JATS-Publishing-1-3-MathML3-XSD", "JATS-journalpublishing1-3-mathml3.xsd"))
	agreement(t, d, "jats.json")
}
