package gofastxml

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// TestOutputDigests guards changes made for speed: it reads every file of
// the dev sets of the corpus (dev/xml*/), with the profile chosen by the
// root element and with generic reading, and digests the JSON of each
// document (or its error). The first run writes the digests to
// GOFAST_DIGESTS; later runs compare with them and fail on any
// difference. It runs only when GOFAST_DIGESTS is set. The file holds
// data derived from the corpus: keep it outside the repository.
func TestOutputDigests(t *testing.T) {
	file := *digestsFlag
	if file == "" {
		file = os.Getenv("GOFAST_DIGESTS")
	}
	if file == "" {
		t.Skip("no digests file: -args -digests=FILE or GOFAST_DIGESTS")
	}
	root := corpusRoot()
	dirs, _ := filepath.Glob(filepath.Join(root, "dev", "xml*"))
	if len(dirs) == 0 {
		t.Skip("no corpus at", root)
	}
	got := map[string]string{}
	for _, dir := range dirs {
		var files []string
		filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
			if err == nil && !d.IsDir() {
				files = append(files, p)
			}
			return nil
		})
		for _, path := range files {
			rel, _ := filepath.Rel(root, path)
			for _, c := range []struct{ name, profile string }{{"auto", ""}, {"generic", ProfileNone}} {
				x := &Extractor{Profile: c.profile}
				doc, err := x.ExtractFile(context.Background(), path, nil)
				h := sha256.New()
				if err != nil {
					fmt.Fprintf(h, "error: %v", err)
				} else {
					doc.Source.Name = rel
					if err := json.NewEncoder(h).Encode(doc); err != nil {
						t.Fatal(err)
					}
				}
				got[rel+" "+c.name] = hex.EncodeToString(h.Sum(nil))
			}
			// ExtractBytes gives what ExtractFile gives.
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			h := sha256.New()
			if doc, err := ExtractBytes(context.Background(), data, nil); err != nil {
				fmt.Fprintf(h, "error: %v", err)
			} else {
				doc.Source.Name = rel
				json.NewEncoder(h).Encode(doc)
			}
			if sum := hex.EncodeToString(h.Sum(nil)); sum != got[rel+" auto"] {
				t.Errorf("%s: ExtractBytes differs from ExtractFile", rel)
			}
		}
	}
	f, err := os.Open(file)
	if os.IsNotExist(err) {
		keys := make([]string, 0, len(got))
		for k := range got {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		var b strings.Builder
		for _, k := range keys {
			fmt.Fprintf(&b, "%s %s\n", got[k], k)
		}
		if err := os.WriteFile(file, []byte(b.String()), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Logf("wrote %d digests to %s", len(got), file)
		return
	}
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	want := map[string]string{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if sum, key, ok := strings.Cut(sc.Text(), " "); ok {
			want[key] = sum
		}
	}
	diff := 0
	for k, sum := range want {
		if got[k] != sum {
			diff++
			if diff <= 20 {
				t.Errorf("%s: output differs", k)
			}
		}
	}
	for k := range got {
		if _, ok := want[k]; !ok {
			t.Errorf("%s: no digest recorded", k)
		}
	}
	t.Logf("%d digests compared, %d differ", len(want), diff)
}

// corpusRoot is GOFAST_CORPUS, or ../.testdata/corpus, or, from a
// worktree one level deeper, ../../.testdata/corpus.
func corpusRoot() string {
	if root := os.Getenv("GOFAST_CORPUS"); root != "" {
		return root
	}
	root := filepath.Join("..", ".testdata", "corpus")
	if _, err := os.Stat(root); err != nil {
		if deeper := filepath.Join("..", "..", ".testdata", "corpus"); dirExists(deeper) {
			return deeper
		}
	}
	return root
}

func dirExists(p string) bool {
	info, err := os.Stat(p)
	return err == nil && info.IsDir()
}

// digestsFlag names the file of TestOutputDigests (or GOFAST_DIGESTS).
var digestsFlag = flag.String("digests", "", "file of output digests for TestOutputDigests (written if absent)")
