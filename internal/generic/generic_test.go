package generic

import (
	"context"
	"strconv"
	"strings"
	"testing"

	"github.com/AOShei/go-fast-core"
	"github.com/AOShei/go-fast-xml/internal/xmlread"
)

// run reads src with the generic consumer.
func run(t *testing.T, src string) (*Generic, *gofastcore.Part) {
	t.Helper()
	ctx, budget := gofastcore.WithTextBudget(context.Background(), gofastcore.Limits{})
	defer budget.Close()
	part := &gofastcore.Part{Kind: gofastcore.PartBody, Number: 1}
	g := New(&Out{Part: part, Budget: budget}, Options{})
	rd := xmlread.NewBytes(ctx, []byte(src), xmlread.Options{})
	if err := rd.Run(g); err != nil {
		t.Fatal(err)
	}
	g.Finish(rd.Offset())
	return g, part
}

// Review finding 1: attribute notes of nested elements, with no end tag
// yet, do not pile up past the queue's bound.
func TestQueueBoundAtStart(t *testing.T) {
	var b strings.Builder
	for range 40 {
		b.WriteString("<e")
		for j := range 1000 {
			b.WriteString(" a" + strconv.Itoa(j) + `="x"`)
		}
		b.WriteString(">")
	}
	b.WriteString("t")
	b.WriteString(strings.Repeat("</e>", 40))
	g, part := run(t, b.String())
	if g.peak > MaxPendingBlocks+1000 {
		t.Errorf("queue reached %d blocks", g.peak)
	}
	if len(part.Blocks) != 40*1000+1 {
		t.Errorf("%d blocks", len(part.Blocks))
	}
}

// Review finding 16: a path is made without recursion at full depth.
func TestDeepPath(t *testing.T) {
	src := strings.Repeat("<a>", 500) + "t" + strings.Repeat("</a>", 500)
	_, part := run(t, src)
	if len(part.Blocks) != 1 || len(part.Blocks[0].Attrs[AttrElement]) > MaxPathBytes {
		t.Errorf("%d blocks", len(part.Blocks))
	}
}
