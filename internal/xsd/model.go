package xsd

import (
	"fmt"
	"strconv"
)

// Content is what an element may contain.
type Content int

// Kinds of content.
const (
	Empty    Content = iota
	Text             // simple content: text only
	Mixed            // text and elements
	Elements         // elements only
	Unknown          // its type is in a part of the schema not read
)

func (c Content) String() string {
	return [...]string{"empty", "text", "mixed", "elements", "unknown"}[c]
}

// AttrType is what the drafting cares about in an attribute's type.
type AttrType int

// Attribute types.
const (
	OtherType AttrType = iota
	ID
	IDREF
	AnyURI
	Language
)

func (t AttrType) String() string {
	return [...]string{"", "ID", "IDREF", "anyURI", "language"}[t]
}

// Attribute is one attribute an element may have.
type Attribute struct {
	Name QName
	Type AttrType
}

// Child is an element that may appear in another.
type Child struct {
	Decl    *Decl
	Repeats bool // maxOccurs above 1, itself or a group around it
	Index   int  // position in the content model, from 0
}

// Decl is an element declaration: a global element, or a local one (one
// per place it is written; a local element in a named type is one Decl
// for every element of that type).
type Decl struct {
	Name     QName
	Global   bool
	Abstract bool
	Content  Content
	Children []Child
	Any      bool // a wildcard: other elements may appear
	Attrs    []Attribute
	Parents  []*Decl
	Repeated bool // repeats in some parent
	Nests    bool // may contain itself, directly or further down
	TypeName QName
	node     *Node
}

// Model is what the schema declares, element by element.
type Model struct {
	Decls []*Decl // global elements in the order read, then local ones in the order met
	// Roots are global elements no other element contains (all global
	// elements, when each is contained somewhere).
	Roots []*Decl
	// Cut says that MaxDecls left declarations out.
	Cut      bool
	MaxDecls int
	Missing  map[QName]string // references to components not read: what they are
}

type typeInfo struct {
	content  Content
	children []Child
	any      bool
	attrs    []Attribute
}

type analyzer struct {
	s       *Set
	m       *Model
	decls   map[*Node]*Decl
	types   map[*Node]*typeInfo
	busy    map[*Node]bool
	subst   map[QName][]QName
	queue   []*Decl
	maxDecl int
}

// MaxDecls bounds the declarations Analyze makes.
const MaxDecls = 100000

// Analyze works out each element declaration's content, children,
// attributes and parents.
func (s *Set) Analyze() *Model {
	a := &analyzer{s: s, m: &Model{MaxDecls: MaxDecls, Missing: map[QName]string{}}, decls: map[*Node]*Decl{},
		types: map[*Node]*typeInfo{}, busy: map[*Node]bool{}, subst: map[QName][]QName{}, maxDecl: MaxDecls}
	for _, q := range s.Order {
		if h, ok := s.Elements[q].Refs["substitutionGroup"]; ok {
			a.subst[h] = append(a.subst[h], q)
		}
	}
	for _, q := range s.Order {
		a.decl(s.Elements[q], true)
	}
	for i := 0; i < len(a.queue); i++ {
		a.fill(a.queue[i])
	}
	contained := map[*Decl]bool{}
	for _, d := range a.m.Decls {
		seen := map[*Decl]bool{}
		for _, c := range d.Children {
			c.Decl.Repeated = c.Decl.Repeated || c.Repeats
			contained[c.Decl] = true
			if !seen[c.Decl] {
				seen[c.Decl] = true
				c.Decl.Parents = append(c.Decl.Parents, d)
			}
		}
	}
	for _, d := range a.m.Decls {
		if d.Global && !contained[d] && !d.Abstract {
			a.m.Roots = append(a.m.Roots, d)
		}
	}
	if len(a.m.Roots) == 0 {
		for _, d := range a.m.Decls {
			if d.Global && !d.Abstract {
				a.m.Roots = append(a.m.Roots, d)
			}
		}
	}
	nests(a.m.Decls)
	return a.m
}

// decl returns the Decl of an element node, making it on first sight.
func (a *analyzer) decl(n *Node, global bool) *Decl {
	if d, ok := a.decls[n]; ok {
		return d
	}
	if len(a.m.Decls) >= a.maxDecl {
		a.m.Cut = true
		return nil
	}
	d := &Decl{Global: global, node: n, Abstract: n.Attr("abstract") == "true"}
	name := n.Attr("name")
	if global {
		d.Name = QName{n.Doc.TNS, name}
	} else {
		form := n.Attr("form")
		if form == "qualified" || form == "" && n.Doc.ElemQualified {
			d.Name = QName{n.Doc.TNS, name}
		} else {
			d.Name = QName{"", name}
		}
	}
	a.decls[n] = d
	a.m.Decls = append(a.m.Decls, d)
	a.queue = append(a.queue, d)
	return d
}

// fill works out a declaration's content from its type.
func (a *analyzer) fill(d *Decl) {
	ti := a.elementType(d.node, 0)
	if ti == nil {
		d.Content = Unknown
		return
	}
	d.Content, d.Children, d.Any, d.Attrs = ti.content, ti.children, ti.any, ti.attrs
}

var anyType = &typeInfo{content: Mixed, any: true}

func (a *analyzer) elementType(n *Node, depth int) *typeInfo {
	if depth > 32 {
		return anyType
	}
	if t, ok := n.Refs["type"]; ok {
		return a.namedType(t)
	}
	for _, c := range n.Children {
		switch c.Local {
		case "complexType":
			return a.complexType(c)
		case "simpleType":
			return &typeInfo{content: Text}
		}
	}
	if h, ok := n.Refs["substitutionGroup"]; ok {
		if hn := a.s.Elements[h]; hn != nil && hn != n {
			return a.elementType(hn, depth+1)
		}
	}
	return anyType
}

func (a *analyzer) namedType(q QName) *typeInfo {
	if q.URI == NS {
		if q.Local == "anyType" {
			return anyType
		}
		return &typeInfo{content: Text}
	}
	n := a.s.Types[q]
	if n == nil {
		a.m.Missing[q] = "type"
		return nil
	}
	if n.Local == "simpleType" {
		return &typeInfo{content: Text}
	}
	return a.complexType(n)
}

// complexType gives a complex type's content, following its base.
func (a *analyzer) complexType(n *Node) *typeInfo {
	if t, ok := a.types[n]; ok {
		return t
	}
	if a.busy[n] {
		return &typeInfo{content: Empty} // a derivation cycle: an error in the schema
	}
	a.busy[n] = true
	defer delete(a.busy, n)
	t := &typeInfo{}
	mixed := n.Attr("mixed") == "true"
	model := n
	for _, c := range n.Children {
		switch c.Local {
		case "simpleContent":
			t.content = Text
			for _, d := range c.Children {
				if d.Local == "extension" || d.Local == "restriction" {
					if b, ok := d.Refs["base"]; ok && b.URI != NS {
						if bn := a.s.Types[b]; bn != nil && bn.Local == "complexType" {
							t.attrs = append(t.attrs, a.complexType(bn).attrs...)
						} else if bn == nil {
							a.m.Missing[b] = "type"
						}
					}
					a.attributes(d, &t.attrs, 0)
				}
			}
			a.types[n] = t
			return t
		case "complexContent":
			if c.Attr("mixed") == "true" {
				mixed = true
			}
			for _, d := range c.Children {
				if d.Local != "extension" && d.Local != "restriction" {
					continue
				}
				model = d
				if b, ok := d.Refs["base"]; ok && !(b.URI == NS && b.Local == "anyType") {
					bt := a.namedType(b)
					if bt != nil {
						if d.Local == "extension" {
							t.children = append(t.children, bt.children...)
							t.any = bt.any
							if bt.content == Mixed {
								mixed = true
							}
						}
						t.attrs = append(t.attrs, bt.attrs...)
					}
				}
			}
		}
	}
	a.particles(model, false, t, 0)
	a.attributes(model, &t.attrs, 0)
	for i := range t.children {
		t.children[i].Index = i
	}
	switch {
	case mixed:
		t.content = Mixed
	case len(t.children) > 0 || t.any:
		t.content = Elements
	default:
		t.content = Empty
	}
	a.types[n] = t
	return t
}

func repeats(n *Node) bool {
	m := n.Attr("maxOccurs")
	if m == "unbounded" {
		return true
	}
	v, err := strconv.Atoi(m)
	return err == nil && v > 1
}

// particles collects the elements a content model allows.
func (a *analyzer) particles(n *Node, rep bool, t *typeInfo, depth int) {
	if depth > 64 {
		return
	}
	for _, c := range n.Children {
		r := rep || repeats(c)
		switch c.Local {
		case "element":
			if c.Attr("maxOccurs") == "0" {
				continue
			}
			if q, ok := c.Refs["ref"]; ok {
				a.addRef(q, r, t, map[QName]bool{})
				continue
			}
			if d := a.decl(c, false); d != nil {
				addChild(t, d, r)
			}
		case "sequence", "choice", "all":
			a.particles(c, r, t, depth+1)
		case "group":
			q, ok := c.Refs["ref"]
			if !ok {
				continue
			}
			g := a.s.Groups[q]
			if g == nil {
				a.m.Missing[q] = "group"
				continue
			}
			if a.busy[g] {
				continue
			}
			a.busy[g] = true
			a.particles(g, r, t, depth+1)
			delete(a.busy, g)
		case "any":
			t.any = true
		}
	}
}

// addRef adds a global element and the members of its substitution group.
func (a *analyzer) addRef(q QName, rep bool, t *typeInfo, seen map[QName]bool) {
	if seen[q] || len(seen) > 1000 {
		return
	}
	seen[q] = true
	n := a.s.Elements[q]
	if n == nil {
		a.m.Missing[q] = "element"
		return
	}
	if d := a.decl(n, true); d != nil && !d.Abstract {
		addChild(t, d, rep)
	}
	for _, m := range a.subst[q] {
		a.addRef(m, rep, t, seen)
	}
}

func addChild(t *typeInfo, d *Decl, rep bool) {
	for i := range t.children {
		if t.children[i].Decl == d {
			// Met twice in one model: it may repeat.
			t.children[i].Repeats = true
			return
		}
	}
	t.children = append(t.children, Child{Decl: d, Repeats: rep})
}

// attributes collects the attribute declarations of n and the groups it
// names.
func (a *analyzer) attributes(n *Node, out *[]Attribute, depth int) {
	if depth > 64 || len(*out) > 4096 {
		return
	}
	for _, c := range n.Children {
		switch c.Local {
		case "attribute":
			if c.Attr("use") == "prohibited" {
				continue
			}
			at := a.attribute(c)
			dup := false
			for _, o := range *out {
				if o.Name == at.Name {
					dup = true
					break
				}
			}
			if !dup && at.Name.Local != "" {
				*out = append(*out, at)
			}
		case "attributeGroup":
			q, ok := c.Refs["ref"]
			if !ok {
				continue
			}
			g := a.s.AttrGroups[q]
			if g == nil {
				a.m.Missing[q] = "attribute group"
				continue
			}
			if a.busy[g] {
				continue
			}
			a.busy[g] = true
			a.attributes(g, out, depth+1)
			delete(a.busy, g)
		}
	}
}

func (a *analyzer) attribute(c *Node) Attribute {
	if q, ok := c.Refs["ref"]; ok {
		at := Attribute{Name: q}
		if g := a.s.Attributes[q]; g != nil {
			at.Type = a.attrType(g, 0)
		} else if q.URI == XMLNS {
			switch q.Local {
			case "id":
				at.Type = ID
			case "lang":
				at.Type = Language
			case "base":
				at.Type = AnyURI
			}
		} else {
			a.m.Missing[q] = "attribute"
		}
		return at
	}
	name := QName{Local: c.Attr("name")}
	form := c.Attr("form")
	if form == "qualified" || form == "" && c.Doc.AttrQualified {
		name.URI = c.Doc.TNS
	}
	return Attribute{Name: name, Type: a.attrType(c, 0)}
}

// attrType follows an attribute's type to the built-in type it restricts.
func (a *analyzer) attrType(n *Node, depth int) AttrType {
	if depth > 32 {
		return OtherType
	}
	q, ok := n.Refs["type"]
	if !ok {
		q, ok = n.Refs["base"]
	}
	if ok {
		if q.URI == NS {
			switch q.Local {
			case "ID":
				return ID
			case "IDREF", "IDREFS":
				return IDREF
			case "anyURI":
				return AnyURI
			case "language":
				return Language
			}
			return OtherType
		}
		if t := a.s.Types[q]; t != nil && t != n {
			return a.attrType(t, depth+1)
		}
		return OtherType
	}
	for _, c := range n.Children {
		if c.Local == "simpleType" || c.Local == "restriction" {
			return a.attrType(c, depth+1)
		}
	}
	return OtherType
}

// nests marks the declarations that may contain themselves, by strongly
// connected components (Tarjan), iteratively so that depth costs no stack.
func nests(ds []*Decl) {
	index := make(map[*Decl]int, len(ds))
	low := make(map[*Decl]int, len(ds))
	on := make(map[*Decl]bool, len(ds))
	var stack []*Decl
	n := 0
	type frame struct {
		d *Decl
		i int
	}
	for _, root := range ds {
		if _, ok := index[root]; ok {
			continue
		}
		work := []frame{{root, 0}}
		index[root], low[root] = n, n
		n++
		stack = append(stack, root)
		on[root] = true
		for len(work) > 0 {
			f := &work[len(work)-1]
			if f.i < len(f.d.Children) {
				c := f.d.Children[f.i].Decl
				f.i++
				if c == f.d {
					c.Nests = true
				}
				if _, ok := index[c]; !ok {
					index[c], low[c] = n, n
					n++
					stack = append(stack, c)
					on[c] = true
					work = append(work, frame{c, 0})
				} else if on[c] {
					low[f.d] = min(low[f.d], index[c])
				}
				continue
			}
			d := f.d
			work = work[:len(work)-1]
			if len(work) > 0 {
				p := work[len(work)-1].d
				low[p] = min(low[p], low[d])
			}
			if low[d] == index[d] {
				var comp []*Decl
				for {
					x := stack[len(stack)-1]
					stack = stack[:len(stack)-1]
					on[x] = false
					comp = append(comp, x)
					if x == d {
						break
					}
				}
				if len(comp) > 1 {
					for _, x := range comp {
						x.Nests = true
					}
				}
			}
		}
	}
}

// Describe is a short account of a declaration for messages and tests.
func (d *Decl) Describe() string {
	return fmt.Sprintf("%s (%s, %d children, %d attributes)", d.Name, d.Content, len(d.Children), len(d.Attrs))
}
