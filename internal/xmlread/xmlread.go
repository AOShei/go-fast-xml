// Package xmlread turns the scanner's tokens into element events for a
// consumer that builds blocks, and keeps what every consumer needs: the
// stack of open elements with their names, namespaces, attributes,
// xml:space and offsets; the document type declaration and its entities;
// and the counts of what the file holds that is reported rather than read
// (external references, processing instructions, repairs, cuts).
//
// The generic reader is one consumer (package generic); a profile-driven
// one implements the same Handler and hands the elements it has no rule
// for to the generic builder.
package xmlread

import (
	"bytes"
	"context"
	"io"

	"github.com/AOShei/go-fast-xml/internal/dtd"
	"github.com/AOShei/go-fast-xml/internal/entity"
	"github.com/AOShei/go-fast-xml/internal/xmlscan"
)

// Namespaces the reader recognises.
const (
	NamespaceXSI      = "http://www.w3.org/2001/XMLSchema-instance"
	NamespaceXInclude = "http://www.w3.org/2001/XInclude"
	// NamespaceXInclude2003 is the namespace of the 2003 draft that some
	// files still use.
	NamespaceXInclude2003 = "http://www.w3.org/2003/XInclude"
)

// Attr is an attribute of an element, as strings that stay valid.
type Attr struct {
	Name  string // qualified, as written
	URI   string // namespace; empty for none
	Local string
	Value string // decoded and normalized as XML requires
}

// Element is an open element.
type Element struct {
	Name  string // qualified, as written, prefix included
	Local string
	URI   string // namespace; empty for none
	Attrs []Attr
	// Depth is 1 for the root.
	Depth int
	// Preserve reports that xml:space="preserve" is in effect.
	Preserve bool
	// Start and End are the offsets of the start tag in the decoded text.
	Start, End int64
	// Data is for the consumer: anything it keeps per element.
	Data any
}

// Attr returns the value of the attribute with the given namespace and
// local name.
func (e *Element) Attr(uri, local string) (string, bool) {
	for i := range e.Attrs {
		if e.Attrs[i].Local == local && e.Attrs[i].URI == uri {
			return e.Attrs[i].Value, true
		}
	}
	return "", false
}

// Handler consumes the events of one reading.
type Handler interface {
	// StartElement is called when e opens; e is the top of the stack.
	StartElement(r *Reader, e *Element)
	// EndElement is called when e closes, before it leaves the stack;
	// start and end are the offsets of its end tag (equal when it has
	// none).
	EndElement(r *Reader, e *Element, start, end int64)
	// Text is character data, CDATA and entity replacements, in pieces.
	Text(r *Reader, text []byte, start, end int64)
	// Comment is a piece of a comment; final marks its last piece.
	Comment(r *Reader, text []byte, final bool, start, end int64)
	// Stop reports that the consumer wants no more events.
	Stop() bool
}

// Kinds of external reference, in the order they are reported.
const (
	RefExternalSubset = iota
	RefExternalEntity
	RefStylesheet
	RefXInclude
	refKinds
)

// RefKindNames names the kinds for messages.
var RefKindNames = [refKinds]string{
	"external DTD subset", "external entity reference", "xml-stylesheet instruction", "XInclude element",
}

// Report counts what the reading found besides the events.
type Report struct {
	// External counts references a conforming processor would load, by
	// kind (Ref*). Nothing is loaded.
	External [refKinds]int
	// PIs counts processing instructions other than the XML declaration
	// and xml-stylesheet.
	PIs int
	// Assumed counts references to undeclared entities read as the
	// character the W3C/HTML entity list gives, in a file with an
	// external DTD subset; Undeclared those kept as written.
	Assumed, Undeclared int
	// Repairs counts places where damaged markup was read leniently.
	Repairs int
	// Elements counts elements read.
	Elements int
	// Root is the root element, Doctype the declaration, if any.
	RootName, RootURI string
	RootLang          string
	SchemaLocation    string
	NoNSSchemaLoc     string
	Doctype           *dtd.Doctype
}

// Options configures a reading.
type Options struct {
	Scan xmlscan.Options
}

// Reader reads one document.
type Reader struct {
	s      *xmlscan.Scanner
	stack  []*Element
	free   []*Element
	names  map[string]string
	ents   map[string][]byte
	Report Report
	ctx    context.Context
}

// New returns a Reader of the decoded text r. ExpansionLimit in opts
// bounds entity expansion.
func New(ctx context.Context, r io.Reader, opts Options) *Reader {
	rd := &Reader{names: map[string]string{}, ctx: ctx}
	so := opts.Scan
	so.Resolve = rd.resolve
	rd.s = xmlscan.New(r, &so)
	return rd
}

// NewBytes is New for text in memory.
func NewBytes(ctx context.Context, data []byte, opts Options) *Reader {
	rd := &Reader{names: map[string]string{}, ctx: ctx}
	so := opts.Scan
	so.Resolve = rd.resolve
	rd.s = xmlscan.NewBytes(data, &so)
	return rd
}

// Scanner returns the scanner, for its Stats and Err.
func (r *Reader) Scanner() *xmlscan.Scanner { return r.s }

// Stack returns the open elements, the root first.
func (r *Reader) Stack() []*Element { return r.stack }

// Top returns the innermost open element, or nil.
func (r *Reader) Top() *Element {
	if len(r.stack) == 0 {
		return nil
	}
	return r.stack[len(r.stack)-1]
}

// Offset returns the offset in the decoded text the reading has reached.
func (r *Reader) Offset() int64 { return r.s.Offset() }

// intern returns b as a string, shared between equal names.
func (r *Reader) intern(b []byte) string {
	if s, ok := r.names[string(b)]; ok {
		return s
	}
	s := string(b)
	if len(r.names) < 4096 && len(b) <= 256 {
		r.names[s] = s
	}
	return s
}

// resolve decides a general entity reference (decision D3 of the plan).
func (r *Reader) resolve(name []byte) ([]byte, xmlscan.Resolution) {
	if d := r.Report.Doctype; d != nil {
		if e, ok := d.Entities[string(name)]; ok {
			if e.External {
				r.Report.External[RefExternalEntity]++
				return nil, xmlscan.Keep
			}
			return e.Value, xmlscan.Expand
		}
		if d.HasExternal {
			if v, ok := r.ents[string(name)]; ok {
				r.Report.Assumed++
				return v, xmlscan.Insert
			}
			if v, ok := entity.Lookup(string(name) + ";"); ok {
				if r.ents == nil {
					r.ents = map[string][]byte{}
				}
				r.ents[string(name)] = []byte(v)
				r.Report.Assumed++
				return r.ents[string(name)], xmlscan.Insert
			}
		}
	}
	r.Report.Undeclared++
	return nil, xmlscan.Keep
}

func isSpaceOnly(b []byte) bool {
	for _, c := range b {
		if c != ' ' && c != '\t' && c != '\n' && c != '\r' {
			return false
		}
	}
	return true
}

// Run reads the document to its end, or until the handler stops or the
// context is done, and returns the scanner's error or the context's.
func (r *Reader) Run(h Handler) error {
	s := r.s
	rootDone := false
	defer func() { r.Report.Repairs += s.Stats.Repairs }()
	for n := 0; ; n++ {
		if n&1023 == 0 {
			if err := r.ctx.Err(); err != nil {
				return err
			}
		}
		if h.Stop() {
			return nil
		}
		switch s.Next() {
		case xmlscan.EOF:
			return s.Err()
		case xmlscan.Doctype:
			if r.Report.Doctype != nil || r.Report.Elements > 0 {
				r.Report.Repairs++ // a second or late declaration is not read
				continue
			}
			r.Report.Doctype = dtd.Parse(s.Text())
			if r.Report.Doctype.HasExternal {
				r.Report.External[RefExternalSubset]++
			}
			r.Report.External[RefExternalEntity] += r.Report.Doctype.ExternalParamRefs
		case xmlscan.PI:
			switch t := s.Target(); {
			case bytes.EqualFold(t, []byte("xml")):
			case string(t) == "xml-stylesheet":
				r.Report.External[RefStylesheet]++
			default:
				r.Report.PIs++
			}
		case xmlscan.Start:
			if len(r.stack) == 0 && rootDone {
				r.Report.Repairs++ // a second root element
			}
			e := r.open()
			r.Report.Elements++
			if e.Depth == 1 && !rootDone {
				r.root(e)
			}
			if e.Local == "include" && (e.URI == NamespaceXInclude || e.URI == NamespaceXInclude2003) {
				r.Report.External[RefXInclude]++
			}
			h.StartElement(r, e)
		case xmlscan.End:
			e := r.Top()
			if e == nil {
				continue
			}
			start, end := s.Span()
			h.EndElement(r, e, start, end)
			r.close()
			if len(r.stack) == 0 {
				rootDone = true
			}
		case xmlscan.Text:
			t := s.Text()
			if len(r.stack) == 0 && !isSpaceOnly(t) {
				r.Report.Repairs++ // text outside the root element
			}
			start, end := s.Span()
			h.Text(r, t, start, end)
		case xmlscan.Comment:
			start, end := s.Span()
			h.Comment(r, s.Text(), s.Final(), start, end)
		}
	}
}

// open makes the Element of the current Start token and pushes it.
func (r *Reader) open() *Element {
	s := r.s
	var e *Element
	if n := len(r.free); n > 0 {
		e = r.free[n-1]
		r.free = r.free[:n-1]
		attrs := e.Attrs[:0]
		*e = Element{Attrs: attrs}
	} else {
		e = &Element{}
	}
	e.Name = r.intern(s.Name())
	e.Local = r.intern(s.Local())
	e.URI = r.intern(s.URI())
	e.Depth = len(r.stack) + 1
	e.Start, e.End = s.Span()
	if p := r.Top(); p != nil {
		e.Preserve = p.Preserve
	}
	for i := range s.NumAttr() {
		uri, local, value := s.AttrAt(i)
		a := Attr{Name: r.intern(s.AttrNameAt(i)), URI: r.intern(uri), Local: r.intern(local), Value: string(value)}
		if a.URI == xmlscan.XMLNamespace && a.Local == "space" {
			switch a.Value {
			case "preserve":
				e.Preserve = true
			case "default":
				e.Preserve = false
			}
		}
		e.Attrs = append(e.Attrs, a)
	}
	r.stack = append(r.stack, e)
	return e
}

func (r *Reader) close() {
	n := len(r.stack)
	e := r.stack[n-1]
	r.stack = r.stack[:n-1]
	e.Data = nil
	r.free = append(r.free, e)
}

// root records what the root element says about the document.
func (r *Reader) root(e *Element) {
	r.Report.RootName, r.Report.RootURI = e.Name, e.URI
	if v, ok := e.Attr(xmlscan.XMLNamespace, "lang"); ok {
		r.Report.RootLang = v
	}
	if v, ok := e.Attr(NamespaceXSI, "schemaLocation"); ok {
		r.Report.SchemaLocation = v
	}
	if v, ok := e.Attr(NamespaceXSI, "noNamespaceSchemaLocation"); ok {
		r.Report.NoNSSchemaLoc = v
	}
}
