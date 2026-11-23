package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCharsetFlag(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.xml")
	// "\xc4\xe0" is "Да" in windows-1251; the file says ISO-8859-1.
	os.WriteFile(a, []byte("<?xml version=\"1.0\" encoding=\"ISO-8859-1\"?><doc><p>\xc4\xe0</p></doc>"), 0o644)
	var out, errs bytes.Buffer
	if code := run(context.Background(), []string{"-charset", "windows-1251", "-format", "text", a}, &out, &errs); code != 0 {
		t.Fatalf("exit %d: %s", code, errs.String())
	}
	if got := strings.TrimSpace(out.String()); got != "Да" {
		t.Errorf("text %q", got)
	}
	out.Reset()
	errs.Reset()
	if code := run(context.Background(), []string{"-charset", "no-such-encoding", a}, &out, &errs); code == 0 || out.Len() > 0 || !strings.Contains(errs.String(), "no-such-encoding") {
		t.Errorf("exit %d, out %q, errors %q", code, out.String(), errs.String())
	}
}
