package gofastxml

// Tests of the phase 1 review's findings, by number.

import (
	"context"
	"errors"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/AOShei/go-fast-core"
)

// lowTags lowers MaxTagBytes for one test.
func lowTags(t *testing.T, n int) {
	old := tagBytes
	tagBytes = n
	t.Cleanup(func() { tagBytes = old })
}

// Finding 12: a negative size is an error, not a panic.
func TestNegativeSize(t *testing.T) {
	_, err := Extract(context.Background(), strings.NewReader("<a/>"), -1, nil)
	if !errors.Is(err, gofastcore.ErrMalformed) {
		t.Errorf("err %v", err)
	}
}

// Finding 7: a first tag over the bound is a limit, not malformed input.
func TestFirstTagTooLarge(t *testing.T) {
	lowTags(t, 4096)
	src := `<root a="` + strings.Repeat("v", 5000) + `">text</root>`
	_, err := ExtractBytes(context.Background(), []byte(src), nil)
	if !errors.Is(err, gofastcore.ErrLimitExceeded) {
		t.Errorf("err %v", err)
	}
}

// Finding 15: a character reference longer than the bound says so.
func TestLongCharacterReference(t *testing.T) {
	lowTags(t, 4096)
	src := `<r><p>a</p><p>&#` + strings.Repeat("0", 10000) + `65;</p></r>`
	doc := extract(t, src, nil)
	if msg := warned(doc, gofastcore.WarnLimitExceeded); !strings.Contains(msg, "character reference") {
		t.Errorf("message %q", msg)
	}
}

// cancelAt cancels a context once a read passes an offset, and records
// how far reads went.
type cancelAt struct {
	r      *strings.Reader
	at     int64
	cancel func()
	most   int64
}

func (c *cancelAt) ReadAt(p []byte, off int64) (int, error) {
	if off > c.at {
		c.cancel()
	}
	c.most = max(c.most, off+int64(len(p)))
	return c.r.ReadAt(p, off)
}

// Finding 5: cancellation is honoured while input is passed over.
func TestCancelWhileSkipping(t *testing.T) {
	src := "<r>" + strings.Repeat("</x>", 4<<20) + "</r>"
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r := &cancelAt{r: strings.NewReader(src), at: 1 << 20, cancel: cancel}
	_, err := Extract(ctx, r, int64(len(src)), nil)
	if !errors.Is(err, context.Canceled) || r.most > 4<<20 {
		t.Errorf("err %v, read to %d of %d", err, r.most, len(src))
	}
}

// Finding 11: metadata values are cut at a character boundary and
// counted; an xml:lang too long for the language is a note.
func TestMetadataCut(t *testing.T) {
	loc := strings.Repeat("é", MaxValueBytes/2+5)
	lang := strings.Repeat("x", 70)
	src := `<r xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:schemaLocation="` + loc + `" xml:lang="` + lang + `"><p>t</p></r>`
	doc := extract(t, src, nil)
	v := doc.Metadata.Extra[ExtraSchemaLocation]
	if len(v) > MaxValueBytes || !utf8.ValidString(v) || !hasLimit(doc, "attribute value") {
		t.Errorf("schema location of %d bytes, valid %v, warnings %+v", len(v), utf8.ValidString(v), doc.Parts[0].Warnings)
	}
	if doc.Metadata.Language != "" || !strings.Contains(lines(doc), `@xml:lang "`+lang+`"`) {
		t.Errorf("language %q, blocks\n%s", doc.Metadata.Language, lines(doc))
	}
}
