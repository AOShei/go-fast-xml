package xmlread

import (
	"context"
	"strings"
	"testing"
)

// held counts the attribute bytes the open elements below the top hold.
type held struct{ most, starts int }

func (h *held) StartElement(r *Reader, e *Element) {
	h.starts++
	if v, _ := e.Attr("", "a"); len(v) != 100000 {
		h.most = -1 << 30 // the handler sees the whole value
	}
	n := 0
	for _, o := range r.Stack()[:len(r.Stack())-1] {
		for _, a := range o.Attrs {
			n += len(a.Name) + len(a.Value)
		}
	}
	h.most = max(h.most, n)
}
func (h *held) EndElement(*Reader, *Element, int64, int64)  {}
func (h *held) Text(*Reader, []byte, int64, int64)          {}
func (h *held) Comment(*Reader, []byte, bool, int64, int64) {}
func (h *held) Stop() bool                                  { return false }

// Review finding 3: open elements do not keep their attributes.
func TestOpenElementsHoldLittle(t *testing.T) {
	tag := `<e a="` + strings.Repeat("v", 100000) + `">`
	src := strings.Repeat(tag, 50) + strings.Repeat("</e>", 50)
	h := &held{}
	if err := NewBytes(context.Background(), []byte(src), Options{}).Run(h); err != nil {
		t.Fatal(err)
	}
	if h.starts != 50 || h.most != 0 {
		t.Errorf("%d starts, open elements held %d bytes", h.starts, h.most)
	}
}
