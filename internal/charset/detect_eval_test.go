package charset

import (
	"bytes"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"unicode/utf8"
)

// TestDetectEval measures detection on a larger set than testdata/detect:
// Wikipedia articles in 30 languages, each encoded in the code pages of
// its language, read whole and in 40 windows each of 4 KiB, 1 KiB and 200
// bytes. gen/fetch_detect_eval.py writes the set; it is not kept in the
// repository (the articles are CC BY-SA).
//
//	python3 gen/fetch_detect_eval.py /tmp/detect-eval
//	GOFASTHTML_DETECT_EVAL=/tmp/detect-eval go test -run TestDetectEval -v ./internal/charset
//
// Each line gives, per size, the windows decided right (or given the same
// text), the windows not decided, and those decided wrong. A wrong answer
// in a window of 1 KiB or more fails the test.
func TestDetectEval(t *testing.T) {
	dir := os.Getenv("GOFASTHTML_DETECT_EVAL")
	if dir == "" {
		t.Skip("GOFASTHTML_DETECT_EVAL is not set")
	}
	want := map[string]string{"cp932": "shift_jis", "gbk": "gbk", "gb18030": "gbk", "cp950": "big5", "cp949": "euc-kr",
		"cp1251": "windows-1251", "cp1252": "windows-1252", "utf_16_le": "utf-16le", "utf_16_be": "utf-16be", "utf8bad": "utf-8"}
	files, _ := filepath.Glob(filepath.Join(dir, "*"))
	sort.Strings(files)
	rng := rand.New(rand.NewSource(1))
	wrongTotal, wrongBig := 0, 0
	for _, f := range files {
		b, _ := os.ReadFile(f)
		ext := f[strings.LastIndex(f, ".")+1:]
		w := want[ext]
		if w == "" {
			w = "unknown"
		}
		var line []string
		for _, size := range []int{0, 4096, 1024, 200} {
			ok, unk, wrong := 0, 0, map[string]int{}
			trials := 1
			if size > 0 {
				trials = 40
			}
			for k := 0; k < trials; k++ {
				s := b
				if size > 0 && len(b) > size {
					o := rng.Intn(len(b) - size)
					if ext == "utf_16_le" || ext == "utf_16_be" {
						o &^= 1
					}
					if ext != "utf_16_le" && ext != "utf_16_be" {
						for o > 0 && b[o-1] >= 0x40 {
							o--
						}
					}
					s = b[o : o+size]
				}
				if utf8.Valid(s) && bytes.IndexByte(s, 0) < 0 {
					ok++ // nothing to detect
					continue
				}
				r := SniffText(s, "")
				got := r.Enc.String()
				if r.Unknown {
					got = "unknown"
				}
				same := false
				if te, ok := map[string]Encoding{"cp1250": Windows1250, "cp1253": Windows1253, "cp1254": Windows1254, "cp1255": Windows1255, "cp1256": Windows1256, "cp1258": Windows1258, "cp874": Windows874}[ext]; ok && !r.Unknown {
					a, _, _, _ := Decode(s, Result{Enc: te}, 1<<30)
					g, _, _, _ := Decode(s, r, 1<<30)
					same = string(a) == string(g)
				}
				switch {
				case got == w || same:
					ok++
				case got == "unknown":
					unk++
				default:
					wrong[got]++
					if size == 0 || size >= 1024 {
						wrongBig++
						if te, ok := decodable[ext]; ok {
							a, _, _, _ := Decode(s, Result{Enc: te}, 1<<30)
							g, _, _, _ := Decode(s, r, 1<<30)
							t.Logf("  %s, %d bytes: %s", ext, len(s), runeDiff(string(a), string(g)))
						}
					}
					wrongTotal++
				}
			}
			line = append(line, fmt.Sprintf("%5d:%2d/%2d/%v", size, ok, unk, wrong))
		}
		t.Logf("%-18s want %-12s %s", filepath.Base(f), w, strings.Join(line, "  "))
	}
	t.Logf("decided wrong: %d, of which in 1 KiB or more: %d", wrongTotal, wrongBig)
	if wrongBig > 0 {
		t.Errorf("%d inputs of 1 KiB or more decided wrong", wrongBig)
	}
}

// runeDiff lists the characters of want that got has otherwise, as
// want→got, each pair once.
func runeDiff(want, got string) string {
	w, g := []rune(want), []rune(got)
	seen := map[[2]rune]bool{}
	var out []string
	for i := range min(len(w), len(g)) {
		if p := [2]rune{w[i], g[i]}; w[i] != g[i] && !seen[p] {
			seen[p] = true
			out = append(out, string(w[i])+"→"+string(g[i]))
		}
	}
	return strings.Join(out, " ")
}
