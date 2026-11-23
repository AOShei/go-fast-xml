// Package profile loads and validates profiles (PROFILE_FORMAT.md): JSON
// files that map the elements of one XML schema to the shared model. A
// loaded profile is immutable and safe for concurrent use.
package profile

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"slices"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/AOShei/go-fast-core"
)

// MaxProfileBytes bounds the size of a profile file; MaxRules its rules.
// MaxCells bounds the entries of a row's "cells" and a table's "header";
// MaxHeaderBytes the text of one header cell.
const (
	MaxProfileBytes = 4 << 20
	MaxRules        = 10000
	MaxCells        = 64
	MaxHeaderBytes  = 256
)

// As is a rule kind.
type As uint8

// Rule kinds.
const (
	AsSection As = iota + 1
	AsHeading
	AsBlock
	AsList
	AsItem
	AsTable
	AsRow
	AsCell
	AsInline
	AsLabel
	AsImage
	AsNote
	AsMetadata
	AsPart
	AsStored
	AsTransparent
	AsSkip
	AsBreak
)

var asNames = map[string]As{
	"section": AsSection, "heading": AsHeading, "block": AsBlock, "list": AsList, "item": AsItem,
	"table": AsTable, "row": AsRow, "cell": AsCell, "inline": AsInline, "label": AsLabel,
	"image": AsImage, "note": AsNote, "metadata": AsMetadata, "part": AsPart, "stored": AsStored,
	"transparent": AsTransparent, "skip": AsSkip, "break": AsBreak,
}

func (a As) String() string {
	for k, v := range asNames {
		if v == a {
			return k
		}
	}
	return "?"
}

// Keys allowed by rule kind.
var (
	common  = []string{"match", "as", "anchor", "attrs", "mark", "display", "extra"}
	allowed = map[As][]string{
		AsSection:     common,
		AsHeading:     append(slices.Clone(common), "level", "marker", "duplicate_of", "key"),
		AsBlock:       append(slices.Clone(common), "kind", "language", "notation", "marker", "value", "duplicate_of", "key", "out_of_line", "split"),
		AsList:        append(slices.Clone(common), "ordered", "split"),
		AsItem:        append(slices.Clone(common), "marker", "value"),
		AsTable:       append(slices.Clone(common), "model", "header"),
		AsRow:         append(slices.Clone(common), "header", "cells"),
		AsCell:        append(slices.Clone(common), "header", "colspan", "rowspan"),
		AsInline:      {"match", "as", "style", "link", "title", "attrs", "extra", "space"},
		AsLabel:       {"match", "as"},
		AsImage:       append(slices.Clone(common), "id", "alt"),
		AsNote:        append(slices.Clone(common), "type", "value", "out_of_line", "split"),
		AsMetadata:    {"match", "as", "field", "key", "value", "also_heading", "join", "extra"},
		AsPart:        append(slices.Clone(common), "kind", "name"),
		AsStored:      {"match", "as", "kind", "extra"},
		AsTransparent: append(slices.Clone(common), "split"),
		AsSkip:        {"match", "as"},
		AsBreak:       {"match", "as"},
	}
	allKeys = map[string]bool{}
)

func init() {
	for _, ks := range allowed {
		for _, k := range ks {
			allKeys[k] = true
		}
	}
}

// Value is a value source (A4).
type Value struct {
	Literal   string
	IsLiteral bool
	Attr      QName  // {"attr": ...}
	Of        *QName // {"attr": ..., "of": ...}
	Child     *Selector
	Structure *Selector
	// AttrChild: {"attr": ..., "child": ...}, the attribute Attr of the
	// first matching child, which is still read by its own rule.
	AttrChild *Selector
}

// Claims reports whether the value is read from a child element.
func (v *Value) Claims() *Selector {
	if v.Child != nil {
		return v.Child
	}
	return v.Structure
}

// KV is a key and its value source.
type KV struct {
	Key   string
	Value *Value
}

// Test is a boolean that is constant or an attribute test.
type Test struct {
	Const bool
	Attr  *QName
	In    []string
}

// Level of a heading.
type Level struct {
	Fixed   int
	Nesting bool
	Attr    *QName
}

// Display is a declared switch.
type Display struct {
	Attr   QName
	In     []string
	Reason gofastcore.Display
}

// Rule is a compiled rule.
type Rule struct {
	Index    int
	Selector *Selector
	As       As

	Kind        gofastcore.Kind // block
	PartKind    gofastcore.PartKind
	StoredKind  string
	Language    string
	Notation    string
	NoteType    string
	Style       gofastcore.Style
	Model       string // rows or cals
	Field       string
	Key         string
	Join        string
	AlsoHeading int
	DuplicateOf string
	OutOfLine   bool // a block or note met inside an open block is a block of its own
	// Split: met inside an open paragraph-like block, the element ends
	// that block, is read on its own, and text after it starts a new
	// block ("split"), as a table or image does.
	Split bool
	// Space: the element's text is set off by white space ("space",
	// inline): a space is held before and after it, where the source has
	// none, as between the fields of a name or a citation.
	Space bool

	Level   Level
	Ordered *Test
	Header  *Test
	Colspan *QName
	Rowspan *QName
	Display *Display

	Marker, Link, Title, ID, Alt, Name, Value, Anchor *Value
	Attrs, Mark, Extra                                []KV

	// Cells are the cells of a row made from its element's attributes
	// ("cells", row). HeaderText is a header row of fixed text ("header",
	// table).
	Cells      []*Value
	HeaderText []string

	// Reads lists the attributes of the element the rule reads.
	Reads []QName
	// Claims lists the value sources read from children.
	Claims []*Value
}

// Match is one entry of "match".
type Match struct {
	Root          QName
	DoctypePublic string
	Attr          []AttrTest
}

// Profile is a loaded profile.
type Profile struct {
	Name        string
	Title       string
	HeadingBase int
	Matches     []Match
	Rules       []*Rule

	byName   map[QName][]*Rule
	wild     []*Rule
	maxSteps int
	anchored bool
	kept     []QName
	tested   []QName
	ofReads  map[QName][]QName
	aliases  map[string]string
	// childAttrs: some value reads an attribute of a child.
	childAttrs bool
}

// ReadsChildAttrs reports whether a rule reads an attribute of a child
// element ({"attr": ..., "child": ...}).
func (p *Profile) ReadsChildAttrs() bool { return p.childAttrs }

// ElementNamespace returns the namespace an element of namespace uri is
// matched in: the target of "namespace_aliases", or uri itself.
func (p *Profile) ElementNamespace(uri string) string {
	if t, ok := p.aliases[uri]; ok {
		return t
	}
	return uri
}

// MaxSteps returns the most steps of any selector.
func (p *Profile) MaxSteps() int { return p.maxSteps }

// Anchored reports whether any selector is anchored at the root.
func (p *Profile) Anchored() bool { return p.anchored }

// KeptAttrs lists the attributes whose values a reader keeps on open
// elements: those that selectors test and those read from ancestors.
func (p *Profile) KeptAttrs() []QName { return p.kept }

// TestedAttrs lists every attribute a selector tests.
func (p *Profile) TestedAttrs() []QName { return p.tested }

// OfReads lists the attributes rules read from an ancestor of this name.
func (p *Profile) OfReads(name QName) []QName { return p.ofReads[name] }

// Lookup returns the rule for the element at index 0 of path, or nil.
func (p *Profile) Lookup(path Path) *Rule {
	cands := p.byName[path.Name(0)]
	i, j := 0, 0
	for i < len(cands) || j < len(p.wild) {
		var r *Rule
		if j >= len(p.wild) || i < len(cands) && before(cands[i], p.wild[j]) {
			r = cands[i]
			i++
		} else {
			r = p.wild[j]
			j++
		}
		if r.Selector.Match(path) {
			return r
		}
	}
	return nil
}

// before orders rules by precedence: more steps, more tests, later rule.
func before(a, b *Rule) bool {
	if x, y := len(a.Selector.Steps), len(b.Selector.Steps); x != y {
		return x > y
	}
	if x, y := a.Selector.Tests(), b.Selector.Tests(); x != y {
		return x > y
	}
	return a.Index > b.Index
}

// MatchesRoot reports whether the profile applies to a document whose
// root is root with attributes attrs (index 0 of path) and DOCTYPE public
// identifier doctypePublic.
func (p *Profile) MatchesRoot(path Path, doctypePublic string) bool {
	for _, m := range p.Matches {
		if path.Name(0) != m.Root {
			continue
		}
		if m.DoctypePublic != "" && !strings.HasPrefix(doctypePublic, m.DoctypePublic) {
			continue
		}
		st := Step{Name: m.Root, Tests: m.Attr}
		if st.matches(path, 0) {
			return true
		}
	}
	return false
}

// Error is a profile that does not load.
type Error struct {
	Rule int // 1-based, 0 for the profile as a whole
	Msg  string
}

func (e *Error) Error() string {
	if e.Rule > 0 {
		return fmt.Sprintf("profile: rule %d %s", e.Rule, e.Msg)
	}
	return "profile: " + e.Msg
}

var nameRE = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,63}$`)

type fileJSON struct {
	Profile          string            `json:"profile"`
	Title            string            `json:"title"`
	Format           int               `json:"format"`
	Namespaces       map[string]string `json:"namespaces"`
	DefaultNamespace string            `json:"default_namespace"`
	NamespaceAliases map[string]string `json:"namespace_aliases"`
	Match            []matchJSON       `json:"match"`
	HeadingBase      int               `json:"heading_base"`
	Rules            []json.RawMessage `json:"rules"`
}

type matchJSON struct {
	Root          string `json:"root"`
	Namespace     string `json:"namespace"`
	DoctypePublic string `json:"doctype_public"`
	Attr          string `json:"attr"`
}

func strict(data []byte, v any) error {
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil {
		return err
	}
	if d.More() {
		return errors.New("data after the JSON value")
	}
	return nil
}

// Load reads and validates a profile. builtin refuses "skip" rules.
func Load(r io.Reader, builtin bool) (*Profile, error) {
	data, err := io.ReadAll(io.LimitReader(r, MaxProfileBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > MaxProfileBytes {
		return nil, &Error{Msg: fmt.Sprintf("larger than %d bytes", MaxProfileBytes)}
	}
	var f fileJSON
	if err := strict(data, &f); err != nil {
		return nil, &Error{Msg: err.Error()}
	}
	if f.Format != 1 {
		return nil, &Error{Msg: fmt.Sprintf("\"format\" is %d; this library reads format 1", f.Format)}
	}
	if !nameRE.MatchString(f.Profile) || f.Profile == "none" {
		return nil, &Error{Msg: fmt.Sprintf("\"profile\" %q is not a name of lower-case letters, digits, '.', '_' and '-' (and not \"none\")", f.Profile)}
	}
	for prefix, uri := range f.Namespaces {
		if prefix == "" || prefix == "xml" || prefix == "xmlns" || strings.ContainsAny(prefix, ":/[]@ ") || uri == "" {
			return nil, &Error{Msg: fmt.Sprintf("namespace prefix %q (%q) cannot be declared", prefix, uri)}
		}
	}
	p := &Profile{Name: f.Profile, Title: f.Title, HeadingBase: f.HeadingBase,
		byName: map[QName][]*Rule{}, ofReads: map[QName][]QName{}}
	if len(f.NamespaceAliases) > 64 {
		return nil, &Error{Msg: "more than 64 \"namespace_aliases\""}
	}
	for from, to := range f.NamespaceAliases {
		if from == "" || from == to || from == "http://www.w3.org/XML/1998/namespace" {
			return nil, &Error{Msg: fmt.Sprintf("namespace alias %q -> %q cannot be declared", from, to)}
		}
		if _, chained := f.NamespaceAliases[to]; chained {
			return nil, &Error{Msg: fmt.Sprintf("namespace alias %q -> %q: the target is itself an alias", from, to)}
		}
		if p.aliases == nil {
			p.aliases = map[string]string{}
		}
		p.aliases[from] = to
	}
	if p.HeadingBase == 0 {
		p.HeadingBase = 1
	}
	if p.HeadingBase < 1 || p.HeadingBase > 6 {
		return nil, &Error{Msg: fmt.Sprintf("\"heading_base\" %d is not 1 to 6", f.HeadingBase)}
	}
	if len(f.Match) == 0 {
		return nil, &Error{Msg: "\"match\" is empty"}
	}
	for i, m := range f.Match {
		if m.Root == "" || strings.ContainsAny(m.Root, ":/[]@ ") {
			return nil, &Error{Msg: fmt.Sprintf("match %d: \"root\" %q is not a local name", i+1, m.Root)}
		}
		mm := Match{Root: QName{URI: m.Namespace, Local: m.Root}, DoctypePublic: m.DoctypePublic}
		if m.Attr != "" {
			st, rest, err := parseStep("*"+m.Attr, f.Namespaces, "")
			if err != nil || rest != "" {
				return nil, &Error{Msg: fmt.Sprintf("match %d: \"attr\" %q: %v", i+1, m.Attr, err)}
			}
			mm.Attr = st.Tests
		}
		p.Matches = append(p.Matches, mm)
	}
	if len(f.Rules) > MaxRules {
		return nil, &Error{Msg: fmt.Sprintf("more than %d rules", MaxRules)}
	}
	kept := map[QName]bool{}
	for i, raw := range f.Rules {
		r, err := compileRule(raw, i, &f, builtin)
		if err != nil {
			return nil, &Error{Rule: i + 1, Msg: err.Error()}
		}
		p.Rules = append(p.Rules, r)
		name, any := r.Selector.LastName()
		if any {
			p.wild = append(p.wild, r)
		} else {
			p.byName[name] = append(p.byName[name], r)
		}
		p.maxSteps = max(p.maxSteps, len(r.Selector.Steps))
		p.anchored = p.anchored || r.Selector.Anchored
		for k, st := range r.Selector.Steps {
			for _, t := range st.Tests {
				p.tested = appendNew(p.tested, t.Name)
				if k < len(r.Selector.Steps)-1 {
					kept[t.Name] = true
				}
			}
		}
		for _, v := range r.values() {
			if v.Of != nil {
				kept[v.Attr] = true
				p.ofReads[*v.Of] = appendNew(p.ofReads[*v.Of], v.Attr)
			}
			s := v.Claims()
			if s == nil && v.AttrChild != nil {
				s, p.childAttrs = v.AttrChild, true
			}
			if s != nil {
				p.maxSteps = max(p.maxSteps, len(s.Steps)+1)
				for _, st := range s.Steps[:len(s.Steps)-1] {
					for _, t := range st.Tests {
						kept[t.Name] = true
					}
				}
			}
		}
	}
	for q := range kept {
		p.kept = append(p.kept, q)
	}
	sort.Slice(p.kept, func(i, j int) bool { return p.kept[i].String() < p.kept[j].String() })
	order := func(rs []*Rule) { sort.SliceStable(rs, func(i, j int) bool { return before(rs[i], rs[j]) }) }
	for _, rs := range p.byName {
		order(rs)
	}
	order(p.wild)
	return p, nil
}

func appendNew(qs []QName, q QName) []QName {
	if slices.Contains(qs, q) {
		return qs
	}
	return append(qs, q)
}

func (r *Rule) values() []*Value {
	var vs []*Value
	for _, v := range []*Value{r.Marker, r.Link, r.Title, r.ID, r.Alt, r.Name, r.Value, r.Anchor} {
		if v != nil {
			vs = append(vs, v)
		}
	}
	for _, kvs := range [][]KV{r.Attrs, r.Mark, r.Extra} {
		for _, kv := range kvs {
			vs = append(vs, kv.Value)
		}
	}
	return append(vs, r.Cells...)
}

var blockKinds = map[string]gofastcore.Kind{
	"paragraph": gofastcore.KindParagraph, "quote": gofastcore.KindQuote, "code": gofastcore.KindCode,
	"caption": gofastcore.KindCaption, "footnote": gofastcore.KindFootnote, "formula": gofastcore.KindFormula,
}

var noteTypes = []string{gofastcore.NoteComment, gofastcore.NoteDeletion, gofastcore.NoteSpeaker, gofastcore.NoteField,
	gofastcore.NoteChoices, gofastcore.NoteAttribute, gofastcore.NoteHeaders, gofastcore.NoteMIMEPreamble, gofastcore.NoteMIMEEpilogue}

var styles = map[string]gofastcore.Style{
	"bold": gofastcore.StyleBold, "italic": gofastcore.StyleItalic, "superscript": gofastcore.StyleSuper,
	"subscript": gofastcore.StyleSub, "code": gofastcore.StyleCode,
}

var fields = []string{"title", "author", "created", "modified", "language", "keyword", "subject", "description", "extra"}

func compileRule(raw json.RawMessage, i int, f *fileJSON, builtin bool) (*Rule, error) {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("not a JSON object: %v", err)
	}
	str := func(key string) (string, error) {
		v, ok := m[key]
		if !ok {
			return "", nil
		}
		var s string
		if err := json.Unmarshal(v, &s); err != nil {
			return "", fmt.Errorf("%q must be a string", key)
		}
		return s, nil
	}
	sel, err := str("match")
	if err != nil {
		return nil, err
	}
	if sel == "" {
		return nil, fmt.Errorf("\"match\" is missing")
	}
	as, err := str("as")
	if err != nil {
		return nil, err
	}
	r := &Rule{Index: i}
	where := fmt.Sprintf("(%q)", sel)
	var ok bool
	if r.As, ok = asNames[as]; !ok {
		return nil, fmt.Errorf("%s: unknown \"as\" %q", where, as)
	}
	if r.Selector, err = ParseSelector(sel, f.Namespaces, f.DefaultNamespace); err != nil {
		return nil, fmt.Errorf("%s: %v", where, err)
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if !allKeys[k] {
			return nil, fmt.Errorf("%s: unknown key %q", where, k)
		}
		if !slices.Contains(allowed[r.As], k) {
			return nil, fmt.Errorf("%s: key %q is not allowed on a %q rule", where, k, as)
		}
	}
	if r.As == AsSkip && builtin {
		return nil, fmt.Errorf("%s: \"skip\" is refused in a built-in profile", where)
	}
	c := compiler{m: m, f: f, where: where}
	kind, err := str("kind")
	if err != nil {
		return nil, err
	}
	switch r.As {
	case AsBlock:
		if r.Kind, ok = blockKinds[kind]; !ok {
			return nil, fmt.Errorf("%s: \"kind\" %q is not one of paragraph, quote, code, caption, footnote, formula", where, kind)
		}
	case AsPart:
		switch kind {
		case "body", "":
			r.PartKind = gofastcore.PartBody
		case "chapter":
			r.PartKind = gofastcore.PartChapter
		default:
			return nil, fmt.Errorf("%s: part \"kind\" %q is not body or chapter", where, kind)
		}
	case AsStored:
		if !slices.Contains(gofastcore.StoredKindNames(), kind) {
			return nil, fmt.Errorf("%s: stored \"kind\" %q is not one of core's stored kinds", where, kind)
		}
		r.StoredKind = kind
	}
	if r.Language, err = str("language"); err != nil {
		return nil, err
	}
	if r.Language != "" && r.Kind != gofastcore.KindCode {
		return nil, fmt.Errorf("%s: \"language\" is for code blocks", where)
	}
	if r.Notation, err = str("notation"); err != nil {
		return nil, err
	}
	if r.Notation != "" {
		if r.Kind != gofastcore.KindFormula {
			return nil, fmt.Errorf("%s: \"notation\" is for formula blocks", where)
		}
		if !slices.Contains([]string{gofastcore.NotationLaTeX, gofastcore.NotationUnicodeMath, gofastcore.NotationPlain}, r.Notation) {
			return nil, fmt.Errorf("%s: \"notation\" %q is not latex, unicodemath or plain", where, r.Notation)
		}
	}
	if r.NoteType, err = str("type"); err != nil {
		return nil, err
	}
	if r.As == AsNote && !slices.Contains(noteTypes, r.NoteType) {
		return nil, fmt.Errorf("%s: note \"type\" %q is not one of core's note types", where, r.NoteType)
	}
	style, err := str("style")
	if err != nil {
		return nil, err
	}
	if style != "" {
		if r.Style, ok = styles[style]; !ok {
			return nil, fmt.Errorf("%s: \"style\" %q is not bold, italic, superscript, subscript or code", where, style)
		}
	}
	if r.Model, err = str("model"); err != nil {
		return nil, err
	}
	switch r.Model {
	case "":
		r.Model = "rows"
	case "rows", "cals":
	default:
		return nil, fmt.Errorf("%s: \"model\" %q is not rows or cals", where, r.Model)
	}
	if r.Field, err = str("field"); err != nil {
		return nil, err
	}
	if r.Key, err = str("key"); err != nil {
		return nil, err
	}
	if r.Join, err = str("join"); err != nil {
		return nil, err
	}
	if v, ok := m["space"]; ok {
		if err := json.Unmarshal(v, &r.Space); err != nil {
			return nil, fmt.Errorf("%s: \"space\" must be true or false", where)
		}
	}
	if r.As == AsMetadata {
		if !slices.Contains(fields, r.Field) {
			return nil, fmt.Errorf("%s: \"field\" %q is not one of %s", where, r.Field, strings.Join(fields, ", "))
		}
		if (r.Field == "extra") != (r.Key != "") {
			return nil, fmt.Errorf("%s: \"key\" goes with \"field\": \"extra\", and only with it", where)
		}
	}
	if r.DuplicateOf, err = str("duplicate_of"); err != nil {
		return nil, err
	}
	if r.DuplicateOf != "" && r.Key == "" {
		return nil, fmt.Errorf("%s: \"duplicate_of\" needs \"key\"", where)
	}
	if r.As != AsMetadata && r.Key != "" && r.DuplicateOf == "" {
		return nil, fmt.Errorf("%s: \"key\" without \"duplicate_of\"", where)
	}
	if r.Key != "" && !validKey(r.Key) {
		return nil, fmt.Errorf("%s: \"key\" %q is empty, too long or has white space", where, r.Key)
	}
	if v, ok := m["out_of_line"]; ok {
		if err := json.Unmarshal(v, &r.OutOfLine); err != nil {
			return nil, fmt.Errorf("%s: \"out_of_line\" must be true or false", where)
		}
	}
	if v, ok := m["split"]; ok {
		if err := json.Unmarshal(v, &r.Split); err != nil {
			return nil, fmt.Errorf("%s: \"split\" must be true or false", where)
		}
		if r.Split && r.OutOfLine {
			return nil, fmt.Errorf("%s: \"split\" and \"out_of_line\" are two ways to read one element; a rule has one", where)
		}
	}
	if v, ok := m["also_heading"]; ok {
		if err := json.Unmarshal(v, &r.AlsoHeading); err != nil || r.AlsoHeading < 1 || r.AlsoHeading > 6 {
			return nil, fmt.Errorf("%s: \"also_heading\" must be a level 1 to 6", where)
		}
		if r.Field != "title" {
			return nil, fmt.Errorf("%s: \"also_heading\" is for the title", where)
		}
	}
	if v, ok := m["level"]; ok {
		if err := c.level(v, &r.Level); err != nil {
			return nil, err
		}
	} else if r.As == AsHeading {
		return nil, fmt.Errorf("%s: a heading needs \"level\"", where)
	}
	if r.Ordered, err = c.test("ordered"); err != nil {
		return nil, err
	}
	if r.As == AsTable {
		if v, ok := m["header"]; ok {
			if err := json.Unmarshal(v, &r.HeaderText); err != nil || len(r.HeaderText) == 0 || len(r.HeaderText) > MaxCells {
				return nil, fmt.Errorf("%s: a table's \"header\" is a list of 1 to %d strings", where, MaxCells)
			}
			for _, h := range r.HeaderText {
				if len(h) > MaxHeaderBytes || !utf8.ValidString(h) {
					return nil, fmt.Errorf("%s: \"header\" text %q is longer than %d bytes or not UTF-8", where, h, MaxHeaderBytes)
				}
			}
		}
	} else if r.Header, err = c.test("header"); err != nil {
		return nil, err
	}
	if v, ok := m["cells"]; ok {
		var raws []json.RawMessage
		if err := json.Unmarshal(v, &raws); err != nil || len(raws) == 0 || len(raws) > MaxCells {
			return nil, fmt.Errorf("%s: \"cells\" is a list of 1 to %d value sources", where, MaxCells)
		}
		for _, raw := range raws {
			cv, err := c.value("cells", raw)
			if err != nil {
				return nil, err
			}
			if cv.IsLiteral || cv.Of != nil || cv.Claims() != nil || cv.AttrChild != nil {
				return nil, fmt.Errorf("%s: a cell of \"cells\" is an attribute of the row's element, {\"attr\": \"name\"}", where)
			}
			r.Cells = append(r.Cells, cv)
		}
	}
	for _, x := range []struct {
		key string
		dst **QName
	}{{"colspan", &r.Colspan}, {"rowspan", &r.Rowspan}} {
		s, err := str(x.key)
		if err != nil {
			return nil, err
		}
		if s != "" {
			q, err := ResolveName(s, f.Namespaces, "")
			if err != nil {
				return nil, fmt.Errorf("%s: %q: %v", where, x.key, err)
			}
			*x.dst = &q
		}
	}
	if v, ok := m["display"]; ok {
		var d struct {
			Attr   string   `json:"attr"`
			In     []string `json:"in"`
			Reason string   `json:"reason"`
		}
		if err := strict(v, &d); err != nil {
			return nil, fmt.Errorf("%s: \"display\": %v", where, err)
		}
		q, err := ResolveName(d.Attr, f.Namespaces, "")
		if err != nil || len(d.In) == 0 {
			return nil, fmt.Errorf("%s: \"display\" needs \"attr\" and \"in\"", where)
		}
		r.Display = &Display{Attr: q, In: d.In}
		switch d.Reason {
		case "hidden":
			r.Display.Reason = gofastcore.DisplayHidden
		case "no_ink":
			r.Display.Reason = gofastcore.DisplayNoInk
		default:
			return nil, fmt.Errorf("%s: \"display\" reason %q is not hidden or no_ink", where, d.Reason)
		}
	}
	for _, x := range []struct {
		key string
		dst **Value
	}{{"marker", &r.Marker}, {"link", &r.Link}, {"title", &r.Title}, {"id", &r.ID}, {"alt", &r.Alt},
		{"name", &r.Name}, {"value", &r.Value}, {"anchor", &r.Anchor}} {
		if v, ok := m[x.key]; ok {
			if *x.dst, err = c.value(x.key, v); err != nil {
				return nil, err
			}
		}
	}
	if r.Title != nil && r.Link == nil {
		return nil, fmt.Errorf("%s: \"title\" is the title of a link and needs \"link\"", where)
	}
	if r.Value != nil && r.As == AsNote && r.NoteType != gofastcore.NoteAttribute && r.Value.Attr.Local != "" {
		return nil, fmt.Errorf("%s: a note whose text is an attribute is of type attribute", where)
	}
	for _, x := range []struct {
		key string
		dst *[]KV
	}{{"attrs", &r.Attrs}, {"mark", &r.Mark}, {"extra", &r.Extra}} {
		if v, ok := m[x.key]; ok {
			if *x.dst, err = c.kvs(x.key, v); err != nil {
				return nil, err
			}
		}
	}
	r.reads()
	return r, nil
}

func validKey(k string) bool {
	return k != "" && len(k) <= 64 && !strings.ContainsAny(k, " \t\n\r")
}

// reads lists what the rule reads of its own element.
func (r *Rule) reads() {
	add := func(q QName) { r.Reads = appendNew(r.Reads, q) }
	for _, t := range r.Selector.Steps[len(r.Selector.Steps)-1].Tests {
		add(t.Name)
	}
	for _, v := range r.values() {
		if v.Attr.Local != "" && v.Of == nil && v.AttrChild == nil {
			add(v.Attr)
		}
		if v.Claims() != nil {
			r.Claims = append(r.Claims, v)
		}
	}
	for _, t := range []*Test{r.Ordered, r.Header} {
		if t != nil && t.Attr != nil {
			add(*t.Attr)
		}
	}
	for _, q := range []*QName{r.Colspan, r.Rowspan} {
		if q != nil {
			add(*q)
		}
	}
	if r.Display != nil {
		add(r.Display.Attr)
	}
	if r.Level.Attr != nil {
		add(*r.Level.Attr)
	}
}

type compiler struct {
	m     map[string]json.RawMessage
	f     *fileJSON
	where string
}

func (c *compiler) level(v json.RawMessage, l *Level) error {
	var n int
	if json.Unmarshal(v, &n) == nil {
		if n < 1 || n > 99 {
			return fmt.Errorf("%s: \"level\" %d is not 1 to 99", c.where, n)
		}
		l.Fixed = n
		return nil
	}
	var s string
	if json.Unmarshal(v, &s) == nil {
		if s != "nesting" {
			return fmt.Errorf("%s: \"level\" %q is not a number, \"nesting\" or an attribute", c.where, s)
		}
		l.Nesting = true
		return nil
	}
	var a struct {
		Attr string `json:"attr"`
	}
	if err := strict(v, &a); err != nil {
		return fmt.Errorf("%s: \"level\": %v", c.where, err)
	}
	q, err := ResolveName(a.Attr, c.f.Namespaces, "")
	if err != nil {
		return fmt.Errorf("%s: \"level\": %v", c.where, err)
	}
	l.Attr = &q
	return nil
}

func (c *compiler) test(key string) (*Test, error) {
	v, ok := c.m[key]
	if !ok {
		return nil, nil
	}
	var b bool
	if json.Unmarshal(v, &b) == nil {
		return &Test{Const: b}, nil
	}
	var a struct {
		Attr string   `json:"attr"`
		In   []string `json:"in"`
	}
	if err := strict(v, &a); err != nil {
		return nil, fmt.Errorf("%s: %q must be true, false or {\"attr\", \"in\"}: %v", c.where, key, err)
	}
	q, err := ResolveName(a.Attr, c.f.Namespaces, "")
	if err != nil || len(a.In) == 0 {
		return nil, fmt.Errorf("%s: %q needs \"attr\" and \"in\"", c.where, key)
	}
	return &Test{Attr: &q, In: a.In}, nil
}

func (c *compiler) value(key string, v json.RawMessage) (*Value, error) {
	var s string
	if json.Unmarshal(v, &s) == nil {
		return &Value{Literal: s, IsLiteral: true}, nil
	}
	var o struct {
		Attr      string `json:"attr"`
		Of        string `json:"of"`
		Child     string `json:"child"`
		Structure string `json:"structure"`
	}
	if err := strict(v, &o); err != nil {
		return nil, fmt.Errorf("%s: %q is not a value source: %v", c.where, key, err)
	}
	n := 0
	for _, s := range []string{o.Attr, o.Child, o.Structure} {
		if s != "" {
			n++
		}
	}
	val := &Value{}
	var err error
	if o.Attr != "" && o.Child != "" && o.Of == "" && o.Structure == "" {
		// The attribute of a child.
		if val.Attr, err = ResolveName(o.Attr, c.f.Namespaces, ""); err != nil {
			return nil, fmt.Errorf("%s: %q: %v", c.where, key, err)
		}
		if val.AttrChild, err = c.relative(key, o.Child); err != nil {
			return nil, err
		}
		return val, nil
	}
	if n != 1 || o.Of != "" && o.Attr == "" {
		return nil, fmt.Errorf("%s: %q needs exactly one of \"attr\" (with \"of\" or \"child\" optionally), \"child\", \"structure\"", c.where, key)
	}
	switch {
	case o.Attr != "":
		if val.Attr, err = ResolveName(o.Attr, c.f.Namespaces, ""); err != nil {
			return nil, fmt.Errorf("%s: %q: %v", c.where, key, err)
		}
		if o.Of != "" {
			q, err := ResolveName(o.Of, c.f.Namespaces, c.f.DefaultNamespace)
			if err != nil {
				return nil, fmt.Errorf("%s: %q: \"of\": %v", c.where, key, err)
			}
			val.Of = &q
		}
	case o.Child != "":
		if val.Child, err = c.relative(key, o.Child); err != nil {
			return nil, err
		}
	default:
		if val.Structure, err = c.relative(key, o.Structure); err != nil {
			return nil, err
		}
	}
	return val, nil
}

func (c *compiler) relative(key, s string) (*Selector, error) {
	sel, err := ParseSelector(s, c.f.Namespaces, c.f.DefaultNamespace)
	if err != nil {
		return nil, fmt.Errorf("%s: %q: %v", c.where, key, err)
	}
	if sel.Anchored {
		return nil, fmt.Errorf("%s: %q: a child selector cannot start with /", c.where, key)
	}
	return sel, nil
}

func (c *compiler) kvs(key string, v json.RawMessage) ([]KV, error) {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(v, &m); err != nil {
		return nil, fmt.Errorf("%s: %q must be an object of keys and value sources", c.where, key)
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var out []KV
	for _, k := range keys {
		if !validKey(k) {
			return nil, fmt.Errorf("%s: %q: key %q is empty, too long or has white space", c.where, key, k)
		}
		if key != "extra" && (k == gofastcore.AttrStored || k == gofastcore.AttrNoteType || k == gofastcore.AttrRole) {
			return nil, fmt.Errorf("%s: %q: key %q is core's, with a closed set of values", c.where, key, k)
		}
		val, err := c.value(key+"."+k, m[k])
		if err != nil {
			return nil, err
		}
		out = append(out, KV{Key: k, Value: val})
	}
	return out, nil
}
