package engine

import (
	"bytes"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/AOShei/go-fast-core"
	"github.com/AOShei/go-fast-xml/internal/generic"
	"github.com/AOShei/go-fast-xml/internal/profile"
	"github.com/AOShei/go-fast-xml/internal/xmlread"
)

// Keys the engine writes for embedded documents ("document" rules).
const (
	// AttrChildren is the indexes in Document.Children of the documents
	// that came from a section, comma separated, on its heading (else
	// its first block).
	AttrChildren = "children"
	// AttrMediaType is the media type of a document rule's text kept in
	// the body (the child was not extracted, or not handed over).
	AttrMediaType = "media_type"
)

// MaxChildNameBytes bounds the anchor written into a child's name.
const MaxChildNameBytes = 256

// ChildLoc is where a child came from: its element in the file (start
// tag to end tag), the anchor of the nearest enclosing identified element
// and the element's path.
type ChildLoc struct {
	Offset, Length int64
	Anchor         string
	Element        string
	name           string // the name the child was given
}

// docCap is the element of a "document" rule being read: its content is
// gathered to be handed over, while its sink gathers the fallback text.
type docCap struct {
	media string
	sink  *sink
	from  int64 // the end of its start tag
	raw   bool  // XHTML read from the file's bytes at the end
	buf   []byte
	open  bool // a descendant's start tag awaits its '>'
	elems bool // it has descendant elements
	cut   bool
}

// hasDocuments reports whether the profile has a "document" rule.
func hasDocuments(p *profile.Profile) bool {
	for _, r := range p.Rules {
		if r.As == profile.AsDocument {
			return true
		}
	}
	return false
}

// startDocument starts the element of a "document" rule.
func (e *Engine) startDocument(f *frame, el *xmlread.Element) {
	media, _ := f.rule.Media(e)
	e.common(f, el)
	e.openSink(f, skBlock, el.Start)
	f.isDoc = true
	e.doc = &docCap{media: media, sink: f.sink, from: el.End,
		raw: media == profile.MediaXHTML && e.opts.Raw != nil}
}

// put appends b to what is handed over, up to the bound.
func (d *docCap) put(b []byte) {
	if d.cut {
		return
	}
	if len(d.buf)+len(b) > generic.MaxBlockBytes {
		b = b[:cutAt(b, generic.MaxBlockBytes-len(d.buf))]
		d.cut = true
	}
	d.buf = append(d.buf, b...)
}

// cutAt returns n or less, so that b[:n] ends at a character boundary.
func cutAt(b []byte, n int) int {
	if n >= len(b) {
		return len(b)
	}
	for n > 0 && !utf8.RuneStart(b[n]) {
		n--
	}
	return n
}

func (d *docCap) closeTag() {
	if d.open {
		d.open = false
		d.put([]byte{'>'})
	}
}

// docStart is a descendant of a document rule's element starting: its
// tag is written to what is handed over (and, for HTML, to the fallback
// text: the markup as written).
func (e *Engine) docStart(el *xmlread.Element) {
	d := e.doc
	d.elems = true
	if d.media == profile.MediaXHTML && blockLevel(el.Local) {
		d.sink.b.Space()
	}
	if d.raw {
		return
	}
	d.closeTag()
	t := e.scratchTag[:0]
	t = append(t, '<')
	t = append(t, el.Name...)
	for _, a := range el.Attrs {
		t = append(t, ' ')
		t = append(t, a.Name...)
		t = append(t, '=', '"')
		t = escape(t, a.Value, true)
		t = append(t, '"')
	}
	e.scratchTag = t
	d.put(t)
	d.open = true
	if d.media == profile.MediaHTML {
		e.docSinkText(d, t)
	}
}

// docEnd is a descendant of a document rule's element ending.
func (e *Engine) docEnd(el *xmlread.Element) {
	d := e.doc
	if d.media == profile.MediaXHTML && blockLevel(el.Local) {
		d.sink.b.Space()
	}
	if d.raw {
		return
	}
	var t []byte
	if d.open {
		d.open = false
		t = []byte("/>")
	} else {
		t = append(append(append(t, "</"...), el.Name...), '>')
	}
	d.put(t)
	if d.media == profile.MediaHTML {
		e.docSinkText(d, t)
	}
}

// docText is character data inside a document rule's element.
func (e *Engine) docText(t []byte) {
	d := e.doc
	if d.raw {
		return
	}
	d.closeTag()
	if d.media == profile.MediaXHTML {
		d.put(escape(nil, string(t), false))
		return
	}
	d.put(t)
}

// docSinkText writes markup into the fallback text, as text.
func (e *Engine) docSinkText(d *docCap, t []byte) {
	s := d.sink
	if s.cut {
		return
	}
	if s.b.Size()+len(t) > generic.MaxBlockBytes {
		t = t[:cutAt(t, generic.MaxBlockBytes-s.b.Size())]
		s.cut, d.cut = true, true
	}
	s.b.Append(t, false)
}

// escape appends v to dst with the characters markup gives meaning to
// written as references.
func escape(dst []byte, v string, attr bool) []byte {
	for i := 0; i < len(v); i++ {
		switch c := v[i]; {
		case c == '<':
			dst = append(dst, "&lt;"...)
		case c == '>':
			dst = append(dst, "&gt;"...)
		case c == '&':
			dst = append(dst, "&amp;"...)
		case c == '"' && attr:
			dst = append(dst, "&quot;"...)
		default:
			dst = append(dst, c)
		}
	}
	return dst
}

var blockNames = map[string]bool{"address": true, "article": true, "aside": true, "blockquote": true, "br": true,
	"dd": true, "div": true, "dl": true, "dt": true, "figcaption": true, "figure": true, "footer": true,
	"h1": true, "h2": true, "h3": true, "h4": true, "h5": true, "h6": true, "header": true, "hr": true,
	"li": true, "ol": true, "p": true, "pre": true, "section": true, "table": true, "td": true, "th": true,
	"tr": true, "ul": true}

// blockLevel reports whether an XHTML element sets its text apart: in
// the fallback text a space stands between it and what is around it.
func blockLevel(local string) bool { return blockNames[local] }

// HasMarkup reports whether text holds a tag (`<` followed by a letter,
// `/` or `!`, and later a `>`) or a character or entity reference (`&`
// name `;`, `&#` digits `;`, `&#x` hex digits `;`).
func HasMarkup(t []byte) bool {
	for i := 0; i < len(t); i++ {
		switch t[i] {
		case '<':
			if i+1 < len(t) && (isLetter(t[i+1]) || t[i+1] == '/' || t[i+1] == '!') && bytes.IndexByte(t[i+2:], '>') >= 0 {
				return true
			}
		case '&':
			if reference(t[i+1:]) {
				return true
			}
		}
	}
	return false
}

func isLetter(c byte) bool { return c|0x20 >= 'a' && c|0x20 <= 'z' }

func isDigit(c byte) bool { return c >= '0' && c <= '9' }

func isHex(c byte) bool { return isDigit(c) || c|0x20 >= 'a' && c|0x20 <= 'f' }

// reference reports whether t (after a `&`) starts with name `;`,
// `#` digits `;` or `#x` hex digits `;`.
func reference(t []byte) bool {
	n := 0
	switch {
	case len(t) > 1 && t[0] == '#' && (t[1] == 'x' || t[1] == 'X'):
		for n = 2; n < len(t) && n < 10 && isHex(t[n]); n++ {
		}
		return n > 2 && n < len(t) && t[n] == ';'
	case len(t) > 0 && t[0] == '#':
		for n = 1; n < len(t) && n < 10 && isDigit(t[n]); n++ {
		}
		return n > 1 && n < len(t) && t[n] == ';'
	}
	for n < len(t) && n < 40 && (isLetter(t[n]) || n > 0 && isDigit(t[n])) {
		n++
	}
	return n > 0 && n < len(t) && t[n] == ';'
}

// endDocument ends the element of a "document" rule: its content is
// handed over, or kept as a paragraph.
func (e *Engine) endDocument(f *frame, cstart, end int64) blockRef {
	d := e.doc
	e.doc = nil
	s := e.closeSink(f)
	defer e.putSink(s)
	d.closeTag()
	data := d.buf
	d.buf = nil
	if d.raw {
		n := cstart - d.from
		if n > generic.MaxBlockBytes {
			n, d.cut = generic.MaxBlockBytes, true
		}
		if n > 0 {
			data = e.opts.Raw(d.from, d.from+n)
			if d.cut {
				// Not past the last whole character.
				i := len(data) - 1
				for i > 0 && i > len(data)-4 && !utf8.RuneStart(data[i]) {
					i--
				}
				if i >= 0 && !utf8.FullRune(data[i:]) {
					data = data[:i]
				}
			}
		}
	}
	if d.cut {
		e.DocsCut++
	}
	paragraph := func(media string) blockRef {
		ref := e.finishSink(s, f, gofastcore.KindParagraph, gofastcore.OriginNative, end)
		if media != "" {
			e.patch(ref, AttrMediaType, media)
		}
		return ref
	}
	if len(bytes.TrimSpace(data)) == 0 || f.rule.WhenMarkup && !d.elems && !HasMarkup(data) {
		if f.rule.WhenMarkup && len(bytes.TrimSpace(data)) > 0 {
			e.Plain++
		}
		return paragraph("")
	}
	if e.opts.Child == nil || e.opts.PartWanted != nil && !e.opts.PartWanted(e.out.Part.Number) {
		return paragraph(d.media)
	}
	if len(e.Children) >= e.opts.MaxChildren {
		e.ChildrenCut++
		return paragraph(d.media)
	}
	loc := ChildLoc{Offset: f.start, Length: max(end-f.start, 0), Element: e.pathTo(f)}
	depth := e.depthOf(f)
	for i := len(e.anchors) - 1; i >= 0; i-- {
		if e.anchors[i].depth < depth {
			loc.Anchor = e.anchors[i].v
			break
		}
	}
	loc.name = e.childName(f, loc.Anchor)
	child, ok := e.opts.Child(loc.name, d.media+"; charset=utf-8", data)
	idx := len(e.Children)
	e.Children = append(e.Children, child)
	e.ChildLocs = append(e.ChildLocs, loc)
	e.linkChild(idx)
	if ok {
		e.keepAfter(s, f)
		return blockRef{}
	}
	return paragraph(d.media)
}

// childName is the element's name and its parent's, and the anchor:
// item/description#guid.
func (e *Engine) childName(f *frame, anchor string) string {
	i := e.depthOf(f) - 1
	name := f.qname
	if i > 0 {
		name = e.frames[i-1].qname + "/" + name
	}
	if len(name) > 2*generic.MaxNameBytes {
		name = name[:cutAt([]byte(name), 2*generic.MaxNameBytes)]
	}
	if anchor != "" {
		if len(anchor) > MaxChildNameBytes {
			anchor = anchor[:cutAt([]byte(anchor), MaxChildNameBytes)]
		}
		name += "#" + anchor
	}
	return e.clean(name)
}

// linkChild records a child in the innermost section it stands in.
func (e *Engine) linkChild(idx int) {
	for i := len(e.frames) - 1; i >= 0; i-- {
		if c := &e.frames[i]; c.rule != nil && c.rule.As == profile.AsSection {
			c.kids = append(c.kids, idx)
			return
		}
	}
}

// noteBlock records the first block and the first heading of the open
// sections, for their "children".
func (e *Engine) noteBlock(ref blockRef, heading bool) {
	inner := true
	for i := len(e.frames) - 1; i >= 0; i-- {
		c := &e.frames[i]
		if c.rule == nil || c.rule.As != profile.AsSection {
			continue
		}
		if !c.secFirst.ok() {
			c.secFirst = ref
		}
		if inner && heading && !c.secHead.ok() {
			c.secHead = ref
		}
		inner = false
	}
}

// endSection writes a section's "children" on its heading, else its
// first block.
func (e *Engine) endSection(f *frame) {
	if len(f.kids) == 0 {
		return
	}
	if a := f.anchor; a != "" {
		// An identifier that came after the child (an RSS guid after the
		// description) is its parent anchor too, and is in its name.
		for _, k := range f.kids {
			l := &e.ChildLocs[k]
			if l.Anchor != "" {
				continue
			}
			l.Anchor = a
			if c := e.Children[k]; c.Source.Name == l.name {
				if len(a) > MaxChildNameBytes {
					a = a[:cutAt([]byte(a), MaxChildNameBytes)]
				}
				c.Source.Name = e.clean(l.name + "#" + a)
			}
		}
	}
	ref := f.secHead
	if !ref.ok() {
		ref = f.secFirst
	}
	var b strings.Builder
	for i, k := range f.kids {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(strconv.Itoa(k))
	}
	e.patch(ref, AttrChildren, b.String())
}
