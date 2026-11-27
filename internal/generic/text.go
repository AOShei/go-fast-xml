package generic

import (
	"bytes"
	"unicode"
	"unicode/utf8"
	"unsafe"

	"github.com/AOShei/go-fast-core"
)

// Builder makes the text of one block from pieces, applying the text
// conventions as it goes: XML white space collapsed to one space (or kept
// as written under xml:space="preserve"), white space trimmed at the ends,
// control characters other than tab and line feed removed, bytes that are
// not UTF-8 written as U+FFFD, ligatures written as their letters.
type Builder struct {
	buf     []byte
	pending []byte // white space held until text follows it
	lead    bool   // white space came before the first character
	started bool   // anything was appended
	// Controls counts control characters removed, Invalid the bytes of
	// names (CleanName) that were not UTF-8. Text and attribute values
	// come from the scanner as valid UTF-8, with its own count.
	Controls, Invalid int
}

// Reset empties the builder, keeping its counts and buffers.
func (b *Builder) Reset() {
	b.buf, b.pending, b.lead, b.started = b.buf[:0], b.pending[:0], false, false
}

// Len returns the bytes of text held, not counting white space at the end.
func (b *Builder) Len() int { return len(b.buf) }

// Size returns the bytes held, white space at the end included.
func (b *Builder) Size() int { return len(b.buf) + len(b.pending) }

// Lead and Trail report that the text appended began or ended with white
// space, which trimming removed.
func (b *Builder) Lead() bool  { return b.lead }
func (b *Builder) Trail() bool { return len(b.pending) > 0 }

// Space appends a boundary of white space, as one collapsed space.
func (b *Builder) Space() {
	if !b.started {
		b.lead, b.started = true, true
	}
	if len(b.pending) == 0 || b.pending[len(b.pending)-1] != ' ' && b.pending[len(b.pending)-1] != '\n' {
		b.pending = append(b.pending, ' ')
	}
}

// LineBreak appends a line break written in the markup (HTML's <br>):
// white space held before it is dropped and a "\n" is added, so that two
// breaks give two. Like other white space it is dropped at the ends, and
// white space after it is not added.
func (b *Builder) LineBreak() {
	if !b.started {
		b.lead, b.started = true, true
	}
	n := len(b.pending)
	for n > 0 && b.pending[n-1] == ' ' {
		n--
	}
	b.pending = append(b.pending[:n], '\n')
}

// Break appends a hard line break: the white space held before it becomes
// one "\n". Like other white space it is dropped at the ends.
func (b *Builder) Break() {
	if !b.started {
		b.lead, b.started = true, true
	}
	b.pending = append(b.pending[:0], '\n')
}

// Pos returns the offset in the text where the next character appended
// will be, white space held before it included.
func (b *Builder) Pos() int {
	if len(b.buf) == 0 {
		return 0
	}
	return len(b.buf) + len(b.pending)
}

func (b *Builder) text() {
	if len(b.pending) > 0 {
		if len(b.buf) > 0 {
			b.buf = append(b.buf, b.pending...)
		}
		b.pending = b.pending[:0]
	}
	b.started = true
}

// Append appends t. Under preserve, XML white space is kept as written.
func (b *Builder) Append(t []byte, preserve bool) {
	for i := 0; i < len(t); {
		c := t[i]
		if c > ' ' && c < 0x7F {
			// A run of printable ASCII, copied in one piece.
			j := i + 1
			for j < len(t) && t[j] > ' ' && t[j] < 0x7F {
				j++
			}
			b.text()
			b.buf = append(b.buf, t[i:j]...)
			i = j
			continue
		}
		if c < 0x80 {
			i++
			switch c {
			case ' ', '\t', '\n', '\r':
				if !preserve {
					b.Space()
					continue
				}
				if !b.started {
					b.lead, b.started = true, true
				}
				if c == '\r' {
					c = '\n'
				}
				b.pending = append(b.pending, c)
			default:
				b.Controls++ // C0 controls and DEL
			}
			continue
		}
		r, n := utf8.DecodeRune(t[i:])
		i += n
		// What is appended is valid UTF-8: the scanner hands on text and
		// attribute values with U+FFFD for each byte that is no character,
		// and counts them.
		switch {
		case r == 0x85:
			// NEL is a line end in XML 1.1 and in the text conventions.
			if preserve {
				b.pending = append(b.pending, '\n')
				b.lead, b.started = b.lead || !b.started, true
			} else {
				b.Space()
			}
		case r < 0xA0:
			b.Controls++ // C1 controls
		case unicode.IsSpace(r):
			// Not XML white space but content (a no-break space): kept as
			// written, though not at the ends.
			if !b.started {
				b.lead, b.started = true, true
			}
			b.pending = utf8.AppendRune(b.pending, r)
		case r >= 0xFB00 && r <= 0xFB06:
			b.text()
			b.buf = append(b.buf, gofastcore.Ligature(r)...)
		default:
			b.text()
			b.buf = append(b.buf, t[i-n:i]...)
		}
	}
}

// String returns the text, trimmed.
func (b *Builder) String() string { return string(b.buf) }

// Split takes the first piece of a text longer than limit, cut at white
// space where there is some in the last part of the piece, else at a
// character boundary, and keeps the rest.
func (b *Builder) Split(limit int) string {
	s, _ := b.SplitAt(limit)
	return s
}

// SplitAt is Split that also returns how many bytes of the text the piece
// took, white space at the cut included: the offset in the old text of
// the rest's first byte.
func (b *Builder) SplitAt(limit int) (string, int) {
	if len(b.buf) <= limit {
		s := string(b.buf)
		b.buf = b.buf[:0]
		return s, len(s)
	}
	cut := -1
	for i := limit; i > limit/2; i-- {
		if c := b.buf[i]; c == ' ' || c == '\n' || c == '\t' {
			cut = i
			break
		}
	}
	if cut < 0 {
		cut = limit
		for cut > 0 && !utf8.RuneStart(b.buf[cut]) {
			cut--
		}
	}
	piece := string(bytes.TrimRightFunc(b.buf[:cut], unicode.IsSpace))
	rest := bytes.TrimLeftFunc(b.buf[cut:], unicode.IsSpace)
	took := len(b.buf) - len(rest)
	b.buf = append(b.buf[:0], rest...)
	return piece, took
}

// SplitWord takes the text up to its last white space as a piece and
// keeps the word after it, which what comes next continues: a text cut
// where a token starts is not cut inside a word. pos is where the caller
// knows that word to start in the text (its source offset is known); a
// text whose last white space is not just before pos, one that ends in
// white space, or one that holds none is taken whole (kept is false).
// took is as in SplitAt.
func (b *Builder) SplitWord(pos int) (piece string, took int, kept bool) {
	if len(b.pending) == 0 && pos > 0 && pos <= len(b.buf) {
		if i := bytes.LastIndexAny(b.buf, " \n\t"); i > 0 && i == pos-1 {
			if p := bytes.TrimRightFunc(b.buf[:i], unicode.IsSpace); len(p) > 0 {
				piece = string(p)
				rest := b.buf[i+1:]
				took = len(b.buf) - len(rest)
				b.buf = append(b.buf[:0], rest...)
				return piece, took, true
			}
		}
	}
	piece = string(b.buf)
	b.Reset()
	return piece, len(piece), false
}

// Clean returns s with the text conventions applied, white space
// collapsed and trimmed, and the counts of what was removed.
// When nothing changes, s itself is returned.
func Clean(s string, preserve bool, b *Builder) string {
	b.Reset()
	// Append only reads its argument: s is read in place, not copied.
	b.Append(unsafe.Slice(unsafe.StringData(s), len(s)), preserve)
	if string(b.buf) == s {
		return s
	}
	return b.String()
}

// CleanName returns s, a name or identifier, valid UTF-8 and with no
// control characters; nothing else is changed. A byte that is not part
// of a character is U+FFFD and counted: the scanner does not check
// names. A U+FFFD already in s is kept and not counted.
func CleanName(s string, b *Builder) string {
	clean := true
	for i := 0; i < len(s); i++ {
		if c := s[i]; c < 0x20 || c == 0x7F || c >= 0x80 {
			clean = false
			break
		}
	}
	if clean {
		return s
	}
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); {
		r, n := utf8.DecodeRuneInString(s[i:])
		i += n
		switch {
		case r == utf8.RuneError && n == 1:
			b.Invalid++
			out = utf8.AppendRune(out, r)
		case r < 0x20 || r >= 0x7F && r < 0xA0:
			b.Controls++
		default:
			out = utf8.AppendRune(out, r)
		}
	}
	return string(out)
}
