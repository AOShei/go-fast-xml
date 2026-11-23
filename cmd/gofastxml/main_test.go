package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AOShei/go-fast-core"
)

func TestCommand(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.xml")
	os.WriteFile(a, []byte(`<doc id="d"><title>Title</title><!-- a note --><p>Body text.</p></doc>`), 0o644)
	os.WriteFile(filepath.Join(dir, "b.xml"), []byte(`<doc><p>Second.</p></doc>`), 0o644)
	os.WriteFile(filepath.Join(dir, "bad.xml"), []byte("not a document"), 0o644)
	os.WriteFile(filepath.Join(dir, "skip.txt"), []byte("<doc>ignored</doc>"), 0o644)

	var out, errs bytes.Buffer
	if code := run(context.Background(), []string{a}, &out, &errs); code != 0 {
		t.Fatalf("exit %d: %s", code, errs.String())
	}
	var doc gofastcore.Document
	if err := json.Unmarshal(out.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	if err := doc.Validate(); err != nil || doc.Source.Name != "a.xml" || doc.Parts[0].Blocks[0].Text != "d" || doc.Parts[0].Blocks[1].Text != "Title" {
		t.Errorf("doc %+v %v", doc.Source, err)
	}

	out.Reset()
	errs.Reset()
	if code := run(context.Background(), []string{"-attributes=false", "-format", "text", a}, &out, &errs); code != 0 {
		t.Fatalf("exit %d: %s", code, errs.String())
	}
	if got := out.String(); strings.TrimSpace(got) != "Title\n\nBody text." {
		t.Errorf("text %q", got)
	}

	out.Reset()
	if code := run(context.Background(), []string{"-attributes=false", "-format", "text", "-stored=markup_comment", a}, &out, &errs); code != 0 || !strings.Contains(out.String(), "a note") {
		t.Errorf("exit %d, stored %q", code, out.String())
	}

	out.Reset()
	errs.Reset()
	code := run(context.Background(), []string{"-format", "markdown", dir}, &out, &errs)
	if code != 1 || !strings.Contains(errs.String(), "bad.xml") {
		t.Errorf("exit %d, stderr %q", code, errs.String())
	}
	if md := out.String(); !strings.Contains(md, "Title") || !strings.Contains(md, "Second.") || strings.Contains(md, "ignored") {
		t.Errorf("markdown %q", md)
	}

	outFile := filepath.Join(dir, "out.json")
	if code := run(context.Background(), []string{"-o", outFile, a}, &out, &errs); code != 0 {
		t.Errorf("-o: exit %d", code)
	}
	if b, err := os.ReadFile(outFile); err != nil || !bytes.Contains(b, []byte(`"format": "xml"`)) {
		t.Errorf("-o wrote %q, %v", b, err)
	}

	for _, bad := range [][]string{{}, {"-format", "pdf", a}, {"-exclude-display=nonsense", a}, {"-stored=nonsense", a}} {
		if code := run(context.Background(), bad, &out, &errs); code != 2 {
			t.Errorf("%v: exit %d", bad, code)
		}
	}
}
