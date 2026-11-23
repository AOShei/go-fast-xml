// Copied from go-fast-odf's hygiene_test.go (same author, MIT License).

package gofastxml

import (
	"go/build"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode"
)

// TestSourceHygiene keeps invisible and look-alike characters out of the
// Go sources: a test that needs one writes it as an escape.
func TestSourceHygiene(t *testing.T) {
	err := filepath.Walk(".", func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() && (strings.HasPrefix(info.Name(), ".") && p != "." || info.Name() == "testdata") {
			return filepath.SkipDir
		}
		if info.IsDir() || filepath.Ext(p) != ".go" {
			return nil
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		line := 1
		for _, r := range string(b) {
			bad := unicode.In(r, unicode.Cf, unicode.Co, unicode.Zl, unicode.Zp) ||
				unicode.Is(unicode.Cc, r) && r != '\n' && r != '\t' ||
				unicode.Is(unicode.Zs, r) && r != ' ' || r >= 0xFB00 && r <= 0xFB06
			if bad {
				t.Errorf("%s:%d: literal U+%04X", p, line, r)
			}
			if r == '\n' {
				line++
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// TestImports keeps the module to the standard library and go-fast
// modules.
func TestImports(t *testing.T) {
	err := filepath.Walk(".", func(p string, info os.FileInfo, err error) error {
		if err != nil || !info.IsDir() || strings.HasPrefix(info.Name(), ".") && p != "." {
			if info != nil && info.IsDir() && p != "." {
				return filepath.SkipDir
			}
			return err
		}
		pkg, err := build.ImportDir(p, 0)
		if err != nil {
			return nil // no Go files here
		}
		for _, imp := range append(append(pkg.Imports, pkg.TestImports...), pkg.XTestImports...) {
			first := strings.Split(imp, "/")[0]
			if strings.Contains(first, ".") && !strings.HasPrefix(imp, "github.com/AOShei/go-fast-") {
				t.Errorf("%s imports %s", p, imp)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
