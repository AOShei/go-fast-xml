package engine

import (
	"slices"
	"strconv"
	"strings"

	"github.com/AOShei/go-fast-core"
	"github.com/AOShei/go-fast-xml/internal/generic"
	"github.com/AOShei/go-fast-xml/internal/profile"
	"github.com/AOShei/go-fast-xml/internal/xmlread"
)

// table builds the grid of one table, in the rows model (rows and cells
// with spans by attribute) or the CALS model (tgroup, colspec, spanspec,
// thead, tbody, tfoot, row, entry).
type table struct {
	// locs has one entry per row of rows: where the row's element is in
	// the file and the anchor it carries, zero for a row no element of the
	// file made (a header row the profile gives). locPending: the last
	// row's element has not ended yet.
	locs       []gofastcore.Location
	locPending bool
	anyLoc     bool
	model      string
	rows       [][]string
	merges     []gofastcore.Merge
	cells      []gofastcore.CellDetail
	header     int  // leading header rows
	body       bool // a row that is not a header row came
	// The row being read: declared a header row (counted at its start),
	// its cells, and whether all of them are header cells (a row of th
	// among the leading rows is a header row, counted at its end).
	rowHead         bool
	rowCells        int
	rowAllHeadCells bool
	occ             []int
	row             []string
	inRow           bool
	col             int
	cs, rs          int
	ncols           int
	// CALS
	colnames map[string]int
	spans    map[string][2]int
	nextCol  int
	inHead   bool
	cut      bool
	// cost is what the table's block will cost the text budget so far:
	// the text of its cells, and core's cell cost for every cell of the
	// grid it will be, each row padded to the widest (ncols). It grows as
	// rows end and as the width grows, so that a table the budget cannot
	// take stops before its grid is padded (over).
	cost int64
	over bool
	e    *Engine
	// fromProfile: the header row is the profile's fixed text. attrNames
	// are the attributes rows were made from, in order of first use.
	fromProfile bool
	attrNames   []string
	// keys are the values of a cell rule's "column_key" met so far, in
	// order of first appearance; key i has column keyBase+i, and its
	// header cell is the key as written.
	keys    []string
	keyBase int
}

// keyColumn returns the column (0-based) of a "column_key" cell whose
// attribute has the value key, the keyed columns starting at base: a new
// key takes the next column. Past MaxTableColumns it returns -1 and the
// cut is counted (limit.exceeded).
func (t *table) keyColumn(e *Engine, base int, key string) int {
	if len(t.keys) == 0 {
		t.keyBase = base
		t.cost += int64(t.ncols) * gofastcore.CellCost // the header row
	}
	if i := slices.Index(t.keys, key); i >= 0 {
		return t.keyBase + i
	}
	if t.keyBase+len(t.keys) >= MaxTableColumns {
		if !t.cut {
			t.cut = true
			e.ColumnsCut++
		}
		return -1
	}
	t.keys = append(t.keys, key)
	t.cost += int64(len(key)) + gofastcore.CellCost
	return t.keyBase + len(t.keys) - 1
}

// keyHeader puts the keys in the table's header row: the profile's, when
// it has one, else a header row of their own before the first row, whose
// cells before the keyed columns are empty.
func (t *table) keyHeader() {
	if len(t.keys) == 0 {
		return
	}
	width := max(t.ncols, t.keyBase+len(t.keys))
	if !t.fromProfile {
		t.rows = slices.Insert(t.rows, 0, make([]string, width))
		t.locs = slices.Insert(t.locs, 0, gofastcore.Location{})
		t.header++
		for i := range t.merges {
			t.merges[i].Row++
		}
		for i := range t.cells {
			t.cells[i].Row++
		}
	}
	h := t.rows[0]
	for len(h) < width {
		h = append(h, "")
	}
	for i, k := range t.keys {
		h[t.keyBase+i] = k
	}
	t.rows[0] = h
	t.ncols = max(t.ncols, width)
}

// attrRow fills the row just started from its element's attributes
// (a row rule's "cells"); an absent attribute gives an empty cell.
func (e *Engine) attrRow(t *table, cells []*profile.Value) {
	for _, cv := range cells {
		v, _ := e.value(cv)
		t.startCell(e, 1, 1, false)
		t.endCell(v, "")
		if n := cv.Attr.Local; !slices.Contains(t.attrNames, n) && len(t.attrNames) < profile.MaxCells {
			t.attrNames = append(t.attrNames, n)
		}
	}
	t.endRow()
	e.tableCost(t)
}

// headerRow gives the table the profile's fixed header row.
func (e *Engine) headerRow(t *table, text []string) {
	t.startRow(true)
	for _, h := range text {
		t.startCell(e, 1, 1, true)
		t.endCell(e.clean(h), "")
	}
	t.endRow()
	t.fromProfile = true
	e.tableCost(t)
}

// tableCost stops reading when the cells held so far are more than the
// text budget has left: a row made from attributes holds more than its
// element's bytes.
func (e *Engine) tableCost(t *table) { t.check() }

// check stops reading (limit.exceeded through the spent budget) when the
// table's block would cost more than the text budget has left.
func (t *table) check() {
	if t.over || t.e == nil {
		return
	}
	if t.cost > t.e.out.Budget.Left() {
		t.over = true
		t.e.out.Budget.Exhaust()
		t.e.out.Stopped = true
	}
}

// widen records that the table is now n columns wide: every row already
// held will be padded to n.
func (t *table) widen(n int) {
	if n <= t.ncols {
		return
	}
	t.cost += int64(n-t.ncols) * int64(len(t.rows)) * gofastcore.CellCost
	t.ncols = n
	t.check()
}

func newTable(e *Engine, model string) *table {
	return &table{model: model, e: e}
}

// tableOf returns the table of the innermost open table element.
func (e *Engine) tableOf() *table {
	for i := len(e.frames) - 1; i >= 0; i-- {
		if t := e.frames[i].table; t != nil {
			return t
		}
		if e.frames[i].cell {
			return nil
		}
	}
	return nil
}

func (t *table) startRow(header bool) {
	if t.inRow {
		t.endRow()
	}
	t.locPending = false
	t.inRow, t.col, t.row = true, 0, t.row[:0]
	t.rowHead, t.rowCells, t.rowAllHeadCells = header && !t.body, 0, true
	if t.rowHead {
		t.header++
	}
}

func (t *table) occupied(c int) bool {
	return c < len(t.occ) && t.occ[c] >= len(t.rows)
}

// startCell places a cell spanning cs columns and rs rows at the next
// free column, or at col when col >= 0.
func (t *table) startCell(e *Engine, cs, rs int, header bool) {
	t.place(e, -1, cs, rs)
	t.rowCells++
	t.rowAllHeadCells = t.rowAllHeadCells && header
}

func (t *table) place(e *Engine, at, cs, rs int) {
	if !t.inRow {
		t.startRow(false)
	}
	if at >= 0 && at >= t.col {
		t.col = at
	}
	for t.occupied(t.col) {
		t.col++
	}
	cs = min(max(cs, 1), MaxTableColumns)
	if rs > MaxRowSpan {
		e.RowSpanCut++
	}
	rs = min(max(rs, 1), MaxRowSpan)
	// A span (namest/nameend, spanname, colspan) over a column a cell
	// from a row above still holds is clipped at that column: no cell is
	// in two merges.
	for c := t.col + 1; c < t.col+cs && c < len(t.occ); c++ {
		if t.occupied(c) {
			cs = c - t.col
			break
		}
	}
	t.cs, t.rs = cs, rs
	if t.col+cs > MaxTableColumns {
		if !t.cut {
			t.cut = true
			e.ColumnsCut++
		}
		t.cs = max(MaxTableColumns-t.col, 0)
	}
}

func (t *table) endCell(text, href string) {
	if t.cs <= 0 {
		return // past MaxTableColumns
	}
	for len(t.row) < t.col+t.cs {
		t.row = append(t.row, "")
	}
	for len(t.occ) < t.col+t.cs {
		t.occ = append(t.occ, -1)
	}
	t.row[t.col] = text
	t.cost += int64(len(text))
	r := len(t.rows)
	for c := t.col; c < t.col+t.cs; c++ {
		t.occ[c] = r + t.rs - 1
	}
	if t.cs > 1 || t.rs > 1 {
		t.merges = append(t.merges, gofastcore.Merge{Row: r, Col: t.col, RowSpan: t.rs, ColSpan: t.cs})
	}
	if href != "" {
		t.cells = append(t.cells, gofastcore.CellDetail{Row: r, Col: t.col, Href: href})
	}
	t.col += t.cs
	t.cs = 0
	t.widen(t.col)
}

// rowFrame returns the frame of the innermost open row of the rows model
// the element on top of the stack stands in (at any depth), or nil when a
// cell or a table comes first.
func (e *Engine) rowFrame() *frame {
	for i := len(e.frames) - 2; i >= 0; i-- {
		f := &e.frames[i]
		if f.rule != nil && (f.rule.As == profile.AsRow || f.rule.OneRow) && f.owner != nil {
			return f
		}
		if f.table != nil || f.cell {
			return nil
		}
	}
	return nil
}

// columnCell writes a column cell's text (a cell rule's "column", a row's
// "rest") at column c of the row being read; a second text for one column
// of a row follows the first after a line break. An empty text adds
// nothing.
func (t *table) columnCell(c int, text, href string) {
	if !t.inRow || c < 0 || c >= MaxTableColumns {
		return
	}
	for len(t.row) <= c {
		t.row = append(t.row, "")
	}
	t.rowCells++
	t.rowAllHeadCells = false
	if text == "" {
		t.widen(len(t.row))
		return
	}
	if t.row[c] != "" {
		t.row[c] += "\n"
		t.cost++
	}
	t.row[c] += text
	t.cost += int64(len(text))
	if href != "" {
		t.cells = append(t.cells, gofastcore.CellDetail{Row: len(t.rows), Col: c, Href: href})
	}
	t.widen(len(t.row))
}

func (t *table) endRow() {
	if !t.inRow {
		return
	}
	t.inRow = false
	if t.over {
		return
	}
	if !t.rowHead {
		if t.rowCells > 0 && t.rowAllHeadCells && !t.body {
			t.header++
		} else {
			t.body = true
		}
	}
	for c := len(t.row); c < len(t.occ); c++ {
		if t.occupied(c) {
			t.row = append(t.row, "")
		}
	}
	t.widen(len(t.row))
	t.cost += int64(t.ncols) * gofastcore.CellCost
	t.check()
	if t.over {
		return
	}
	t.rows = append(t.rows, append([]string(nil), t.row...))
	t.locs, t.locPending = append(t.locs, gofastcore.Location{}), true
}

// rowDone records where the element of the row just ended is in the file,
// and its anchor.
func (t *table) rowDone(start, end int64, anchor string) {
	if !t.locPending || len(t.locs) == 0 {
		return
	}
	t.locPending = false
	t.locs[len(t.locs)-1] = gofastcore.Location{Offset: start, Length: max(end-start, 0), Anchor: anchor}
	t.cost += int64(len(anchor))
	t.anyLoc = true
}

// block returns the table's block; ok is false for a table with no cells,
// and for one the text budget cannot take (reading has stopped).
func (t *table) block(e *Engine, start, end int64) (gofastcore.Block, bool) {
	t.endRow()
	if t.over || e.out.Stopped || len(t.rows) == 0 || t.ncols == 0 {
		return gofastcore.Block{}, false
	}
	if t.fromProfile && len(t.rows) == 1 {
		// Only the profile's header row: no row of the file (a UBL
		// TaxTotal with no TaxSubtotal) makes no table.
		return gofastcore.Block{}, false
	}
	t.keyHeader()
	for i, r := range t.rows {
		for len(r) < t.ncols {
			r = append(r, "")
		}
		t.rows[i] = r
	}
	n := len(t.rows)
	merges := t.merges[:0]
	for _, m := range t.merges {
		m.RowSpan = min(m.RowSpan, n-m.Row)
		m.ColSpan = min(m.ColSpan, t.ncols-m.Col)
		if m.RowSpan > 1 || m.ColSpan > 1 {
			merges = append(merges, m)
		}
	}
	merges = disjoint(merges, n, t.ncols)
	tb := &gofastcore.Table{Rows: t.rows, HeaderRows: min(t.header, n), Merges: merges}
	if e.opts.Spans {
		tb.Cells = t.cells
	}
	if t.anyLoc && len(t.locs) == n {
		tb.RowLocs = t.locs
	}
	b := gofastcore.Block{Kind: gofastcore.KindTable, Table: tb,
		Loc: gofastcore.Location{Offset: start, Length: max(end-start, 0)}}
	if t.fromProfile || len(t.attrNames) > 0 || len(t.keys) > 0 {
		b.Attrs = map[string]string{}
	}
	if len(t.keys) > 0 && !t.fromProfile {
		// The header row holds the keys, attribute values of the file.
		b.Attrs[AttrHeaderSource] = "attribute"
	}
	if t.fromProfile {
		// The header row is the profile's text, not the file's; the
		// table is still native: every row and cell is declared by the
		// file, nothing is guessed (core's inferred).
		b.Attrs[AttrHeaderSource] = "profile"
	}
	if len(t.keys) > 0 && t.fromProfile {
		// The profile's header text, then the keys.
		b.Attrs[AttrHeaderSource] = "profile attribute"
	}
	if len(t.attrNames) > 0 {
		b.Attrs[generic.AttrAttribute] = strings.Join(t.attrNames, " ")
	}
	return b, true
}

// disjoint returns the merges with none sharing a cell with one before it:
// placing clips spans at occupied columns, and this check makes sure. A
// merge that would share a cell is left out (its cells stay plain).
func disjoint(merges []gofastcore.Merge, rows, cols int) []gofastcore.Merge {
	if len(merges) < 2 {
		return merges
	}
	taken := make([]uint64, (rows*cols+63)/64)
	out := merges[:0]
	for _, m := range merges {
		free := true
		for r := m.Row; r < m.Row+m.RowSpan && free; r++ {
			for c := m.Col; c < m.Col+m.ColSpan; c++ {
				if i := r*cols + c; taken[i/64]&(1<<(i%64)) != 0 {
					free = false
					break
				}
			}
		}
		if !free {
			continue
		}
		for r := m.Row; r < m.Row+m.RowSpan; r++ {
			for c := m.Col; c < m.Col+m.ColSpan; c++ {
				i := r*cols + c
				taken[i/64] |= 1 << (i % 64)
			}
		}
		out = append(out, m)
	}
	return out
}

// calsTable returns the innermost open CALS table when the element just
// started is directly in its structure.
func (e *Engine) calsTable() *table {
	for i := len(e.frames) - 2; i >= 0; i-- {
		f := &e.frames[i]
		if f.table != nil {
			if f.table.model == "cals" {
				return f.table
			}
			return nil
		}
		if f.cals == "" || f.cals == "entry" {
			return nil
		}
	}
	return nil
}

// calsStart reads an element of the CALS model. It reports false for an
// element that is not one, which is read by its own rule.
func (e *Engine) calsStart(t *table, f *frame, el *xmlread.Element) bool {
	attr := func(n string) string { v, _ := el.Attr("", n); return strings.TrimSpace(v) }
	switch el.Local {
	case "tgroup":
		t.colnames, t.spans, t.nextCol = map[string]int{}, map[string][2]int{}, 0
		if t.inRow {
			t.endRow()
		}
	case "colspec":
		c := t.nextCol
		if n, err := strconv.Atoi(attr("colnum")); err == nil && n >= 1 && n <= MaxTableColumns {
			c = n - 1
		}
		if name := attr("colname"); name != "" && len(t.colnames) < MaxTableColumns {
			if t.colnames == nil {
				t.colnames = map[string]int{}
			}
			t.colnames[name] = c
		}
		t.nextCol = c + 1
	case "spanspec":
		if t.colnames != nil && len(t.spans) < MaxTableColumns {
			a, okA := t.colnames[attr("namest")]
			b, okB := t.colnames[attr("nameend")]
			if okA && okB && b >= a {
				t.spans[attr("spanname")] = [2]int{a, b}
			}
		}
	case "thead":
		t.inHead = true
	case "tbody", "tfoot":
		t.inHead = false
	case "row":
		t.startRow(t.inHead)
	case "entry":
		at, cs := -1, 1
		if c, ok := t.colnames[attr("colname")]; ok {
			at = c
		}
		if s, ok := t.spans[attr("spanname")]; ok {
			at, cs = s[0], s[1]-s[0]+1
		}
		a, okA := t.colnames[attr("namest")]
		b, okB := t.colnames[attr("nameend")]
		if okA {
			at = a
			if okB && b >= a {
				cs = b - a + 1
			}
		}
		rs := 1
		if n, err := strconv.Atoi(attr("morerows")); err == nil && n > 0 {
			rs = n + 1
		}
		t.place(e, at, cs, rs)
		f.cell = true
		e.openSink(f, skCell, el.Start)
	default:
		return false
	}
	f.role = roleContainer
	if f.cell {
		f.role = roleSink
	}
	f.cals, f.owner = el.Local, t
	return true
}

// calsEnd ends an element of the CALS model.
func (e *Engine) calsEnd(f *frame, start, end int64) {
	t := f.owner
	if t == nil {
		return
	}
	switch f.cals {
	case "row":
		t.endRow()
		t.rowDone(start, end, f.anchor)
	case "thead":
		t.inHead = false
	case "entry":
		if f.sink != nil {
			s := e.closeSink(f)
			t.endCell(s.b.String(), s.href)
			for _, b := range s.after {
				e.add(b, f, false)
			}
			e.putSink(s)
		}
	}
}
