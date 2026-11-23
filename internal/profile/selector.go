package profile

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// QName is a namespace URI and a local name.
type QName struct {
	URI, Local string
}

func (q QName) String() string {
	if q.URI == "" {
		return q.Local
	}
	return "{" + q.URI + "}" + q.Local
}

// Test operators of an attribute test.
const (
	OpExists = iota // [@a]
	OpEquals        // [@a='v']
	OpToken         // [@a~='t']
)

// AttrTest is one attribute test of a step.
type AttrTest struct {
	Name  QName
	Op    int
	Value string
}

// Step is one step of a selector: a name, or any name, with attribute
// tests.
type Step struct {
	Name  QName
	Any   bool
	Tests []AttrTest
}

// Selector is a compiled selector. Steps go from the outermost element to
// the element matched.
type Selector struct {
	Source   string
	Anchored bool
	Steps    []Step
	tests    int
}

// Path is the element being matched and its ancestors: index 0 is the
// element, 1 its parent, and so on. Attr of an ancestor need answer only
// for the names KeptAttrs lists.
type Path interface {
	Len() int
	Name(i int) QName
	Attr(i int, name QName) (string, bool)
}

// TokenPath is a Path that answers token tests itself, splitting each
// value once per element rather than once per test.
type TokenPath interface {
	Path
	HasToken(i int, name QName, token string) bool
}

// MaxSelectorBytes bounds a selector's text, MaxSteps its steps.
const (
	MaxSelectorBytes = 4096
	MaxSteps         = 64
)

// ParseSelector compiles a selector. Prefixes are resolved with ns; an
// unprefixed element name is in defaultNS, an unprefixed attribute name in
// no namespace.
func ParseSelector(s string, ns map[string]string, defaultNS string) (*Selector, error) {
	if len(s) > MaxSelectorBytes {
		return nil, fmt.Errorf("selector longer than %d bytes", MaxSelectorBytes)
	}
	if !utf8.ValidString(s) {
		return nil, fmt.Errorf("selector is not valid UTF-8")
	}
	sel := &Selector{Source: s}
	p := s
	if strings.HasPrefix(p, "/") {
		sel.Anchored = true
		p = p[1:]
	}
	if p == "" {
		return nil, fmt.Errorf("empty selector")
	}
	for {
		st, rest, err := parseStep(p, ns, defaultNS)
		if err != nil {
			return nil, fmt.Errorf("selector %q: %v", s, err)
		}
		sel.Steps = append(sel.Steps, st)
		sel.tests += len(st.Tests)
		if len(sel.Steps) > MaxSteps {
			return nil, fmt.Errorf("selector %q has more than %d steps", s, MaxSteps)
		}
		if rest == "" {
			break
		}
		if rest[0] != '/' || len(rest) == 1 {
			return nil, fmt.Errorf("selector %q: unexpected %q", s, rest)
		}
		p = rest[1:]
	}
	return sel, nil
}

func isNameByte(c byte) bool {
	switch c {
	case '/', '[', ']', '@', '=', '\'', '"', '~', '*', ' ', '\t', '\n', '\r', '(', ')', ',', '|':
		return false
	}
	return c > ' '
}

func scanName(p string) (string, string) {
	i := 0
	for i < len(p) && isNameByte(p[i]) {
		i++
	}
	return p[:i], p[i:]
}

// ResolveName resolves a prefixed or unprefixed name.
func ResolveName(name string, ns map[string]string, defaultNS string) (QName, error) {
	if name == "" {
		return QName{}, fmt.Errorf("missing name")
	}
	for _, part := range strings.Split(name, ":") {
		if part == "" || part[0] == '.' || part[0] == '-' || part[0] >= '0' && part[0] <= '9' {
			return QName{}, fmt.Errorf("bad name %q", name)
		}
	}
	prefix, local, ok := strings.Cut(name, ":")
	if !ok {
		return QName{URI: defaultNS, Local: name}, nil
	}
	if prefix == "" || local == "" || strings.Contains(local, ":") {
		return QName{}, fmt.Errorf("bad name %q", name)
	}
	if prefix == "xml" {
		return QName{URI: "http://www.w3.org/XML/1998/namespace", Local: local}, nil
	}
	uri, ok := ns[prefix]
	if !ok {
		return QName{}, fmt.Errorf("prefix %q is not declared in \"namespaces\"", prefix)
	}
	return QName{URI: uri, Local: local}, nil
}

func parseStep(p string, ns map[string]string, defaultNS string) (Step, string, error) {
	var st Step
	if strings.HasPrefix(p, "*") {
		st.Any = true
		p = p[1:]
	} else {
		name, rest := scanName(p)
		q, err := ResolveName(name, ns, defaultNS)
		if err != nil {
			return st, "", err
		}
		st.Name, p = q, rest
	}
	for strings.HasPrefix(p, "[@") {
		p = p[2:]
		name, rest := scanName(p)
		q, err := ResolveName(name, ns, "")
		if err != nil {
			return st, "", fmt.Errorf("attribute test: %v", err)
		}
		t := AttrTest{Name: q}
		p = rest
		switch {
		case strings.HasPrefix(p, "]"):
			t.Op = OpExists
			p = p[1:]
		case strings.HasPrefix(p, "="), strings.HasPrefix(p, "~="):
			t.Op = OpEquals
			if p[0] == '~' {
				t.Op = OpToken
				p = p[1:]
			}
			p = p[1:]
			if p == "" || (p[0] != '\'' && p[0] != '"') {
				return st, "", fmt.Errorf("attribute test: value must be quoted")
			}
			q := p[0]
			end := strings.IndexByte(p[1:], q)
			if end < 0 {
				return st, "", fmt.Errorf("attribute test: unterminated value")
			}
			t.Value = p[1 : 1+end]
			p = p[2+end:]
			if !strings.HasPrefix(p, "]") {
				return st, "", fmt.Errorf("attribute test: missing ]")
			}
			p = p[1:]
			if t.Op == OpToken && (t.Value == "" || strings.ContainsAny(t.Value, " \t\n\r")) {
				return st, "", fmt.Errorf("attribute test: ~= takes one token")
			}
		default:
			return st, "", fmt.Errorf("attribute test: expected ], = or ~=")
		}
		st.Tests = append(st.Tests, t)
	}
	if p != "" && p[0] != '/' {
		return st, "", fmt.Errorf("unexpected %q", p)
	}
	return st, p, nil
}

// LastName returns the name of the last step, and whether it is "*".
func (s *Selector) LastName() (QName, bool) {
	st := &s.Steps[len(s.Steps)-1]
	return st.Name, st.Any
}

// Tests returns the number of attribute tests of all steps.
func (s *Selector) Tests() int { return s.tests }

// Match reports whether the selector matches the element at index 0 of p.
func (s *Selector) Match(p Path) bool {
	n := len(s.Steps)
	if p.Len() < n || s.Anchored && p.Len() != n {
		return false
	}
	for k := range n {
		if !s.Steps[n-1-k].matches(p, k) {
			return false
		}
	}
	return true
}

// MatchFrom reports whether the selector, read relative to the element at
// index base of p, matches the element at index 0: the path from below
// base to 0 has exactly the selector's steps.
func (s *Selector) MatchFrom(p Path, base int) bool {
	if base != len(s.Steps) {
		return false
	}
	for k := range base {
		if !s.Steps[base-1-k].matches(p, k) {
			return false
		}
	}
	return true
}

func (st *Step) matches(p Path, i int) bool {
	if !st.Any && p.Name(i) != st.Name {
		return false
	}
	for _, t := range st.Tests {
		v, ok := p.Attr(i, t.Name)
		if !ok {
			return false
		}
		switch t.Op {
		case OpEquals:
			if v != t.Value {
				return false
			}
		case OpToken:
			if tp, ok := p.(TokenPath); ok {
				if !tp.HasToken(i, t.Name, t.Value) {
					return false
				}
			} else if !HasToken(v, t.Value) {
				return false
			}
		}
	}
	return true
}

// HasToken reports whether t is one of the white-space separated tokens
// of v, without allocating.
func HasToken(v, t string) bool {
	for v != "" {
		i := strings.IndexFunc(v, isNotSpace)
		if i < 0 {
			return false
		}
		v = v[i:]
		j := strings.IndexFunc(v, unicode.IsSpace)
		if j < 0 {
			j = len(v)
		}
		if v[:j] == t {
			return true
		}
		v = v[j:]
	}
	return false
}

func isNotSpace(r rune) bool { return !unicode.IsSpace(r) }

// Tokens returns the set of white-space separated tokens of v.
func Tokens(v string) map[string]bool {
	set := map[string]bool{}
	for _, f := range strings.Fields(v) {
		set[f] = true
	}
	return set
}
