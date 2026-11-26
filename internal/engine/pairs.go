package engine

import (
	"strings"

	"github.com/AOShei/go-fast-core"
	"github.com/AOShei/go-fast-xml/internal/generic"
	"github.com/AOShei/go-fast-xml/internal/profile"
	"github.com/AOShei/go-fast-xml/internal/xmlread"
)

// pairTarget is kept on the frame of a "pairs" table, or of a row (or
// one_row table) with a "rest_table": where the names and values read
// inside it go. rest: the row's "rest_table", nil for a "pairs" table;
// first: the first row of the rest table that belongs to the row being
// read (its id is written when the row ends).
type pairTarget struct {
	t     *table
	local bool
	// parent: names start with the element the table's element stands
	// in ("pairs": "parent").
	parent bool
	// empty: an element with no content and no attributes is a row with
	// an empty value ("pairs_empty").
	empty bool
	rest  *profile.RestTable
	first int
}

// pairScope returns the index of the frame of the "pairs" table or
// "rest_table" row the element on top of the stack is read in, or -1:
// only elements read as names and values stand between them.
func (e *Engine) pairScope() int {
	for i := len(e.frames) - 2; i >= 0; i-- {
		f := &e.frames[i]
		if f.pt != nil {
			return i
		}
		if !f.pair {
			return -1
		}
	}
	return -1
}

// keepsRule reports whether an element inside a names-and-values scope is
// read by its own rule: a "document" rule (an attachment), and in a row
// a "column" cell. Every other element is a name and value.
func (e *Engine) keepsRule(rule *profile.Rule, at int) bool {
	if rule == nil {
		return false
	}
	return rule.As == profile.AsDocument ||
		rule.As == profile.AsCell && rule.Column > 0 && e.frames[at].pt.rest != nil
}

// beforeChild is called when an element starts inside a pair element p
// whose value has no text yet: p holds elements, not a value, and its
// attributes are rows where it stands. It reports false when p already
// has text (mixed content: the element is read into p's value).
func (e *Engine) beforeChild(p *frame) bool {
	if !p.pair || p.sink == nil || e.cur != p.sink {
		return true
	}
	if p.sink.b.Len() > 0 {
		return false
	}
	s := e.closeSink(p)
	for _, b := range s.after {
		e.add(b, p, false)
	}
	e.putSink(s)
	if p.rule == nil {
		p.role = roleContainer
	}
	e.pairAttrRows(p, p.tagEnd)
	return true
}

// startPair reads the element on top of the stack as a name and value of
// the scope at frame index at.
func (e *Engine) startPair(f *frame, el *xmlread.Element, at int) {
	f.pair, f.pairAt, f.tagEnd = true, at, el.End
	f.pairName = e.pairName(at, e.frames[at].pt.local)
	e.pairAttrs(f, el)
	if e.cur == nil {
		e.flushStray()
	}
	e.openSink(f, skCell, el.Start)
}

// pairAttrs keeps the element's attributes, each a row of its own after
// the element's value, named <name>/@<attribute>.
func (e *Engine) pairAttrs(f *frame, el *xmlread.Element) {
	for i := range el.Attrs {
		a := &el.Attrs[i]
		if a.Value == "" || generic.Skipped(el, a) {
			continue
		}
		v := e.clean(a.Value)
		if v == "" {
			continue
		}
		n := generic.CleanName(generic.CutString(a.Local, generic.MaxNameBytes), &e.scratch)
		f.pairAttrs = append(f.pairAttrs, kv{f.pairName + "/@" + n, v})
	}
}

// pairName is the name of the element on top of the stack: its local
// name, or ("path") its path of local names. In a "pairs" table the path
// starts with the table's own element (SellerTradeParty/Name), which is
// what tells one party's table from another's in rendered text; in a
// "rest_table" it starts below the row's element, whose id is the row's
// first cell. The scope's own element is named by its local name. With
// "pairs": "parent" every name has the name of the table's parent
// element before it (ProcurementProject/ID), for a table that is one
// value of a record whose other values are read by rules of their own.
func (e *Engine) pairName(at int, local bool) string {
	n := len(e.frames)
	parent := e.frames[at].pt.parent && at > 0
	if local || at == n-1 && !parent {
		return generic.CleanName(generic.CutString(e.frames[n-1].name.Local, generic.MaxNameBytes), &e.scratch)
	}
	var b strings.Builder
	from := at + 1
	if e.frames[at].pt.rest == nil {
		from = at
	}
	if parent {
		from = at - 1
	}
	for i := from; i < n; i++ {
		s := generic.CutString(e.frames[i].name.Local, generic.MaxNameBytes)
		if b.Len()+len(s)+1 > generic.MaxPathBytes {
			e.out.PathsCut++
			break
		}
		if b.Len() > 0 {
			b.WriteByte('/')
		}
		b.WriteString(s)
	}
	return generic.CleanName(b.String(), &e.scratch)
}

// endPair writes a pair element's value and its attributes as rows.
func (e *Engine) endPair(f *frame, end int64) {
	if f.sink != nil {
		s := e.closeSink(f)
		if text := s.b.String(); text != "" || len(f.pairAttrs) == 0 && e.frames[f.pairAt].pt.empty {
			e.pairRow(f, f.pairName, text, end)
		}
		for _, b := range s.after {
			e.add(b, f, false)
		}
		e.putSink(s)
	}
	e.pairAttrRows(f, end)
}

// pairAttrRows writes the attributes kept by pairAttrs, cited at the
// element (end: its end, or the end of its start tag when it holds
// elements).
func (e *Engine) pairAttrRows(f *frame, end int64) {
	for _, a := range f.pairAttrs {
		e.pairRow(f, a.k, a.v, end)
	}
	f.pairAttrs = nil
}

// pairRow writes one row of name and value, cited at the element's
// place: to a "pairs" table, or to a row's rest table, its id cell to be
// filled when the row ends.
func (e *Engine) pairRow(f *frame, name, value string, end int64) {
	pt := e.frames[f.pairAt].pt
	t := pt.t
	t.startRow(false)
	c := 0
	if pt.rest != nil && pt.rest.ID > 0 {
		t.columnCell(0, "", "")
		c = 1
	}
	t.columnCell(c, name, "")
	t.columnCell(c+1, value, "")
	t.endRow()
	t.rowDone(f.start, max(end, f.start), "")
}

// startRest opens the rest table of the row (or one_row table) f, owned
// by the table t.
func (e *Engine) startRest(f *frame, t *table) {
	rt := f.rule.RestTable
	if t.rest == nil {
		t.rest = newTable(e, "rows")
		if len(rt.Header) > 0 {
			e.headerRow(t.rest, rt.Header)
		}
	}
	f.pt = &pairTarget{t: t.rest, local: rt.Names == "local", rest: rt, first: len(t.rest.rows)}
}

// endRest writes the row's id (the text of its "id" column) in the first
// cell of the rows its rest table took from it. Call before the row ends.
func (e *Engine) endRestTable(f *frame, row *table) {
	pt := f.pt
	if pt == nil || pt.rest == nil || pt.rest.ID == 0 {
		return
	}
	id := ""
	if c := pt.rest.ID - 1; row.inRow && c < len(row.row) {
		id = row.row[c]
	}
	if id == "" {
		return
	}
	t := pt.t
	for k := pt.first; k < len(t.rows); k++ {
		if len(t.rows[k]) > 0 {
			t.rows[k][0] = id
			t.cost += int64(len(id))
		}
	}
	t.check()
}

// restBlock is the rest table of t, cited from its first row's element
// to its last's.
func (e *Engine) restBlock(t *table) (gofastcore.Block, bool) {
	var start, end int64 = -1, 0
	for _, l := range t.locs {
		if l.Length == 0 && l.Offset == 0 {
			continue
		}
		if start < 0 {
			start = l.Offset
		}
		end = max(end, l.Offset+l.Length)
	}
	if start < 0 {
		return gofastcore.Block{}, false
	}
	return t.block(e, start, end)
}
