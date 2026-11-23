// Adapted from github.com/AOShei/go-fast-odf internal/xmlscan (commit
// 2cf8642), itself copied from github.com/AOShei/go-fast-ooxml
// internal/xmlscan (MIT License, Copyright (c) 2025 Andrew O'Shei; see
// LICENSE and NOTICE). Extended here for XML in general: namespace URIs,
// token offsets, comments, processing instructions and the document type
// declaration as tokens, general entities expanded within bounds, and
// recovery from damaged markup.

// Package xmlscan is a streaming pull tokenizer for XML.
//
//   - It does not allocate per token. Names, attribute values and text are
//     slices of the scanner's window, valid until the next call to Next.
//   - It reads through a window that is refilled as it goes, so input of
//     any size is read in bounded memory. Text, CDATA and comments are
//     delivered in chunks when they do not fit; a tag, a processing
//     instruction and the document type declaration must fit, up to
//     Options.MaxTokenBytes.
//   - It resolves namespace prefixes, to the URI and to small numbers the
//     caller chooses.
//   - It never reads anything but its input. General entity references
//     other than the five predefined ones are given to Options.Resolve,
//     and a replacement read as markup is read from memory, within a bound
//     on nesting and on the bytes all expansions produce together.
//   - It recovers from damaged markup the way a lenient reader does and
//     counts each repair: an end tag that matches an element further out
//     closes what is inside it, one that matches nothing is dropped,
//     elements left open at the end are closed, a "<" or "&" that starts
//     no markup is text, an attribute without a value or quotes is read as
//     far as it goes.
//   - Elements nested deeper than Options.MaxDepth are read as part of
//     their ancestor: their tags are passed over, their text is kept.
package xmlscan

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math"
	"unicode/utf8"
)

// Kind is the type of a token.
type Kind uint8

const (
	// EOF means the input ended or an error occurred. Err tells which.
	EOF Kind = iota
	// Start is a start tag. An empty-element tag (<a/>) gives Start and
	// then End.
	Start
	// End is an end tag.
	End
	// Text is character data or a CDATA section. One run of text in the
	// source may arrive as several Text tokens.
	Text
	// Comment is the content of a comment. A long comment arrives as
	// several Comment tokens; Final reports the last one.
	Comment
	// PI is a processing instruction: Target and Text.
	PI
	// Doctype is a document type declaration. Text is what lies between
	// "<!DOCTYPE" and the final ">".
	Doctype
)

// Space identifies an XML namespace. Callers number the namespaces they
// care about from FirstSpace and pass the numbering in Options.Spaces.
type Space uint16

const (
	// NoSpace is the absence of a namespace: an attribute without a
	// prefix, or an element where no default namespace is in force.
	NoSpace Space = iota
	// SpaceXML is the namespace of the reserved "xml" prefix (xml:space).
	SpaceXML
	// SpaceUnknown is any namespace not listed in Options.Spaces, and any
	// prefix that was never declared.
	SpaceUnknown
	// FirstSpace is the first number available to callers.
	FirstSpace
)

// The namespaces bound to the reserved prefixes.
const (
	XMLNamespace   = "http://www.w3.org/XML/1998/namespace"
	XMLNSNamespace = "http://www.w3.org/2000/xmlns/"
)

// ErrTokenTooLarge is the error when a tag, a processing instruction or
// the document type declaration is larger than Options.MaxTokenBytes.
var ErrTokenTooLarge = errors.New("xmlscan: a tag is larger than the token limit")

// ErrReferenceTooLarge is the ErrTokenTooLarge of a character reference
// longer than the token limit (leading zeros); errors.Is matches both.
var ErrReferenceTooLarge = fmt.Errorf("%w: a character reference", ErrTokenTooLarge)

// Resolution says how a general entity reference is read.
type Resolution uint8

const (
	// Keep keeps the reference in the text as written.
	Keep Resolution = iota
	// Insert puts the replacement in the text as it is, as characters.
	Insert
	// Expand reads the replacement in place of the reference, as markup.
	Expand
)

// Resolver decides how a general entity reference other than the five
// predefined ones is read, and gives its replacement.
type Resolver func(name []byte) (repl []byte, how Resolution)

// Options configures a Scanner. The zero value is usable.
type Options struct {
	// Spaces maps namespace URIs to the caller's numbers. It is only read.
	Spaces map[string]Space
	// MaxDepth bounds element nesting: an element deeper is read as part
	// of its ancestor, and counted in Stats.Flattened. Zero means
	// DefaultMaxDepth.
	MaxDepth int
	// MaxTokenBytes bounds the size of one tag, processing instruction or
	// document type declaration. Zero means DefaultMaxTokenBytes.
	MaxTokenBytes int
	// MaxAttrs bounds the attributes kept from one tag; the rest are
	// counted in Stats.AttrsCut. Zero means DefaultMaxAttrs.
	MaxAttrs int
	// BufferSize is the initial window size when reading from an
	// io.Reader. Zero means DefaultBufferSize.
	BufferSize int
	// Resolve decides general entity references other than the
	// predefined ones. Nil keeps them as written, counted as repairs.
	Resolve Resolver
	// MaxEntityDepth bounds how deeply replacements read as markup nest.
	// Zero means DefaultMaxEntityDepth.
	MaxEntityDepth int
	// ExpansionLimit bounds the bytes of replacement text read in place
	// of references, all expansions together. Zero means no expansion at
	// all; negative means no limit.
	ExpansionLimit int64
	// BaseOffset is the offset of the first byte read, added to every
	// offset the scanner reports.
	BaseOffset int64
}

// Defaults for Options.
const (
	DefaultMaxDepth       = 512
	DefaultMaxTokenBytes  = 16 << 20
	DefaultBufferSize     = 64 << 10
	DefaultMaxAttrs       = 1024
	DefaultMaxEntityDepth = 16
	// MaxEntityName is the longest entity name looked up. A longer one is
	// not read as a reference.
	MaxEntityName = 64
)

// Stats counts what the scanner did besides reading well-formed input.
type Stats struct {
	// Repairs counts the places where damaged markup was read leniently.
	Repairs int
	// Flattened counts elements nested deeper than Options.MaxDepth.
	Flattened int
	// AttrsCut counts attributes past Options.MaxAttrs.
	AttrsCut int
	// EntitiesCut counts references kept as written because expanding
	// them would pass Options.MaxEntityDepth or Options.ExpansionLimit.
	EntitiesCut int
	// EntitiesRecursive counts references to an entity inside its own
	// replacement, kept as written.
	EntitiesRecursive int
	// Expanded is the bytes of replacement text read in place of
	// references.
	Expanded int64
}

type attr struct {
	name, value []byte
}

const (
	bindNone       = -1 // no namespace
	bindXML        = -2 // the reserved xml prefix
	bindUndeclared = -3 // a prefix never declared
	bindEnded      = -4 // the End token of an element that declared its own namespace
)

type elem struct {
	nameStart int32 // offset of the qualified name in Scanner.names
	nameLen   int32
	colon     int32 // index of ':' in the name, or -1
	bind      int32 // the binding of the element's namespace, or a bind* value
	bindMark  int32 // len(binds) before this element's declarations
	bindBytes int32 // len(bindNames) before this element's declarations
}

type binding struct {
	start, end int32 // prefix in Scanner.bindNames; empty for the default namespace
	uend       int32 // the URI follows the prefix, to uend
	space      Space
	slot       int32 // index in Scanner.heads of this prefix
	prev       int32 // the binding this one shadows, or -1
}

// Above this many namespace declarations in scope, prefixes are looked up
// in a hash table.
const linearBindings = 8

type mode uint8

const (
	modeNormal mode = iota
	modeComment
	modeCDATA
)

// frame is the reading state saved while the replacement of an entity
// reference is read.
type frame struct {
	r        io.Reader
	buf      []byte
	pos, end int
	base     int64
	eof, own bool
	name     []byte
}

// Scanner reads XML tokens. Create one with New or NewBytes.
type Scanner struct {
	r    io.Reader
	buf  []byte // the window
	pos  int    // start of the unconsumed input in buf
	end  int    // end of valid data in buf
	base int64  // offset in the input of buf[0]
	eof  bool   // the reader is exhausted
	own  bool   // buf belongs to the scanner and may be written
	err  error
	mode mode

	spaces    map[string]Space
	maxDepth  int
	maxToken  int
	maxAttrs  int
	resolve   Resolver
	maxEntity int
	expLeft   int64 // expansion bytes left; negative once a reference was refused
	closing   int   // elements still to be closed by a recovered end tag
	over      int   // open elements beyond maxDepth

	frames           []frame
	refStart, refEnd int64 // the outermost reference being expanded
	constructStart   int64 // where the current comment or CDATA section began

	// The current token.
	kind       Kind
	qname      []byte
	local      []byte
	bind       int32
	text       []byte
	target     []byte
	final      bool
	tokStart   int64
	tokEnd     int64
	attrs      []attr
	pendingEnd bool // an empty-element tag still owes its End

	stack      []elem
	names      []byte // qualified names of the open elements
	binds      []binding
	bindNames  []byte
	slots      map[string]int32 // prefix -> index in heads
	heads      []int32          // per prefix: its innermost binding, or -1
	freeSlots  []int32          // slots of heads no prefix in scope uses
	scratch    []byte           // decoded text of the current token
	attrBuf    []byte           // decoded attribute values of the current token
	attrNested [][]byte         // entities being expanded in an attribute value
	endURI     []byte
	endSpace   Space

	// Stats counts repairs, cuts and expansion.
	Stats Stats
}

// New returns a Scanner reading from r.
func New(r io.Reader, opts *Options) *Scanner {
	size := DefaultBufferSize
	if opts != nil && opts.BufferSize > 0 {
		size = opts.BufferSize
	}
	s := &Scanner{r: r, own: true, buf: make([]byte, size)}
	s.configure(opts)
	return s
}

// NewBytes returns a Scanner reading from data, which it does not modify
// or copy.
func NewBytes(data []byte, opts *Options) *Scanner {
	s := &Scanner{buf: data, end: len(data), eof: true}
	s.configure(opts)
	return s
}

func (s *Scanner) configure(opts *Options) {
	s.maxDepth, s.maxToken, s.maxAttrs, s.maxEntity = DefaultMaxDepth, DefaultMaxTokenBytes, DefaultMaxAttrs, DefaultMaxEntityDepth
	if opts == nil {
		return
	}
	s.spaces, s.resolve, s.base = opts.Spaces, opts.Resolve, opts.BaseOffset
	if opts.MaxDepth > 0 {
		s.maxDepth = opts.MaxDepth
	}
	if opts.MaxTokenBytes > 0 {
		s.maxToken = opts.MaxTokenBytes
	}
	if opts.MaxAttrs > 0 {
		s.maxAttrs = opts.MaxAttrs
	}
	if opts.MaxEntityDepth > 0 {
		s.maxEntity = opts.MaxEntityDepth
	}
	s.expLeft = opts.ExpansionLimit
	if s.expLeft < 0 {
		s.expLeft = math.MaxInt64
	}
}

// SetResolver replaces Options.Resolve, as a reader does once it has read
// the document type declaration.
func (s *Scanner) SetResolver(r Resolver) { s.resolve = r }

// Err returns the error that ended the scan, or nil at a clean end of
// input.
func (s *Scanner) Err() error { return s.err }

// Offset returns the byte offset in the input of the next unread byte. While
// the replacement of an entity reference is read, it is the offset of the
// reference.
func (s *Scanner) Offset() int64 { return s.here() }

func (s *Scanner) here() int64 {
	if len(s.frames) > 0 {
		return s.refStart
	}
	return s.base + int64(s.pos)
}

func (s *Scanner) hereEnd() int64 {
	if len(s.frames) > 0 {
		return s.refEnd
	}
	return s.base + int64(s.pos)
}

// Span returns the byte offsets in the input where the current token
// starts and ends. A token read from the replacement of an entity
// reference has the offsets of the outermost reference.
func (s *Scanner) Span() (start, end int64) { return s.tokStart, s.tokEnd }

// InEntity reports whether the current token comes from the replacement
// of an entity reference.
func (s *Scanner) InEntity() bool { return len(s.frames) > 0 }

// Depth returns the number of open elements. After a Start token it
// includes that element; after an End token it does not.
func (s *Scanner) Depth() int { return len(s.stack) }

// Kind returns the kind of the current token, as last returned by Next.
func (s *Scanner) Kind() Kind { return s.kind }

// Local returns the local name of the current Start or End token.
func (s *Scanner) Local() []byte { return s.local }

// Name returns the qualified name of the current Start or End token as it
// is written in the source, prefix included.
func (s *Scanner) Name() []byte { return s.qname }

// Space returns the namespace of the current Start or End token.
func (s *Scanner) Space() Space { return s.spaceOf(s.bind) }

// URI returns the namespace URI of the current Start or End token, empty
// for none or an undeclared prefix.
func (s *Scanner) URI() []byte { return s.uriOf(s.bind) }

func (s *Scanner) spaceOf(b int32) Space {
	switch b {
	case bindNone:
		return NoSpace
	case bindXML:
		return SpaceXML
	case bindUndeclared:
		return SpaceUnknown
	case bindEnded:
		return s.endSpace
	}
	return s.binds[b].space
}

var xmlNamespace = []byte(XMLNamespace)

func (s *Scanner) uriOf(b int32) []byte {
	switch b {
	case bindNone, bindUndeclared:
		return nil
	case bindXML:
		return xmlNamespace
	case bindEnded:
		return s.endURI
	}
	bd := &s.binds[b]
	return s.bindNames[bd.end:bd.uend]
}

// Text returns the content of the current Text, Comment, PI or Doctype
// token. Text has references decoded; all have line ends normalized to
// "\n", except Doctype, which is as written.
func (s *Scanner) Text() []byte { return s.text }

// Target returns the target of the current PI token.
func (s *Scanner) Target() []byte { return s.target }

// Final reports whether the current Comment token is the last piece of
// its comment.
func (s *Scanner) Final() bool { return s.final }

func (s *Scanner) fail(err error) {
	if s.err == nil {
		s.err = err
	}
	s.kind = EOF
}

// Abort ends the scan: from now on Next returns EOF and Err returns err,
// unless the scan had failed already.
func (s *Scanner) Abort(err error) { s.fail(err) }

// more reads more input into the window, keeping the unconsumed part,
// which moves to the front. It reports whether any bytes were added.
func (s *Scanner) more() bool {
	if s.eof || s.err != nil {
		return false
	}
	if s.pos > 0 {
		n := copy(s.buf, s.buf[s.pos:s.end])
		s.base += int64(s.pos)
		s.pos, s.end = 0, n
	}
	if s.end == len(s.buf) {
		if len(s.buf) >= s.maxToken {
			err := ErrTokenTooLarge
			if s.mode == modeNormal && s.end > 0 && s.buf[0] != '<' {
				// Text that cannot be cut: a character reference
				// with more digits than the window holds.
				err = ErrReferenceTooLarge
			}
			s.fail(fmt.Errorf("%w (%d bytes) at byte %d", err, s.maxToken, s.here()))
			return false
		}
		grown := make([]byte, min(2*len(s.buf), s.maxToken))
		copy(grown, s.buf[:s.end])
		s.buf = grown
	}
	for range 64 {
		n, err := s.r.Read(s.buf[s.end:])
		s.end += n
		if err != nil {
			s.eof = true
			if err != io.EOF {
				s.fail(err)
				return false
			}
			return n > 0
		}
		if n > 0 {
			return true
		}
	}
	s.fail(io.ErrNoProgress)
	return false
}

// Next advances to the next token and returns its kind.
func (s *Scanner) Next() Kind {
	if s.pendingEnd {
		s.pendingEnd = false
		s.pop()
		s.kind = End
		s.tokStart = s.tokEnd
		return End
	}
	if s.err != nil {
		return EOF
	}
	if s.closing > 0 {
		s.closing--
		s.pop()
		s.kind = End
		s.tokStart = s.tokEnd
		return End
	}
	s.attrs, s.attrBuf = s.attrs[:0], s.attrBuf[:0]
	for {
		if s.pos >= s.end && !s.more() {
			if s.err != nil {
				return EOF
			}
			if s.mode != modeNormal {
				if s.unterminatedMode() {
					return s.kind
				}
				continue
			}
			if len(s.frames) > 0 {
				s.popFrame()
				continue
			}
			if len(s.stack) > 0 {
				// Close what the input left open.
				s.Stats.Repairs++
				s.closing = len(s.stack) - 1
				s.over = 0
				s.pop()
				s.kind = End
				s.tokStart, s.tokEnd = s.here(), s.here()
				return End
			}
			s.kind = EOF
			return EOF
		}
		var status scanStatus
		switch {
		case s.mode == modeComment:
			status = s.comment()
		case s.mode == modeCDATA:
			status = s.cdata()
		case s.buf[s.pos] != '<':
			status = s.charData()
		default:
			status = s.markup()
		}
		switch status {
		case emitted:
			return s.kind
		case needMore:
			atEnd := s.eof
			if !s.more() {
				if s.err != nil {
					return EOF
				}
				if atEnd {
					// Markup that runs to the end of the input (or of a
					// replacement) is not markup: it is kept as text.
					s.Stats.Repairs++
					s.text = s.decodeLiteral(s.buf[s.pos:s.end])
					s.tokStart = s.here()
					s.pos = s.end
					s.tokEnd = s.hereEnd()
					s.kind = Text
					return Text
				}
			}
		}
		// skipped: go round again
	}
}

type scanStatus uint8

const (
	emitted  scanStatus = iota // a token is ready
	skipped                    // input was consumed without producing a token
	needMore                   // the window ends inside the construct
)

// unterminatedMode ends a comment or CDATA section that runs to the end
// of the input, and reports whether that made a token.
func (s *Scanner) unterminatedMode() bool {
	s.Stats.Repairs++
	m := s.mode
	s.mode = modeNormal
	if m == modeComment {
		s.text, s.final, s.kind = nil, true, Comment
		s.tokStart, s.tokEnd = s.constructStart, s.hereEnd()
		return true
	}
	return false
}

func (s *Scanner) popFrame() {
	f := s.frames[len(s.frames)-1]
	s.frames = s.frames[:len(s.frames)-1]
	s.r, s.buf, s.pos, s.end, s.base, s.eof, s.own = f.r, f.buf, f.pos, f.end, f.base, f.eof, f.own
}

// charData scans text up to the next '<'.
func (s *Scanner) charData() scanStatus {
	b := s.buf[s.pos:s.end]
	n := bytes.IndexByte(b, '<')
	if n < 0 {
		switch {
		case s.eof:
			n = len(b)
		case s.pos > 0 || s.end < len(s.buf):
			return needMore // there is room to look further before splitting
		default:
			// The window is full of text. Hand over what can be cut
			// cleanly and carry on; text need not fit in the window.
			if n = safeCut(b); n == 0 {
				return needMore
			}
		}
	}
	raw := b[:n]
	start := s.here()
	if raw[0] == '&' && s.resolve != nil {
		if st, done := s.entityRef(raw, start); done {
			return st
		}
	}
	text, used := s.decodeText(raw)
	s.pos += used
	s.tokStart, s.tokEnd = start, s.hereEnd()
	if len(text) == 0 {
		return skipped
	}
	s.text = text
	s.kind = Text
	return emitted
}

// refName returns the name of the named reference at the start of raw
// (raw[0] is '&') and the length of the reference, or false.
func refName(raw []byte) ([]byte, int, bool) {
	if len(raw) < 3 || raw[1] == '#' {
		return nil, 0, false
	}
	for i := 1; i < len(raw) && i <= MaxEntityName+1; i++ {
		switch raw[i] {
		case ';':
			if i < 2 {
				return nil, 0, false
			}
			return raw[1:i], i + 1, true
		case '&', '<', '>', '"', '\'', ' ', '\t', '\n', '\r', '#':
			return nil, 0, false
		}
	}
	return nil, 0, false
}

func predefined(name []byte) byte {
	switch string(name) {
	case "lt":
		return '<'
	case "gt":
		return '>'
	case "amp":
		return '&'
	case "quot":
		return '"'
	case "apos":
		return '\''
	}
	return 0
}

// entityRef handles a general entity reference at the start of raw other
// than a predefined one. A replacement to insert, or the reference kept as
// written, is a Text token. A replacement read as markup is started, or,
// when a bound or recursion refuses it, the reference as written is a Text
// token. It reports false when raw does not start with such a reference.
func (s *Scanner) entityRef(raw []byte, start int64) (scanStatus, bool) {
	name, n, ok := refName(raw)
	if !ok || predefined(name) != 0 {
		return 0, false
	}
	repl, how := s.resolve(name)
	if how != Expand {
		s.text = raw[:n]
		if how == Insert {
			s.text = repl
		}
		s.pos += n
		s.tokStart, s.tokEnd = start, s.hereEnd()
		if len(s.text) == 0 {
			return skipped, true
		}
		s.kind = Text
		return emitted, true
	}
	refused := false
	for i := range s.frames {
		if bytes.Equal(s.frames[i].name, name) {
			s.Stats.EntitiesRecursive++
			refused = true
			break
		}
	}
	if !refused && (len(s.frames) >= s.maxEntity || s.expLeft < int64(len(repl))) {
		s.Stats.EntitiesCut++
		s.expLeft = -1 // a bound reached cuts there: no expansion after it
		refused = true
	}
	if refused {
		s.text = raw[:n]
		s.pos += n
		s.tokStart, s.tokEnd = start, s.hereEnd()
		s.kind = Text
		return emitted, true
	}
	s.expLeft -= int64(len(repl))
	s.Stats.Expanded += int64(len(repl))
	if len(s.frames) == 0 {
		s.refStart, s.refEnd = start, start+int64(n)
	}
	s.pos += n
	s.frames = append(s.frames, frame{r: s.r, buf: s.buf, pos: s.pos, end: s.end, base: s.base, eof: s.eof, own: s.own, name: name})
	s.r, s.buf, s.pos, s.end, s.base, s.eof, s.own = nil, repl, 0, len(repl), 0, true, false
	return skipped, true
}

// safeCut returns how much of b can be delivered as a text chunk without
// splitting a UTF-8 sequence, an entity reference or a CR LF pair.
func safeCut(b []byte) int {
	n := len(b)
	if n > 0 && b[n-1] == '\r' {
		n--
	}
	// An incomplete multi-byte sequence at the end.
	for back := 1; back <= 3 && back <= n; back++ {
		c := b[n-back]
		if c < 0x80 {
			break
		}
		if c >= 0xC0 { // a lead byte: is its sequence complete?
			need := 2
			if c >= 0xF0 {
				need = 4
			} else if c >= 0xE0 {
				need = 3
			}
			if back < need {
				n -= back
			}
			break
		}
	}
	// An entity reference that has started but not ended.
	from := max(0, n-MaxEntityName-2)
	if i := bytes.LastIndexByte(b[from:n], '&'); i >= 0 {
		if bytes.IndexByte(b[from+i:n], ';') < 0 {
			n = from + i
		}
		return n
	}
	// And further back, a character written by its number with many
	// zeros in front: "&#", then digits to the end.
	if i := bytes.LastIndexByte(b[:n], '&'); i >= 0 && i+1 < n && b[i+1] == '#' {
		for _, c := range b[i+2 : n] {
			if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F' || c == 'x' || c == 'X') {
				return n
			}
		}
		n = i
	}
	return n
}

// cutPoint returns how much of a full window of comment or CDATA b can
// be delivered, keeping back a possible partial terminator, CR LF pair or
// UTF-8 sequence.
func cutPoint(b []byte) int {
	n := len(b) - 2
	if n > 0 && b[n-1] == '\r' {
		n--
	}
	for n > 0 && !utf8.RuneStart(b[n]) {
		n--
	}
	return n
}

// comment delivers the content of a comment, in chunks if it is larger
// than the window.
func (s *Scanner) comment() scanStatus {
	b := s.buf[s.pos:s.end]
	if i := bytes.Index(b, []byte("-->")); i >= 0 {
		s.text = s.normalizeLines(b[:i])
		s.pos += i + 3
		s.mode = modeNormal
		s.final, s.kind = true, Comment
		s.tokStart, s.tokEnd = s.constructStart, s.hereEnd()
		return emitted
	}
	if s.eof {
		s.Stats.Repairs++ // not terminated: what there is is the comment
		s.text = s.normalizeLines(b)
		s.pos = s.end
		s.mode = modeNormal
		s.final, s.kind = true, Comment
		s.tokStart, s.tokEnd = s.constructStart, s.hereEnd()
		return emitted
	}
	if s.pos > 0 || s.end < len(s.buf) {
		return needMore
	}
	n := cutPoint(b)
	if n <= 0 {
		return needMore
	}
	s.text = s.normalizeLines(b[:n])
	s.pos += n
	s.final, s.kind = false, Comment
	s.tokStart, s.tokEnd = s.constructStart, s.hereEnd()
	return emitted
}

// cdata delivers the content of a CDATA section as text, in chunks if it
// is larger than the window.
func (s *Scanner) cdata() scanStatus {
	b := s.buf[s.pos:s.end]
	start := s.here()
	if i := bytes.Index(b, []byte("]]>")); i >= 0 {
		s.text = s.normalizeLines(b[:i])
		s.pos += i + 3
		s.mode = modeNormal
		if i == 0 {
			return skipped
		}
		s.tokStart, s.tokEnd = start, s.hereEnd()
		s.kind = Text
		return emitted
	}
	if s.eof {
		s.Stats.Repairs++
		s.text = s.normalizeLines(b)
		s.pos = s.end
		s.mode = modeNormal
		s.tokStart, s.tokEnd = start, s.hereEnd()
		s.kind = Text
		return emitted
	}
	if s.pos > 0 || s.end < len(s.buf) {
		return needMore
	}
	n := cutPoint(b)
	if n <= 0 {
		return needMore
	}
	s.text = s.normalizeLines(b[:n])
	s.pos += n
	s.tokStart, s.tokEnd = start, s.hereEnd()
	s.kind = Text
	return emitted
}

var (
	cdataOpen   = []byte("<![CDATA[")
	commentOpen = []byte("<!--")
	doctypeOpen = []byte("<!DOCTYPE")
)

func isNameStart(c byte) bool {
	return c >= 0x80 || c|0x20 >= 'a' && c|0x20 <= 'z' || c == '_' || c == ':'
}

// stray makes a '<' that starts no markup a Text token of its own.
func (s *Scanner) stray() scanStatus {
	s.Stats.Repairs++
	s.tokStart = s.here()
	s.text = s.buf[s.pos : s.pos+1]
	s.pos++
	s.tokEnd = s.hereEnd()
	s.kind = Text
	return emitted
}

// markup scans a construct that starts with '<'.
func (s *Scanner) markup() scanStatus {
	b := s.buf[s.pos:s.end]
	if len(b) < 2 {
		if s.eof {
			return s.stray()
		}
		return needMore
	}
	switch c := b[1]; {
	case c == '/':
		return s.endTag(b)
	case c == '?':
		return s.pi(b)
	case c == '!':
		switch {
		case bytes.HasPrefix(b, commentOpen):
			s.constructStart = s.here()
			s.pos += len(commentOpen)
			s.mode = modeComment
			return skipped
		case bytes.HasPrefix(b, cdataOpen):
			s.pos += len(cdataOpen)
			s.mode = modeCDATA
			return skipped
		case len(b) >= len(doctypeOpen) && bytes.EqualFold(b[:len(doctypeOpen)], doctypeOpen):
			n := doctypeEnd(b)
			if n < 0 {
				return needMore
			}
			s.tokStart = s.here()
			s.text = b[len(doctypeOpen) : n-1]
			s.pos += n
			s.tokEnd = s.hereEnd()
			s.kind = Doctype
			return emitted
		case len(b) < len(doctypeOpen) && !s.eof:
			return needMore
		}
		return s.stray()
	case isNameStart(c):
		return s.startTag(b)
	}
	return s.stray()
}

// doctypeEnd returns the length of the document type declaration at the
// start of b, or -1 if it does not end in b. Quoted strings, comments and
// processing instructions in its internal subset may hold '>'.
func doctypeEnd(b []byte) int {
	depth := 0
	for i := len(doctypeOpen); i < len(b); i++ {
		switch c := b[i]; c {
		case '"', '\'':
			j := bytes.IndexByte(b[i+1:], c)
			if j < 0 {
				return -1
			}
			i += j + 1
		case '<':
			if bytes.HasPrefix(b[i:], commentOpen) {
				j := bytes.Index(b[i+4:], []byte("-->"))
				if j < 0 {
					return -1
				}
				i += 4 + j + 2
			} else if i+1 < len(b) && b[i+1] == '?' {
				j := bytes.Index(b[i+2:], []byte("?>"))
				if j < 0 {
					return -1
				}
				i += 2 + j + 1
			}
		case '[':
			depth++
		case ']':
			if depth > 0 {
				depth--
			}
		case '>':
			if depth == 0 {
				return i + 1
			}
		}
	}
	return -1
}

// pi scans a processing instruction.
func (s *Scanner) pi(b []byte) scanStatus {
	i := bytes.Index(b[2:], []byte("?>"))
	if i < 0 {
		return needMore
	}
	i += 2
	body := b[2:i]
	t := 0
	for t < len(body) && !isSpace(body[t]) {
		t++
	}
	s.target = body[:t]
	data := body[t:]
	for len(data) > 0 && isSpace(data[0]) {
		data = data[1:]
	}
	s.tokStart = s.here()
	s.text = s.normalizeLines(data)
	s.pos += i + 2
	s.tokEnd = s.hereEnd()
	s.kind = PI
	return emitted
}

// endTag scans an end tag.
func (s *Scanner) endTag(b []byte) scanStatus {
	start := s.here()
	// The usual case: the end tag of the innermost element, written with
	// no space. Compare it in place.
	if n := len(s.stack); n > 0 && s.over == 0 {
		top := &s.stack[n-1]
		end := 2 + int(top.nameLen)
		if end < len(b) && b[end] == '>' && string(b[2:end]) == string(s.names[top.nameStart:top.nameStart+top.nameLen]) {
			s.pos += end + 1
			s.pop()
			s.kind = End
			s.tokStart, s.tokEnd = start, s.hereEnd()
			return emitted
		}
	}
	if len(b) < 3 {
		return needMore
	}
	if !isNameStart(b[2]) {
		return s.stray()
	}
	i := bytes.IndexByte(b, '>')
	if i < 0 {
		return needMore
	}
	name := b[2:i]
	for j, c := range name {
		if isSpace(c) {
			name = name[:j]
			break
		}
	}
	if s.over > 0 {
		// Inside elements read as part of their ancestor.
		s.over--
		s.pos += i + 1
		return skipped
	}
	matched := len(s.stack) - 1
	for ; matched >= 0 && !s.named(matched, name); matched-- {
	}
	if matched < 0 {
		// An end tag that closes nothing is dropped.
		s.Stats.Repairs++
		s.pos += i + 1
		return skipped
	}
	// Close the elements left open inside the matched one first; closing
	// is then non-zero and Next hands out their Ends.
	s.closing = len(s.stack) - 1 - matched
	if s.closing > 0 {
		s.Stats.Repairs++
	}
	s.pos += i + 1
	s.pop()
	s.kind = End
	s.tokStart, s.tokEnd = start, s.hereEnd()
	return emitted
}

// named reports whether the open element at stack index i has the given
// qualified name.
func (s *Scanner) named(i int, name []byte) bool {
	e := &s.stack[i]
	return string(s.names[e.nameStart:e.nameStart+e.nameLen]) == string(name)
}

func isSpace(c byte) bool { return c == ' ' || c == '\n' || c == '\t' || c == '\r' }

// startTag parses a start tag or empty-element tag beginning at b[0]. The
// whole tag must be in the window.
func (s *Scanner) startTag(b []byte) scanStatus {
	i := 1
	for i < len(b) && !isSpace(b[i]) && b[i] != '>' && b[i] != '/' && b[i] != '<' {
		i++
	}
	if i == len(b) {
		return needMore
	}
	name := b[1:i]
	s.attrs = s.attrs[:0]
	selfClose, declares := false, false
	repairs, cut := 0, 0
	for {
		for i < len(b) && isSpace(b[i]) {
			i++
		}
		if i == len(b) {
			return needMore
		}
		c := b[i]
		if c == '>' {
			i++
			break
		}
		if c == '<' {
			// A tag that never closed: it ends where the next begins.
			repairs++
			break
		}
		if c == '/' {
			if i+1 == len(b) {
				return needMore
			}
			if b[i+1] == '>' {
				selfClose = true
				i += 2
				break
			}
			repairs++ // a stray '/'
			i++
			continue
		}
		// An attribute: name, '=', quoted value.
		start := i
		for i < len(b) && b[i] != '=' && !isSpace(b[i]) && b[i] != '>' && b[i] != '/' && b[i] != '<' {
			i++
		}
		if i == len(b) {
			return needMore
		}
		attrName := b[start:i]
		for i < len(b) && isSpace(b[i]) {
			i++
		}
		if i == len(b) {
			return needMore
		}
		if b[i] != '=' {
			repairs++ // an attribute without a value
			continue
		}
		i++
		for i < len(b) && isSpace(b[i]) {
			i++
		}
		if i == len(b) {
			return needMore
		}
		var value []byte
		if quote := b[i]; quote == '"' || quote == '\'' {
			i++
			// Most values are a few bytes long: look by hand first, and
			// only call out for the long ones.
			n, limit := 0, min(len(b)-i, 24)
			for n < limit && b[i+n] != quote {
				n++
			}
			if n == limit {
				m := bytes.IndexByte(b[i+n:], quote)
				if m < 0 {
					return needMore
				}
				n += m
			}
			value = b[i : i+n]
			i += n + 1
		} else {
			repairs++ // a value without quotes runs to white space or '>'
			v := i
			for i < len(b) && !isSpace(b[i]) && b[i] != '>' {
				i++
			}
			if i == len(b) {
				return needMore
			}
			value = b[v:i]
		}
		if len(attrName) == 0 {
			repairs++
			continue
		}
		if len(s.attrs) >= s.maxAttrs {
			cut++
			continue
		}
		if attrName[0] == 'x' && len(attrName) >= 5 {
			declares = true
		}
		s.attrs = append(s.attrs, attr{attrName, value})
	}
	s.Stats.Repairs += repairs
	start := s.here()
	s.pos += i
	if len(s.stack) >= s.maxDepth || s.over > 0 {
		// Too deep: read as part of the ancestor.
		s.Stats.Flattened++
		if !selfClose {
			s.over++
		}
		s.attrs = s.attrs[:0]
		return skipped
	}
	s.Stats.AttrsCut += cut
	s.push(name, bytes.IndexByte(name, ':'), declares)
	s.pendingEnd = selfClose
	s.kind = Start
	s.tokStart, s.tokEnd = start, s.hereEnd()
	return emitted
}

var xmlnsPrefix = []byte("xmlns")

// push opens an element: it records the namespace declarations among the
// attributes just parsed, resolves the element's own namespace and puts
// the element on the stack. colon is the index of ':' in name, or -1.
func (s *Scanner) push(name []byte, colon int, declares bool) {
	e := elem{
		nameStart: int32(len(s.names)),
		nameLen:   int32(len(name)),
		colon:     int32(colon),
		bindMark:  int32(len(s.binds)),
		bindBytes: int32(len(s.bindNames)),
	}
	for i := 0; declares && i < len(s.attrs); i++ {
		a := &s.attrs[i]
		if len(a.name) < 5 || a.name[0] != 'x' || !bytes.HasPrefix(a.name, xmlnsPrefix) {
			continue
		}
		var prefix []byte
		switch {
		case len(a.name) == 5:
		case a.name[5] == ':':
			prefix = a.name[6:]
		default:
			continue // an ordinary attribute whose name starts with "xmlns"
		}
		uri := s.attrValue(a)
		space := SpaceUnknown
		if len(uri) == 0 {
			space = NoSpace // xmlns="" undeclares the default namespace
		} else if v, ok := s.spaces[string(uri)]; ok {
			space = v
		}
		slot, known := s.slots[string(prefix)]
		if !known {
			if s.slots == nil {
				s.slots = map[string]int32{}
			}
			if n := len(s.freeSlots); n > 0 {
				slot = s.freeSlots[n-1]
				s.freeSlots = s.freeSlots[:n-1]
			} else {
				slot = int32(len(s.heads))
				s.heads = append(s.heads, -1)
			}
			s.slots[string(prefix)] = slot
		}
		start := int32(len(s.bindNames))
		s.bindNames = append(s.bindNames, prefix...)
		end := int32(len(s.bindNames))
		s.bindNames = append(s.bindNames, uri...)
		s.binds = append(s.binds, binding{start, end, int32(len(s.bindNames)), space, slot, s.heads[slot]})
		s.heads[slot] = int32(len(s.binds) - 1)
	}

	var prefix []byte
	if e.colon >= 0 {
		prefix = name[:e.colon]
	}
	// A child usually has its parent's prefix. If this tag declared
	// nothing, the parent's answer is the answer.
	if n := len(s.stack); n > 0 && int(e.bindMark) == len(s.binds) {
		p := &s.stack[n-1]
		var parentPrefix []byte
		if p.colon >= 0 {
			parentPrefix = s.names[p.nameStart : p.nameStart+p.colon]
		}
		if string(parentPrefix) == string(prefix) {
			e.bind = p.bind
			goto resolved
		}
	}
	e.bind = s.resolveBind(prefix, true)
resolved:
	s.names = append(s.names, name...)
	s.stack = append(s.stack, e)
	s.qname = s.names[e.nameStart : e.nameStart+e.nameLen]
	s.local = s.qname[e.colon+1:]
	s.bind = e.bind
}

// resolveBind returns the binding of prefix. An element without a prefix
// takes the default namespace; an attribute without one has none.
func (s *Scanner) resolveBind(prefix []byte, element bool) int32 {
	if len(prefix) == 0 && !element {
		return bindNone
	}
	if len(s.binds) <= linearBindings {
		for i := len(s.binds) - 1; i >= 0; i-- {
			b := &s.binds[i]
			if string(s.bindNames[b.start:b.end]) == string(prefix) {
				return s.emptyIsNone(int32(i))
			}
		}
	} else if slot, ok := s.slots[string(prefix)]; ok && s.heads[slot] >= 0 {
		return s.emptyIsNone(s.heads[slot])
	}
	switch {
	case len(prefix) == 0:
		return bindNone
	case string(prefix) == "xml":
		return bindXML
	}
	return bindUndeclared
}

func (s *Scanner) emptyIsNone(i int32) int32 {
	if b := &s.binds[i]; b.uend == b.end {
		return bindNone
	}
	return i
}

// pop closes the innermost element and makes it the current token.
func (s *Scanner) pop() {
	e := s.stack[len(s.stack)-1]
	s.stack = s.stack[:len(s.stack)-1]
	s.qname = s.names[e.nameStart : e.nameStart+e.nameLen]
	s.local = s.qname[e.colon+1:]
	s.bind = e.bind
	// The name stays readable until the next push: names are only
	// truncated here, and qname points below the new length.
	s.names = s.names[:e.nameStart]
	if e.bind >= e.bindMark {
		// The element declared its own namespace, which goes out of
		// scope here: keep a copy for the End token.
		s.endURI = append(s.endURI[:0], s.uriOf(e.bind)...)
		s.endSpace = s.binds[e.bind].space
		s.bind = bindEnded
	}
	for i := len(s.binds) - 1; i >= int(e.bindMark); i-- {
		b := &s.binds[i]
		s.heads[b.slot] = b.prev // what this element shadowed is visible again
		if b.prev < 0 {
			// The prefix is out of scope: its slot is free again, so the
			// table holds only the prefixes of the open elements.
			delete(s.slots, string(s.bindNames[b.start:b.end]))
			s.freeSlots = append(s.freeSlots, b.slot)
		}
	}
	s.binds = s.binds[:e.bindMark]
	s.bindNames = s.bindNames[:e.bindBytes]
}

// NumAttr returns the number of attributes kept on the current Start
// token, namespace declarations included.
func (s *Scanner) NumAttr() int { return len(s.attrs) }

// AttrNameAt returns the qualified name of the i-th attribute of the
// current Start token as written in the source, prefix included.
func (s *Scanner) AttrNameAt(i int) []byte { return s.attrs[i].name }

// AttrAt returns the i-th attribute of the current Start token: its
// namespace URI (empty for none), local name and decoded value. A
// namespace declaration has the xmlns namespace.
func (s *Scanner) AttrAt(i int) (uri, local, value []byte) {
	a := &s.attrs[i]
	colon := bytes.IndexByte(a.name, ':')
	switch {
	case colon < 0 && string(a.name) == "xmlns":
		uri = xmlnsNamespace
	case colon >= 0 && string(a.name[:colon]) == "xmlns":
		uri = xmlnsNamespace
	case colon >= 0:
		uri = s.uriOf(s.resolveBind(a.name[:colon], false))
	}
	return uri, a.name[colon+1:], s.attrValue(a)
}

var xmlnsNamespace = []byte(XMLNSNamespace)

// Attr returns the value of the attribute with the given namespace URI
// (empty for none) and local name on the current Start token.
func (s *Scanner) Attr(uri, local string) ([]byte, bool) {
	for i := range s.attrs {
		a := &s.attrs[i]
		colon := bytes.IndexByte(a.name, ':')
		if string(a.name[colon+1:]) != local {
			continue
		}
		u, _, v := s.AttrAt(i)
		if string(u) == uri {
			return v, true
		}
	}
	return nil, false
}

func (s *Scanner) attrValue(a *attr) []byte {
	if !needsAttrDecode(a.value) {
		return a.value
	}
	start := len(s.attrBuf)
	s.attrBuf = s.decodeAttr(s.attrBuf, a.value)
	// A value that needed decoding may leave attrBuf's backing array when
	// it grows; earlier values keep pointing at the old array, which stays
	// valid, so they are unaffected.
	return s.attrBuf[start:]
}

func needsAttrDecode(v []byte) bool {
	for _, c := range v {
		if c == '&' || c == '\t' || c == '\n' || c == '\r' {
			return true
		}
	}
	return false
}

// decodeAttr appends the decoded form of raw to buf. White space
// characters written literally become spaces, as XML requires for
// attribute values. Entities to expand are expanded here, as text.
func (s *Scanner) decodeAttr(buf, raw []byte) []byte {
	for i := 0; i < len(raw); {
		switch c := raw[i]; c {
		case '&':
			if name, n, ok := refName(raw[i:]); ok && predefined(name) == 0 && s.resolve != nil {
				switch repl, how := s.resolve(name); how {
				case Expand:
					buf = s.expandAttr(buf, raw[i:i+n], name, repl)
				case Insert:
					buf = append(buf, repl...)
				default:
					buf = append(buf, raw[i:i+n]...)
				}
				i += n
				continue
			}
			var n int
			buf, n, _ = s.reference(buf, raw[i:])
			i += n
		case '\r':
			// Line ends are normalized first, so CR LF is one line end
			// and becomes one space.
			buf = append(buf, ' ')
			i++
			if i < len(raw) && raw[i] == '\n' {
				i++
			}
		case '\t', '\n':
			buf = append(buf, ' ')
			i++
		default:
			buf = append(buf, c)
			i++
		}
	}
	return buf
}

// expandAttr appends the expansion of an entity reference in an
// attribute value, or the reference as written where a bound or
// recursion refuses it.
func (s *Scanner) expandAttr(buf, ref, name, repl []byte) []byte {
	for _, open := range s.attrNested {
		if bytes.Equal(open, name) {
			s.Stats.EntitiesRecursive++
			return append(buf, ref...)
		}
	}
	if len(s.attrNested)+len(s.frames) >= s.maxEntity || s.expLeft < int64(len(repl)) {
		s.Stats.EntitiesCut++
		s.expLeft = -1
		return append(buf, ref...)
	}
	s.expLeft -= int64(len(repl))
	s.Stats.Expanded += int64(len(repl))
	s.attrNested = append(s.attrNested, name)
	buf = s.decodeAttr(buf, repl)
	s.attrNested = s.attrNested[:len(s.attrNested)-1]
	return buf
}

// decodeText returns raw with references decoded and line ends
// normalized, and how much of raw it took: it stops before a reference
// whose replacement is read as markup.
func (s *Scanner) decodeText(raw []byte) ([]byte, int) {
	if bytes.IndexByte(raw, '&') < 0 {
		return s.normalizeLines(raw), len(raw)
	}
	buf := s.scratch[:0]
	i := 0
	for i < len(raw) {
		switch c := raw[i]; c {
		case '&':
			var n int
			var stop bool
			buf, n, stop = s.reference(buf, raw[i:])
			if stop {
				s.scratch = buf
				return buf, i
			}
			i += n
		case '\r':
			buf = append(buf, '\n')
			i++
			if i < len(raw) && raw[i] == '\n' {
				i++
			}
		default:
			// Copy the run up to the next special byte in one go.
			j := i + 1
			for j < len(raw) && raw[j] != '&' && raw[j] != '\r' {
				j++
			}
			buf = append(buf, raw[i:j]...)
			i = j
		}
	}
	s.scratch = buf
	return buf, i
}

// decodeLiteral returns raw with line ends normalized and nothing else
// decoded, for markup kept as text.
func (s *Scanner) decodeLiteral(raw []byte) []byte { return s.normalizeLines(raw) }

// normalizeLines turns CR LF and lone CR into LF, as XML requires.
func (s *Scanner) normalizeLines(raw []byte) []byte {
	if bytes.IndexByte(raw, '\r') < 0 {
		return raw
	}
	buf := s.scratch[:0]
	for i := 0; i < len(raw); i++ {
		c := raw[i]
		if c == '\r' {
			c = '\n'
			if i+1 < len(raw) && raw[i+1] == '\n' {
				i++
			}
		}
		buf = append(buf, c)
	}
	s.scratch = buf
	return buf
}

// reference decodes the entity or character reference at the start of raw
// (raw[0] is '&'), appends the result to buf and returns how many bytes of
// raw it consumed. It reports stop, consuming nothing, for a reference
// whose replacement is read as markup. A reference that cannot be decoded
// is kept as written.
func (s *Scanner) reference(buf, raw []byte) ([]byte, int, bool) {
	if len(raw) >= 2 && raw[1] == '#' {
		// A character by its number, which may be written with any
		// number of zeros in front: the digits run to the semicolon.
		i, base := 2, rune(10)
		if i < len(raw) && (raw[i] == 'x' || raw[i] == 'X') {
			base, i = 16, i+1
		}
		first := i
		var r rune
	digits:
		for ; i < len(raw); i++ {
			var d rune
			switch c := raw[i]; {
			case c >= '0' && c <= '9':
				d = rune(c - '0')
			case base == 16 && c >= 'a' && c <= 'f':
				d = rune(c-'a') + 10
			case base == 16 && c >= 'A' && c <= 'F':
				d = rune(c-'A') + 10
			default:
				break digits
			}
			if r = r*base + d; r > utf8.MaxRune {
				r = utf8.MaxRune + 1
			}
		}
		if i == first || i == len(raw) || raw[i] != ';' {
			s.Stats.Repairs++
			return append(buf, '&'), 1, false
		}
		if r > utf8.MaxRune || (r >= 0xD800 && r <= 0xDFFF) {
			s.Stats.Repairs++
			r = utf8.RuneError
		}
		// A reference to a control character gives the character; the
		// text conventions are applied where text is made.
		return utf8.AppendRune(buf, r), i + 1, false
	}
	name, n, ok := refName(raw)
	if !ok {
		s.Stats.Repairs++
		return append(buf, '&'), 1, false
	}
	if c := predefined(name); c != 0 {
		return append(buf, c), n, false
	}
	if s.resolve == nil {
		s.Stats.Repairs++
		return append(buf, raw[:n]...), n, false
	}
	// The reference is read by entityRef, as a token of its own.
	return buf, 0, true
}
