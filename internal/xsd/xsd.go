// Package xsd reads an XML Schema (XSD 1.0) as data, with this module's
// own XML reader, far enough to say what elements a schema declares and
// what each may contain: global and local elements, named and anonymous
// types, derivation by extension and restriction, groups and attribute
// groups, substitution groups, wildcards and the types of attributes.
//
// xs:include, xs:import and xs:redefine are followed on disk only,
// relative to the file that names them. A location that is a URL, an
// absolute path, or names no readable file is not opened; it is reported
// in Set.Problems. Nothing is fetched. Everything is bounded by Limits.
package xsd

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/AOShei/go-fast-xml/internal/xmlenc"
	"github.com/AOShei/go-fast-xml/internal/xmlread"
	"github.com/AOShei/go-fast-xml/internal/xmlscan"
)

// Namespaces.
const (
	NS    = "http://www.w3.org/2001/XMLSchema"
	XMLNS = "http://www.w3.org/XML/1998/namespace"
)

// QName is a namespace-qualified name.
type QName struct{ URI, Local string }

func (q QName) String() string {
	if q.URI == "" {
		return q.Local
	}
	return "{" + q.URI + "}" + q.Local
}

// Limits bound one reading of a schema and its includes.
type Limits struct {
	MaxFiles int   // schema documents read
	MaxDepth int   // include/import nesting
	MaxBytes int64 // bytes of all documents together
	MaxNodes int   // schema elements kept, all documents together
}

// DefaultLimits are generous for published schemas (JATS with MathML is
// under 2 MB in about 60 files).
var DefaultLimits = Limits{MaxFiles: 512, MaxDepth: 32, MaxBytes: 32 << 20, MaxNodes: 400000}

// Node is one element of the XSD namespace in a schema document.
type Node struct {
	Local    string            // xs:element is "element"
	Attrs    map[string]string // unqualified attributes, as written (whitespace trimmed)
	Refs     map[string]QName  // QName-valued attributes resolved: type, ref, base, substitutionGroup, itemType
	Children []*Node
	Doc      *Doc
}

// Attr returns an unqualified attribute.
func (n *Node) Attr(name string) string { return n.Attrs[name] }

// Doc is one schema document.
type Doc struct {
	Path          string // as opened
	TNS           string // target namespace (a chameleon include's is the includer's)
	ElemQualified bool   // elementFormDefault="qualified"
	AttrQualified bool   // attributeFormDefault="qualified"
	Prefixes      map[string]string
}

// ProblemKind says why part of a schema is missing.
type ProblemKind int

// Kinds of problem.
const (
	NotFetched ProblemKind = iota // a URL or absolute location, not opened
	NotFound                      // a relative location naming no readable file
	NoLocation                    // an import with no schemaLocation
	Unreadable                    // a file that is not a schema document
	LimitCut                      // a limit left out the rest
)

// Problem is a part of the schema that was not read.
type Problem struct {
	Kind     ProblemKind
	Location string // as written (or the path, for Unreadable)
	From     string // the document that names it
	What     string // include, import, redefine; the namespace of an import
	Msg      string
}

// Set is a schema: its documents and global components.
type Set struct {
	Docs                                   []*Doc
	Elements, Types, Groups, AttrGroups, Attributes map[QName]*Node
	// Order lists the global elements in the order they were read.
	Order    []QName
	Problems []Problem
	Bytes    int64
	nodes    int
	lim      Limits
	read     func(path string, max int64) ([]byte, error)
	loaded   map[string]bool
	ctx      context.Context
}

// ErrLimit is the error of a limit that stopped the entry document.
var ErrLimit = errors.New("xsd: limit exceeded")

func (s *Set) problem(p Problem) {
	if len(s.Problems) < 1000 {
		s.Problems = append(s.Problems, p)
	}
}

func newSet(ctx context.Context, lim Limits, read func(string, int64) ([]byte, error)) *Set {
	if lim.MaxFiles <= 0 {
		lim.MaxFiles = DefaultLimits.MaxFiles
	}
	if lim.MaxDepth <= 0 {
		lim.MaxDepth = DefaultLimits.MaxDepth
	}
	if lim.MaxBytes <= 0 {
		lim.MaxBytes = DefaultLimits.MaxBytes
	}
	if lim.MaxNodes <= 0 {
		lim.MaxNodes = DefaultLimits.MaxNodes
	}
	return &Set{
		Elements: map[QName]*Node{}, Types: map[QName]*Node{}, Groups: map[QName]*Node{},
		AttrGroups: map[QName]*Node{}, Attributes: map[QName]*Node{},
		lim: lim, read: read, loaded: map[string]bool{}, ctx: ctx,
	}
}

// Load reads the schema at path and the documents it includes, imports
// and redefines on disk. The error is for the entry document only: one
// that cannot be read or is not a schema.
func Load(ctx context.Context, path string, lim Limits) (*Set, error) {
	s := newSet(ctx, lim, readFile)
	return s, s.load(path, "", 0)
}

// LoadBytes reads one schema document held in memory; any document it
// names is reported as not found.
func LoadBytes(ctx context.Context, name string, data []byte, lim Limits) (*Set, error) {
	s := newSet(ctx, lim, func(p string, max int64) ([]byte, error) {
		if p == name {
			if int64(len(data)) > max {
				return nil, ErrLimit
			}
			return data, nil
		}
		return nil, os.ErrNotExist
	})
	return s, s.load(name, "", 0)
}

func readFile(path string, max int64) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !st.Mode().IsRegular() {
		return nil, fmt.Errorf("%s is not a regular file", path)
	}
	if st.Size() > max {
		return nil, ErrLimit
	}
	data, err := io.ReadAll(io.LimitReader(f, max+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > max {
		return nil, ErrLimit
	}
	return data, nil
}

// load reads one document. chameleon is the namespace an include without
// a target namespace takes ("" for the entry and for imports).
func (s *Set) load(path, chameleon string, depth int) error {
	key := path + "\x00" + chameleon
	if s.loaded[key] {
		return nil
	}
	s.loaded[key] = true
	if len(s.Docs) >= s.lim.MaxFiles {
		s.problem(Problem{Kind: LimitCut, Location: path, Msg: fmt.Sprintf("more than %d schema documents; the rest are not read", s.lim.MaxFiles)})
		return ErrLimit
	}
	data, err := s.read(path, s.lim.MaxBytes-s.Bytes)
	if errors.Is(err, ErrLimit) {
		s.problem(Problem{Kind: LimitCut, Location: path, Msg: fmt.Sprintf("the schema documents pass %d bytes together; this one is not read", s.lim.MaxBytes)})
		return ErrLimit
	}
	if err != nil {
		return err
	}
	s.Bytes += int64(len(data))
	if err := s.ctx.Err(); err != nil {
		return err
	}
	doc := &Doc{Path: path, Prefixes: map[string]string{}}
	root, err := s.parse(doc, data, chameleon)
	if err != nil {
		return err
	}
	if root == nil || root.Local != "schema" {
		return fmt.Errorf("%s: not an XML Schema document (no xs:schema root element)", path)
	}
	s.Docs = append(s.Docs, doc)
	for _, c := range root.Children {
		switch c.Local {
		case "include", "import", "redefine":
			s.follow(doc, c, depth)
		}
	}
	for _, c := range root.Children {
		switch c.Local {
		case "redefine":
			for _, r := range c.Children {
				s.register(r, true)
			}
		default:
			s.register(c, false)
		}
	}
	return nil
}

// follow loads the document an include, import or redefine names.
func (s *Set) follow(doc *Doc, c *Node, depth int) {
	loc := c.Attr("schemaLocation")
	what := c.Local
	if c.Local == "import" {
		what = "import of namespace " + quoteNS(c.Attr("namespace"))
	}
	p := Problem{Location: loc, From: doc.Path, What: what}
	switch {
	case loc == "":
		if c.Local == "import" && (c.Attr("namespace") == XMLNS || c.Attr("namespace") == NS) {
			return // known without a document
		}
		p.Kind, p.Msg = NoLocation, "no schemaLocation; its components are not read"
		s.problem(p)
		return
	case isURL(loc):
		p.Kind, p.Msg = NotFetched, "a URL; not fetched"
		s.problem(p)
		return
	case filepath.IsAbs(loc) || strings.HasPrefix(loc, "/") || strings.HasPrefix(loc, "\\"):
		p.Kind, p.Msg = NotFetched, "an absolute location; only locations relative to the including file are opened"
		s.problem(p)
		return
	}
	if depth+1 > s.lim.MaxDepth {
		p.Kind, p.Msg = LimitCut, fmt.Sprintf("includes nested deeper than %d; not read", s.lim.MaxDepth)
		s.problem(p)
		return
	}
	target := filepath.Join(filepath.Dir(doc.Path), filepath.FromSlash(loc))
	cham := ""
	if c.Local != "import" {
		cham = doc.TNS
	}
	err := s.load(target, cham, depth+1)
	switch {
	case err == nil:
	case errors.Is(err, ErrLimit):
	case errors.Is(err, os.ErrNotExist):
		p.Kind, p.Msg = NotFound, "no such file; its components are not read"
		s.problem(p)
	default:
		p.Kind, p.Msg = Unreadable, err.Error()
		s.problem(p)
	}
}

func quoteNS(ns string) string {
	if ns == "" {
		return "(none)"
	}
	return ns
}

// isURL reports a location with a scheme (two or more letters before a
// colon, so that a drive letter is not one) or a network path.
func isURL(loc string) bool {
	if strings.HasPrefix(loc, "//") {
		return true
	}
	i := strings.IndexByte(loc, ':')
	if i < 2 {
		return false
	}
	for _, c := range loc[:i] {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '+' || c == '-' || c == '.') {
			return false
		}
	}
	return true
}

// register records a global component. The first declaration of a name
// wins, except that a redefinition replaces it; a redefinition that
// refers to its own name refers to the original, kept under a name no
// document can write.
func (s *Set) register(n *Node, redefine bool) {
	var m map[QName]*Node
	switch n.Local {
	case "element":
		m = s.Elements
	case "complexType", "simpleType":
		m = s.Types
	case "group":
		m = s.Groups
	case "attributeGroup":
		m = s.AttrGroups
	case "attribute":
		m = s.Attributes
	default:
		return
	}
	name := n.Attr("name")
	if name == "" {
		return
	}
	q := QName{n.Doc.TNS, name}
	old, had := m[q]
	if redefine && had {
		orig := QName{q.URI, q.Local + "\x00original"}
		m[orig] = old
		renameSelf(n, q, orig, 0)
		m[q] = n
		return
	}
	if had {
		return
	}
	m[q] = n
	if n.Local == "element" {
		s.Order = append(s.Order, q)
	}
}

func renameSelf(n *Node, from, to QName, depth int) {
	if depth > 64 {
		return
	}
	for k, v := range n.Refs {
		if v == from && (k == "base" || k == "ref") {
			n.Refs[k] = to
		}
	}
	for _, c := range n.Children {
		renameSelf(c, from, to, depth+1)
	}
}

// qnameAttrs are the attributes whose value is a QName.
var qnameAttrs = map[string]bool{"type": true, "ref": true, "base": true, "substitutionGroup": true, "itemType": true}

// handler builds the tree of XSD elements of one document.
type handler struct {
	s         *Set
	doc       *Doc
	chameleon string
	stack     []*Node // nil for an element outside the schema vocabulary
	scopes    []map[string]string
	root      *Node
	cut       bool
}

func (s *Set) parse(doc *Doc, data []byte, chameleon string) (*Node, error) {
	sample := data
	if len(sample) > 4096 {
		sample = sample[:4096]
	}
	res, err := xmlenc.Sniff(sample)
	if err != nil {
		return nil, fmt.Errorf("%s: %v", doc.Path, err)
	}
	src := bytes.NewReader(data[res.Skip:])
	opts := xmlread.Options{Scan: xmlscan.Options{ExpansionLimit: 1<<20 + 8*int64(len(data)), MaxDepth: 256}}
	r := xmlread.New(s.ctx, xmlenc.NewReader(src, res), opts)
	h := &handler{s: s, doc: doc, chameleon: chameleon, scopes: []map[string]string{{"xml": XMLNS}}}
	if err := r.Run(h); err != nil {
		if s.ctx.Err() != nil {
			return nil, err
		}
		return nil, fmt.Errorf("%s: %v", doc.Path, err)
	}
	if h.cut {
		s.problem(Problem{Kind: LimitCut, Location: doc.Path, Msg: fmt.Sprintf("the schema documents hold more than %d schema elements; the rest of this document is not read", s.lim.MaxNodes)})
	}
	return h.root, nil
}

func (h *handler) StartElement(r *xmlread.Reader, e *xmlread.Element) {
	scope := h.scopes[len(h.scopes)-1]
	cloned := false
	for _, a := range e.Attrs {
		if a.URI != xmlscan.XMLNSNamespace {
			continue
		}
		prefix := ""
		if a.Name != "xmlns" {
			prefix = a.Local
		}
		if !cloned {
			ns := make(map[string]string, len(scope)+1)
			for k, v := range scope {
				ns[k] = v
			}
			scope, cloned = ns, true
		}
		scope[prefix] = a.Value
		if prefix != "" && a.Value != "" && len(h.doc.Prefixes) < 256 {
			if _, ok := h.doc.Prefixes[a.Value]; !ok {
				h.doc.Prefixes[a.Value] = prefix
			}
		}
	}
	h.scopes = append(h.scopes, scope)
	var parent *Node
	if n := len(h.stack); n > 0 {
		parent = h.stack[n-1]
	}
	skip := e.URI != NS || e.Local == "annotation" || (len(h.stack) > 0 && parent == nil) || h.cut
	if !skip && h.s.nodes >= h.s.lim.MaxNodes {
		h.cut, skip = true, true
	}
	if skip {
		h.stack = append(h.stack, nil)
		return
	}
	h.s.nodes++
	n := &Node{Local: e.Local, Attrs: map[string]string{}, Doc: h.doc}
	for _, a := range e.Attrs {
		if a.URI != "" {
			continue
		}
		v := strings.TrimSpace(a.Value)
		n.Attrs[a.Local] = v
		if qnameAttrs[a.Local] {
			if n.Refs == nil {
				n.Refs = map[string]QName{}
			}
			n.Refs[a.Local] = h.resolve(scope, v)
		}
	}
	if parent == nil && len(h.stack) == 0 {
		if h.root == nil {
			h.root = n
			if n.Local == "schema" {
				h.doc.TNS = n.Attr("targetNamespace")
				if h.doc.TNS == "" {
					h.doc.TNS = h.chameleon
				}
				h.doc.ElemQualified = n.Attr("elementFormDefault") == "qualified"
				h.doc.AttrQualified = n.Attr("attributeFormDefault") == "qualified"
			}
		}
	} else if parent != nil {
		parent.Children = append(parent.Children, n)
	}
	h.stack = append(h.stack, n)
}

// resolve reads a QName in the scope of its element. An unprefixed name
// takes the default namespace; in a chameleon document, no namespace
// becomes the includer's.
func (h *handler) resolve(scope map[string]string, v string) QName {
	prefix, local := "", v
	if i := strings.IndexByte(v, ':'); i >= 0 {
		prefix, local = v[:i], v[i+1:]
	}
	q := QName{URI: scope[prefix], Local: local}
	if q.URI == "" && h.chameleon != "" {
		q.URI = h.chameleon
	}
	return q
}

func (h *handler) EndElement(r *xmlread.Reader, e *xmlread.Element, start, end int64) {
	h.stack = h.stack[:len(h.stack)-1]
	h.scopes = h.scopes[:len(h.scopes)-1]
}

func (h *handler) Text(*xmlread.Reader, []byte, int64, int64)          {}
func (h *handler) Comment(*xmlread.Reader, []byte, bool, int64, int64) {}
func (h *handler) Stop() bool                                          { return false }
