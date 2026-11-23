// Package dtd reads a document type declaration for what an XML reader
// that loads nothing needs from it: the external subset's identifiers and
// the general entities the internal subset declares. Element, attribute
// list and notation declarations are passed over. Parameter entities are
// recorded and never expanded; nothing is loaded.
package dtd

import (
	"bytes"
	"unicode/utf8"
)

// Doctype is what was read.
type Doctype struct {
	// Name is the root element name the declaration gives.
	Name string
	// PublicID and SystemID identify the external subset; HasExternal
	// reports that there is one.
	PublicID, SystemID string
	HasExternal        bool
	// Entities are the general entities, by name. The first declaration
	// of a name is the one that counts.
	Entities map[string]*Entity
	// ParamRefs counts references to parameter entities in the internal
	// subset; ExternalParamRefs those to parameter entities declared
	// external, which a validating processor would load.
	ParamRefs, ExternalParamRefs int
	// Damaged counts places where the declaration was not well formed and
	// was read past.
	Damaged int
}

// Entity is a general entity.
type Entity struct {
	// Value is the replacement text of an internal entity: character
	// references expanded, line ends normalized, other references kept.
	Value []byte
	// External reports an entity declared with SYSTEM or PUBLIC; Unparsed
	// one with NDATA as well.
	External, Unparsed bool
	SystemID           string
}

// Parse reads body, the text of a declaration between "<!DOCTYPE" and the
// final ">".
func Parse(body []byte) *Doctype {
	p := &parser{b: body, d: &Doctype{Entities: map[string]*Entity{}}, params: map[string]bool{}}
	p.doctype()
	return p.d
}

type parser struct {
	b      []byte
	i      int
	d      *Doctype
	params map[string]bool // parameter entity name -> declared external
}

func isSpace(c byte) bool { return c == ' ' || c == '\t' || c == '\r' || c == '\n' }

func (p *parser) space() bool {
	start := p.i
	for p.i < len(p.b) && isSpace(p.b[p.i]) {
		p.i++
	}
	return p.i > start
}

func (p *parser) name() []byte {
	start := p.i
	for p.i < len(p.b) {
		c := p.b[p.i]
		if isSpace(c) || c == '[' || c == '>' || c == '"' || c == '\'' || c == ';' || c == '%' || c == '<' {
			break
		}
		p.i++
	}
	return p.b[start:p.i]
}

func (p *parser) literal() ([]byte, bool) {
	if p.i >= len(p.b) || p.b[p.i] != '"' && p.b[p.i] != '\'' {
		return nil, false
	}
	q := p.b[p.i]
	j := bytes.IndexByte(p.b[p.i+1:], q)
	if j < 0 {
		p.i = len(p.b)
		p.d.Damaged++
		return nil, false
	}
	v := p.b[p.i+1 : p.i+1+j]
	p.i += j + 2
	return v, true
}

func (p *parser) keyword(k string) bool {
	if bytes.HasPrefix(p.b[p.i:], []byte(k)) {
		p.i += len(k)
		return true
	}
	return false
}

// externalID reads SYSTEM "s" or PUBLIC "p" "s", if there.
func (p *parser) externalID() (public, system string, ok bool) {
	switch {
	case p.keyword("SYSTEM"):
		p.space()
		s, _ := p.literal()
		return "", string(s), true
	case p.keyword("PUBLIC"):
		p.space()
		pub, _ := p.literal()
		p.space()
		s, _ := p.literal()
		return string(pub), string(s), true
	}
	return "", "", false
}

func (p *parser) doctype() {
	p.space()
	p.d.Name = string(p.name())
	p.space()
	if pub, sys, ok := p.externalID(); ok {
		p.d.PublicID, p.d.SystemID, p.d.HasExternal = pub, sys, true
		p.space()
	}
	if p.i < len(p.b) && p.b[p.i] == '[' {
		p.i++
		p.subset()
	}
}

// subset reads the internal subset up to its closing ']'.
func (p *parser) subset() {
	for p.i < len(p.b) {
		p.space()
		if p.i >= len(p.b) {
			return
		}
		rest := p.b[p.i:]
		switch {
		case rest[0] == ']':
			p.i++
			return
		case bytes.HasPrefix(rest, []byte("<!--")):
			p.skipTo("-->", 4)
		case bytes.HasPrefix(rest, []byte("<?")):
			p.skipTo("?>", 2)
		case bytes.HasPrefix(rest, []byte("<!ENTITY")):
			p.i += len("<!ENTITY")
			p.entity()
		case bytes.HasPrefix(rest, []byte("<!")):
			p.skipDecl()
		case rest[0] == '%':
			p.i++
			n := p.name()
			if p.i < len(p.b) && p.b[p.i] == ';' {
				p.i++
			}
			p.d.ParamRefs++
			if p.params[string(n)] {
				p.d.ExternalParamRefs++
			}
		default:
			p.d.Damaged++
			p.i++
		}
	}
}

func (p *parser) skipTo(end string, from int) {
	j := bytes.Index(p.b[p.i+from:], []byte(end))
	if j < 0 {
		p.d.Damaged++
		p.i = len(p.b)
		return
	}
	p.i += from + j + len(end)
}

// skipDecl passes over a markup declaration, whose quoted strings may hold
// '>'.
func (p *parser) skipDecl() {
	for p.i < len(p.b) {
		switch p.b[p.i] {
		case '"', '\'':
			p.literal()
			continue
		case '>':
			p.i++
			return
		}
		p.i++
	}
	p.d.Damaged++
}

func (p *parser) entity() {
	if !p.space() {
		p.d.Damaged++
	}
	param := false
	if p.i < len(p.b) && p.b[p.i] == '%' {
		param = true
		p.i++
		p.space()
	}
	name := string(p.name())
	p.space()
	e := &Entity{}
	if v, ok := p.literal(); ok {
		e.Value = replacement(v)
	} else if _, sys, ok := p.externalID(); ok {
		e.External, e.SystemID = true, sys
		p.space()
		if p.keyword("NDATA") {
			e.Unparsed = true
			p.space()
			p.name()
		}
	} else {
		p.d.Damaged++
	}
	p.skipDecl()
	if name == "" {
		p.d.Damaged++
		return
	}
	if param {
		if _, seen := p.params[name]; !seen {
			p.params[name] = e.External
		}
		return
	}
	if _, seen := p.d.Entities[name]; !seen {
		p.d.Entities[name] = e
	}
}

// replacement makes the replacement text of an entity value literal:
// character references are expanded and line ends normalized; entity and
// parameter entity references are kept.
func replacement(v []byte) []byte {
	if bytes.IndexByte(v, '&') < 0 && bytes.IndexByte(v, '\r') < 0 {
		return bytes.Clone(v)
	}
	out := make([]byte, 0, len(v))
	for i := 0; i < len(v); i++ {
		c := v[i]
		switch {
		case c == '\r':
			out = append(out, '\n')
			if i+1 < len(v) && v[i+1] == '\n' {
				i++
			}
		case c == '&' && i+1 < len(v) && v[i+1] == '#':
			r, n := charRef(v[i:])
			if n == 0 {
				out = append(out, c)
				continue
			}
			out = utf8.AppendRune(out, r)
			i += n - 1
		default:
			out = append(out, c)
		}
	}
	return out
}

// charRef decodes the character reference at the start of b, returning
// the character and its length, or a length of 0.
func charRef(b []byte) (rune, int) {
	i, base := 2, rune(10)
	if i < len(b) && (b[i] == 'x' || b[i] == 'X') {
		base, i = 16, 3
	}
	first := i
	var r rune
	for ; i < len(b); i++ {
		c := b[i]
		var d rune
		switch {
		case c >= '0' && c <= '9':
			d = rune(c - '0')
		case base == 16 && c >= 'a' && c <= 'f':
			d = rune(c-'a') + 10
		case base == 16 && c >= 'A' && c <= 'F':
			d = rune(c-'A') + 10
		default:
			goto done
		}
		if r = r*base + d; r > utf8.MaxRune {
			r = utf8.MaxRune + 1
		}
	}
done:
	if i == first || i >= len(b) || b[i] != ';' {
		return 0, 0
	}
	if r > utf8.MaxRune || r >= 0xD800 && r <= 0xDFFF {
		r = utf8.RuneError
	}
	return r, i + 1
}
