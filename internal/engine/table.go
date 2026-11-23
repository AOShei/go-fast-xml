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
	model  string
	rows   [][]string
	merges []gofastcore.Merge
	cells  []gofastcore.CellDetail
	header int  // leading header rows
	body   bool // a row that is not a header row came
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
	// cost is what the cells hold so far, against the text budget.
	cost int64
	// fromProfile: the header row is the profile's fixed text. attrNames
	// are the attributes rows were made from, in order of first use.
	fromProfile bool
	attrNames   []string
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
func (e *Engine) tableCost(t *table) {
	if t.cost > e.out.Budget.Left() {
		e.out.Budget.Exhaust()
		e.out.Stopped = true
	}
}

func newTable(model string, start int64) *table {
	return &table{model: model}
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
	rs = min(max(rs, 1), MaxRowSpan)
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
	t.cost += int64(gofastcore.CellCost + len(text))
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
	t.ncols = max(t.ncols, t.col)
	t.cs = 0
}

func (t *table) endRow() {
	if !t.inRow {
		return
	}
	t.inRow = false
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
	t.ncols = max(t.ncols, len(t.row))
	t.rows = append(t.rows, append([]string(nil), t.row...))
}

// block returns the table's block; ok is false for a table with no cells.
func (t *table) block(e *Engine, start, end int64) (gofastcore.Block, bool) {
	t.endRow()
	if len(t.rows) == 0 || t.ncols == 0 {
		return gofastcore.Block{}, false
	}
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
	tb := &gofastcore.Table{Rows: t.rows, HeaderRows: min(t.header, n), Merges: merges}
	if e.opts.Spans {
		tb.Cells = t.cells
	}
	b := gofastcore.Block{Kind: gofastcore.KindTable, Table: tb,
		Loc: gofastcore.Location{Offset: start, Length: max(end-start, 0)}}
	if t.fromProfile || len(t.attrNames) > 0 {
		b.Attrs = map[string]string{}
	}
	if t.fromProfile {
		// The header row is the profile's text, not the file's; the
		// table is still native: every row and cell is declared by the
		// file, nothing is guessed (core's inferred).
		b.Attrs[AttrHeaderSource] = "profile"
	}
	if len(t.attrNames) > 0 {
		b.Attrs[generic.AttrAttribute] = strings.Join(t.attrNames, " ")
	}
	return b, true
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
func (e *Engine) calsEnd(f *frame) {
	t := f.owner
	if t == nil {
		return
	}
	switch f.cals {
	case "row":
		t.endRow()
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
