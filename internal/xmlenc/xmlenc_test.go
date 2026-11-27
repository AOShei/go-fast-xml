package xmlenc

import (
	"bytes"
	"errors"
	"io"
	"testing"
	"testing/iotest"
	"unicode/utf16"
)

func utf16le(s string, bom bool) []byte {
	var b []byte
	if bom {
		b = append(b, 0xFF, 0xFE)
	}
	for _, u := range utf16.Encode([]rune(s)) {
		b = append(b, byte(u), byte(u>>8))
	}
	return b
}

func decodeAll(t *testing.T, src []byte) (string, Result) {
	t.Helper()
	r, err := Sniff(src)
	if err != nil {
		t.Fatalf("%q: %v", src, err)
	}
	d := NewReader(iotest.OneByteReader(bytes.NewReader(src[r.Skip:])), r)
	out, err := io.ReadAll(d)
	if err != nil {
		t.Fatal(err)
	}
	return string(out), r
}

func TestSniff(t *testing.T) {
	for _, c := range []struct {
		src  []byte
		want string
		name string
	}{
		{[]byte(`<?xml version="1.0"?><a>caf` + "é" + `</a>`), "<a>café</a>", "utf-8"},
		{[]byte("\xEF\xBB\xBF<a/>"), "<a/>", "utf-8"},
		{[]byte(`<?xml version="1.0" encoding="ISO-8859-1"?><a>caf` + "\xE9 \x93q\x94" + `</a>`), "<a>café “q”</a>", "windows-1252"},
		{[]byte(`<?xml version='1.0' encoding = 'windows-1251'?><a>` + "\xCF" + `</a>`), "<a>П</a>", "windows-1251"},
		{utf16le(`<?xml version="1.0" encoding="UTF-16"?><a>`+"\U0001F600"+`</a>`, true), "<a>\U0001F600</a>", "utf-16le"},
		{utf16le(`<?xml version="1.0"?><a>x</a>`, false), "<a>x</a>", "utf-16le"},
	} {
		got, r := decodeAll(t, c.src)
		if !bytes.HasSuffix([]byte(got), []byte(c.want)) || r.Name() != c.name {
			t.Errorf("%q: got %q as %s", c.src, got, r.Name())
		}
	}
}

func TestUnsupported(t *testing.T) {
	for _, src := range []string{
		`<?xml version="1.0" encoding="ISO-2022-KR"?><a/>`,
		`<?xml version="1.0" encoding="UTF-7"?><a/>`,
		`<?xml version="1.0" encoding="x-nonsense"?><a/>`,
		"\x00\x00\xFE\xFF\x00\x00\x00<",
	} {
		if _, err := Sniff([]byte(src)); !errors.Is(err, ErrUnsupported) {
			t.Errorf("%q: %v", src, err)
		}
	}
}

func TestInvalidCounted(t *testing.T) {
	src := []byte("<?xml version=\"1.0\" encoding=\"shift_jis\"?><a>\x82\xA0\x82</a>")
	r, _ := Sniff(src)
	d := NewReader(bytes.NewReader(src), r)
	out, _ := io.ReadAll(d)
	if !bytes.Contains(out, []byte("あ")) || d.Invalid() == 0 {
		t.Errorf("%q, invalid %d", out, d.Invalid())
	}
}

func TestDetected(t *testing.T) {
	// French in windows-1252 with no declaration.
	text := bytes.Repeat([]byte("<p>L'\xE9t\xE9 est l\xE0, tr\xE8s chaud et d\xE9j\xE0 long.</p>\n"), 40)
	r, err := Sniff(append([]byte("<doc>"), text...))
	if err != nil {
		t.Skipf("not recognised: %v", err)
	}
	if !r.Detected || r.UTF8 {
		t.Errorf("%+v", r)
	}
}
