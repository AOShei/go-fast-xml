// Package generic builds blocks from XML with no knowledge of its schema
// (README, "How XML maps without a profile"), and holds the part writer every consumer of
// element events shares.
//
// An element that directly contains character data other than white
// space is a text element: one inferred paragraph with its own text and
// that of its descendants in document order. An element that holds only
// elements is a container and has no block of its own. A text element
// inside another is part of the outer one's block.
//
// Whether an element is a text element is known only when its direct
// text comes, which may follow child elements whose own blocks are
// already made. Those blocks wait in a queue while any ancestor is
// undecided, and are taken into the ancestor's block if it turns out to
// be a text element. The queue is bounded (MaxPendingBytes): past it,
// every undecided element still open is taken for a container and the
// queue is written out; such an element that has text after that point
// gives a block from there on.
package generic

import (
	"bytes"
	"slices"
	"strings"

	"github.com/AOShei/go-fast-core"
	"github.com/AOShei/go-fast-xml/internal/xmlread"
	"github.com/AOShei/go-fast-xml/internal/xmlscan"
)

// Fixed bounds. What they cut is reported with limit.exceeded.
const (
	// MaxBlockBytes bounds the text of one block. A longer text is split
	// into blocks marked Continued between the scanner's pieces (which
	// end after white space where there is some); white space at a cut
	// is dropped, as at a block's ends.
	MaxBlockBytes = 1 << 20
	// MaxNameBytes bounds an element or attribute name as it appears in
	// Attrs.
	MaxNameBytes = 256
	// MaxPathBytes bounds Attrs["element"], the path of names from the
	// root: MaxNameBytes times 16.
	MaxPathBytes = MaxNameBytes * 16
	// MaxValueBytes bounds the text of an attribute's note and an anchor.
	MaxValueBytes = 1 << 20
	// MaxPendingBytes and MaxPendingBlocks bound the blocks that wait for
	// an undecided ancestor (see the package comment).
	MaxPendingBytes  = 4 << 20
	MaxPendingBlocks = 8192
	// MaxLangBytes bounds the root's xml:lang taken for the document's
	// language; a longer one is an attribute note.
	MaxLangBytes = 64
)

// Keys of Block.Attrs this library writes.
const (
	// AttrElement is the path of element names from the root, as written
	// (prefixes included), joined with "/".
	AttrElement = "element"
	// AttrAttribute is the name of the attribute an attribute note holds,
	// as written.
	AttrAttribute = "attribute"
)

// Out writes blocks to a part within the text budget, and counts what
// the text conventions and the fixed bounds did.
type Out struct {
	Part   *gofastcore.Part
	Budget *gofastcore.TextBudget
	// Clean collects the counts of removed control characters and
	// invalid bytes.
	Clean Builder
	// Stopped is set when the text budget is spent; StoppedAt is the
	// offset reading had reached.
	Stopped   bool
	StoppedAt int64
	// Counts of cuts by the fixed bounds.
	NamesCut, PathsCut, ValuesCut int
	// Splits counts texts split into Continued blocks.
	Splits int
	// PendingCut counts cuts by MaxPendingBlocks and MaxPendingBytes that
	// show in the output: a block given as a piece because more than
	// MaxPendingBlocks notes waited for it, and an element taken for a
	// container because the queue passed its bound, which then had text
	// of its own.
	PendingCut int
	// Hint is the room for blocks the part is given when its first block
	// is added (see BlocksHint); zero lets the part grow from nothing.
	Hint int
	// UTF8: the file is UTF-8, its bytes the text's (see WordMark).
	UTF8 bool
}

// Add appends b, taking BlockCost, its text and its element path from the
// budget. It reports false, and adds nothing, once the budget is spent.
func (o *Out) Add(b gofastcore.Block) bool {
	if o.Stopped {
		return false
	}
	if !o.Budget.Take(gofastcore.BlockCost + len(b.Text) + len(b.Attrs[AttrElement])) {
		o.Stopped = true
		return false
	}
	o.Append(b)
	return true
}

// Append appends b to the part, its cost already taken. The first block
// of the document gives the part Hint blocks of room.
func (o *Out) Append(b gofastcore.Block) {
	if o.Hint > 0 && o.Part.Blocks == nil && o.Part.Kind != gofastcore.PartStored {
		o.Part.Blocks = make([]gofastcore.Block, 0, o.Hint)
		o.Hint = 0
	}
	o.Part.Blocks = append(o.Part.Blocks, b)
}

// Reserve makes room for n more blocks in the part at once.
func (o *Out) Reserve(n int) {
	if len(o.Part.Blocks)+n <= cap(o.Part.Blocks) {
		return
	}
	if o.Part.Blocks == nil && o.Part.Kind != gofastcore.PartStored {
		n = max(n, o.Hint)
		o.Hint = 0
	}
	o.Part.Blocks = slices.Grow(o.Part.Blocks, n)
}

// BlocksHint is the room for blocks given to a document of size bytes
// before its first block: one block per BlockHintBytes of input, at most
// MaxBlocksHint.
func BlocksHint(size int64) int {
	return int(min(size/BlockHintBytes, MaxBlocksHint))
}

// BlockHintBytes and MaxBlocksHint size BlocksHint. Service-Public files
// make a block for every 126 to 419 bytes of input with the profile (1st
// to 95th percentile), every 43 to 89 bytes generically.
const (
	BlockHintBytes = 128
	MaxBlocksHint  = 4096
)

// Options are the generic reader's settings.
type Options struct {
	// NoAttributes leaves attribute notes out.
	NoAttributes bool
}

type state uint8

const (
	undecided state = iota // no direct text yet; blocks of children wait
	textual                // a text element: its block is being built
	container              // taken for a container
	inline                 // inside a text element
)

type frame struct {
	el     *xmlread.Element
	state  state
	qStart int    // where the element's content starts in the queue
	path   string // computed when needed
	pathOK bool
	start  int64 // where its block starts
	pieces int   // blocks already given for its text
	anchor string
	id     string // the element's own identifier, read at its start
	// forced: taken for a container because the queue passed its bound.
	forced bool
}

type item struct {
	block       gofastcore.Block
	para        bool // a paragraph a text-element ancestor takes in
	space       bool // white space between children, no block
	lead, trail bool // the paragraph's text began or ended with white space
}

// Generic is the generic consumer. It implements xmlread.Handler.
type Generic struct {
	out   *Out
	opts  Options
	doc   frame // depth 0: text outside the root element
	stack []frame
	queue []item
	qSize int

	owner   int // depth of the text element whose block is being built, or -1
	b       Builder
	after   []gofastcore.Block // notes and comments that follow the owner's block
	pieceAt int64              // where the next piece of the owner's block starts
	word    WordMark           // where the owner's last word starts

	cb       Builder // a comment
	inCom    bool
	comStart int64
	comParts int
	comInner bool // the comment began inside a text element

	paths   map[string]string
	scratch []byte
	peak    int // the longest the queue has been, for tests
	taken   []item
}

// New returns a generic consumer writing to out.
func New(out *Out, opts Options) *Generic {
	return &Generic{out: out, opts: opts, owner: -1, doc: frame{state: container}, paths: map[string]string{}}
}

// Stop implements xmlread.Handler.
func (g *Generic) Stop() bool { return g.out.Stopped }

func (g *Generic) frameAt(depth int) *frame {
	if depth == 0 {
		return &g.doc
	}
	return &g.stack[depth-1]
}

// StartElement implements xmlread.Handler.
func (g *Generic) StartElement(r *xmlread.Reader, e *xmlread.Element) {
	if g.owner == 0 {
		g.emitOwner(e.Start)
		g.flush(false)
	}
	// The element's attributes are readable only now: its identifier is
	// kept for when it becomes a text element.
	g.stack = append(g.stack, frame{el: e, id: g.anchorOf(e)})
	f := &g.stack[len(g.stack)-1]
	if g.owner > 0 {
		f.state = inline
		g.after = g.attrNotes(len(g.stack), g.after)
		if o := g.frameAt(g.owner); o.anchor == "" && o.pieces == 0 {
			o.anchor = f.id // the identifier of an element in the block
		}
		if len(g.after) > MaxPendingBlocks {
			g.out.PendingCut++
			g.cutOwner(e.Start) // the notes waiting for the block are bounded too
		}
		return
	}
	n := len(g.queue)
	notes := g.attrNotes(len(g.stack), nil)
	for _, b := range notes {
		g.enqueue(item{block: b})
	}
	f = &g.stack[len(g.stack)-1]
	f.qStart = n + len(notes)
	if g.overBound() {
		g.flush(false)
	}
}

// EndElement implements xmlread.Handler.
func (g *Generic) EndElement(r *xmlread.Reader, e *xmlread.Element, start, end int64) {
	d := len(g.stack)
	if d == 0 {
		return
	}
	if g.owner == d {
		g.emitOwner(end)
	}
	g.stack = g.stack[:d-1]
	g.flush(false)
}

// Text implements xmlread.Handler.
func (g *Generic) Text(r *xmlread.Reader, t []byte, start, end int64) {
	preserve := false
	if e := r.Top(); e != nil {
		preserve = e.Preserve
	}
	if g.owner < 0 {
		d := len(g.stack)
		f := g.frameAt(d)
		if spaceOnly(t) {
			if f.state == undecided && len(g.queue) > f.qStart {
				last := &g.queue[len(g.queue)-1]
				switch {
				case preserve && last.space:
					// White space kept as written waits as it is.
					last.block.Text += string(t)
					g.qSize += len(t)
				case preserve:
					g.enqueue(item{space: true, block: gofastcore.Block{Text: string(t)}})
				case !last.space:
					g.enqueue(item{space: true})
				}
				if g.overBound() {
					g.flush(false)
				}
			}
			return
		}
		g.becomeText(d, start)
	}
	if g.b.Size() > 0 && g.b.Size()+len(t) > MaxBlockBytes {
		// The block is cut where this token starts, so each piece has
		// the source span of its own text.
		g.cutOwner(start)
	}
	pos0, trail := g.b.Pos(), g.b.Trail()
	g.b.Append(t, preserve)
	g.word.Note(&g.b, t, start, end, pos0, trail, g.out.UTF8)
	if g.b.Len() > MaxBlockBytes {
		g.splitOwner(start) // a token longer than a block: not from the scanner's input
	}
}

// cutOwner gives the owner's text so far as a piece ending at offset at,
// with the notes and comments that follow it, and writes out everything
// before it. White space at the cut is dropped, as at a block's ends.
func (g *Generic) cutOwner(at int64) {
	f := g.frameAt(g.owner)
	g.out.Splits++
	// Cut at the last white space already held, where the word after it
	// starts in the file is known: that word is kept whole for the next
	// piece, which starts where it does.
	pos := -1
	if g.word.OK {
		pos = g.word.Pos
	}
	end := at
	text, _, kept := g.b.SplitWord(pos)
	if kept {
		end = g.word.At
	}
	g.word.OK = false
	if text != "" {
		g.enqueue(item{block: g.paragraph(f, text, end)})
		f.pieces++
	}
	g.pieceAt = end
	for _, b := range g.after {
		g.enqueue(item{block: b})
	}
	g.after = g.after[:0]
	g.flush(true)
}

// WordMark records where the last word of a text being built starts: in
// the file (At) and in the text (Pos), when both are known (OK). A long
// text cut while reading is cut there, so that no word is cut in two
// and each piece's span holds its text.
type WordMark struct {
	At  int64
	Pos int
	OK  bool
}

// Note updates m for the text t, at [start,end) in the file, just
// appended to b; pos0 and trail are b.Pos() and b.Trail() before the
// append. utf8: the file's bytes are the text's, so that a token as long
// as its span holds no reference and no line end changed.
func (m *WordMark) Note(b *Builder, t []byte, start, end int64, pos0 int, trail, utf8 bool) {
	i := bytes.LastIndexAny(t, " \t\n\r")
	if i < 0 {
		if pos0 == 0 || trail { // the word starts with this token
			*m = WordMark{At: start, Pos: pos0, OK: true}
		}
		return
	}
	tail := len(t) - i - 1
	exact := int64(len(t)) == end-start && (utf8 || isASCII(t))
	*m = WordMark{At: end - int64(tail), Pos: b.Len() - tail, OK: exact}
}

func isASCII(t []byte) bool {
	for _, c := range t {
		if c >= 0x80 {
			return false
		}
	}
	return true
}

func spaceOnly(t []byte) bool {
	for _, c := range t {
		if c != ' ' && c != '\t' && c != '\n' && c != '\r' {
			return false
		}
	}
	return true
}

// becomeText makes the element at depth d a text element. An undecided
// one takes in the blocks of its children that wait in the queue.
func (g *Generic) becomeText(d int, at int64) {
	f := g.frameAt(d)
	g.owner = d
	g.b.Reset()
	g.word.OK = false
	g.after = g.after[:0]
	f.pieces = 0
	f.anchor = f.id
	if f.state != undecided {
		if f.forced {
			// Taken for a container at the queue's bound, it has text:
			// it gives a block from here on, not one block.
			f.forced = false
			g.out.PendingCut++
		}
		f.state, f.start = textual, at
		g.pieceAt = at
		return
	}
	f.state, f.start = textual, f.el.Start
	g.pieceAt = f.start
	// The waiting items leave the queue first: taking them in may cut the
	// block, which writes the queue out.
	g.taken = append(g.taken[:0], g.queue[f.qStart:]...)
	for _, it := range g.taken {
		g.qSize -= len(it.block.Text) + gofastcore.BlockCost
	}
	clear(g.queue[f.qStart:])
	g.queue = g.queue[:f.qStart]
	for _, it := range g.taken {
		switch {
		case it.space && it.block.Text != "":
			g.b.Append([]byte(it.block.Text), true)
		case it.space:
			g.b.Space()
		case it.para:
			if g.b.Size() > 0 && g.b.Size()+len(it.block.Text)+2 > MaxBlockBytes {
				g.cutOwner(it.block.Loc.Offset)
			}
			if it.lead {
				g.b.Space()
			}
			g.b.Append([]byte(it.block.Text), true)
			if it.trail {
				g.b.Space()
			}
			if f.anchor == "" && it.block.Loc.Anchor != "" {
				f.anchor = it.block.Loc.Anchor
			}
		default:
			g.after = append(g.after, it.block)
		}
	}
	clear(g.taken)
}

// emitOwner ends the owner's block at offset end and queues it with the
// notes and comments that follow it.
func (g *Generic) emitOwner(end int64) {
	f := g.frameAt(g.owner)
	lead, trail := g.b.Lead(), g.b.Trail()
	if text := g.b.String(); text != "" {
		g.enqueue(item{block: g.paragraph(f, text, end), para: f.pieces == 0, lead: lead, trail: trail})
	}
	for _, b := range g.after {
		g.enqueue(item{block: b})
	}
	g.after = g.after[:0]
	g.b.Reset()
	g.word.OK = false
	if g.owner == 0 {
		g.doc.state = container
	}
	g.owner = -1
}

func (g *Generic) paragraph(f *frame, text string, end int64) gofastcore.Block {
	b := gofastcore.Block{Kind: gofastcore.KindParagraph, Origin: gofastcore.OriginInferred, Text: text,
		Continued: f.pieces > 0,
		Loc:       gofastcore.Location{Offset: g.pieceAt, Length: max(end-g.pieceAt, 0), Anchor: f.anchor}}
	if f.pieces > 0 {
		b.Loc.Anchor = ""
	}
	if f.el != nil {
		b.Attrs = map[string]string{AttrElement: g.path(g.owner)}
	}
	return b
}

// splitOwner gives the first piece of an owner's text that has grown past
// MaxBlockBytes, and writes out everything before it.
func (g *Generic) splitOwner(at int64) {
	f := g.frameAt(g.owner)
	for g.b.Len() > MaxBlockBytes {
		piece := g.b.Split(MaxBlockBytes)
		g.word.OK = false
		g.out.Splits++
		if piece != "" {
			g.enqueue(item{block: g.paragraph(f, piece, at)})
			f.pieces++
			g.pieceAt = at
		}
		for _, b := range g.after {
			g.enqueue(item{block: b})
		}
		g.after = g.after[:0]
	}
	g.flush(true)
}

// Comment implements xmlread.Handler.
func (g *Generic) Comment(r *xmlread.Reader, t []byte, final bool, start, end int64) {
	if !g.inCom {
		g.inCom, g.comStart, g.comParts = true, start, 0
		g.comInner = g.owner > 0
		if g.owner == 0 {
			g.emitOwner(start)
		}
		g.cb.Reset()
	}
	if g.cb.Size() > 0 && g.cb.Size()+len(t) > MaxBlockBytes {
		// A long comment is cut between the scanner's pieces, each of
		// which has its own span.
		g.comment(g.cb.String(), start)
		g.cb.Reset()
		g.comStart = start
	}
	g.cb.Append(t, true)
	for g.cb.Len() > MaxBlockBytes {
		g.comment(g.cb.Split(MaxBlockBytes), end) // a piece longer than a block: not from the scanner's input
		g.comStart = end
	}
	if final {
		g.comment(g.cb.String(), end)
		g.cb.Reset()
		g.inCom = false
	}
}

func (g *Generic) comment(text string, end int64) {
	if text == "" {
		return
	}
	b := gofastcore.Block{Kind: gofastcore.KindNote, Text: text, Continued: g.comParts > 0,
		Attrs: map[string]string{gofastcore.AttrStored: gofastcore.StoredMarkupComment},
		Loc:   gofastcore.Location{Offset: g.comStart, Length: max(end-g.comStart, 0)}}
	g.comParts++
	if g.comInner && g.owner > 0 && len(g.after) >= MaxPendingBlocks {
		g.out.PendingCut++
		g.cutOwner(g.comStart)
	}
	if g.comInner && g.owner > 0 {
		g.after = append(g.after, b)
		return
	}
	g.enqueue(item{block: b})
	if g.owner < 0 {
		g.flush(false)
	}
}

func (g *Generic) enqueue(it item) {
	g.queue = append(g.queue, it)
	g.qSize += len(it.block.Text) + gofastcore.BlockCost
	g.peak = max(g.peak, len(g.queue))
}

// overBound reports whether the queue has passed its bound.
func (g *Generic) overBound() bool {
	return g.qSize > MaxPendingBytes || len(g.queue) > MaxPendingBlocks
}

// flush writes out the blocks no undecided element can still take in. It
// takes every undecided element for a container, and writes out all, when
// forced or when the queue has passed its bound.
func (g *Generic) flush(force bool) {
	n := len(g.queue)
	if n == 0 {
		return
	}
	u := -1
	for i := range g.stack {
		if g.stack[i].state == undecided {
			u = i
			break
		}
	}
	if u >= 0 {
		if force || g.overBound() {
			for i := u; i < len(g.stack); i++ {
				if g.stack[i].state == undecided {
					g.stack[i].state = container
					g.stack[i].forced = !force
				}
			}
		} else {
			n = g.stack[u].qStart
		}
	}
	if n == 0 {
		return
	}
	g.out.Reserve(n)
	for i := range n {
		if g.queue[i].space {
			continue
		}
		if !g.out.Add(g.queue[i].block) {
			break
		}
	}
	for i := range n {
		g.qSize -= len(g.queue[i].block.Text) + gofastcore.BlockCost
	}
	copy(g.queue, g.queue[n:])
	clear(g.queue[len(g.queue)-n:])
	g.queue = g.queue[:len(g.queue)-n]
	for i := range g.stack {
		g.stack[i].qStart = max(g.stack[i].qStart-n, 0)
	}
}

// Finish ends the reading at offset end: what is still open is closed and
// everything is written out.
func (g *Generic) Finish(end int64) {
	if g.inCom {
		g.comment(g.cb.String(), end)
		g.inCom = false
	}
	if g.owner >= 0 {
		g.emitOwner(end)
	}
	g.stack = g.stack[:0]
	g.flush(true)
}

// path returns Attrs["element"] for the element at depth d.
// The paths of the ancestors are made first, from the outermost one not
// yet made, in a loop: nesting does not recurse.
func (g *Generic) path(d int) string {
	from := d
	for from > 1 && !g.stack[from-2].pathOK {
		from--
	}
	for i := from; i < d; i++ {
		g.pathOf(i)
	}
	return g.pathOf(d)
}

// pathOf makes the path of the element at depth d, its parent's made.
func (g *Generic) pathOf(d int) string {
	f := &g.stack[d-1]
	if f.pathOK {
		return f.path
	}
	var parent string
	if d > 1 {
		parent = g.stack[d-2].path
	}
	name := f.el.Name
	if len(name) > MaxNameBytes {
		name = cutString(name, MaxNameBytes)
		g.out.NamesCut++
	}
	buf := append(g.scratch[:0], parent...)
	if len(buf) > 0 {
		buf = append(buf, '/')
	}
	buf = append(buf, name...)
	if len(buf) > MaxPathBytes {
		cut := bytes.LastIndexByte(buf[:MaxPathBytes], '/')
		if cut <= 0 {
			cut = len(cutString(string(buf), MaxPathBytes))
		}
		buf = buf[:cut]
		g.out.PathsCut++
	}
	g.scratch = buf
	p, ok := g.paths[string(buf)]
	if !ok {
		p = CleanName(string(buf), &g.out.Clean)
		if len(g.paths) < 16384 {
			g.paths[string(buf)] = p
		}
	}
	f.path, f.pathOK = p, true
	return p
}

// CutString cuts s to at most n bytes at a character boundary.
func CutString(s string, n int) string { return cutString(s, n) }

func cutString(s string, n int) string {
	if len(s) <= n {
		return s
	}
	for n > 0 && s[n]&0xC0 == 0x80 {
		n--
	}
	return s[:n]
}

// anchorOf returns the identifier that names a text element's block:
// xml:id, else id.
func (g *Generic) anchorOf(e *xmlread.Element) string {
	v, ok := e.Attr(xmlscan.XMLNamespace, "id")
	if !ok {
		v, ok = e.Attr("", "id")
	}
	if !ok {
		return ""
	}
	v = strings.TrimSpace(v)
	if len(v) > MaxValueBytes {
		v = cutString(v, MaxValueBytes)
		g.out.ValuesCut++
	}
	return CleanName(v, &g.out.Clean)
}

// Skipped reports whether an attribute is not a note: a namespace
// declaration, xml:space, and on the root xml:lang and the schema
// location hints, which go to the metadata.
func Skipped(e *xmlread.Element, a *xmlread.Attr) bool {
	switch a.URI {
	case xmlscan.XMLNSNamespace:
		return true
	case xmlscan.XMLNamespace:
		return a.Local == "space" || a.Local == "lang" && e.Depth == 1 && len(strings.TrimSpace(a.Value)) <= MaxLangBytes
	case xmlread.NamespaceXSI:
		return e.Depth == 1 && (a.Local == "schemaLocation" || a.Local == "noNamespaceSchemaLocation")
	}
	return false
}

// attrNotes appends the attribute notes of the element at depth d.
func (g *Generic) attrNotes(d int, dst []gofastcore.Block) []gofastcore.Block {
	if g.opts.NoAttributes {
		return dst
	}
	f := &g.stack[d-1]
	e := f.el
	var tmp Builder
	for i := range e.Attrs {
		a := &e.Attrs[i]
		if a.Value == "" || Skipped(e, a) {
			continue
		}
		v := a.Value
		if len(v) > MaxValueBytes {
			v = cutString(v, MaxValueBytes)
			g.out.ValuesCut++
		}
		text := Clean(v, false, &tmp)
		g.out.Clean.Controls += tmp.Controls
		g.out.Clean.Invalid += tmp.Invalid
		tmp.Controls, tmp.Invalid = 0, 0
		if text == "" {
			continue
		}
		name := a.Name
		if len(name) > MaxNameBytes {
			name = cutString(name, MaxNameBytes)
			g.out.NamesCut++
		}
		dst = append(dst, gofastcore.Block{Kind: gofastcore.KindNote, Origin: gofastcore.OriginInferred, Text: text,
			Attrs: map[string]string{gofastcore.AttrNoteType: gofastcore.NoteAttribute,
				AttrAttribute: CleanName(name, &g.out.Clean), AttrElement: g.path(d)},
			Loc: gofastcore.Location{Offset: e.Start, Length: e.End - e.Start}})
	}
	return dst
}

// Counts returns the control characters removed and invalid bytes met in
// all the text made.
func (g *Generic) Counts() (controls, invalid int) {
	return g.b.Controls + g.cb.Controls + g.out.Clean.Controls, g.b.Invalid + g.cb.Invalid + g.out.Clean.Invalid
}
