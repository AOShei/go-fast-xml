// Package xmlenc decides the character encoding of an XML document and
// decodes it to UTF-8 as a stream.
//
// The decision follows the XML 1.0 specification (section 4.3.3 and
// appendix F): a byte order mark, then the first bytes of the XML
// declaration where they show UTF-16 without a mark, then the encoding
// the declaration names, then UTF-8. A document that declares no encoding
// and is not UTF-8 is read in the encoding recognised from its bytes where
// one is certain (go-fast-html's detection), and refused otherwise.
package xmlenc

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"slices"
	"sort"
	"unicode/utf8"

	"github.com/AOShei/go-fast-core/charset"
)

// ErrUnsupported is returned, wrapped, for an encoding that is not read.
var ErrUnsupported = errors.New("encoding not read")

// SampleSize is how much of the start of the input Sniff wants: the XML
// declaration, and enough text to tell UTF-8 or recognise an encoding.
const SampleSize = 1 << 20

// Result is the decision.
type Result struct {
	// Enc is the encoding.
	Enc charset.Encoding
	// UTF8 reports that the input is read as UTF-8: the bytes are used as
	// they are, and offsets in the text are offsets in the file.
	UTF8 bool
	// Skip is the bytes at the start that are not text (a byte order
	// mark).
	Skip int
	// Declared is the encoding the XML declaration names, lowercased and
	// held to 64 bytes, or empty.
	Declared string
	// Detected reports that nothing declared the encoding and the input is
	// not UTF-8: Enc was recognised from the bytes.
	Detected bool
}

// Name returns the name of the encoding.
func (r Result) Name() string {
	if r.UTF8 {
		return "utf-8"
	}
	return r.Enc.String()
}

// Sniff decides the encoding from sample, the first SampleSize bytes of
// the input (all of it if shorter).
func Sniff(sample []byte) (Result, error) {
	switch {
	case bytes.HasPrefix(sample, []byte{0xEF, 0xBB, 0xBF}):
		r := Result{UTF8: true, Skip: 3}
		r.Declared = declared(sample[3:])
		return r, nil
	case bytes.HasPrefix(sample, []byte{0x00, 0x00, 0xFE, 0xFF}), bytes.HasPrefix(sample, []byte{0xFF, 0xFE, 0x00, 0x00}):
		return Result{}, fmt.Errorf("UTF-32 (byte order mark): %w", ErrUnsupported)
	case bytes.HasPrefix(sample, []byte{0xFE, 0xFF}):
		return Result{Enc: charset.UTF16BE, Skip: 2, Declared: declared16(sample[2:], true)}, nil
	case bytes.HasPrefix(sample, []byte{0xFF, 0xFE}):
		return Result{Enc: charset.UTF16LE, Skip: 2, Declared: declared16(sample[2:], false)}, nil
	case bytes.HasPrefix(sample, []byte{0x00, 0x3C, 0x00, 0x3F}):
		return Result{Enc: charset.UTF16BE, Declared: declared16(sample, true)}, nil
	case bytes.HasPrefix(sample, []byte{0x3C, 0x00, 0x3F, 0x00}):
		return Result{Enc: charset.UTF16LE, Declared: declared16(sample, false)}, nil
	case bytes.HasPrefix(sample, []byte{0x00, 0x00, 0x00, 0x3C}), bytes.HasPrefix(sample, []byte{0x3C, 0x00, 0x00, 0x00}):
		return Result{}, fmt.Errorf("UTF-32: %w", ErrUnsupported)
	case bytes.HasPrefix(sample, []byte{0x4C, 0x6F, 0xA7, 0x94}):
		return Result{}, fmt.Errorf("EBCDIC: %w", ErrUnsupported)
	}
	label := declared(sample)
	r := Result{Declared: label}
	switch label {
	case "":
	case "utf-8", "utf8", "utf-16", "utf-16le", "utf-16be", "unicode", "iso-10646-ucs-2", "ucs-2":
		// UTF-16 named in a declaration that reads as ASCII is not
		// UTF-16: the bytes are what decide.
		r.UTF8 = true
		return r, nil
	default:
		enc, ok := charset.Lookup(label)
		if !ok {
			return r, fmt.Errorf("the document declares the encoding %q: %w", label, ErrUnsupported)
		}
		r.Enc = enc
		r.UTF8 = enc == charset.UTF8
		return r, nil
	}
	// Nothing declared: UTF-8, unless the bytes say otherwise.
	if utf8.Valid(trimPartial(sample)) {
		r.UTF8 = true
		return r, nil
	}
	got := charset.SniffText(sample, "")
	if got.Unknown {
		return r, fmt.Errorf("the document declares no encoding, is not UTF-8, and its encoding could not be recognised: %w", ErrUnsupported)
	}
	if got.Enc == charset.UTF8 {
		r.UTF8 = true
		return r, nil
	}
	r.Enc, r.Skip, r.Detected = got.Enc, got.BOM, true
	return r, nil
}

// SniffAs is Sniff with the encoding given from outside the input: enc
// is used whatever the XML declaration names or the bytes suggest. A
// byte order mark still wins (as go-fast-html's Charset), and a UTF-32
// one is refused.
func SniffAs(sample []byte, enc charset.Encoding) (Result, error) {
	if bytes.HasPrefix(sample, []byte{0xEF, 0xBB, 0xBF}) || bytes.HasPrefix(sample, []byte{0xFE, 0xFF}) || bytes.HasPrefix(sample, []byte{0xFF, 0xFE}) {
		return Sniff(sample)
	}
	r := Result{Enc: enc, UTF8: enc == charset.UTF8}
	switch enc {
	case charset.UTF16BE, charset.UTF16LE:
		r.Declared = declared16(sample, enc == charset.UTF16BE)
	default:
		r.Declared = declared(sample)
	}
	return r, nil
}

// trimPartial drops a UTF-8 sequence cut off at the end of a sample.
func trimPartial(b []byte) []byte {
	for back := 1; back <= 3 && back <= len(b); back++ {
		if utf8.RuneStart(b[len(b)-back]) {
			if !utf8.FullRune(b[len(b)-back:]) {
				return b[:len(b)-back]
			}
			break
		}
	}
	return b
}

// declared returns the encoding named in the XML declaration at the start
// of b, lowercased, or "".
func declared(b []byte) string {
	if !bytes.HasPrefix(b, []byte("<?xml")) || len(b) < 6 || !isSpace(b[5]) {
		return ""
	}
	end := bytes.Index(b[:min(len(b), 1024)], []byte("?>"))
	if end < 0 {
		return ""
	}
	decl := b[5:end]
	i := bytes.Index(decl, []byte("encoding"))
	if i < 0 {
		return ""
	}
	rest := bytes.TrimLeft(decl[i+len("encoding"):], " \t\r\n")
	if len(rest) == 0 || rest[0] != '=' {
		return ""
	}
	rest = bytes.TrimLeft(rest[1:], " \t\r\n")
	if len(rest) == 0 || rest[0] != '"' && rest[0] != '\'' {
		return ""
	}
	j := bytes.IndexByte(rest[1:], rest[0])
	if j < 0 {
		return ""
	}
	label := bytes.ToLower(bytes.TrimSpace(rest[1 : 1+j]))
	if len(label) > 64 {
		label = label[:64]
	}
	for _, c := range label {
		if c < 0x20 || c >= 0x7F {
			return "?"
		}
	}
	return string(label)
}

// declared16 is declared for UTF-16: the declaration is ASCII, so the
// low byte of each unit is its character.
func declared16(b []byte, be bool) string {
	var ascii []byte
	for i := 0; i+1 < len(b) && len(ascii) < 1024; i += 2 {
		hi, lo := b[i], b[i+1]
		if !be {
			hi, lo = lo, hi
		}
		if hi != 0 || lo >= 0x80 {
			break
		}
		ascii = append(ascii, lo)
	}
	return declared(ascii)
}

func isSpace(c byte) bool { return c == ' ' || c == '\t' || c == '\r' || c == '\n' }

// Reader decodes a stream to UTF-8. Input read as UTF-8 is passed through
// as it is.
type Reader struct {
	r       io.Reader
	dec     charset.Decoder // the stream's decoder: it keeps ISO-2022-JP's state
	utf8    bool
	src     []byte
	n       int // bytes of src held
	out     []byte
	off     int // bytes of out already given
	eof     bool
	err     error
	invalid int

	// The map back to the source: where each decoded chunk starts, in
	// the decoded text and in the source. One entry per 32 KiB of
	// source.
	cps             []checkpoint
	decSum, usedSum int64
}

// st is the decoder as it was where the chunk starts, so that Map
// decodes the chunk again from the same state (ISO-2022-JP's character
// set; no other encoding has any).
type checkpoint struct {
	dec, src int64
	st       charset.Decoder
}

// NewReader returns a reader of r, which starts after the bytes Sniff
// said to skip, that gives UTF-8.
func NewReader(r io.Reader, res Result) *Reader {
	d := &Reader{r: r, utf8: res.UTF8}
	d.dec.Reset(res.Enc)
	return d
}

// Invalid returns the byte sequences so far that were not a character in
// the encoding and became U+FFFD. Invalid UTF-8 passed through is not
// counted here.
func (d *Reader) Invalid() int { return d.invalid }

const chunk = 32 << 10

// Map replaces offsets in the decoded text, sorted in increasing order,
// by the offsets in the source of the characters that start at or cover
// them; an offset at the end of what was decoded gives the end of the
// source read. src is the source, the text starting at offset base. Only
// what is needed is read again, a decoded chunk at a time, in one pass.
// Input read as UTF-8 needs no map: its offsets are kept.
func (d *Reader) Map(src io.ReaderAt, base int64, offs []int64) error {
	if d.utf8 || len(d.cps) == 0 {
		return nil
	}
	ci := -1
	var buf []byte
	var dpos int64
	var bi, limit int
	var st charset.Decoder
	for k, off := range offs {
		i := sort.Search(len(d.cps), func(i int) bool { return d.cps[i].dec > off }) - 1
		if i < 0 {
			i = 0
		}
		if i != ci {
			ci = i
			from, to := d.cps[i].src, d.usedSum
			if i+1 < len(d.cps) {
				to = d.cps[i+1].src
			}
			limit = int(to - from)
			// The bytes after the chunk that its decoding looked at.
			end := to + charset.MaxChar - 1
			buf = slices.Grow(buf[:0], int(end-from))[:end-from]
			n, err := src.ReadAt(buf, base+from)
			if n < limit {
				if err == nil {
					err = io.ErrUnexpectedEOF
				}
				return err
			}
			buf = buf[:n]
			dpos, bi, st = d.cps[i].dec, 0, d.cps[i].st
		}
		for dpos < off && bi < limit {
			// A copy steps first: a character that covers the offset is
			// not passed, and the state stays where it starts.
			next := st
			dn, n := next.Step(buf, bi)
			if dpos+int64(dn) > off {
				break
			}
			st = next
			dpos += int64(dn)
			bi += n
		}
		offs[k] = d.cps[ci].src + int64(bi)
	}
	return nil
}

func (d *Reader) Read(p []byte) (int, error) {
	if d.utf8 {
		return d.r.Read(p)
	}
	for d.off == len(d.out) {
		if d.eof {
			if d.err != nil {
				return 0, d.err
			}
			return 0, io.EOF
		}
		if d.src == nil {
			d.src = make([]byte, chunk)
		}
		m, err := io.ReadFull(d.r, d.src[d.n:])
		d.n += m
		if err != nil {
			d.eof = true
			if err != io.EOF && err != io.ErrUnexpectedEOF {
				d.err = err
			}
		}
		var used, bad int
		start := d.dec
		d.out, used, bad = d.dec.Decode(d.out[:0], d.src[:d.n], d.eof)
		d.cps = append(d.cps, checkpoint{d.decSum, d.usedSum, start})
		d.decSum += int64(len(d.out))
		d.usedSum += int64(used)
		d.off = 0
		d.invalid += bad
		d.n = copy(d.src, d.src[used:d.n])
	}
	n := copy(p, d.out[d.off:])
	d.off += n
	return n, nil
}
