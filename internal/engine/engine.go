// Package engine reads a document with a profile (PROFILE_FORMAT.md): it
// consumes the element events of package xmlread, as the generic reader
// does, and makes the blocks the profile's rules say. Elements with no
// rule are read generically and counted.
package engine

import (
	"fmt"
	"math"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/AOShei/go-fast-core"
	"github.com/AOShei/go-fast-xml/internal/generic"
	"github.com/AOShei/go-fast-xml/internal/profile"
	"github.com/AOShei/go-fast-xml/internal/xmlread"
	"github.com/AOShei/go-fast-xml/internal/xmlscan"
)

// Fixed bounds. What they cut is reported with limit.exceeded.
const (
	// MaxTableColumns bounds the columns of a table.
	MaxTableColumns = 1024
	// MaxRowSpan bounds how many rows one cell spans.
	MaxRowSpan = 65535
	// MaxNamesCounted bounds the distinct names counted in the
	// xml.unmapped_* warnings; the rest are counted together.
	MaxNamesCounted = 1000
	// MaxParts bounds the parts a profile's part rules start.
	MaxParts = 100000
)

// Keys of Block.Attrs the engine writes.
const (
	// AttrAnchorParent is the identifier of the nearest enclosing element
	// a rule names with "anchor", on a block with no identifier of its own.
	AttrAnchorParent = "anchor_parent"
	// AttrHeaderSource is "profile" on a table whose header row is the
	// profile's fixed text.
	AttrHeaderSource = "header_source"
	// AttrTitle is a link's title ("title") in its stored record.
	AttrTitle = "title"
	// AttrHref is the address of a link, on its record in the stored part.
	AttrHref = "href"
)

// Options are the engine's settings.
type Options struct {
	Spans    bool
	NoImages bool
	// Child hands over the content of a "document" rule's element; it
	// returns the document to list in Children and whether it was
	// extracted. Without it the content is kept as a paragraph.
	Child func(name, mediaType string, data []byte) (*gofastcore.Document, bool)
	// Raw returns the bytes of the file from start to end, for XHTML
	// content; nil when the file is not UTF-8 (the content is then
	// written from the decoded text).
	Raw func(start, end int64) []byte
	// MaxChildren bounds the documents handed over.
	MaxChildren int
	// PartWanted reports whether the part of that number is in the
	// output; the content of document rules in other parts is not handed
	// over.
	PartWanted func(number int) bool
}

type role uint8

const (
	roleContainer role = iota
	roleSink
	roleInline
	roleSkip
	roleDoc // inside a "document" rule's element
)

type sinkKind uint8

const (
	skBlock   sinkKind = iota // paragraph-like: nested blocks are lines
	skHeading                 // heading or label: nested blocks unwrapped
	skItem
	skCell
	skCapture
	skStray
)

type structLevel struct{ open, items bool }

type sink struct {
	// gen counts the sink's uses: a frame holding a sink that has since
	// been finished and reused for another block sees another gen.
	gen      uint64
	kind     sinkKind
	b        generic.Builder
	spans    []gofastcore.Span
	after    []gofastcore.Block
	marker   string
	start    int64
	end      int64
	pieces   int
	pieceAt  int64
	word     generic.WordMark // where the last word starts
	done     bool             // an item has its text
	fresh    bool             // a line was just started (an item's marker written)
	auto     bool             // the marker is an ordered list's number, not from the file
	verbatim bool             // white space kept as written (code)
	restart  bool             // a part broke the block: the next text starts a block
	// structure captures
	structure bool
	sb        []byte
	levels    []structLevel
	cut       bool
	href      string // a cell's first link
}

func (s *sink) reset(kind sinkKind, start int64) {
	s.gen++
	s.kind, s.start, s.end, s.pieces, s.pieceAt, s.done, s.restart = kind, start, start, 0, start, false, false
	s.b.Reset()
	s.word.OK = false
	s.spans, s.after, s.marker = s.spans[:0], nil, ""
	s.structure, s.sb, s.levels, s.cut, s.href = false, s.sb[:0], s.levels[:0], false, ""
	s.fresh, s.auto, s.verbatim = false, false, false
}

type blockRef struct {
	part *gofastcore.Part
	i    int
}

func (r blockRef) ok() bool { return r.part != nil }

func (r blockRef) get() *gofastcore.Block { return &r.part.Blocks[r.i] }

type kv struct{ k, v string }

type slot struct {
	v      *profile.Value
	key    string
	use    useKind
	filled bool
}

type useKind uint8

const (
	useAttr  useKind = iota // the element's own block
	useFirst                // the first block made inside a container
	useMark
	useExtra
	useMarker
	useAlt
	useID
	useName
	useValue
	useLink
	useTitle
	useAnchor
)

type held struct {
	block   gofastcore.Block
	dupOf   string
	key     string
	inStore bool
}

type frame struct {
	name    profile.QName
	qname   string
	rule    *profile.Rule
	role    role
	start   int64
	kept    []xmlread.Attr
	sink    *sink
	prevCur *sink
	// suspended: a part, table, image or "split" element met inside an
	// open block, which it ended; resume is that block's sink, resumed
	// when the element ends.
	suspended bool
	// outOfLine: an out_of_line rule's element read beside an open block,
	// whose waiting label it does not take; resume is that block's sink.
	outOfLine bool
	resume    *sink
	// labelFresh: a label read inline at the start of a line.
	labelFresh bool

	sections int
	quotes   int
	ld       int
	inItem   bool
	ordered  bool
	counter  int

	marks, anchors int
	display        gofastcore.Display

	slots      []slot
	capture    *slot // this element is captured for a slot of frame captureOf
	captureOf  int
	structRoot bool

	table *table
	owner *table // the table of a row or cell
	cell  bool
	cals  string // the element of the CALS model this is

	partAt *gofastcore.Part
	idxAt  int

	first     blockRef
	firstSet  []kv
	own       []kv
	anchor    string
	ref       blockRef
	held      *held
	dup       *held
	lastChild string
	lastRef   blockRef

	span      *sink
	spanGen   uint64 // span.gen when the inline began
	spanStart int
	href      string
	title     string
	valAttr   string
	// spaced: the sink of an inline "space" rule, given a space when the
	// element ends.
	spaced *sink
	// broken: the sink a block, list or item was read into as lines; a
	// line break is owed after the element ends, written before the next
	// text (none at the block's end: white space there is dropped).
	broken *sink
	// tokens: the token sets of the element's values that rules tested.
	tokens map[profile.QName]map[string]bool

	val    string
	hasVal bool
	part   *gofastcore.Part
	imgAlt bool

	// isDoc: the element of a "document" rule. kids: the children that
	// came from this section; secFirst and secHead: its first block and
	// first heading.
	isDoc             bool
	kids              []int
	secFirst, secHead blockRef
}

type mark struct {
	k, v  string
	depth int
}

type anchorAt struct {
	v     string
	depth int
}

type labelAt struct {
	text  string
	depth int
	start int64
	end   int64
}

// Engine is the profile consumer. It implements xmlread.Handler.
type Engine struct {
	p    *profile.Profile
	out  *generic.Out
	opts Options
	// Meta collects the metadata the rules give.
	Meta gofastcore.Metadata
	// extra collects the values of each Metadata.Extra key, joined as
	// they come and written to Meta.Extra once, at Finish.
	extra map[string]*strings.Builder
	// wordCut: a piece is being cut while reading (cutSink), at the last
	// white space held rather than where the next token starts.
	wordCut bool

	parts  []*gofastcore.Part
	stored *gofastcore.Part
	usedBy bool // the first part was started by a part rule

	frames  []frame
	attrs   []xmlread.Attr // the element being started
	cur     *sink
	stray   *sink
	strayAt int
	marks   []mark
	anchors []anchorAt
	label   *labelAt
	// readAttrs: the attributes of the element being started that a
	// rule of an open element read ({"attr": ..., "child": ...}).
	readAttrs     []profile.QName
	hasChildAttrs bool

	cache  map[string]*profile.Rule
	keybuf []byte
	free   []*sink

	// Counts for the warnings of the current part.
	unmapped, unmappedAttrs, skipped map[string]int
	clamped, mathml                  int
	// Counts for the whole reading.
	ValuesCut, ColumnsCut, SpansCut, TextCut, PartsCut int
	// RowSpanCut counts cells whose row span was cut to MaxRowSpan.
	RowSpanCut int
	// PendingCut counts blocks cut because more than MaxPendingBlocks
	// attribute notes waited for them.
	PendingCut        int
	Controls, Invalid int

	// Embedded documents ("document" rules): the documents and where each
	// came from, in file order. DocsCut counts contents cut to
	// MaxBlockBytes, ChildrenCut those not handed over past MaxChildren,
	// Plain those kept as a paragraph because they held no markup.
	Children                    []*gofastcore.Document
	ChildLocs                   []ChildLoc
	DocsCut, ChildrenCut, Plain int
	docs                        bool
	doc                         *docCap
	scratchTag                  []byte

	com      generic.Builder
	inCom    bool
	comStart int64
	comParts int
	scratch  generic.Builder
}

// New returns an engine reading with p into out, whose Part is the first
// part.
func New(p *profile.Profile, out *generic.Out, opts Options) *Engine {
	e := &Engine{p: p, out: out, opts: opts, parts: []*gofastcore.Part{out.Part},
		cache: map[string]*profile.Rule{}, hasChildAttrs: p.ReadsChildAttrs(), docs: hasDocuments(p)}
	e.resetCounts()
	return e
}

func (e *Engine) resetCounts() {
	e.unmapped, e.unmappedAttrs, e.skipped = map[string]int{}, map[string]int{}, map[string]int{}
	e.clamped, e.mathml = 0, 0
}

// Parts returns the parts made, the stored part last if there is one.
func (e *Engine) Parts() []*gofastcore.Part {
	ps := slices.Clone(e.parts)
	if e.stored != nil {
		e.stored.Number = ps[len(ps)-1].Number + 1
		ps = append(ps, e.stored)
	}
	return ps
}

// Stop implements xmlread.Handler.
func (e *Engine) Stop() bool { return e.out.Stopped }

// Path, for the profile's matching: index 0 is the element being started.

func (e *Engine) Len() int { return len(e.frames) }

func (e *Engine) Name(i int) profile.QName { return e.frames[len(e.frames)-1-i].name }

func (e *Engine) Attr(i int, q profile.QName) (string, bool) {
	attrs := e.frames[len(e.frames)-1-i].kept
	if i == 0 {
		attrs = e.attrs
	}
	for k := range attrs {
		if attrs[k].Local == q.Local && attrs[k].URI == q.URI {
			return attrs[k].Value, true
		}
	}
	return "", false
}

// HasToken implements profile.TokenPath: an element's tested value is
// split into its tokens once, the first time a rule tests it.
func (e *Engine) HasToken(i int, q profile.QName, tok string) bool {
	f := &e.frames[len(e.frames)-1-i]
	set, ok := f.tokens[q]
	if !ok {
		v, _ := e.Attr(i, q)
		set = profile.Tokens(v)
		if f.tokens == nil {
			f.tokens = map[profile.QName]map[string]bool{}
		}
		f.tokens[q] = set
	}
	return set[tok]
}

func (e *Engine) getSink(kind sinkKind, start int64) *sink {
	var s *sink
	if n := len(e.free); n > 0 {
		s = e.free[n-1]
		e.free = e.free[:n-1]
	} else {
		s = &sink{}
	}
	s.reset(kind, start)
	return s
}

func (e *Engine) putSink(s *sink) {
	e.Controls += s.b.Controls
	e.Invalid += s.b.Invalid
	s.b.Controls, s.b.Invalid = 0, 0
	if len(e.free) < 64 {
		e.free = append(e.free, s)
	}
}

// active returns the sink text goes to, or nil.
func (e *Engine) active() *sink {
	if e.cur != nil {
		return e.cur
	}
	return e.stray
}

// lookup returns the rule of the element on top of the stack.
func (e *Engine) lookup() *profile.Rule {
	key := e.keybuf[:0]
	n := min(len(e.frames), e.p.MaxSteps())
	// The key holds tested attribute values of up to 128 bytes; a longer
	// one makes the element's rule be looked up afresh, without building
	// the rest of the key, so that the cost per element does not grow
	// with the values its ancestors hold.
	cacheable := true
keyLoop:
	for i := range n {
		q := e.Name(i)
		key = append(key, q.URI...)
		key = append(key, 0)
		key = append(key, q.Local...)
		key = append(key, 0)
		for _, t := range e.p.TestedAttrs() {
			if v, ok := e.Attr(i, t); ok {
				if len(v) > 128 {
					cacheable = false
					break keyLoop
				}
				key = append(key, t.Local...)
				key = append(key, 1)
				key = append(key, v...)
				key = append(key, 2)
			}
		}
		key = append(key, 3)
	}
	if e.p.Anchored() && len(e.frames) <= e.p.MaxSteps() {
		key = append(key, 4)
	}
	e.keybuf = key
	if cacheable {
		if r, ok := e.cache[string(key)]; ok {
			return r
		}
	}
	r := e.p.Lookup(e)
	if cacheable && len(e.cache) < 4096 {
		e.cache[string(key)] = r
	}
	return r
}

func (e *Engine) top() *frame { return &e.frames[len(e.frames)-1] }

func (e *Engine) parent() *frame {
	if len(e.frames) < 2 {
		return nil
	}
	return &e.frames[len(e.frames)-2]
}

// StartElement implements xmlread.Handler.
func (e *Engine) StartElement(r *xmlread.Reader, el *xmlread.Element) {
	e.attrs = el.Attrs
	e.frames = append(e.frames, frame{name: profile.QName{URI: e.p.ElementNamespace(el.URI), Local: el.Local}, qname: el.Name, start: el.Start})
	f := e.top()
	if kept := e.p.KeptAttrs(); len(kept) > 0 {
		for _, a := range el.Attrs {
			for _, q := range kept {
				if a.Local == q.Local && a.URI == q.URI {
					f.kept = append(f.kept, a)
				}
			}
		}
	}
	p := e.parent()
	if p != nil {
		f.sections, f.ld, f.inItem, f.display, f.quotes = p.sections, p.ld, p.inItem, p.display, p.quotes
		f.capture, f.captureOf = p.capture, p.captureOf
		if e.doc != nil {
			// Content of a document rule's element: handed over as it is.
			f.role = roleDoc
			e.docStart(el)
			return
		}
		if p.role == roleSkip {
			f.role = roleSkip
			return
		}
	}
	f.marks, f.anchors = len(e.marks), len(e.anchors)
	f.partAt, f.idxAt = e.out.Part, len(e.out.Part.Blocks)
	e.readAttrs = e.readAttrs[:0]
	if e.hasChildAttrs {
		e.readAttrs = e.childAttrs()
	}
	if f.capture != nil {
		// Inside an element read for a value: its text, or its outline.
		f.role = roleInline
		if s := e.cur; s != nil && s.structure {
			e.structStart(s, el)
		} else if rule := e.lookup(); rule == nil {
			e.count(e.unmapped, el.Name)
		} else if rule.As == profile.AsInline && s != nil {
			if rule.Space {
				s.b.Space()
				f.spaced = s
			}
			if rule.Value != nil {
				e.inlineValue(r, rule, el)
			}
		} else if rule.As == profile.AsBreak && s != nil {
			// A value is one line: a break in it is a space.
			s.b.Space()
		}
		return
	}
	if p != nil && p.held != nil {
		h := p.held
		p.held = nil
		if el.Local == h.dupOf {
			f.dup = h
		} else {
			e.addHeld(h)
		}
	}
	if s, idx := e.claim(); s != nil {
		e.startCapture(f, s, idx, el)
		return
	}
	if t := e.calsTable(); t != nil && e.cur == nil {
		if e.calsStart(t, f, el) {
			return
		}
	}
	rule := e.lookup()
	f.rule = rule
	if rule == nil {
		e.startUnmapped(f, el)
		return
	}
	e.countAttrs(f, el)
	if rule.As == profile.AsSkip {
		f.role = roleSkip
		e.count(e.skipped, el.Name)
		return
	}
	if rule.As == profile.AsInline {
		e.startInline(f, el)
		if rule.Value != nil {
			e.inlineValue(r, rule, el)
		}
		return
	}
	if rule.As == profile.AsDocument {
		if e.cur != nil {
			// Inside an open block its text stays in that block.
			f.role = roleContainer
			return
		}
		e.flushStray()
		e.startDocument(f, el)
		return
	}
	if rule.As == profile.AsBreak {
		// A line break inside a block is a "\n" (core's hard break); in
		// a value, a label or a metadata field, a space; between blocks,
		// nothing.
		f.role = roleInline
		if s := e.active(); s != nil && !s.structure {
			if s.kind == skCapture {
				s.b.Space()
			} else {
				s.b.LineBreak()
			}
		}
		return
	}
	if e.cur != nil && (rule.As == profile.AsPart ||
		(rule.As == profile.AsTable || rule.As == profile.AsImage || rule.Split) && e.cur.kind == skBlock) && e.breakOpen(el.Start) {
		// The open block is finished; the part (or a table, an image or
		// a "split" rule's element met inside a paragraph-like block) is
		// read on its own, and what follows it in the block starts a new
		// block.
		f.resume, e.cur, f.suspended = e.cur, nil, true
		e.flushStray()
		e.startRule(f, el)
		return
	}
	if rule.OutOfLine && e.cur != nil && (e.cur.kind == skBlock || e.cur.kind == skHeading || e.cur.kind == skItem || e.inLabel()) {
		// An out-of-line note: a block of its own, made where it ends
		// (before the block it stands in, which is made at its end);
		// the open block goes on after it.
		f.resume, e.cur, f.outOfLine = e.cur, nil, true
		e.startRule(f, el)
		return
	}
	if e.cur != nil {
		if e.nested(f, el) {
			return
		}
	} else {
		e.flushStray()
	}
	e.startRule(f, el)
}

// inLabel reports whether the open sink is a label's.
func (e *Engine) inLabel() bool {
	if e.cur == nil || e.cur.kind != skCapture {
		return false
	}
	for i := len(e.frames) - 2; i >= 0; i-- {
		if o := &e.frames[i]; o.sink == e.cur {
			return o.rule != nil && o.rule.As == profile.AsLabel && o.capture == nil
		}
	}
	return false
}

// labelInline reports whether a label met inside the open block s is read
// as text of the block: when the block has text or a marker already, or a
// label is waiting for it, the label's text cannot be its marker.
func (e *Engine) labelInline(s *sink) bool {
	// An ordered list's generated number (auto) gives way to the file's
	// label: a label first in such an item is its marker.
	return s.b.Len() > 0 || s.marker != "" && !s.auto || e.label != nil
}

// count adds one to name in m, within MaxNamesCounted names.
func (e *Engine) count(m map[string]int, name string) {
	if _, ok := m[name]; !ok && len(m) >= MaxNamesCounted {
		name = "(other names)"
	}
	m[name]++
}

// countAttrs counts the attributes of a mapped element no rule reads.
func (e *Engine) countAttrs(f *frame, el *xmlread.Element) {
	for i := range el.Attrs {
		a := &el.Attrs[i]
		if generic.Skipped(el, a) || a.URI == xmlscan.XMLNamespace && a.Local == "id" {
			continue
		}
		q := profile.QName{URI: a.URI, Local: a.Local}
		if slices.Contains(f.rule.Reads, q) || slices.Contains(e.p.OfReads(f.name), q) || slices.Contains(e.readAttrs, q) {
			continue
		}
		e.count(e.unmappedAttrs, el.Name+"@"+a.Name)
	}
}

// claim finds an open element whose rule reads the element on top of the
// stack as a value (A4), and returns its unfilled slot.
func (e *Engine) claim() (*slot, int) {
	n := len(e.frames)
	for k := 1; k < n && k < e.p.MaxSteps(); k++ {
		o := &e.frames[n-1-k]
		for i := range o.slots {
			s := &o.slots[i]
			if !s.filled && s.v.AttrChild == nil && s.v.Claims().MatchFrom(e, k) {
				return s, n - 1 - k
			}
		}
	}
	return nil, 0
}

// childAttrs fills the slots of open elements that read an attribute of
// the element on top of the stack ({"attr": ..., "child": ...}), which is
// then read by its own rule as usual. It returns the attributes read.
func (e *Engine) childAttrs() []profile.QName {
	var read []profile.QName
	n := len(e.frames)
	for k := 1; k < n && k < e.p.MaxSteps(); k++ {
		o := &e.frames[n-1-k]
		for i := range o.slots {
			s := &o.slots[i]
			if s.filled || s.v.AttrChild == nil || !s.v.AttrChild.MatchFrom(e, k) {
				continue
			}
			s.filled = true
			read = append(read, s.v.Attr)
			if v, ok := e.Attr(0, s.v.Attr); ok {
				e.fill(o, s, e.clean(v))
			}
		}
	}
	return read
}

func (e *Engine) startCapture(f *frame, s *slot, owner int, el *xmlread.Element) {
	e.flushStray()
	s.filled = true
	f.role, f.capture, f.captureOf, f.structRoot = roleSink, s, owner, true
	// What the value gives its owner outlives this element.
	f.marks, f.anchors = math.MaxInt, math.MaxInt
	f.sink = e.getSink(skCapture, el.Start)
	f.prevCur, e.cur = e.cur, f.sink
	if s.v.Structure != nil {
		f.sink.structure = true
		f.sink.levels = append(f.sink.levels, structLevel{})
		return
	}
	// The attributes of a child read for its text are not read.
	for i := range el.Attrs {
		if a := &el.Attrs[i]; !generic.Skipped(el, a) && !e.readAttr(a) {
			e.count(e.unmappedAttrs, el.Name+"@"+a.Name)
		}
	}
}

// readAttr reports whether a rule of an open element read attribute a of
// the element being started.
func (e *Engine) readAttr(a *xmlread.Attr) bool {
	for _, q := range e.readAttrs {
		if q.Local == a.Local && q.URI == a.URI {
			return true
		}
	}
	return false
}

// structStart writes an element into an outline: name(attr=value, ...
func (e *Engine) structStart(s *sink, el *xmlread.Element) {
	lv := &s.levels[len(s.levels)-1]
	switch {
	case !lv.open && !lv.items && len(s.levels) > 1:
		s.sb = append(s.sb, '(')
		lv.open = true
	case lv.items:
		s.sb = append(s.sb, ", "...)
	}
	lv.items = true
	s.sb = append(s.sb, el.Name...)
	nl := structLevel{}
	for i := range el.Attrs {
		a := &el.Attrs[i]
		if generic.Skipped(el, a) {
			continue
		}
		if !nl.open {
			s.sb = append(s.sb, '(')
			nl.open = true
		} else {
			s.sb = append(s.sb, ", "...)
		}
		nl.items = true
		s.sb = append(s.sb, a.Name...)
		s.sb = append(s.sb, '=')
		s.sb = append(s.sb, a.Value...)
	}
	s.levels = append(s.levels, nl)
	e.boundStruct(s)
}

func (e *Engine) structEnd(s *sink) {
	if len(s.levels) <= 1 {
		return
	}
	if lv := s.levels[len(s.levels)-1]; lv.open {
		s.sb = append(s.sb, ')')
	}
	s.levels = s.levels[:len(s.levels)-1]
	e.boundStruct(s)
}

func (e *Engine) boundStruct(s *sink) {
	if len(s.sb) > generic.MaxValueBytes && !s.cut {
		s.cut = true
		e.ValuesCut++
	}
}

func (e *Engine) startUnmapped(f *frame, el *xmlread.Element) {
	e.count(e.unmapped, el.Name)
	if s := e.active(); s != nil {
		// Inside a block: its text stays in the block, its attributes
		// follow it as notes.
		f.role = roleInline
		s.after = e.attrNotes(el, s.after)
		if len(s.after) > generic.MaxPendingBlocks {
			// The notes waiting for the block are bounded, as in
			// generic reading: the block's text so far is given as a
			// piece (when the sink makes a block) and the notes follow.
			e.PendingCut++
			e.wordCut = true
			if e.givePiece(s, el.Start) {
				e.out.Splits++
			}
			e.wordCut = false
			after := s.after
			s.after = nil
			for _, b := range after {
				e.add(b, e.sinkFrame(s), false)
			}
		}
		return
	}
	f.role = roleContainer
	for _, b := range e.attrNotes(el, nil) {
		e.add(b, f, false)
	}
}

// attrNotes makes the notes of the attributes of an element with no rule,
// as the generic reader does.
func (e *Engine) attrNotes(el *xmlread.Element, dst []gofastcore.Block) []gofastcore.Block {
	for i := range el.Attrs {
		a := &el.Attrs[i]
		if a.Value == "" || generic.Skipped(el, a) || e.readAttr(a) {
			continue
		}
		text := e.clean(a.Value)
		if text == "" {
			continue
		}
		dst = append(dst, gofastcore.Block{Kind: gofastcore.KindNote, Origin: gofastcore.OriginInferred, Text: text,
			Attrs: map[string]string{gofastcore.AttrNoteType: gofastcore.NoteAttribute,
				generic.AttrAttribute: generic.CleanName(generic.CutString(a.Name, generic.MaxNameBytes), &e.scratch),
				generic.AttrElement:   e.path()},
			Loc: gofastcore.Location{Offset: el.Start, Length: el.End - el.Start}})
	}
	return dst
}

// path is the path of element names from the root, for blocks of
// generic reading.
func (e *Engine) path() string {
	var b strings.Builder
	for i := range e.frames {
		n := generic.CutString(e.frames[i].qname, generic.MaxNameBytes)
		if b.Len()+len(n)+1 > generic.MaxPathBytes {
			e.out.PathsCut++
			break
		}
		if i > 0 {
			b.WriteByte('/')
		}
		b.WriteString(n)
	}
	return generic.CleanName(b.String(), &e.scratch)
}

// clean applies the text conventions to a value, cut to MaxValueBytes.
func (e *Engine) clean(v string) string {
	if len(v) > generic.MaxValueBytes {
		v = generic.CutString(v, generic.MaxValueBytes)
		e.ValuesCut++
	}
	s := generic.Clean(v, false, &e.scratch)
	e.Controls += e.scratch.Controls
	e.Invalid += e.scratch.Invalid
	e.scratch.Controls, e.scratch.Invalid = 0, 0
	return s
}

// value resolves a value source that does not read a child. ok is false
// for one that does: it is read when the child comes.
func (e *Engine) value(v *profile.Value) (string, bool) {
	switch {
	case v == nil:
		return "", true
	case v.IsLiteral:
		return v.Literal, true
	case v.Claims() != nil, v.AttrChild != nil:
		return "", false
	case v.Of != nil:
		for i := len(e.frames) - 2; i >= 0; i-- {
			if e.frames[i].name == *v.Of {
				for _, a := range e.frames[i].kept {
					if a.Local == v.Attr.Local && a.URI == v.Attr.URI {
						return e.clean(a.Value), true
					}
				}
				return "", true
			}
		}
		return "", true
	case v.Codepoint:
		s, ok := e.Attr(0, v.Attr)
		if !ok {
			return "", true
		}
		if c, ok := codepoint(s); ok {
			return string(c), true
		}
		// Not a character: nothing is written, and the attribute is
		// counted as not read.
		for _, a := range e.attrs {
			if a.Local == v.Attr.Local && a.URI == v.Attr.URI {
				e.count(e.unmappedAttrs, e.top().qname+"@"+a.Name)
				break
			}
		}
		return "", true
	default:
		if s, ok := e.Attr(0, v.Attr); ok {
			return e.clean(s), true
		}
		return "", true
	}
}

// codepoint reads a code point written in hexadecimal (Formex
// QUOT.START@CODE="2018"): 1 to 6 digits, white space around them
// allowed. Surrogates, values past U+10FFFF and controls (category Cc)
// are not characters to write.
func codepoint(s string) (rune, bool) {
	s = strings.TrimSpace(s)
	if s == "" || len(s) > 6 {
		return 0, false
	}
	n, err := strconv.ParseUint(s, 16, 32)
	if err != nil || n > unicode.MaxRune || n >= 0xD800 && n <= 0xDFFF || unicode.IsControl(rune(n)) {
		return 0, false
	}
	return rune(n), true
}

// resolve resolves v now, or registers a slot for it on f.
func (e *Engine) resolve(f *frame, v *profile.Value, key string, use useKind) (string, bool) {
	s, ok := e.value(v)
	if !ok {
		f.slots = append(f.slots, slot{v: v, key: key, use: use})
	}
	return s, ok
}

func (e *Engine) startInline(f *frame, el *xmlread.Element) {
	f.role = roleInline
	s := e.active()
	if s == nil {
		e.flushHeld()
		s = e.newStray(el.Start)
	}
	r := f.rule
	if r.Space {
		s.b.Space()
		f.spaced = s
	}
	f.span, f.spanGen, f.spanStart = s, s.gen, s.b.Pos()
	if r.Link != nil {
		f.href, _ = e.resolve(f, r.Link, "", useLink)
	}
	if r.Title != nil {
		f.title, _ = e.resolve(f, r.Title, "", useTitle)
	}
	for _, x := range r.Attrs {
		if v, ok := e.resolve(f, x.Value, x.Key, useAttr); ok && v != "" {
			f.own = append(f.own, kv{x.Key, v})
		}
	}
	e.extras(f, r)
}

func (e *Engine) newStray(at int64) *sink {
	s := e.getSink(skStray, at)
	e.stray, e.strayAt = s, len(e.frames)-1
	if p := e.parent(); p != nil && e.top().role == roleInline {
		e.strayAt = len(e.frames) - 2
	}
	return s
}

// flushStray writes out the text that stood directly in a container.
func (e *Engine) flushStray() {
	s := e.stray
	if s == nil {
		return
	}
	e.stray = nil
	e.finishSink(s, &e.frames[min(e.strayAt, len(e.frames)-1)], gofastcore.KindParagraph, gofastcore.OriginInferred, s.end)
	e.putSink(s)
}

// nested handles a rule's element inside a block's text. It reports true
// when the element was taken into the text.
func (e *Engine) nested(f *frame, el *xmlread.Element) bool {
	s := e.cur
	r := f.rule
	switch s.kind {
	case skCapture:
		f.role = roleInline
		return true
	case skItem:
		if r.As == profile.AsLabel && e.labelInline(s) {
			f.role = roleInline
			s.b.Space()
			return true
		}
		switch r.As {
		case profile.AsSection, profile.AsTransparent, profile.AsLabel, profile.AsMetadata, profile.AsStored:
			return false
		case profile.AsBlock:
			if !s.done && s.b.Len() == 0 && r.Value == nil {
				f.role = roleInline
				f.broken = s
				e.applyOwn(f)
				return true
			}
		}
		e.closeItem()
		return false
	case skHeading:
		switch r.As {
		case profile.AsBlock, profile.AsList, profile.AsItem, profile.AsSection, profile.AsTransparent,
			profile.AsTable, profile.AsRow, profile.AsCell, profile.AsHeading, profile.AsNote:
			f.role = roleInline
			s.b.Space()
			return true
		}
		return false
	default: // skBlock, skCell
		if r.As == profile.AsLabel && (s.kind == skCell || e.labelInline(s)) {
			f.role = roleInline
			if !s.fresh {
				s.b.Space()
			}
			f.labelFresh = s.fresh
			return true
		}
		switch r.As {
		case profile.AsBlock, profile.AsHeading, profile.AsNote, profile.AsRow:
			f.role = roleInline
			if !s.fresh {
				s.b.Break()
			}
			s.fresh = false
			f.broken = s
			return true
		case profile.AsList:
			f.role = roleInline
			f.ordered = e.test(r.Ordered)
			f.counter = 0
			f.broken = s
			return true
		case profile.AsItem:
			f.role = roleInline
			f.broken = s
			s.b.Break()
			if p := e.parent(); p != nil {
				m := ""
				if r.Marker != nil {
					m, _ = e.value(r.Marker)
				} else if p.ordered {
					p.counter++
					m = strconv.Itoa(p.counter) + "."
				}
				if m != "" {
					s.b.Append([]byte(m+" "), false)
				}
			}
			s.fresh = true
			return true
		case profile.AsCell:
			f.role = roleInline
			if s.b.Len() > 0 {
				s.b.Append([]byte("\t"), true)
			}
			return true
		case profile.AsSection, profile.AsTransparent, profile.AsTable:
			f.role = roleInline
			return true
		}
		return false
	}
}

// closeItem gives the item being built its block; what follows in the
// item comes after it.
func (e *Engine) closeItem() {
	var f *frame
	for i := len(e.frames) - 1; i >= 0; i-- {
		if e.frames[i].sink == e.cur {
			f = &e.frames[i]
			break
		}
	}
	if f == nil {
		return
	}
	s := f.sink
	e.cur = f.prevCur
	f.sink = nil
	f.role = roleContainer
	f.ref = e.finishSink(s, f, gofastcore.KindListItem, gofastcore.OriginNative, s.end)
	e.putSink(s)
}

func (e *Engine) test(t *profile.Test) bool {
	if t == nil {
		return false
	}
	if t.Attr == nil {
		return t.Const
	}
	v, ok := e.Attr(0, *t.Attr)
	return ok && slices.Contains(t.In, strings.TrimSpace(v))
}

// startRule starts an element whose rule makes blocks or holds them.
func (e *Engine) startRule(f *frame, el *xmlread.Element) {
	r := f.rule
	f.role = roleContainer
	e.common(f, el)
	switch r.As {
	case profile.AsSection:
		f.sections++
	case profile.AsHeading, profile.AsBlock, profile.AsNote:
		if r.Value != nil {
			f.val, f.hasVal = e.resolve(f, r.Value, "", useValue)
			if r.Value.Claims() == nil && !r.Value.IsLiteral && r.Value.Of == nil {
				f.valAttr = r.Value.Attr.Local
			}
			return
		}
		kind := skBlock
		if r.Kind == gofastcore.KindQuote {
			f.quotes++
		}
		if r.As == profile.AsHeading {
			kind = skHeading
			if r.Level.Attr != nil {
				f.counter = spanValue(e.Attr(0, *r.Level.Attr))
			}
		}
		e.openSink(f, kind, el.Start)
		f.sink.verbatim = r.Kind == gofastcore.KindCode // code is verbatim
		if r.Marker != nil {
			f.sink.marker, _ = e.resolve(f, r.Marker, "", useMarker)
		}
		if r.Kind == gofastcore.KindFormula && el.URI == "http://www.w3.org/1998/Math/MathML" {
			e.mathml++
		}
	case profile.AsLabel:
		e.openSink(f, skCapture, el.Start)
	case profile.AsItem:
		p := e.parent()
		lvl := max(f.ld, 1)
		if f.inItem {
			lvl = f.ld + 1
		}
		f.ld, f.inItem = lvl, true
		if r.Value != nil {
			// The item's text when its element gives none (DITA's
			// topicref@href).
			f.val, f.hasVal = e.resolve(f, r.Value, "", useValue)
			if r.Value.Claims() == nil && !r.Value.IsLiteral && r.Value.Of == nil {
				f.valAttr = r.Value.Attr.Local
			}
		}
		e.openSink(f, skItem, el.Start)
		if r.Marker != nil {
			f.sink.marker, _ = e.resolve(f, r.Marker, "", useMarker)
		} else if p != nil && p.ordered && p.rule != nil && p.rule.As == profile.AsList {
			p.counter++
			f.sink.marker, f.sink.auto = strconv.Itoa(p.counter)+".", true
		}
	case profile.AsList:
		f.ld++
		f.inItem = false
		f.ordered = e.test(r.Ordered)
	case profile.AsTable:
		f.table = newTable(e, r.Model)
		if len(r.HeaderText) > 0 {
			e.headerRow(f.table, r.HeaderText)
		}
	case profile.AsRow:
		if t := e.tableOf(); t != nil {
			f.owner = t
			t.startRow(e.test(r.Header))
			if len(r.Cells) > 0 {
				e.attrRow(t, r.Cells)
			}
		}
	case profile.AsCell:
		if t := e.tableOf(); t != nil {
			f.owner = t
			cs, rs := 1, 1
			if r.Colspan != nil {
				cs = spanValue(e.Attr(0, *r.Colspan))
			}
			if r.Rowspan != nil {
				rs = spanValue(e.Attr(0, *r.Rowspan))
			}
			t.startCell(e, cs, rs, e.test(r.Header))
			f.cell = true
			e.openSink(f, skCell, el.Start)
		}
	case profile.AsImage:
		e.startImage(f, el)
	case profile.AsMetadata:
		if r.Value == nil {
			e.openSink(f, skCapture, el.Start)
		} else {
			f.val, f.hasVal = e.resolve(f, r.Value, "", useValue)
		}
	case profile.AsStored:
		e.openSink(f, skCapture, el.Start)
	case profile.AsPart:
		e.startPart(f)
	}
}

func spanValue(s string, ok bool) int {
	if !ok {
		return 1
	}
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil || n < 1 {
		return 1
	}
	return n
}

func (e *Engine) openSink(f *frame, kind sinkKind, at int64) {
	f.role = roleSink
	f.sink = e.getSink(kind, at)
	f.prevCur, e.cur = e.cur, f.sink
}

// common applies the keys every block-making or container rule may carry.
func (e *Engine) common(f *frame, el *xmlread.Element) {
	r := f.rule
	if r.Anchor != nil {
		if v, ok := e.resolve(f, r.Anchor, "", useAnchor); ok && v != "" {
			f.anchor = v
			e.anchors = append(e.anchors, anchorAt{v, len(e.frames)})
		}
	} else if v, ok := el.Attr(xmlscan.XMLNamespace, "id"); ok {
		f.anchor = e.clean(v)
	}
	for _, x := range r.Mark {
		if v, ok := e.resolve(f, x.Value, x.Key, useMark); ok && v != "" {
			e.marks = append(e.marks, mark{x.Key, v, len(e.frames)})
		}
	}
	container := r.As == profile.AsSection || r.As == profile.AsTransparent || r.As == profile.AsList ||
		r.As == profile.AsPart
	for _, x := range r.Attrs {
		use := useAttr
		if container {
			use = useFirst
		}
		if v, ok := e.resolve(f, x.Value, x.Key, use); ok && v != "" {
			if container {
				f.firstSet = append(f.firstSet, kv{x.Key, v})
			} else {
				f.own = append(f.own, kv{x.Key, v})
			}
		}
	}
	if d := r.Display; d != nil {
		if v, ok := e.Attr(0, d.Attr); ok && slices.Contains(d.In, strings.TrimSpace(v)) {
			f.display |= d.Reason
		}
	}
	e.extras(f, r)
}

func (e *Engine) extras(f *frame, r *profile.Rule) {
	for _, x := range r.Extra {
		if v, ok := e.resolve(f, x.Value, x.Key, useExtra); ok {
			e.setExtra(x.Key, v, r.Join)
		}
	}
}

func (e *Engine) setExtra(k, v, join string) {
	if v == "" {
		return
	}
	if e.extra == nil {
		e.extra = map[string]*strings.Builder{}
	}
	b := e.extra[k]
	if b == nil {
		b = &strings.Builder{}
		e.extra[k] = b
	}
	if b.Len() > 0 {
		if join == "" {
			join = "\n"
		}
		if b.Len()+len(join)+len(v) > generic.MaxValueBytes {
			e.ValuesCut++
			return
		}
		b.WriteString(join)
	}
	b.WriteString(v)
}

// flushExtra writes the collected Metadata.Extra values, once each.
func (e *Engine) flushExtra() {
	for k, b := range e.extra {
		if e.Meta.Extra == nil {
			e.Meta.Extra = map[string]string{}
		}
		e.Meta.Extra[k] = b.String()
	}
	e.extra = nil
}

// applyOwn gives a nested element's own attrs to the block it joins.
func (e *Engine) applyOwn(f *frame) {
	e.common(f, &xmlread.Element{Attrs: e.attrs})
}

func (e *Engine) startImage(f *frame, el *xmlread.Element) {
	r := f.rule
	if e.opts.NoImages {
		return
	}
	id, _ := e.resolve(f, r.ID, "", useID)
	alt := ""
	if r.Alt != nil {
		alt, _ = e.resolve(f, r.Alt, "", useAlt)
	}
	f.imgAlt = true
	b := gofastcore.Block{Kind: gofastcore.KindImage, Text: alt, Image: &gofastcore.Image{ID: id},
		Loc: gofastcore.Location{Offset: el.Start, Length: el.End - el.Start}}
	f.ref = e.add(b, f, true)
}

func (e *Engine) startPart(f *frame) {
	e.flushStray()
	e.flushLabel()
	cur := e.out.Part
	if !e.usedBy && len(e.parts) == 1 && len(cur.Blocks) == 0 {
		e.usedBy = true
	} else if len(e.parts) >= MaxParts {
		e.PartsCut++
		return
	} else {
		e.endPart()
		np := &gofastcore.Part{Number: len(e.parts) + 1}
		e.parts = append(e.parts, np)
		e.out.Part = np
		f.partAt, f.idxAt = np, 0
	}
	e.out.Part.Kind = f.rule.PartKind
	f.part = e.out.Part
	if f.rule.Name != nil {
		if v, ok := e.resolve(f, f.rule.Name, "", useName); ok {
			e.out.Part.Name = v
		}
	}
}

// endPart writes the warnings of the part being left.
func (e *Engine) endPart() {
	e.PartWarnings(e.out.Part)
	e.resetCounts()
}

// add appends b to the current part with what its place gives it: the
// marks and display reasons of its containers, its anchor, a waiting
// label. own says the block is made by f's own element.
func (e *Engine) add(b gofastcore.Block, f *frame, own bool) blockRef {
	if e.out.Stopped {
		return blockRef{}
	}
	if e.label != nil && b.Kind != gofastcore.KindNote && !(own && f != nil && f.outOfLine) {
		// A waiting label marks this block; one that cannot be its
		// marker (the block has one, or is a table or image) is a block
		// of its own before it, so that no label text is lost.
		if b.Marker != "" || b.Kind == gofastcore.KindTable || b.Kind == gofastcore.KindImage {
			e.flushLabel()
		} else {
			b.Marker = e.label.text
			e.label = nil
		}
	}
	if b.Origin == 0 {
		b.Origin = gofastcore.OriginNative
	}
	if f != nil {
		b.Display |= f.display
	}
	attrs := b.Attrs
	set := func(k, v string) {
		if attrs == nil {
			attrs = map[string]string{}
		}
		attrs[k] = v
	}
	for _, m := range e.marks {
		set(m.k, m.v)
	}
	// A container's attrs go to the first block made inside it.
	for i := range e.frames {
		if b.Attrs[gofastcore.AttrStored] != "" || b.Origin == gofastcore.OriginInferred && b.Kind == gofastcore.KindNote {
			break
		}
		c := &e.frames[i]
		if len(c.firstSet) > 0 && !c.first.ok() {
			for _, x := range c.firstSet {
				set(x.k, x.v)
			}
			c.first = blockRef{e.out.Part, len(e.out.Part.Blocks)}
		} else if !c.first.ok() && c.rule != nil && hasUse(c.slots, useFirst) {
			c.first = blockRef{e.out.Part, len(e.out.Part.Blocks)}
		}
	}
	if own && f != nil {
		for _, x := range f.own {
			set(x.k, x.v)
		}
		if b.Loc.Anchor == "" {
			b.Loc.Anchor = f.anchor
		}
	}
	if b.Loc.Anchor == "" {
		depth := len(e.frames) + 1
		if own && f != nil {
			depth = e.depthOf(f)
		}
		for i := len(e.anchors) - 1; i >= 0; i-- {
			if e.anchors[i].depth < depth {
				set(AttrAnchorParent, e.anchors[i].v)
				break
			}
		}
	}
	if b.Kind == gofastcore.KindHeading && b.Level > 6 {
		set("level", strconv.Itoa(int(b.Level)))
		b.Level = 6
		e.clamped++
	}
	b.Attrs = attrs
	cost := gofastcore.BlockCost + len(b.Text) + len(b.Marker)
	for k, v := range b.Attrs {
		cost += len(k) + len(v)
	}
	if b.Table != nil {
		for _, row := range b.Table.Rows {
			for _, c := range row {
				cost += gofastcore.CellCost + len(c)
			}
		}
		for _, d := range b.Table.Cells {
			cost += len(d.Href)
		}
	}
	for _, s := range b.Spans {
		cost += len(s.Href)
	}
	if !e.out.Budget.Take(cost) {
		e.out.Stopped = true
		return blockRef{}
	}
	e.out.Append(b)
	ref := blockRef{e.out.Part, len(e.out.Part.Blocks) - 1}
	if e.docs && b.Attrs[gofastcore.AttrStored] == "" {
		e.noteBlock(ref, b.Kind == gofastcore.KindHeading)
	}
	return ref
}

func hasUse(ss []slot, u useKind) bool {
	for i := range ss {
		if ss[i].use == u && !ss[i].filled {
			return true
		}
	}
	return false
}

func (e *Engine) depthOf(f *frame) int {
	for i := range e.frames {
		if &e.frames[i] == f {
			return i + 1
		}
	}
	return len(e.frames)
}

// patch sets an attribute on a block made already, from the budget.
func (e *Engine) patch(r blockRef, k, v string) {
	if !r.ok() || v == "" || e.out.Stopped {
		return
	}
	if !e.out.Budget.Take(len(k) + len(v)) {
		e.out.Stopped = true
		return
	}
	b := r.get()
	if b.Attrs == nil {
		b.Attrs = map[string]string{}
	}
	b.Attrs[k] = v
}

// finishSink makes the block of a sink, with the notes that follow it.
func (e *Engine) finishSink(s *sink, f *frame, kind gofastcore.Kind, origin gofastcore.Origin, end int64) blockRef {
	var ref blockRef
	if s.auto && e.label != nil {
		// The file's label numbers the item, not the list's count.
		s.marker, s.auto = "", false
	}
	fromValue := false
	if kind == gofastcore.KindListItem && s.b.Size() == 0 && f != nil && f.hasVal && f.val != "" && s.pieces == 0 && !s.done {
		s.b.Append([]byte(f.val), false)
		fromValue = true
	}
	if text := s.b.String(); text != "" || s.marker != "" && !s.auto {
		if text == "" {
			// A block whose only text is its label: the label is its text.
			text, s.marker = s.marker, ""
		}
		b := gofastcore.Block{Kind: kind, Origin: origin, Text: text, Marker: s.marker, Continued: s.pieces > 0,
			Loc: gofastcore.Location{Offset: s.pieceAt, Length: max(end-s.pieceAt, 0)}}
		if len(s.spans) > 0 {
			b.Spans = e.spans(s, len(text))
		}
		if origin == gofastcore.OriginInferred {
			b.Attrs = map[string]string{generic.AttrElement: e.pathTo(f)}
		}
		if kind == gofastcore.KindListItem {
			b.Level = uint8(min(max(f.ld, 1), 255))
			b.Ordered = e.orderedItem(f)
			if fromValue && f.valAttr != "" {
				b.Attrs = map[string]string{generic.AttrAttribute: f.valAttr}
			}
		}
		if kind == gofastcore.KindHeading {
			b.Level = uint8(min(e.level(f), 255))
		}
		e.kindAttrs(&b, f)
		ref = e.add(b, f, origin == gofastcore.OriginNative)
	}
	for _, b := range s.after {
		e.add(b, f, false)
	}
	s.after = nil
	return ref
}

func (e *Engine) orderedItem(f *frame) bool {
	i := e.depthOf(f) - 2
	return i >= 0 && e.frames[i].ordered && e.frames[i].rule != nil && e.frames[i].rule.As == profile.AsList
}

func (e *Engine) kindAttrs(b *gofastcore.Block, f *frame) {
	if b.Kind == gofastcore.KindQuote {
		q := 1
		if f != nil {
			q = f.quotes
		}
		b.Level = uint8(min(max(q, 1), 255))
	}
	if f == nil || f.rule == nil {
		return
	}
	r := f.rule
	set := func(k, v string) {
		if b.Attrs == nil {
			b.Attrs = map[string]string{}
		}
		b.Attrs[k] = v
	}
	if r.Language != "" {
		set("language", r.Language)
	}
	if r.Notation != "" {
		set("notation", r.Notation)
	}
	if r.As == profile.AsNote {
		set(gofastcore.AttrNoteType, r.NoteType)
	}
}

func (e *Engine) pathTo(f *frame) string {
	n := e.depthOf(f)
	saved := e.frames
	e.frames = e.frames[:n]
	p := e.path()
	e.frames = saved
	return p
}

// spans returns the spans of a sink that fall in a text of length n.
func (e *Engine) spans(s *sink, n int) []gofastcore.Span {
	var out []gofastcore.Span
	for _, sp := range s.spans {
		sp.End = min(sp.End, n)
		if sp.Start < sp.End {
			out = append(out, sp)
		}
	}
	return out
}

// level is the level of the heading made by f.
func (e *Engine) level(f *frame) int {
	r := f.rule
	switch {
	case r == nil:
		return 1
	case r.Level.Nesting:
		return max(e.p.HeadingBase+f.sections-1, 1)
	case r.Level.Attr != nil:
		return max(f.counter, 1) // read at the start into counter
	case r.Level.Fixed > 0:
		return r.Level.Fixed
	}
	return 1
}

// Text implements xmlread.Handler.
func (e *Engine) Text(r *xmlread.Reader, t []byte, start, end int64) {
	if len(e.frames) > 0 && e.top().role == roleSkip {
		return
	}
	s := e.active()
	if s == nil {
		if spaceOnly(t) || len(e.frames) == 0 {
			return
		}
		e.flushHeld()
		s = e.newStray(start)
	}
	if s.structure {
		if spaceOnly(t) {
			return
		}
		lv := &s.levels[len(s.levels)-1]
		if !lv.open && len(s.levels) > 1 {
			s.sb = append(s.sb, '(')
			lv.open = true
		} else if lv.items {
			s.sb = append(s.sb, ", "...)
		}
		lv.items = true
		s.sb = strconv.AppendQuote(s.sb, strings.TrimSpace(string(t)))
		e.boundStruct(s)
		return
	}
	preserve := s.verbatim
	if el := r.Top(); el != nil {
		preserve = preserve || el.Preserve
	}
	if e.doc != nil && s == e.doc.sink {
		// A document rule's text: handed over, and gathered as its
		// fallback paragraph, both cut at MaxBlockBytes.
		e.docText(t)
		if s.cut {
			return
		}
		if s.b.Size()+len(t) > generic.MaxBlockBytes {
			t = t[:cutAt(t, generic.MaxBlockBytes-s.b.Size())]
			s.cut, e.doc.cut = true, true
		}
	}
	if s.b.Size() > 0 && s.b.Size()+len(t) > generic.MaxBlockBytes {
		e.cutSink(s, start)
	}
	if s.cut {
		return
	}
	if s.restart && !spaceOnly(t) {
		s.restart, s.pieceAt = false, start
	}
	pos0, trail := s.b.Pos(), s.b.Trail()
	s.b.Append(t, preserve)
	s.word.Note(&s.b, t, start, end, pos0, trail, e.out.UTF8)
	s.end = max(s.end, end)
	if !spaceOnly(t) {
		s.fresh = false
	}
	if s.b.Len() > generic.MaxBlockBytes {
		e.cutSink(s, start)
	}
}

func spaceOnly(t []byte) bool {
	for _, c := range t {
		if c != ' ' && c != '\t' && c != '\n' && c != '\r' {
			return false
		}
	}
	return true
}

// cutSink deals with a text that reached MaxBlockBytes: a block's text so
// far is given as a piece and the rest continues it; a value or a cell is
// cut there.
func (e *Engine) cutSink(s *sink, at int64) {
	e.wordCut = true
	ok := e.givePiece(s, at)
	e.wordCut = false
	if !ok {
		if !s.cut {
			s.cut = true
			e.TextCut++
		}
		return
	}
	e.out.Splits++
}

// sinkFrame returns the frame whose element opened s, or nil.
func (e *Engine) sinkFrame(s *sink) *frame {
	for i := len(e.frames) - 1; i >= 0; i-- {
		if e.frames[i].sink == s {
			return &e.frames[i]
		}
	}
	return nil
}

// givePiece gives a block's text so far as a block of its own, when the
// sink is one that makes a block; it reports whether it did.
func (e *Engine) givePiece(s *sink, at int64) bool {
	f := e.sinkFrame(s)
	switch {
	case s.kind == skStray:
		f = &e.frames[min(e.strayAt, len(e.frames)-1)]
		e.piece(s, f, gofastcore.KindParagraph, gofastcore.OriginInferred, at)
	case f != nil && (s.kind == skBlock || s.kind == skItem || s.kind == skHeading) && f.capture == nil && f.rule != nil &&
		f.rule.As != profile.AsLabel && f.rule.DuplicateOf == "":
		kind := gofastcore.KindParagraph
		switch f.rule.As {
		case profile.AsHeading:
			kind = gofastcore.KindHeading
		case profile.AsItem:
			kind = gofastcore.KindListItem
		case profile.AsBlock:
			kind = f.rule.Kind
		case profile.AsNote:
			kind = gofastcore.KindNote
		}
		e.piece(s, f, kind, gofastcore.OriginNative, at)
	default:
		return false
	}
	return true
}

// breakOpen finishes the open block before a part that starts inside it:
// its text so far is a block, and text after the part starts a new block
// (not a continued piece: it is in another part). It reports false for a
// sink that makes no block of its own (a cell, a value), which the part
// is then read into as before.
func (e *Engine) breakOpen(at int64) bool {
	s := e.cur
	if s.structure {
		return false
	}
	if s.b.Size() == 0 && s.marker != "" && !s.auto && s.kind != skCell && s.kind != skCapture && s.kind != skStray {
		// A label that marks a block with no text yet (the block's text
		// so far is empty, as after an earlier break) is its text: the
		// piece made here holds it, so it is not lost.
		s.b.Append([]byte(s.marker), false)
		s.marker = ""
	}
	if !e.givePiece(s, at) {
		return false
	}
	s.pieces, s.restart = 0, true
	return true
}

func (e *Engine) piece(s *sink, f *frame, kind gofastcore.Kind, origin gofastcore.Origin, at int64) {
	for s.b.Len() > generic.MaxBlockBytes || s.b.Size() > 0 {
		var text string
		took, all, kept, end := 0, false, false, at
		if s.b.Len() > generic.MaxBlockBytes {
			text, took = s.b.SplitAt(generic.MaxBlockBytes)
			s.word.OK = false
		} else if e.wordCut && s.word.OK {
			// Cut while reading: at the last white space held, the word
			// after it kept for the text that continues it, which starts
			// where the word does.
			text, took, kept = s.b.SplitWord(s.word.Pos)
			all = !kept
			if kept {
				end = s.word.At
			}
			s.word.OK = false
		} else {
			text, all = s.b.String(), true
			took = len(text)
			s.b.Reset()
			s.word.OK = false
		}
		if text == "" {
			break
		}
		b := gofastcore.Block{Kind: kind, Origin: origin, Text: text, Marker: s.marker, Continued: s.pieces > 0,
			Loc: gofastcore.Location{Offset: s.pieceAt, Length: max(end-s.pieceAt, 0)}}
		b.Spans = e.cutSpans(s, len(text), took, all)
		if origin == gofastcore.OriginInferred {
			b.Attrs = map[string]string{generic.AttrElement: e.pathTo(f)}
		}
		if kind == gofastcore.KindListItem {
			b.Level = uint8(min(max(f.ld, 1), 255))
		}
		if kind == gofastcore.KindHeading {
			b.Level = uint8(min(e.level(f), 255))
		}
		e.kindAttrs(&b, f)
		r := e.add(b, f, s.pieces == 0)
		if s.pieces == 0 && f != nil {
			f.ref = r
		}
		s.marker = ""
		s.pieces++
		s.pieceAt = end
		if kept {
			break
		}
	}
	if s.b.Size() == 0 {
		s.spans = s.spans[:0]
	}
	if s.kind == skItem {
		s.done = true
	}
}

// cutSpans returns the spans of a sink's piece of n bytes, the first took
// bytes of its text (all of it when all is set), and moves the spans and
// open inline elements of the rest to the rest's offsets. A span across
// the cut is in both pieces.
func (e *Engine) cutSpans(s *sink, n, took int, all bool) []gofastcore.Span {
	var out []gofastcore.Span
	rest := s.spans[:0]
	for _, sp := range s.spans {
		if p := (gofastcore.Span{Start: sp.Start, End: min(sp.End, n), Style: sp.Style, Href: sp.Href, Display: sp.Display}); p.Start < p.End {
			out = append(out, p)
		}
		sp.Start, sp.End = max(sp.Start-took, 0), sp.End-took
		if !all && sp.Start < sp.End {
			rest = append(rest, sp)
		}
	}
	s.spans = rest
	for i := range e.frames {
		if f := &e.frames[i]; f.span == s && f.spanGen == s.gen {
			// An inline element still open: its text so far is a span of
			// this piece; the rest of it will be one of the next.
			if r := f.rule; e.opts.Spans && r != nil && (r.Style != 0 || f.href != "") && f.spanStart < n && len(out) < 100000 {
				out = append(out, gofastcore.Span{Start: f.spanStart, End: n, Style: r.Style, Href: f.href})
			}
			f.spanStart = max(f.spanStart-took, 0)
			if all {
				f.spanStart = 0
			}
		}
	}
	return out
}

// Comment implements xmlread.Handler: a comment is a note marked stored,
// at its place.
func (e *Engine) Comment(r *xmlread.Reader, t []byte, final bool, start, end int64) {
	if !e.inCom {
		e.inCom, e.comStart, e.comParts = true, start, 0
		e.com.Reset()
	}
	if e.com.Size() > 0 && e.com.Size()+len(t) > generic.MaxBlockBytes {
		e.comment(e.com.String(), start)
		e.com.Reset()
		e.comStart = start
	}
	e.com.Append(t, true)
	for e.com.Len() > generic.MaxBlockBytes {
		e.comment(e.com.Split(generic.MaxBlockBytes), end)
		e.comStart = end
	}
	if final {
		e.comment(e.com.String(), end)
		e.com.Reset()
		e.inCom = false
	}
}

func (e *Engine) comment(text string, end int64) {
	if text == "" {
		return
	}
	b := gofastcore.Block{Kind: gofastcore.KindNote, Text: text, Continued: e.comParts > 0,
		Attrs: map[string]string{gofastcore.AttrStored: gofastcore.StoredMarkupComment},
		Loc:   gofastcore.Location{Offset: e.comStart, Length: max(end-e.comStart, 0)}}
	e.comParts++
	if s := e.active(); s != nil && len(s.after) < generic.MaxPendingBlocks {
		s.after = append(s.after, b)
		return
	}
	var f *frame
	if len(e.frames) > 0 {
		f = e.top()
	}
	e.add(b, f, false)
}

// EndElement implements xmlread.Handler.
func (e *Engine) EndElement(r *xmlread.Reader, el *xmlread.Element, start, end int64) {
	n := len(e.frames)
	if n == 0 {
		return
	}
	f := &e.frames[n-1]
	var ref blockRef
	switch {
	case f.role == roleSkip:
	case f.role == roleDoc:
		e.docEnd(el)
	case f.isDoc:
		ref = e.endDocument(f, start, end)
	case f.capture != nil && f.structRoot:
		ref = e.endCapture(f)
	case f.capture != nil:
		if s := e.cur; s != nil && s.structure {
			e.structEnd(s)
		}
	case f.role == roleInline:
		e.endInline(f, end)
		if f.rule != nil && f.rule.As == profile.AsLabel && e.cur != nil {
			// A label read inline is followed by a space; one that starts
			// a line (an item's) keeps the line open for what follows.
			e.cur.b.Space()
			e.cur.fresh = f.labelFresh
		}
	case f.cals != "":
		e.calsEnd(f, f.start, end)
	default:
		ref = e.endRule(f, end)
	}
	if f.spaced != nil && f.spaced == e.active() {
		f.spaced.b.Space()
	}
	if f.broken != nil && f.broken == e.active() {
		f.broken.b.Break()
	}
	if f.held != nil {
		e.addHeld(f.held)
		f.held = nil
	}
	if e.stray != nil && e.strayAt >= n-1 {
		e.flushStray()
	}
	if e.label != nil && e.label.depth >= n-1 {
		e.flushLabel() // a label whose container ends with no block after it
	}
	if f.suspended || f.outOfLine {
		e.cur = f.resume
	}
	if len(f.kids) > 0 {
		e.endSection(f)
	}
	e.marks = e.marks[:min(f.marks, len(e.marks))]
	e.anchors = e.anchors[:min(f.anchors, len(e.anchors))]
	if p := e.parent(); p != nil {
		p.lastChild, p.lastRef = el.Local, ref
	}
	e.frames = e.frames[:n-1]
}

// inlineValue writes an inline rule's "value", an attribute of its own
// element, in place in the open block's text, as the element's text
// would be (TEI p/pb@n, Formex QUOT.START@CODE).
func (e *Engine) inlineValue(r *xmlread.Reader, rule *profile.Rule, el *xmlread.Element) {
	if v, _ := e.value(rule.Value); v != "" {
		e.Text(r, []byte(v), el.Start, el.Start)
	}
}

func (e *Engine) endInline(f *frame, end int64) {
	s := f.span
	if s == nil || f.rule == nil || s.gen != f.spanGen {
		// No sink, or the block the inline began in was finished (its
		// sink is now another block's).
		return
	}
	s.end = max(s.end, end)
	r := f.rule
	if stop := s.b.Len(); e.opts.Spans && (r.Style != 0 || f.href != "") && stop > f.spanStart {
		sp := gofastcore.Span{Start: f.spanStart, End: stop, Style: r.Style, Href: f.href}
		if s.kind == skCell {
			// A cell's link is in Table.Cells.
			if s.href == "" && f.href != "" {
				s.href = f.href
			}
			sp.Href = ""
		}
		switch {
		case sp.Style == 0 && sp.Href == "":
		case len(s.spans) < 100000:
			s.spans = append(s.spans, sp)
		default:
			e.SpansCut++
		}
	}
	if f.title != "" || len(f.own) > 0 {
		// What a link has besides its address: a record in the stored
		// part (S6: core's Span has no field for it). Its text is the
		// link's own text (its address when it has none); the title is an
		// attr of its own.
		attrs := map[string]string{gofastcore.AttrStored: gofastcore.StoredCustomProperty, gofastcore.AttrName: f.qname}
		if f.title != "" {
			attrs[AttrTitle] = f.title
		}
		text := e.clean(spanText(s, f.spanStart))
		if text == "" {
			text = f.href
		}
		if text == "" {
			return
		}
		if f.href != "" {
			attrs[AttrHref] = f.href
		}
		for _, x := range f.own {
			attrs[x.k] = x.v
		}
		e.addStored(gofastcore.Block{Kind: gofastcore.KindNote, Text: text, Attrs: attrs,
			Loc: gofastcore.Location{Offset: f.start, Length: max(end-f.start, 0)}})
	}
}

func spanText(s *sink, from int) string {
	t := s.b.String()
	if from >= len(t) {
		return ""
	}
	return t[from:]
}

func (e *Engine) storedPart() *gofastcore.Part {
	if e.stored == nil {
		e.stored = &gofastcore.Part{Kind: gofastcore.PartStored}
	}
	return e.stored
}

func (e *Engine) addStored(b gofastcore.Block) {
	if e.out.Stopped {
		return
	}
	saved := e.out.Part
	e.out.Part = e.storedPart()
	b.Origin = gofastcore.OriginNative
	cost := gofastcore.BlockCost + len(b.Text)
	for k, v := range b.Attrs {
		cost += len(k) + len(v)
	}
	if e.out.Budget.Take(cost) {
		e.out.Part.Blocks = append(e.out.Part.Blocks, b)
	} else {
		e.out.Stopped = true
	}
	e.out.Part = saved
}

// endCapture ends an element read for a value, and gives the value where
// it goes.
func (e *Engine) endCapture(f *frame) blockRef {
	s := f.sink
	e.cur = f.prevCur
	var v string
	if s.structure {
		v = string(s.sb)
		if len(v) > generic.MaxValueBytes {
			v = generic.CutString(v, generic.MaxValueBytes)
		}
		v = generic.CleanName(strings.ToValidUTF8(v, "�"), &e.scratch)
	} else {
		v = s.b.String()
	}
	for _, b := range s.after {
		e.add(b, f, false)
	}
	e.putSink(s)
	f.sink = nil
	if f.captureOf < len(e.frames) {
		e.fill(&e.frames[f.captureOf], f.capture, v)
	}
	return blockRef{}
}

// fill gives a value read from a child to the use its owner has for it.
func (e *Engine) fill(o *frame, s *slot, v string) {
	if v == "" {
		return
	}
	switch s.use {
	case useAttr:
		if o.ref.ok() {
			e.patch(o.ref, s.key, v)
		} else {
			o.own = append(o.own, kv{s.key, v})
		}
	case useFirst:
		if o.first.ok() {
			e.patch(o.first, s.key, v)
		} else {
			o.firstSet = append(o.firstSet, kv{s.key, v})
		}
	case useMark:
		// Blocks made inside the owner already, then those to come.
		if o.partAt == e.out.Part {
			for i := o.idxAt; i < len(o.partAt.Blocks); i++ {
				e.patch(blockRef{o.partAt, i}, s.key, v)
			}
		}
		e.marks = append(e.marks, mark{s.key, v, e.depthOf(o)})
	case useExtra:
		join := ""
		if o.rule != nil {
			join = o.rule.Join
		}
		e.setExtra(s.key, v, join)
	case useMarker:
		if o.sink != nil {
			o.sink.marker = v
		} else if o.ref.ok() {
			o.ref.get().Marker = v
		}
	case useAlt:
		if o.ref.ok() && !e.out.Stopped {
			if e.out.Budget.Take(len(v)) {
				o.ref.get().Text = v
			} else {
				e.out.Stopped = true
			}
		}
	case useID:
		if o.ref.ok() && o.ref.get().Image != nil {
			o.ref.get().Image.ID = v
		}
	case useName:
		if o.part != nil {
			o.part.Name = v
		}
	case useValue:
		o.val, o.hasVal = v, true
	case useLink:
		o.href = v
	case useTitle:
		o.title = v
	case useAnchor:
		o.anchor = v
		e.anchors = append(e.anchors, anchorAt{v, e.depthOf(o)})
	}
}

// endRule ends an element whose rule makes or holds blocks.
func (e *Engine) endRule(f *frame, end int64) blockRef {
	r := f.rule
	if r == nil {
		return blockRef{}
	}
	if f.dup != nil && f.sink == nil && !f.hasVal {
		defer func() {
			if f.dup != nil {
				e.addHeld(f.dup)
			}
		}()
	}
	switch r.As {
	case profile.AsHeading, profile.AsBlock, profile.AsNote:
		kind := r.Kind
		switch r.As {
		case profile.AsHeading:
			kind = gofastcore.KindHeading
		case profile.AsNote:
			kind = gofastcore.KindNote
		}
		if f.sink == nil {
			if f.hasVal && f.val != "" {
				b := gofastcore.Block{Kind: kind, Text: f.val, Loc: gofastcore.Location{Offset: f.start, Length: max(end-f.start, 0)}}
				if f.valAttr != "" {
					b.Attrs = map[string]string{generic.AttrAttribute: f.valAttr}
				}
				if kind == gofastcore.KindHeading {
					b.Level = uint8(min(e.level(f), 255))
				}
				e.kindAttrs(&b, f)
				f.ref = e.add(b, f, true)
			}
			return f.ref
		}
		s := e.closeSink(f)
		if r.DuplicateOf != "" && s.pieces == 0 {
			e.holdDuplicate(f, s, kind, end)
		} else {
			f.ref = e.finishSink(s, f, kind, gofastcore.OriginNative, end)
			if s.pieces > 0 {
				e.out.Splits++
			}
		}
		e.putSink(s)
		e.matchDuplicate(f)
	case profile.AsItem:
		if f.sink != nil {
			s := e.closeSink(f)
			f.ref = e.finishSink(s, f, gofastcore.KindListItem, gofastcore.OriginNative, end)
			e.putSink(s)
		} else if f.ref.ok() {
			l := f.ref.get()
			l.Loc.Length = max(end-l.Loc.Offset, 0)
		}
	case profile.AsLabel:
		s := e.closeSink(f)
		text := s.b.String()
		e.keepAfter(s, f)
		e.putSink(s)
		if text == "" {
			break
		}
		if o := e.cur; o != nil && o.kind != skCapture && o.kind != skStray {
			if o.marker == "" || o.auto {
				// The file's label is the marker; an ordered list's
				// generated number gives way to it.
				o.marker, o.auto = text, false
			}
			break
		}
		e.flushLabel()
		e.label = &labelAt{text: text, depth: len(e.frames) - 2, start: f.start, end: end}
	case profile.AsCell:
		if f.sink != nil {
			s := e.closeSink(f)
			if t := f.owner; t != nil {
				t.endCell(s.b.String(), s.href)
			}
			for _, b := range s.after {
				e.add(b, f, false)
			}
			e.putSink(s)
		}
	case profile.AsRow:
		if t := f.owner; t != nil {
			t.endRow()
			t.rowDone(f.start, end, f.anchor)
		}
	case profile.AsTable:
		e.flushStray()
		if t := f.table; t != nil {
			if b, ok := t.block(e, f.start, end); ok {
				f.ref = e.add(b, f, true)
			}
		}
	case profile.AsImage:
		if f.ref.ok() {
			l := f.ref.get()
			l.Loc.Length = max(end-l.Loc.Offset, 0)
		}
	case profile.AsMetadata:
		v := f.val
		if f.sink != nil {
			s := e.closeSink(f)
			v = s.b.String()
			e.keepAfter(s, f)
			e.putSink(s)
		}
		e.metadata(f, v, end)
	case profile.AsStored:
		s := e.closeSink(f)
		if text := s.b.String(); text != "" {
			e.addStored(gofastcore.Block{Kind: gofastcore.KindNote, Text: text,
				Attrs: map[string]string{gofastcore.AttrStored: r.StoredKind},
				Loc:   gofastcore.Location{Offset: f.start, Length: max(end-f.start, 0)}})
		}
		e.keepAfter(s, f)
		e.putSink(s)
	}
	return f.ref
}

// keepAfter gives the notes that followed a value's text (the attributes
// of elements with no rule, comments) a place, so that they are not
// lost with the sink: after the open block, else where the element
// stood.
func (e *Engine) keepAfter(s *sink, f *frame) {
	for _, b := range s.after {
		if o := e.active(); o != nil && o != s && len(o.after) < generic.MaxPendingBlocks {
			o.after = append(o.after, b)
		} else {
			e.add(b, f, false)
		}
	}
	s.after = nil
}

func (e *Engine) closeSink(f *frame) *sink {
	s := f.sink
	e.cur = f.prevCur
	f.sink = nil
	return s
}

// holdDuplicate keeps the block of an element that may repeat a sibling
// (duplicate_of) until that sibling is known.
func (e *Engine) holdDuplicate(f *frame, s *sink, kind gofastcore.Kind, end int64) {
	text := s.b.String()
	p := e.parent()
	if p != nil && p.lastChild == f.rule.DuplicateOf && p.lastRef.ok() {
		// The sibling came first.
		if b := p.lastRef.get(); b.Text != text && text != "" {
			e.patch(p.lastRef, f.rule.Key, text)
		}
		for _, b := range s.after {
			e.add(b, f, false)
		}
		return
	}
	b := gofastcore.Block{Kind: kind, Text: text, Marker: s.marker,
		Loc: gofastcore.Location{Offset: s.pieceAt, Length: max(end-s.pieceAt, 0)}}
	if len(s.spans) > 0 {
		b.Spans = e.spans(s, len(text))
	}
	if kind == gofastcore.KindHeading {
		b.Level = uint8(min(e.level(f), 255))
	}
	e.kindAttrs(&b, f)
	if p == nil || text == "" {
		if text != "" {
			f.ref = e.add(b, f, true)
		}
		return
	}
	for _, x := range f.own {
		if b.Attrs == nil {
			b.Attrs = map[string]string{}
		}
		b.Attrs[x.k] = x.v
	}
	if f.anchor != "" {
		b.Loc.Anchor = f.anchor
	}
	p.held = &held{block: b, dupOf: f.rule.DuplicateOf, key: f.rule.Key}
	for _, a := range s.after {
		e.add(a, f, false)
	}
}

// matchDuplicate compares a block with the sibling held before it.
func (e *Engine) matchDuplicate(f *frame) {
	h := f.dup
	if h == nil {
		return
	}
	f.dup = nil
	if !f.ref.ok() {
		e.addHeld(h)
		return
	}
	if b := f.ref.get(); b.Text != h.block.Text {
		e.patch(f.ref, h.key, h.block.Text)
	}
}

func (e *Engine) addHeld(h *held) {
	var f *frame
	if len(e.frames) > 0 {
		f = e.top()
	}
	e.add(h.block, f, false)
}

// flushHeld writes out a held block before text that follows it.
func (e *Engine) flushHeld() {
	if len(e.frames) == 0 {
		return
	}
	if f := e.top(); f.held != nil {
		h := f.held
		f.held = nil
		e.addHeld(h)
	}
}

// flushLabel writes a label no block took as a paragraph of its own.
func (e *Engine) flushLabel() {
	l := e.label
	if l == nil {
		return
	}
	e.label = nil
	var f *frame
	if l.depth >= 0 && l.depth < len(e.frames) {
		f = &e.frames[l.depth]
	}
	e.add(gofastcore.Block{Kind: gofastcore.KindParagraph, Text: l.text,
		Loc: gofastcore.Location{Offset: l.start, Length: max(l.end-l.start, 0)}}, f, false)
}

// metadata sets a metadata field.
func (e *Engine) metadata(f *frame, v string, end int64) {
	r := f.rule
	if v == "" {
		return
	}
	m := &e.Meta
	switch r.Field {
	// A field that holds one value takes the first; the others go to
	// Extra under the field's name, so that none is lost.
	case "title":
		switch {
		case m.Title == "":
			m.Title = v
		case r.AlsoHeading == 0:
			e.setExtra("title", v, r.Join)
		}
		if r.AlsoHeading > 0 {
			e.add(gofastcore.Block{Kind: gofastcore.KindHeading, Level: uint8(r.AlsoHeading), Text: v,
				Loc: gofastcore.Location{Offset: f.start, Length: max(end-f.start, 0)}}, f, true)
		}
	case "author":
		if len(m.Authors) < 1000 {
			m.Authors = append(m.Authors, v)
		} else {
			e.ValuesCut++
		}
	case "keyword":
		if len(m.Keywords) < 1000 {
			m.Keywords = append(m.Keywords, v)
		} else {
			e.ValuesCut++
		}
	case "subject":
		if m.Subject == "" {
			m.Subject = v
		} else {
			e.setExtra("subject", v, r.Join)
		}
	case "language":
		if m.Language == "" && len(v) <= generic.MaxLangBytes {
			m.Language = v
		} else {
			e.setExtra("language", v, r.Join)
		}
	case "created", "modified":
		t, ok := parseDate(v)
		switch {
		case r.Field == "created" && ok && m.Created.IsZero():
			m.Created = t
		case r.Field == "modified" && ok && m.Modified.IsZero():
			m.Modified = t
		default:
			e.setExtra(r.Field, v, r.Join)
		}
	case "description":
		e.setExtra("description", v, r.Join)
	case "extra":
		e.setExtra(r.Key, v, r.Join)
	}
}

func parseDate(s string) (time.Time, bool) {
	for _, layout := range []string{time.RFC3339Nano, "2006-01-02T15:04:05", "2006-01-02"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// Finish ends the reading at offset end.
func (e *Engine) Finish(end int64) {
	if e.inCom {
		e.comment(e.com.String(), end)
		e.inCom = false
	}
	for len(e.frames) > 0 {
		e.EndElement(nil, &xmlread.Element{}, end, end)
	}
	e.flushStray()
	e.flushLabel()
	e.flushExtra()
}

// Counts returns the control characters removed and invalid bytes met in
// all the text made.
func (e *Engine) Counts() (controls, invalid int) {
	return e.Controls + e.com.Controls + e.scratch.Controls, e.Invalid + e.com.Invalid + e.scratch.Invalid
}

// PartWarnings writes the engine's warnings about part p.
func (e *Engine) PartWarnings(p *gofastcore.Part) {
	if len(e.unmapped) > 0 {
		p.Warn(gofastcore.SeverityInfo, WarnUnmappedElements, "elements with no rule in the profile, read generically: "+names(e.unmapped))
	}
	if len(e.unmappedAttrs) > 0 {
		p.Warn(gofastcore.SeverityInfo, WarnUnmappedAttributes, "attributes no rule of the profile reads: "+names(e.unmappedAttrs))
	}
	if len(e.skipped) > 0 {
		p.Warn(gofastcore.SeverityInfo, WarnSkipped, "elements left out by the profile's skip rules: "+names(e.skipped))
	}
	if e.clamped > 0 {
		p.Warn(gofastcore.SeverityInfo, gofastcore.WarnHeadingLevelClamped,
			fmt.Sprintf("%d headings deeper than level 6 have level 6, their level in Attrs[\"level\"]", e.clamped))
	}
	if e.mathml > 0 {
		p.Warn(gofastcore.SeverityWarning, gofastcore.WarnInputUnreadContent,
			fmt.Sprintf("MathML (%d formulas) is not converted; its text is kept as plain text", e.mathml))
	}
}

// Warning codes of the engine.
const (
	WarnUnmappedElements   = "xml.unmapped_elements"
	WarnUnmappedAttributes = "xml.unmapped_attributes"
	WarnSkipped            = "xml.skipped_elements"
)

// MaxNamesListed bounds the names a warning lists.
const MaxNamesListed = 100

func names(m map[string]int) string {
	type nc struct {
		n string
		c int
	}
	var l []nc
	total := 0
	for k, v := range m {
		l = append(l, nc{k, v})
		total += v
	}
	sort.Slice(l, func(i, j int) bool {
		if l[i].c != l[j].c {
			return l[i].c > l[j].c
		}
		return l[i].n < l[j].n
	})
	var b strings.Builder
	fmt.Fprintf(&b, "%d in all", total)
	for i, x := range l {
		if i == MaxNamesListed {
			fmt.Fprintf(&b, "; and %d other names", len(l)-i)
			break
		}
		sep := "; "
		if i == 0 {
			sep = ": "
		}
		var clean generic.Builder
		fmt.Fprintf(&b, "%s%s (%d)", sep, generic.CleanName(strings.ToValidUTF8(generic.CutString(x.n, generic.MaxNameBytes), "�"), &clean), x.c)
	}
	return b.String()
}
