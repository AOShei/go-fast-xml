// Package draft writes a draft profile from what an XML Schema declares,
// for a person to finish. Every rule it writes carries "review": true,
// which the profile loader refuses: a draft is not a profile.
//
// Suggestions come from the content model first (what an element may
// contain and where it may appear) and from the element's name only to
// break a tie; a rule that rests on the name alone says so in its note.
// Name guessing happens here only, never during extraction.
package draft

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/AOShei/go-fast-xml/internal/xsd"
)

// kv is one key of a rule, in the order written.
type kv struct {
	k string
	v any
}

// rule is one suggested rule.
type rule struct {
	sel  string
	as   string
	keys []kv
	note string
}

func (r rule) same(o rule) bool {
	if r.as != o.as || len(r.keys) != len(o.keys) {
		return false
	}
	for i := range r.keys {
		if r.keys[i].k != o.keys[i].k || enc(r.keys[i].v) != enc(o.keys[i].v) {
			return false
		}
	}
	return true
}

// suggestion is what a declaration is suggested to be.
type suggestion struct {
	as     string
	keys   []kv
	why    []string
	byName bool
	// kids are rules for children whose meaning this one decides
	// (a section's heading, a list's items, a table's rows and cells).
	kids []kidRule
}

type kidRule struct {
	child  *xsd.Decl
	parent *xsd.Decl // the parent in the selector; nil: the suggestion's own element
	bare   bool      // the child's name alone (an XHTML tr, under thead, tbody or table)
	as     string
	keys   []kv
	why    string
}

// Input is what the draft is made from.
type Input struct {
	Entry    string // the schema file named
	Set      *xsd.Set
	Model    *xsd.Model
	Problems []string // what was not read, as sentences
}

type writer struct {
	in       Input
	prefix   map[string]string // namespace -> prefix
	defNS    string
	sugg     map[*xsd.Decl]*suggestion
	headings map[*xsd.Decl]*xsd.Decl // section -> its heading child
}

// Write returns the draft profile.
func Write(in Input) []byte {
	w := &writer{in: in, prefix: map[string]string{}, sugg: map[*xsd.Decl]*suggestion{}, headings: map[*xsd.Decl]*xsd.Decl{}}
	w.namespaces()
	for _, d := range in.Model.Decls {
		if h := headingChild(d); h != nil {
			w.headings[d] = h
		}
	}
	for _, d := range in.Model.Decls {
		w.sugg[d] = w.suggest(d)
	}
	return w.write()
}

// ---- names -------------------------------------------------------------

var splitRE = regexp.MustCompile(`[-_.:]`)

func lname(d *xsd.Decl) string {
	return strings.ToLower(splitRE.ReplaceAllString(d.Name.Local, ""))
}

func in(s string, set ...string) bool {
	for _, x := range set {
		if s == x {
			return true
		}
	}
	return false
}

func titleName(d *xsd.Decl) bool {
	n := lname(d)
	if in(n, "title", "titre", "head", "heading", "intitule", "hd", "titreriche") {
		return true
	}
	for _, p := range []string{"sub", "sous", "sur", "alt", "short", "running", "trans"} {
		if strings.HasPrefix(n, p) {
			return false
		}
	}
	return strings.HasSuffix(n, "title") || strings.HasSuffix(n, "titre")
}

func paraName(d *xsd.Decl) bool {
	return in(lname(d), "p", "para", "paragraph", "paragraphe", "alinea", "simpara", "al")
}
func listName(d *xsd.Decl) bool {
	return in(lname(d), "list", "liste", "ul", "ol", "dl", "itemizedlist", "orderedlist", "enumeration", "enumeration")
}
func itemName(d *xsd.Decl) bool {
	return in(lname(d), "item", "li", "listitem", "puce", "element")
}
func tableName(d *xsd.Decl) bool {
	return in(lname(d), "table", "tableau", "informaltable", "tbl")
}
func rowName(d *xsd.Decl) bool { return in(lname(d), "row", "tr", "rangee", "ligne") }
func cellName(d *xsd.Decl) bool {
	return in(lname(d), "cell", "td", "th", "entry", "cellule", "colonne")
}
func footnoteName(d *xsd.Decl) bool {
	return in(lname(d), "fn", "footnote", "notebaspage", "notedebasdepage", "authorialnote")
}
func imageName(d *xsd.Decl) bool {
	return in(lname(d), "img", "image", "graphic", "inlinegraphic", "imagedata", "illustration", "picture", "photo")
}
func breakName(d *xsd.Decl) bool { return in(lname(d), "br", "lb", "sbr", "eol", "break") }

func styleName(d *xsd.Decl) string {
	switch n := lname(d); {
	case in(n, "b", "bold", "strong", "gras"):
		return "bold"
	case in(n, "i", "italic", "em", "emphasis", "italique", "it"):
		return "italic"
	case in(n, "sup", "superscript", "exposant"):
		return "superscript"
	case in(n, "sub", "subscript", "indice"):
		return "subscript"
	case in(n, "code", "tt", "monospace", "literal"):
		return "code"
	}
	return ""
}

// ---- content model -----------------------------------------------------

func texty(d *xsd.Decl) bool { return d.Content == xsd.Text || d.Content == xsd.Mixed }

// headingChild is the child that titles d: the first child of the
// content model when it is text, once; else (the name as tie-breaker) a
// text child, once, whose name is a title's.
func headingChild(d *xsd.Decl) *xsd.Decl {
	if d.Content != xsd.Elements || len(d.Children) < 2 {
		return nil
	}
	if c := d.Children[0]; texty(c.Decl) && !c.Repeats && c.Decl != d {
		return c.Decl
	}
	for _, c := range d.Children {
		if texty(c.Decl) && !c.Repeats && titleName(c.Decl) {
			return c.Decl
		}
	}
	// A loose model (a repeated choice of num, heading, ...) makes every
	// child repeat: the name alone then picks the title.
	for _, c := range d.Children {
		if texty(c.Decl) && titleName(c.Decl) && c.Decl != d {
			return c.Decl
		}
	}
	return nil
}

func uriAttr(d *xsd.Decl) string {
	for _, a := range d.Attrs {
		if a.Type == xsd.AnyURI && !(a.Name.URI == xsd.XMLNS && a.Name.Local == "base") {
			return a.Name.Local
		}
	}
	return ""
}

func (w *writer) attrName(a xsd.Attribute) string {
	if a.Name.URI == "" {
		return a.Name.Local
	}
	return w.pfx(a.Name.URI) + ":" + a.Name.Local
}

func (w *writer) idAttr(d *xsd.Decl) string {
	for _, a := range d.Attrs {
		if a.Type == xsd.ID {
			return w.attrName(a)
		}
	}
	return ""
}

func (w *writer) linkAttr(d *xsd.Decl) string {
	for _, a := range d.Attrs {
		if a.Type == xsd.AnyURI && !(a.Name.URI == xsd.XMLNS && a.Name.Local == "base") {
			return w.attrName(a)
		}
	}
	return ""
}

func parentsBy(d *xsd.Decl) (mixed, elements int) {
	for _, p := range d.Parents {
		switch p.Content {
		case xsd.Mixed:
			mixed++
		case xsd.Elements:
			elements++
		}
	}
	return
}

// blockSiblings reports that d stands, in some elements-only parent,
// beside prose: a sibling with mixed content, or d itself repeating.
func blockSiblings(d *xsd.Decl) bool {
	for _, p := range d.Parents {
		if p.Content != xsd.Elements {
			continue
		}
		for _, c := range p.Children {
			if c.Decl == d && c.Repeats {
				return true
			}
			if c.Decl != d && c.Decl.Content == xsd.Mixed {
				return true
			}
		}
	}
	return false
}

func (w *writer) sectionLike(d *xsd.Decl) bool {
	_, ok := w.headings[d]
	return ok && (d.Nests || d.Repeated)
}

// tableShape finds a repeated child whose own children are one or two
// repeated elements with content: rows of cells.
func tableShape(d *xsd.Decl) (row *xsd.Decl, cells []*xsd.Decl) {
	for _, c := range d.Children {
		r := c.Decl
		if !c.Repeats || r.Content != xsd.Elements || len(r.Children) == 0 || len(r.Children) > 2 || r == d {
			continue
		}
		var cs []*xsd.Decl
		ok := true
		for _, cc := range r.Children {
			if !cc.Repeats || cc.Decl.Content == xsd.Empty || cc.Decl == r || cc.Decl == d {
				ok = false
				break
			}
			cs = append(cs, cc.Decl)
		}
		if ok {
			return r, cs
		}
	}
	return nil, nil
}

func childNamed(d *xsd.Decl, names ...string) *xsd.Decl {
	for _, c := range d.Children {
		if in(lname(c.Decl), names...) {
			return c.Decl
		}
	}
	return nil
}

// suggest decides what d is suggested to be, or nil.
func (w *writer) suggest(d *xsd.Decl) *suggestion {
	s := w.suggestKind(d)
	if s == nil {
		return nil
	}
	if id := w.idAttr(d); id != "" && !in(s.as, "inline", "break", "metadata", "label") {
		s.keys = append(s.keys, kv{"anchor", map[string]string{"attr": id}})
		s.why = append(s.why, "xs:ID attribute "+id)
	}
	return s
}

func (w *writer) suggestKind(d *xsd.Decl) *suggestion {
	switch d.Content {
	case xsd.Empty:
		if a := w.linkAttr(d); a != "" {
			return &suggestion{as: "image", keys: []kv{{"id", map[string]string{"attr": a}}}, why: []string{"empty, with the xs:anyURI attribute " + a}}
		}
		if imageName(d) {
			return &suggestion{as: "image", why: []string{"empty"}, byName: true}
		}
		if breakName(d) {
			return &suggestion{as: "break", why: []string{"empty"}, byName: true}
		}
		return nil
	case xsd.Elements:
		return w.container(d)
	case xsd.Text, xsd.Mixed:
		return w.prose(d)
	}
	return nil
}

func (w *writer) container(d *xsd.Decl) *suggestion {
	// CALS and XHTML tables, by their names.
	if tableName(d) {
		if childNamed(d, "tgroup") != nil {
			return &suggestion{as: "table", keys: []kv{{"model", "cals"}}, why: []string{"a tgroup child (CALS)"}, byName: true}
		}
		tr := childNamed(d, "tr")
		var groups []*xsd.Decl
		for _, c := range d.Children {
			if in(lname(c.Decl), "thead", "tbody", "tfoot") {
				groups = append(groups, c.Decl)
				if tr == nil {
					tr = childNamed(c.Decl, "tr")
				}
			}
		}
		if tr != nil {
			s := &suggestion{as: "table", why: []string{"tr rows (XHTML)"}, byName: true}
			for _, g := range groups {
				s.kids = append(s.kids, kidRule{child: g, as: "transparent", why: "a row group of an XHTML table"})
			}
			s.kids = append(s.kids, kidRule{child: tr, bare: true, as: "row", why: "a row of an XHTML table"})
			for _, c := range tr.Children {
				switch lname(c.Decl) {
				case "td":
					s.kids = append(s.kids, kidRule{child: c.Decl, parent: tr, as: "cell", keys: spans(c.Decl), why: "a data cell"})
				case "th":
					s.kids = append(s.kids, kidRule{child: c.Decl, parent: tr, as: "cell", keys: append([]kv{{"header", true}}, spans(c.Decl)...), why: "a header cell"})
				}
			}
			return s
		}
	}
	// A list: one child, repeated, that is not itself a titled section.
	if len(d.Children) == 1 && !d.Any {
		c := d.Children[0]
		if c.Repeats && c.Decl != d && c.Decl.Content != xsd.Empty && !w.sectionLike(c.Decl) {
			if _, cells := tableShape(d); cells == nil || !tableName(d) && !rowName(c.Decl) {
				return &suggestion{as: "list", why: []string{"its only child, " + c.Decl.Name.Local + ", repeats"},
					kids: []kidRule{{child: c.Decl, as: "item", why: "the repeated child of a list"}}}
			}
		}
	}
	// A table: rows of cells beside other children that do not repeat
	// (a title, a caption), or named so.
	reps := 0
	for _, c := range d.Children {
		if c.Repeats {
			reps++
		}
	}
	if row, cells := tableShape(d); row != nil && (len(d.Children) > 1 && reps == 1 || reps <= 2 && (tableName(d) || rowName(row))) {
		s := &suggestion{as: "table", why: []string{fmt.Sprintf("three levels of repetition (%s, %s)", row.Name.Local, cells[0].Name.Local)}}
		s.kids = append(s.kids, kidRule{child: row, as: "row", why: "a repeated child of a table whose children repeat"})
		for _, c := range cells {
			s.kids = append(s.kids, kidRule{child: c, parent: row, as: "cell", why: "a repeated child of a row"})
		}
		return s
	}
	if h := w.headings[d]; h != nil && (d.Nests || d.Repeated || w.parentSection(d)) {
		why := "elements only, titled by " + h.Name.Local
		hw := "the first child of a section, text"
		if d.Children[0].Decl != h {
			hw = "a text child of a section, named as a title"
		}
		switch {
		case d.Nests:
			why += ", nests"
		case d.Repeated:
			why += ", repeats"
		default:
			why += ", inside a section"
		}
		return &suggestion{as: "section", why: []string{why},
			kids: []kidRule{{child: h, as: "heading", keys: []kv{{"level", "nesting"}}, why: hw}}}
	}
	if footnoteName(d) {
		return &suggestion{as: "block", keys: []kv{{"kind", "footnote"}}, why: []string{"elements only"}, byName: true}
	}
	if listName(d) {
		return nil // a list by name whose shape is not a list's: left to the reviewer
	}
	return &suggestion{as: "transparent", why: []string{"elements only, no title or repetition that marks a section, list or table"}}
}

func (w *writer) parentSection(d *xsd.Decl) bool {
	for _, p := range d.Parents {
		if w.sectionLike(p) {
			return true
		}
	}
	return false
}

func spans(d *xsd.Decl) []kv {
	var out []kv
	for _, a := range d.Attrs {
		if a.Name.URI == "" && (a.Name.Local == "colspan" || a.Name.Local == "rowspan") {
			out = append(out, kv{a.Name.Local, a.Name.Local})
		}
	}
	return out
}

func (w *writer) prose(d *xsd.Decl) *suggestion {
	mixed, elems := parentsBy(d)
	link := w.linkAttr(d)
	if mixed > elems {
		s := &suggestion{as: "inline", why: []string{fmt.Sprintf("%s content, inside mixed content (%d of %d parents)", d.Content, mixed, mixed+elems)}}
		if link != "" {
			s.keys = append(s.keys, kv{"link", map[string]string{"attr": link}})
			s.why = append(s.why, "xs:anyURI attribute "+link)
		}
		if st := styleName(d); st != "" {
			s.keys = append(s.keys, kv{"style", st})
			s.why = append(s.why, "style "+st+" from the name alone")
		}
		return s
	}
	if elems == 0 {
		return nil // a root, or an element used nowhere
	}
	if footnoteName(d) {
		return &suggestion{as: "block", keys: []kv{{"kind", "footnote"}}, why: []string{d.Content.String() + " content"}, byName: true}
	}
	if d.Content == xsd.Mixed {
		return &suggestion{as: "block", keys: []kv{{"kind", "paragraph"}}, why: []string{"mixed content among elements-only content"}}
	}
	if blockSiblings(d) {
		return &suggestion{as: "block", keys: []kv{{"kind", "paragraph"}}, why: []string{"text content beside block siblings"}}
	}
	if paraName(d) {
		return &suggestion{as: "block", keys: []kv{{"kind", "paragraph"}}, why: []string{"text content"}, byName: true}
	}
	return nil
}
