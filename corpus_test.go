package gofastxml

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/AOShei/go-fast-core"
)

// TestCorpus runs the extractor over the public corpus (GOFAST_CORPUS, or
// ../.testdata/corpus): every file under dev/xml*/ and heldout/xml*/,
// each with a time limit and bounded budgets. It logs one tally per set
// (counts only: held-out files are scored, never looked at) and fails on
// a panic, a Validate failure or a timeout. The whole run stops after two
// minutes. Its shape follows go-fast-odf's corpus test.
func TestCorpus(t *testing.T) {
	root := os.Getenv("GOFAST_CORPUS")
	if root == "" {
		root = filepath.Join("..", ".testdata", "corpus")
	}
	if _, err := os.Stat(root); err != nil {
		t.Skip("no corpus at", root)
	}
	deadline := time.Now().Add(2 * time.Minute)
	for _, set := range []string{"dev", "heldout"} {
		dirs, _ := filepath.Glob(filepath.Join(root, set, "xml*"))
		tally := newTally()
		for _, dir := range dirs {
			format := filepath.Base(dir)
			var files []string
			filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
				if err == nil && !d.IsDir() {
					files = append(files, p)
				}
				return nil
			})
			sort.Strings(files)
			for _, path := range files {
				if time.Now().After(deadline) {
					tally.counts["not run (time)"]++
					continue
				}
				res := corpusOne(path)
				tally.add(format, res)
				label := path
				if set == "heldout" {
					label = format + " (held-out file)"
				}
				switch {
				case res.panic != "":
					t.Errorf("%s: panic: %s", label, firstLine(res.panic, set))
				case res.timeout:
					t.Errorf("%s: no result within the time limit", label)
				case res.invalid != nil:
					t.Errorf("%s: Validate: %s", label, firstLine(res.invalid.Error(), set))
				}
				if set == "dev" && testing.Verbose() && res.err != nil {
					t.Logf("ERROR %s: %v", path, res.err)
				}
				if set == "dev" && testing.Verbose() && res.doc != nil {
					for _, w := range res.doc.AllWarnings() {
						if w.Severity == gofastcore.SeverityError {
							t.Logf("%s %s: %s", w.Code, path, w.Message)
						}
					}
				}
			}
		}
		if len(dirs) == 0 {
			t.Logf("%s: no xml directories", set)
			continue
		}
		t.Logf("%s:\n%s", set, tally)
	}
}

func firstLine(s, set string) string {
	if set == "heldout" {
		// A message can quote the file; held-out files are not shown.
		if i := strings.IndexByte(s, ':'); i > 0 {
			return s[:i]
		}
	}
	if i := strings.IndexByte(s, '\n'); i > 0 {
		return s[:i]
	}
	return s
}

type corpusResult struct {
	doc     *gofastcore.Document
	err     error
	panic   string
	timeout bool
	invalid error
	took    time.Duration
}

func corpusOne(path string) corpusResult {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	done := make(chan corpusResult, 1)
	start := time.Now()
	go func() {
		var res corpusResult
		defer func() {
			if p := recover(); p != nil {
				res.panic = fmt.Sprintf("%v\n%s", p, debug.Stack())
			}
			done <- res
		}()
		opts := &gofastcore.Options{Limits: gofastcore.Limits{MaxTextBytes: 32 << 20, MaxDecodedBytes: 256 << 20}}
		res.doc, res.err = ExtractFile(ctx, path, opts)
		if res.err == nil {
			res.invalid = res.doc.Validate()
		}
	}()
	select {
	case res := <-done:
		if errors.Is(res.err, context.DeadlineExceeded) {
			res.timeout = true
		}
		res.took = time.Since(start)
		return res
	case <-time.After(15 * time.Second):
		return corpusResult{timeout: true}
	}
}

type tally struct {
	files    map[string]int
	counts   map[string]int
	blocks   map[string]int
	warnings map[string]int
	took     time.Duration
}

func newTally() *tally {
	return &tally{files: map[string]int{}, counts: map[string]int{}, blocks: map[string]int{}, warnings: map[string]int{}}
}

func (t *tally) add(format string, r corpusResult) {
	t.files[format]++
	t.took += r.took
	switch {
	case r.panic != "":
		t.counts["panic"]++
	case r.timeout:
		t.counts["timeout"]++
	case r.err != nil:
		kind := "other"
		for _, e := range []error{gofastcore.ErrMalformed, gofastcore.ErrUnsupported, gofastcore.ErrLimitExceeded} {
			if errors.Is(r.err, e) {
				kind = strings.TrimPrefix(e.Error(), "gofastcore: ")
			}
		}
		t.counts["error: "+kind]++
	case r.invalid != nil:
		t.counts["validate failure"]++
	default:
		t.counts["ok"]++
	}
	if r.doc == nil {
		return
	}
	for _, b := range r.doc.AllBlocks() {
		key := b.Kind.String()
		switch {
		case b.Attrs[gofastcore.AttrStored] != "":
			key = "stored " + b.Attrs[gofastcore.AttrStored]
		case b.Attrs[gofastcore.AttrNoteType] != "":
			key = "note " + b.Attrs[gofastcore.AttrNoteType]
		}
		t.blocks[key]++
		if b.Display != 0 {
			t.blocks["display "+b.Display.String()]++
		}
	}
	for _, w := range r.doc.AllWarnings() {
		t.warnings[w.Code]++
	}
}

func (t *tally) String() string {
	var sb strings.Builder
	line := func(title string, m map[string]int) {
		keys := make([]string, 0, len(m))
		for k := range m {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		fmt.Fprintf(&sb, "  %s:", title)
		for _, k := range keys {
			fmt.Fprintf(&sb, " %s=%d", k, m[k])
		}
		sb.WriteString("\n")
	}
	line("files", t.files)
	line("results", t.counts)
	line("blocks", t.blocks)
	line("warnings by code", t.warnings)
	fmt.Fprintf(&sb, "  time: %v\n", t.took.Round(time.Millisecond))
	return sb.String()
}
