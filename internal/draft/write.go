package draft

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/AOShei/go-fast-xml/internal/xsd"
)

// MaxRoots bounds the match entries drafted from root elements.
const MaxRoots = 32

var prefixRE = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_.-]{0,31}$`)

func (w *writer) namespaces() {
	uris := map[string]bool{}
	noNS := false
	for _, d := range w.in.Model.Decls {
		if d.Name.URI == "" {
			noNS = true
		}
		uris[d.Name.URI] = true
		for _, a := range d.Attrs {
			uris[a.Name.URI] = true
		}
	}
	if len(w.in.Set.Docs) > 0 && !noNS {
		w.defNS = w.in.Set.Docs[0].TNS
	}
	list := make([]string, 0, len(uris))
	for u := range uris {
		if u != "" && u != w.defNS && u != xsd.XMLNS {
			list = append(list, u)
		}
	}
	sort.Strings(list)
	used := map[string]bool{"xml": true, "xmlns": true}
	for _, u := range list {
		p := ""
		for _, d := range w.in.Set.Docs {
			if c, ok := d.Prefixes[u]; ok && prefixRE.MatchString(c) && !used[c] {
				p = c
				break
			}
		}
		for i := 1; p == ""; i++ {
			if c := fmt.Sprintf("ns%d", i); !used[c] {
				p = c
			}
		}
		used[p] = true
		w.prefix[u] = p
	}
}

func (w *writer) pfx(uri string) string {
	if uri == xsd.XMLNS {
		return "xml"
	}
	return w.prefix[uri]
}

func (w *writer) sel(d *xsd.Decl) string {
	if d.Name.URI == w.defNS {
		return d.Name.Local
	}
	return w.pfx(d.Name.URI) + ":" + d.Name.Local
}

func enc(v any) string {
	if m, ok := v.(map[string]string); ok {
		keys := make([]string, 0, len(m))
		for k := range m {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		parts := make([]string, len(keys))
		for i, k := range keys {
			parts[i] = enc(k) + ": " + enc(m[k])
		}
		return "{" + strings.Join(parts, ", ") + "}"
	}
	var b bytes.Buffer
	e := json.NewEncoder(&b)
	e.SetEscapeHTML(false)
	if err := e.Encode(v); err != nil {
		return `""`
	}
	return strings.TrimSuffix(b.String(), "\n")
}

func note(s *suggestion) string {
	n := strings.Join(s.why, "; ")
	if s.byName {
		n += "; rests on the name"
	}
	return n
}

// rules assembles the rules: one per element name where every
// declaration of it gets the same suggestion, else one per parent; and
// the rules a container decides for its children.
func (w *writer) rules() []rule {
	decls := w.in.Model.Decls
	type kidOut struct {
		after *xsd.Decl
		r     rule
	}
	taken := map[string]bool{}
	var kids []kidOut
	for _, d := range decls {
		s := w.sugg[d]
		if s == nil {
			continue
		}
		for _, k := range s.kids {
			var sel string
			switch {
			case k.bare:
				sel = w.sel(k.child)
			case k.parent != nil:
				sel = w.sel(k.parent) + "/" + w.sel(k.child)
			default:
				sel = w.sel(d) + "/" + w.sel(k.child)
			}
			if taken[sel] {
				continue
			}
			taken[sel] = true
			kids = append(kids, kidOut{d, rule{sel: sel, as: k.as, keys: k.keys, note: k.why}})
		}
	}
	groups := map[xsd.QName][]*xsd.Decl{}
	var order []xsd.QName
	for _, d := range decls {
		if _, ok := groups[d.Name]; !ok {
			order = append(order, d.Name)
		}
		groups[d.Name] = append(groups[d.Name], d)
	}
	own := map[*xsd.Decl][]rule{}
	for _, q := range order {
		g := groups[q]
		var decided []*xsd.Decl
		for _, d := range g {
			if w.sugg[d] != nil && !w.kidOfAll(d) {
				decided = append(decided, d)
			}
		}
		if len(decided) == 0 {
			continue
		}
		one := len(decided) == len(g)
		first := w.rule(decided[0], w.sel(decided[0]))
		for _, d := range decided[1:] {
			if !w.rule(d, "").same(first) {
				one = false
			}
		}
		if one {
			if !taken[first.sel] {
				taken[first.sel] = true
				own[decided[0]] = append(own[decided[0]], first)
			}
			continue
		}
		for _, d := range decided {
			var sels []string
			for _, p := range d.Parents {
				sels = append(sels, w.sel(p)+"/"+w.sel(d))
			}
			if len(d.Parents) == 0 {
				sels = append(sels, "/"+w.sel(d))
			}
			for _, sel := range sels {
				if taken[sel] {
					continue
				}
				taken[sel] = true
				r := w.rule(d, sel)
				r.note += fmt.Sprintf("; %s has %d declarations that differ, this one where it stands here", d.Name.Local, len(g))
				own[d] = append(own[d], r)
			}
		}
	}
	var out []rule
	ki := 0
	for _, d := range decls {
		out = append(out, own[d]...)
		for ki < len(kids) && kids[ki].after == d {
			out = append(out, kids[ki].r)
			ki++
		}
	}
	return out
}

func (w *writer) rule(d *xsd.Decl, sel string) rule {
	s := w.sugg[d]
	return rule{sel: sel, as: s.as, keys: s.keys, note: note(s)}
}

// undecidedEntry describes an element the draft has no rule for.
type undecidedEntry struct {
	Element    string   `json:"element"`
	Content    string   `json:"content"`
	Parents    []string `json:"parents,omitempty"`
	Children   []string `json:"children,omitempty"`
	Attributes []string `json:"attributes,omitempty"`
	Any        bool     `json:"any,omitempty"`
}

func capped(xs []string, n int) []string {
	sort.Strings(xs)
	xs = uniq(xs)
	if len(xs) > n {
		xs = append(xs[:n:n], fmt.Sprintf("(and %d more)", len(xs)-n))
	}
	return xs
}

func uniq(xs []string) []string {
	out := xs[:0]
	for i, x := range xs {
		if i == 0 || x != xs[i-1] {
			out = append(out, x)
		}
	}
	return out
}

func (w *writer) undecided() []undecidedEntry {
	byName := map[xsd.QName]*undecidedEntry{}
	var order []xsd.QName
	for _, d := range w.in.Model.Decls {
		if w.sugg[d] != nil || w.isKid(d) {
			continue
		}
		e := byName[d.Name]
		if e == nil {
			e = &undecidedEntry{Element: w.sel(d), Content: d.Content.String()}
			byName[d.Name] = e
			order = append(order, d.Name)
		} else if e.Content != d.Content.String() && !strings.Contains(e.Content, d.Content.String()) {
			e.Content += " or " + d.Content.String()
		}
		for _, p := range d.Parents {
			e.Parents = append(e.Parents, w.sel(p))
		}
		for _, c := range d.Children {
			e.Children = append(e.Children, w.sel(c.Decl))
		}
		for _, a := range d.Attrs {
			s := w.attrName(a)
			if a.Type != xsd.OtherType {
				s += " (" + a.Type.String() + ")"
			}
			e.Attributes = append(e.Attributes, s)
		}
		e.Any = e.Any || d.Any
	}
	out := make([]undecidedEntry, 0, len(order))
	for _, q := range order {
		e := byName[q]
		e.Parents, e.Children, e.Attributes = capped(e.Parents, 12), capped(e.Children, 16), capped(e.Attributes, 16)
		out = append(out, *e)
	}
	return out
}

// isKid reports an element some container decided the meaning of.
func (w *writer) isKid(d *xsd.Decl) bool {
	for _, p := range d.Parents {
		if w.decides(p, d) {
			return true
		}
	}
	return false
}

// kidOfAll reports an element whose every parent decides its meaning: it
// needs no rule of its own.
func (w *writer) kidOfAll(d *xsd.Decl) bool {
	for _, p := range d.Parents {
		if !w.decides(p, d) {
			return false
		}
	}
	return len(d.Parents) > 0
}

// decides reports that p's suggestion makes a rule for its child d. A
// cell's row decides it through the table.
func (w *writer) decides(p, d *xsd.Decl) bool {
	for _, q := range append([]*xsd.Decl{p}, p.Parents...) {
		if s := w.sugg[q]; s != nil {
			for _, k := range s.kids {
				if k.child == d && (k.parent == nil && q == p || k.parent == p || k.bare) {
					return true
				}
			}
		}
	}
	return false
}

var nameRE = regexp.MustCompile(`[^a-z0-9._-]+`)

const draftNote = "A draft profile written by gofastxml -draft-profile from an XML Schema, for a person to finish. " +
	"Each rule is a suggestion marked \"review\": true; LoadProfile refuses the file until every mark is removed. " +
	"Suggestions come from the content model (what an element contains and where it appears); a note that says " +
	"\"rests on the name\" means the element's name decided it. Elements under \"undecided\" have no rule and are read generically. " +
	"Rules for metadata, labels, notes, parts and stored content are not drafted."

func (w *writer) write() []byte {
	var b bytes.Buffer
	entry := w.in.Entry
	base := strings.TrimSuffix(filepath.Base(entry), filepath.Ext(entry))
	name := strings.Trim(nameRE.ReplaceAllString(strings.ToLower(base), "-"), "-._")
	if name == "" {
		name = "schema"
	}
	name = "draft-" + name
	if len(name) > 64 {
		name = name[:64]
	}
	fmt.Fprintf(&b, "{\n  \"profile\": %s,\n  \"title\": %s,\n  \"format\": 1,\n", enc(name), enc("Draft from "+filepath.Base(entry)+": review every rule"))

	var files []string
	dir := filepath.Dir(entry)
	for _, d := range w.in.Set.Docs {
		rel, err := filepath.Rel(dir, d.Path)
		if err != nil {
			rel = d.Path
		}
		files = append(files, filepath.ToSlash(rel))
	}
	var missing []string
	for q, what := range w.in.Model.Missing {
		missing = append(missing, what+" "+q.String())
	}
	sort.Strings(missing)
	b.WriteString("  \"notes\": {\n")
	fmt.Fprintf(&b, "    \"draft\": %s,\n", enc(draftNote))
	fmt.Fprintf(&b, "    \"schema_files\": %s,\n", list(files, "      "))
	if len(w.in.Problems) > 0 {
		fmt.Fprintf(&b, "    \"not_read\": %s,\n", list(w.in.Problems, "      "))
	}
	if len(missing) > 0 {
		fmt.Fprintf(&b, "    \"missing_components\": %s,\n", list(capped(missing, 200), "      "))
	}
	if w.in.Model.Cut {
		fmt.Fprintf(&b, "    \"cut\": %s,\n", enc(fmt.Sprintf("more than %d element declarations; the rest are not in the draft", w.in.Model.MaxDecls)))
	}
	und := w.undecided()
	b.WriteString("    \"undecided\": [")
	for i, e := range und {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString("\n      " + enc(e))
	}
	if len(und) > 0 {
		b.WriteString("\n    ")
	}
	b.WriteString("]\n  },\n")

	if len(w.prefix) > 0 {
		ns := make([]string, 0, len(w.prefix))
		for u, p := range w.prefix {
			ns = append(ns, enc(p)+": "+enc(u))
		}
		sort.Strings(ns)
		b.WriteString("  \"namespaces\": {\n    " + strings.Join(ns, ",\n    ") + "\n  },\n")
	}
	if w.defNS != "" {
		fmt.Fprintf(&b, "  \"default_namespace\": %s,\n", enc(w.defNS))
	}
	b.WriteString("  \"match\": [")
	for i, r := range w.in.Model.Roots {
		if i == MaxRoots {
			break
		}
		if i > 0 {
			b.WriteByte(',')
		}
		m := `{"root": ` + enc(r.Name.Local)
		if r.Name.URI != "" {
			m += `, "namespace": ` + enc(r.Name.URI)
		}
		b.WriteString("\n    " + m + "}")
	}
	if len(w.in.Model.Roots) > 0 {
		b.WriteString("\n  ")
	}
	b.WriteString("],\n  \"rules\": [")
	for i, r := range w.rules() {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString("\n    {\"match\": " + enc(r.sel) + ", \"as\": " + enc(r.as))
		for _, k := range r.keys {
			b.WriteString(", " + enc(k.k) + ": " + enc(k.v))
		}
		b.WriteString(", \"review\": true")
		if r.note != "" {
			b.WriteString(", \"note\": " + enc(r.note))
		}
		b.WriteString("}")
	}
	b.WriteString("\n  ]\n}\n")
	return b.Bytes()
}

func list(xs []string, indent string) string {
	if len(xs) == 0 {
		return "[]"
	}
	parts := make([]string, len(xs))
	for i, x := range xs {
		parts[i] = indent + enc(x)
	}
	return "[\n" + strings.Join(parts, ",\n") + "\n" + indent[2:] + "]"
}
