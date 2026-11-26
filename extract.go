// Package gofastxml extracts text from XML documents of any schema into
// go-fast-core's shared document model.
//
// The document is read as a stream: besides the document returned,
// memory is bounded by fixed bounds (one tag, one block, the blocks
// waiting for an undecided element), not by the file. Nothing a file refers to is loaded or run: no
// external DTD, entity, schema, stylesheet or XInclude. See README.md for
// how XML maps to the model.
package gofastxml

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"strconv"
	"strings"

	"github.com/AOShei/go-fast-core"
	"github.com/AOShei/go-fast-xml/internal/charset"
	"github.com/AOShei/go-fast-xml/internal/engine"
	"github.com/AOShei/go-fast-xml/internal/generic"
	"github.com/AOShei/go-fast-xml/internal/xmlenc"
	"github.com/AOShei/go-fast-xml/internal/xmlread"
	"github.com/AOShei/go-fast-xml/internal/xmlscan"
)

// Defaults for the limits of gofastcore.Options.Limits, used where a limit
// is zero. A negative limit means no limit.
const (
	// DefaultMaxInputBytes bounds the size of the file.
	DefaultMaxInputBytes = 4 << 30
	// DefaultMaxDecodedBytes bounds the bytes entity expansion may
	// produce, with the documents a caller embeds this one in.
	DefaultMaxDecodedBytes = 1 << 30
)

// Fixed bounds. What one cuts is reported with limit.exceeded.
const (
	// MaxDepth bounds element nesting. Elements deeper are read as part
	// of their ancestor: their text is kept, their tags and attributes
	// are not read.
	MaxDepth = xmlscan.DefaultMaxDepth
	// MaxAttributes bounds the attributes read from one element.
	MaxAttributes = xmlscan.DefaultMaxAttrs
	// MaxTagBytes bounds one tag, processing instruction or document type
	// declaration. Reading stops at a larger one.
	MaxTagBytes = xmlscan.DefaultMaxTokenBytes
	// MaxEntityDepth bounds how deeply entity replacements nest.
	MaxEntityDepth = xmlscan.DefaultMaxEntityDepth
	// EntityExpansionBase and EntityExpansionRatio bound the bytes entity
	// expansion may produce in one document: EntityExpansionBase plus
	// EntityExpansionRatio times the size of the input, and never more
	// than the decode budget (Limits.MaxDecodedBytes) has left.
	EntityExpansionBase  = 1 << 20
	EntityExpansionRatio = 8
	// MaxBlockBytes bounds the text of one block; a longer text is split
	// into blocks marked Continued.
	MaxBlockBytes = generic.MaxBlockBytes
	// MaxNameBytes bounds an element or attribute name in Attrs.
	MaxNameBytes = generic.MaxNameBytes
	// MaxPathBytes bounds Attrs["element"].
	MaxPathBytes = generic.MaxPathBytes
	// MaxValueBytes bounds an attribute note and an anchor.
	MaxValueBytes = generic.MaxValueBytes
)

// Warning codes of this library. Of the codes shared by all go-fast
// libraries it raises text.unmapped_glyphs, text.control_characters_removed,
// text.encoding_detected, input.repaired, input.unread_content,
// input.external_reference, limit.exceeded, part.no_text and, with a
// profile, structure.heading_level_clamped. Profile reading adds
// WarnUnmappedElements, WarnUnmappedAttributes and WarnSkippedElements.
const (
	// WarnEntitiesAssumed (info): references to entities the file does
	// not declare, in a file with an external DTD subset, were read as
	// the characters HTML 4 gives those names (its 252 character
	// entities; other names stay as written).
	WarnEntitiesAssumed = "xml.entities_assumed"
	// WarnEntityUndeclared (warning): references to entities the file
	// does not declare were kept in the text as written.
	WarnEntityUndeclared = "xml.entity_undeclared"
)

// Keys of Block.Attrs and Metadata.Extra this library writes.
const (
	// AttrElement is the path of element names from the root, as
	// written, joined with "/", on every block made from an element.
	AttrElement = generic.AttrElement
	// AttrAttribute is the name of the attribute an attribute note holds.
	AttrAttribute = generic.AttrAttribute

	ExtraRoot                      = "root"
	ExtraRootNamespace             = "root_namespace"
	ExtraSchemaLocation            = "schema_location"
	ExtraNoNamespaceSchemaLocation = "no_namespace_schema_location"
	ExtraDoctype                   = "doctype"
	ExtraDoctypePublic             = "doctype_public"
	ExtraDoctypeSystem             = "doctype_system"
	ExtraEncoding                  = "encoding"
)

// Embedded documents: the content of an element a profile's "document"
// rule names (an RSS description holding HTML) is handed to
// gofastcore.Options.ExtractChild and the result appended to
// Document.Children, in file order.
const (
	// AttrChildren, on the heading of the section the element stands in
	// (else its first block), is the indexes in Document.Children of the
	// documents that came from that section, comma separated.
	AttrChildren = engine.AttrChildren
	// AttrMediaType is the media type of a document rule's content kept
	// in the body as a paragraph, when the child was not extracted.
	AttrMediaType = engine.AttrMediaType
	// Keys of each child's Metadata.Extra: the bytes of the element in
	// the file (start tag to end tag, as a block's Loc), the anchor of
	// the nearest enclosing identified element, and the element's path.
	ExtraParentOffset  = "xml.parent_offset"
	ExtraParentLength  = "xml.parent_length"
	ExtraParentAnchor  = "xml.parent_anchor"
	ExtraParentElement = "xml.parent_element"
	// MaxChildren bounds the documents handed over from one document;
	// past it the content stays in the body as a paragraph, with
	// limit.exceeded.
	MaxChildren = 10000
)

// maxChildren is MaxChildren; tests lower it.
var maxChildren = MaxChildren

// Attributes says what becomes of attributes.
type Attributes uint8

const (
	// AttributesNotes, the default, makes each attribute with a value a
	// note of type attribute at its element's place.
	AttributesNotes Attributes = iota
	// AttributesNone leaves attribute notes out, for data-style XML.
	AttributesNone
)

// Extractor extracts XML documents. The zero value is ready to use and
// safe for concurrent use.
type Extractor struct {
	// Attributes says what becomes of attributes in generic reading.
	Attributes Attributes
	// Profiles are tried, in order, before the built-in profiles: the
	// first whose "match" the root element meets is used.
	Profiles []*Profile
	// Profile forces the profile of that name, from Profiles or the
	// built-in ones, whatever the root element; ProfileNone ("none")
	// forces generic reading. Empty means choose by the root element.
	Profile string
	// Charset is the character encoding of the input, given from outside
	// it, as a label such as the charset parameter of a MIME Content-Type
	// gives it ("windows-1251", "shift_jis"). It is used whatever the XML
	// declaration names or the bytes suggest; a byte order mark still
	// overrides it, as with go-fast-html's Charset. A label that names no
	// encoding this library decodes is an error (ErrUnsupported) before
	// anything is read. Empty means none.
	Charset string
}

// charset returns the encoding Charset names; given is false when it is
// empty.
func (e *Extractor) charset() (enc charset.Encoding, given bool, err error) {
	if e.Charset == "" {
		return enc, false, nil
	}
	enc, ok := charset.Lookup(e.Charset)
	if !ok {
		return enc, false, fmt.Errorf("gofastxml: Extractor.Charset %q names no encoding this library decodes: %w", e.Charset, gofastcore.ErrUnsupported)
	}
	return enc, true, nil
}

var _ gofastcore.Extractor = (*Extractor)(nil)

// Formats lists the formats this library extracts.
func (e *Extractor) Formats() []string { return []string{"xml"} }

// Formats lists the formats this library extracts.
func Formats() []string { return (&Extractor{}).Formats() }

// Extract reads an XML document with default settings. It is
// (&Extractor{}).Extract.
func Extract(ctx context.Context, r io.ReaderAt, size int64, opts *gofastcore.Options) (*gofastcore.Document, error) {
	return (&Extractor{}).Extract(ctx, r, size, opts)
}

// ExtractBytes extracts a document held in memory with default settings.
func ExtractBytes(ctx context.Context, data []byte, opts *gofastcore.Options) (*gofastcore.Document, error) {
	return (&Extractor{}).extract(ctx, bytes.NewReader(data), int64(len(data)), opts, data)
}

// ExtractFile opens the named file and extracts it with default settings.
func ExtractFile(ctx context.Context, path string, opts *gofastcore.Options) (*gofastcore.Document, error) {
	return (&Extractor{}).ExtractFile(ctx, path, opts)
}

// ExtractFile opens the named file and extracts it. The file name is
// recorded as the document's source name unless opts.Name is set.
func (e *Extractor) ExtractFile(ctx context.Context, path string, opts *gofastcore.Options) (*gofastcore.Document, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	var o gofastcore.Options
	if opts != nil {
		o = *opts
	}
	if o.Name == "" {
		o.Name = path
	}
	return e.Extract(ctx, f, info.Size(), &o)
}

// Extract reads size bytes from r and returns the document: one part of
// kind body, number 1. An error wraps one of the gofastcore errors:
//
//   - ErrMalformed: the input has no element, or size is negative.
//   - ErrUnsupported: the input is in an encoding this library does not
//     read: one it declares, UTF-32 or EBCDIC, or, with none declared and
//     the bytes not UTF-8, one that could not be recognised.
//   - ErrLimitExceeded: the input is larger than Limits.MaxInputBytes, or
//     a tag before any element is larger than MaxTagBytes.
//   - ErrNoPartsSelected: Options.Parts does not select part 1.
func (e *Extractor) Extract(ctx context.Context, r io.ReaderAt, size int64, opts *gofastcore.Options) (*gofastcore.Document, error) {
	return e.extract(ctx, r, size, opts, nil)
}

// extract is Extract; mem, when not nil, is the input r reads, held in
// memory, which is read in place and never written.
func (e *Extractor) extract(ctx context.Context, r io.ReaderAt, size int64, opts *gofastcore.Options, mem []byte) (*gofastcore.Document, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var o gofastcore.Options
	if opts != nil {
		o = *opts
	}
	override, given, err := e.charset()
	if err != nil {
		return nil, err
	}
	if r == nil {
		return nil, fmt.Errorf("gofastxml: no input to read: %w", gofastcore.ErrMalformed)
	}
	if size < 0 {
		return nil, fmt.Errorf("gofastxml: input size %d is negative: %w", size, gofastcore.ErrMalformed)
	}
	if limit := gofastcore.ResolveLimit(o.Limits.MaxInputBytes, DefaultMaxInputBytes); size > limit {
		return nil, fmt.Errorf("gofastxml: input is %d bytes, limit is %d: %w", size, limit, gofastcore.ErrLimitExceeded)
	}
	forced, none, err := e.forced()
	if err != nil {
		return nil, err
	}
	var sample []byte
	if n := min(size, xmlenc.SampleSize); mem != nil {
		sample = mem[:n:n] // in memory: read in place, not copied
	} else {
		sample = make([]byte, n)
		if n, err := r.ReadAt(sample, 0); n < len(sample) {
			if err == nil {
				err = io.ErrUnexpectedEOF
			}
			return nil, fmt.Errorf("gofastxml: reading the input: %w", err)
		}
	}
	var enc xmlenc.Result
	if given {
		enc, err = xmlenc.SniffAs(sample, override)
	} else {
		enc, err = xmlenc.Sniff(sample)
	}
	if err != nil {
		return nil, fmt.Errorf("gofastxml: %w: %w", err, gofastcore.ErrUnsupported)
	}
	// When the sample holds the whole input, it is UTF-8 and smaller than
	// the window, the scanner reads the sample in place: it would read
	// the same bytes into a window that holds them all, with the same
	// tokens, so no window is made and nothing is read twice.
	var whole []byte
	if enc.UTF8 && int64(len(sample)) == size && len(sample)-enc.Skip < min(xmlscan.DefaultBufferSize, tagBytes) {
		whole = sample[enc.Skip:]
	}
	sample = nil

	ctx, decode := gofastcore.WithDecodeBudget(ctx, o.Limits, DefaultMaxDecodedBytes)
	defer decode.Close()
	ctx, text := gofastcore.WithTextBudget(ctx, o.Limits)
	defer text.Close()

	expansion := int64(EntityExpansionBase)
	if size < (1<<62)/EntityExpansionRatio {
		expansion += EntityExpansionRatio * size
	}
	expansion = min(expansion, max(decode.Left(), 0))

	dec := xmlenc.NewReader(io.NewSectionReader(r, int64(enc.Skip), size-int64(enc.Skip)), enc)
	scan := xmlscan.Options{ExpansionLimit: expansion, MaxTokenBytes: tagBytes, BufferSize: min(xmlscan.DefaultBufferSize, tagBytes)}
	if enc.UTF8 {
		scan.BaseOffset = int64(enc.Skip)
	}
	var rd *xmlread.Reader
	if whole != nil {
		rd = xmlread.NewBytes(ctx, whole, xmlread.Options{Scan: scan})
	} else {
		rd = xmlread.New(ctx, dec, xmlread.Options{Scan: scan})
	}

	doc := gofastcore.New("xml")
	doc.Source.Name, doc.Source.Size, doc.Source.MIME = o.Name, size, "application/xml"
	part := &gofastcore.Part{Kind: gofastcore.PartBody, Number: 1}
	out := &generic.Out{Part: part, Budget: text, Hint: generic.BlocksHint(size), UTF8: enc.UTF8}
	g := generic.New(out, generic.Options{NoAttributes: e.Attributes == AttributesNone})
	d := &dispatch{x: e, out: out, g: g, h: g, forced: forced, none: none,
		opts: engineOptions(&o)}
	maxParts := gofastcore.ResolveLimit(int64(o.Limits.MaxParts), engine.MaxParts)
	var expTaken int64 // entity expansion taken from the decode budget
	childCut := 0      // children not read: the decode budget was spent
	d.opts.MaxChildren = maxChildren
	d.opts.PartWanted = func(n int) bool { return o.Parts.Contains(n) && int64(n) <= maxParts }
	d.opts.Child = func(name, mt string, data []byte) (*gofastcore.Document, bool) {
		// The decode budget says what is left before the child reads.
		exp := rd.Scanner().Stats.Expanded
		decode.Take(exp - expTaken)
		expTaken = exp
		var c *gofastcore.Document
		if decode.Take(int64(len(data))) {
			c = o.ExtractChild(ctx, name, mt, bytes.NewReader(data), int64(len(data)))
		} else {
			childCut++
			c = o.ExtractChild(ctx, name, mt, nil, int64(len(data)))
		}
		return c, childExtracted(c)
	}
	if enc.UTF8 {
		d.opts.Raw = func(start, end int64) []byte {
			b := make([]byte, end-start)
			if mem != nil && end <= int64(len(mem)) {
				copy(b, mem[start:end])
				return b
			}
			n, _ := r.ReadAt(b, start)
			return b[:n]
		}
	}

	runErr := rd.Run(d)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	d.Finish(rd.Offset())
	stats := rd.Scanner().Stats
	decode.Take(stats.Expanded - expTaken)
	rep := &rd.Report
	if rep.Elements == 0 {
		if errors.Is(runErr, xmlscan.ErrTokenTooLarge) {
			return nil, fmt.Errorf("gofastxml: %s before any element was read: %w", tooLarge(runErr), gofastcore.ErrLimitExceeded)
		}
		if runErr != nil {
			return nil, fmt.Errorf("gofastxml: reading the input: %w", runErr)
		}
		return nil, fmt.Errorf("gofastxml: the input has no XML element: %w", gofastcore.ErrMalformed)
	}

	parts := []*gofastcore.Part{part}
	if d.eng != nil {
		parts = d.eng.Parts()
		out.ValuesCut += d.eng.ValuesCut
		out.PendingCut += d.eng.PendingCut
	}
	metadata(doc, rep, enc, out)
	if d.used != nil {
		mergeMetadata(&doc.Metadata, &d.eng.Meta)
		doc.Metadata.Extra[ExtraProfile] = d.used.Name()
	}
	docWarnings(doc, rep, enc, dec, stats)
	at := rd.Offset()
	last := parts[len(parts)-1]
	if last.Kind == gofastcore.PartStored {
		last = parts[len(parts)-2]
	}
	var locs []engine.ChildLoc
	if d.eng != nil {
		locs = d.eng.ChildLocs
	}
	if !enc.UTF8 {
		var err error
		if at, err = sourceOffsets(parts, locs, dec, r, enc.Skip, at); err != nil {
			last.Warn(gofastcore.SeverityError, gofastcore.WarnPartFailed,
				fmt.Sprintf("reading the input again to map locations to it failed (%v); locations are offsets in the text decoded to UTF-8", err))
		}
	}
	if d.eng != nil {
		d.eng.PartWarnings(last)
		engineWarnings(last, d.eng)
		children(doc, d.eng)
		if childCut > 0 {
			last.Warn(gofastcore.SeverityError, gofastcore.WarnLimitExceeded,
				fmt.Sprintf("the decode budget (Limits.MaxDecodedBytes) was spent; %s not handed to the child extractor and kept in the body", plural(childCut, "embedded document was", "embedded documents were")))
		}
	}
	partWarnings(last, d, out, rep, dec, stats, runErr, at, size)
	for _, p := range parts {
		if p.Kind == gofastcore.PartStored && o.Parts != (gofastcore.PartRange{}) {
			continue
		}
		if p.Kind != gofastcore.PartStored && !o.Parts.Contains(p.Number) {
			continue
		}
		if p.Kind != gofastcore.PartStored && int64(len(doc.Parts)) >= maxParts {
			doc.Warn(gofastcore.SeverityError, gofastcore.WarnLimitExceeded,
				fmt.Sprintf("the document has more parts than Limits.MaxParts (%d); the parts after part %d were left out", maxParts, doc.Parts[len(doc.Parts)-1].Number))
			continue
		}
		p.Finish()
		doc.Parts = append(doc.Parts, *p)
	}
	if len(doc.Parts) == 0 {
		return nil, fmt.Errorf("gofastxml: the document has %d parts, none selected: %w", len(parts), gofastcore.ErrNoPartsSelected)
	}
	return doc, nil
}

func engineOptions(o *gofastcore.Options) engine.Options {
	return engine.Options{Spans: o.Spans, NoImages: o.Images == gofastcore.ImagesNone}
}

// mergeMetadata adds what a profile's rules gave to the metadata.
func mergeMetadata(dst, src *gofastcore.Metadata) {
	if src.Title != "" {
		dst.Title = src.Title
	}
	dst.Authors = append(dst.Authors, src.Authors...)
	dst.Keywords = append(dst.Keywords, src.Keywords...)
	if src.Subject != "" {
		dst.Subject = src.Subject
	}
	if src.Language != "" {
		dst.Language = src.Language
	}
	if !src.Created.IsZero() {
		dst.Created = src.Created
	}
	if !src.Modified.IsZero() {
		dst.Modified = src.Modified
	}
	for k, v := range src.Extra {
		dst.Extra[k] = v
	}
}

func engineWarnings(part *gofastcore.Part, e *engine.Engine) {
	limit := func(msg string) { part.Warn(gofastcore.SeverityError, gofastcore.WarnLimitExceeded, msg) }
	if e.ColumnsCut > 0 {
		limit(fmt.Sprintf("%s past %d columns cut", plural(e.ColumnsCut, "table", "tables"), engine.MaxTableColumns))
	}
	if e.TextCut > 0 {
		limit(fmt.Sprintf("%s (table cells, values, labels) longer than %d bytes cut", plural(e.TextCut, "text", "texts"), MaxBlockBytes))
	}
	if e.RowSpanCut > 0 {
		limit(fmt.Sprintf("%s spanning more than %d rows cut to that span", plural(e.RowSpanCut, "cell", "cells"), engine.MaxRowSpan))
	}
	if e.SpansCut > 0 {
		limit(fmt.Sprintf("%s past 100000 in one block left out", plural(e.SpansCut, "span", "spans")))
	}
	if e.DocsCut > 0 {
		limit(fmt.Sprintf("%s of embedded documents longer than %d bytes cut", plural(e.DocsCut, "content", "contents"), MaxBlockBytes))
	}
	if e.ChildrenCut > 0 {
		limit(fmt.Sprintf("%s past %d not handed to the child extractor; their text is in the body", plural(e.ChildrenCut, "embedded document", "embedded documents"), maxChildren))
	}
	if e.AttachmentsCut > 0 {
		limit(fmt.Sprintf("%s longer than %d bytes of base64, or past %d embedded documents, not handed to the child extractor and not in the body", plural(e.AttachmentsCut, "attachment", "attachments"), engine.MaxAttachmentBytes, maxChildren))
	}
	if e.AttachmentsBad > 0 {
		part.Warn(gofastcore.SeverityWarning, gofastcore.WarnInputStreamDamaged,
			fmt.Sprintf("%s not valid base64: not handed to the child extractor; the text is in the body as written", plural(e.AttachmentsBad, "attachment", "attachments")))
	}
	if e.PartsCut > 0 {
		limit(fmt.Sprintf("%s past %d not started; their content is in the last part", plural(e.PartsCut, "part", "parts"), engine.MaxParts))
	}
}

// sourceOffsets turns the locations of the blocks, and at, from offsets
// in the decoded text into offsets in the file, for input not read as
// UTF-8. The offsets are mapped in increasing order, in one pass over the
// part of the input they reach.
func sourceOffsets(parts []*gofastcore.Part, locs []engine.ChildLoc, dec *xmlenc.Reader, r io.ReaderAt, skip int, at int64) (int64, error) {
	var offs []int64
	for _, part := range parts {
		for _, b := range part.Blocks {
			offs = append(offs, b.Loc.Offset, b.Loc.Offset+b.Loc.Length)
		}
	}
	for _, l := range locs {
		offs = append(offs, l.Offset, l.Offset+l.Length)
	}
	offs = append(offs, at)
	slices.Sort(offs)
	offs = slices.Compact(offs)
	src := slices.Clone(offs)
	if err := dec.Map(r, int64(skip), src); err != nil {
		return at, err
	}
	get := func(o int64) int64 {
		i, _ := slices.BinarySearch(offs, o)
		return src[i] + int64(skip)
	}
	for _, part := range parts {
		for i := range part.Blocks {
			l := &part.Blocks[i].Loc
			start, end := get(l.Offset), get(l.Offset+l.Length)
			l.Offset, l.Length = start, end-start
		}
	}
	for i := range locs {
		l := &locs[i]
		start, end := get(l.Offset), get(l.Offset+l.Length)
		l.Offset, l.Length = start, end-start
	}
	return get(at), nil
}

// childExtracted reports whether ExtractChild extracted the document, or
// returned the stub of one it did not.
func childExtracted(c *gofastcore.Document) bool {
	for _, w := range c.Warnings {
		switch w.Code {
		case gofastcore.WarnChildNotExtracted, gofastcore.WarnChildUnsupported,
			gofastcore.WarnChildEncrypted, gofastcore.WarnChildFailed:
			return false
		}
	}
	return true
}

// children appends the documents the engine handed over, each with where
// it came from in its Metadata.Extra.
func children(doc *gofastcore.Document, eng *engine.Engine) {
	for i, c := range eng.Children {
		l := eng.ChildLocs[i]
		if c.Metadata.Extra == nil {
			c.Metadata.Extra = map[string]string{}
		}
		c.Metadata.Extra[ExtraParentOffset] = strconv.FormatInt(l.Offset, 10)
		c.Metadata.Extra[ExtraParentLength] = strconv.FormatInt(l.Length, 10)
		c.Metadata.Extra[ExtraParentElement] = l.Element
		if l.Anchor != "" {
			c.Metadata.Extra[ExtraParentAnchor] = l.Anchor
		}
		doc.Children = append(doc.Children, c)
	}
	eng.Children, eng.ChildLocs = nil, nil
}

// tagBytes is MaxTagBytes; tests lower it.
var tagBytes = MaxTagBytes

// tooLarge says what was larger than MaxTagBytes.
func tooLarge(err error) string {
	if errors.Is(err, xmlscan.ErrReferenceTooLarge) {
		return fmt.Sprintf("a character reference is larger than %d bytes", tagBytes)
	}
	return fmt.Sprintf("a tag, processing instruction or document type declaration is larger than %d bytes", tagBytes)
}

func metadata(doc *gofastcore.Document, rep *xmlread.Report, enc xmlenc.Result, out *generic.Out) {
	var clean generic.Builder
	extra := map[string]string{}
	set := func(k, v string) {
		if v = strings.TrimSpace(v); v != "" {
			if len(v) > MaxValueBytes {
				v = generic.CutString(v, MaxValueBytes)
				out.ValuesCut++
			}
			extra[k] = generic.CleanName(strings.ToValidUTF8(v, "�"), &clean)
		}
	}
	set(ExtraRoot, rep.RootName)
	set(ExtraRootNamespace, rep.RootURI)
	set(ExtraSchemaLocation, strings.Join(strings.Fields(rep.SchemaLocation), " "))
	set(ExtraNoNamespaceSchemaLocation, rep.NoNSSchemaLoc)
	if d := rep.Doctype; d != nil {
		set(ExtraDoctype, d.Name)
		set(ExtraDoctypePublic, d.PublicID)
		set(ExtraDoctypeSystem, d.SystemID)
	}
	set(ExtraEncoding, enc.Name())
	doc.Metadata.Extra = extra
	if lang := strings.TrimSpace(rep.RootLang); lang != "" && len(lang) <= generic.MaxLangBytes {
		doc.Metadata.Language = generic.CleanName(strings.ToValidUTF8(lang, "�"), &clean)
	}
}

func plural(n int, one, many string) string {
	if n == 1 {
		return "1 " + one
	}
	return fmt.Sprintf("%d %s", n, many)
}

func docWarnings(doc *gofastcore.Document, rep *xmlread.Report, enc xmlenc.Result, dec *xmlenc.Reader, stats xmlscan.Stats) {
	if enc.Detected {
		doc.Warn(gofastcore.SeverityWarning, gofastcore.WarnTextEncodingDetected,
			fmt.Sprintf("no encoding is declared and the text is not UTF-8; it was read as %s, recognised from its bytes", enc.Name()))
	}
	if n := rep.Repairs + stats.EntitiesRecursive; n > 0 {
		doc.Warn(gofastcore.SeverityWarning, gofastcore.WarnInputRepaired,
			fmt.Sprintf("%s where the markup is not well formed read leniently (mismatched or unclosed tags, a stray < or &, attributes without quotes or values, references to an entity inside its own replacement)",
				plural(n, "place", "places")))
	}
	for kind, n := range rep.External {
		if n > 0 {
			doc.Warn(gofastcore.SeverityWarning, gofastcore.WarnInputExternalReference,
				fmt.Sprintf("%s (%d); nothing was loaded or fetched", xmlread.RefKindNames[kind], n))
		}
	}
	if rep.PIs > 0 {
		doc.Warn(gofastcore.SeverityWarning, gofastcore.WarnInputUnreadContent,
			fmt.Sprintf("%s, not read as text", plural(rep.PIs, "processing instruction", "processing instructions")))
	}
}

func partWarnings(part *gofastcore.Part, g interface{ Counts() (int, int) }, out *generic.Out, rep *xmlread.Report, dec *xmlenc.Reader, stats xmlscan.Stats, runErr error, at, size int64) {
	if rep.Assumed > 0 {
		part.Warn(gofastcore.SeverityInfo, WarnEntitiesAssumed,
			fmt.Sprintf("%s to entities the file does not declare read as the HTML 4 character entities of those names; the file's external DTD subset was not loaded",
				plural(rep.Assumed, "reference", "references")))
	}
	if rep.Undeclared > 0 {
		part.Warn(gofastcore.SeverityWarning, WarnEntityUndeclared,
			fmt.Sprintf("%s to entities the file does not declare kept in the text as written", plural(rep.Undeclared, "reference", "references")))
	}
	controls, invalid := g.Counts()
	if n := invalid + dec.Invalid(); n > 0 {
		part.Warn(gofastcore.SeverityWarning, gofastcore.WarnTextUnmappedGlyphs,
			fmt.Sprintf("%s not valid in the encoding written as U+FFFD", plural(n, "byte sequence", "byte sequences")))
	}
	if controls > 0 {
		part.Warn(gofastcore.SeverityInfo, gofastcore.WarnTextControlCharactersRemoved,
			fmt.Sprintf("%s removed", plural(controls, "control character", "control characters")))
	}
	limit := func(msg string) { part.Warn(gofastcore.SeverityError, gofastcore.WarnLimitExceeded, msg) }
	if stats.EntitiesCut > 0 {
		limit(fmt.Sprintf("%s kept as written: expanding them would pass the bound on entity nesting (%d) or on the bytes entity expansion may produce",
			plural(stats.EntitiesCut, "entity reference", "entity references"), MaxEntityDepth))
	}
	if stats.Flattened > 0 {
		limit(fmt.Sprintf("%s nested deeper than %d read as part of their ancestor; their attributes were not read",
			plural(stats.Flattened, "element", "elements"), MaxDepth))
	}
	if stats.AttrsCut > 0 {
		limit(fmt.Sprintf("%s past %d on one element not read", plural(stats.AttrsCut, "attribute", "attributes"), MaxAttributes))
	}
	if out.PendingCut > 0 {
		limit(fmt.Sprintf("%s by the bound on what waits for a block (%d blocks, %d bytes): more notes waited for a block, or more blocks for an element not yet known to be a block, than it allows; that block's text was given in pieces, with the waiting blocks between them",
			plural(out.PendingCut, "cut", "cuts"), generic.MaxPendingBlocks, generic.MaxPendingBytes))
	}
	if out.NamesCut > 0 {
		limit(fmt.Sprintf("%s longer than %d bytes cut", plural(out.NamesCut, "name", "names"), MaxNameBytes))
	}
	if out.PathsCut > 0 {
		limit(fmt.Sprintf("%s of element names longer than %d bytes cut", plural(out.PathsCut, "path", "paths"), MaxPathBytes))
	}
	if n := out.ValuesCut + stats.ValuesCut; n > 0 {
		limit(fmt.Sprintf("%s longer than %d bytes (or than what was left of %d bytes for all the values of their tag) cut",
			plural(n, "attribute value", "attribute values"), MaxValueBytes, MaxTagBytes))
	}
	if stats.BindingsCut > 0 {
		limit(fmt.Sprintf("%s not read: those in scope held %d bytes", plural(stats.BindingsCut, "namespace declaration", "namespace declarations"), xmlscan.MaxBindingBytes))
	}
	switch {
	case out.Stopped:
		limit(fmt.Sprintf("the text budget (Limits.MaxTextBytes) was spent; reading stopped with about %d bytes of input left unread", max(size-at, 0)))
	case errors.Is(runErr, xmlscan.ErrTokenTooLarge):
		limit(fmt.Sprintf("%s; reading stopped there with about %d bytes of input left unread", tooLarge(runErr), max(size-at, 0)))
	case runErr != nil:
		part.Warn(gofastcore.SeverityError, gofastcore.WarnPartFailed, fmt.Sprintf("reading stopped: %v", runErr))
	}
	if len(part.Blocks) == 0 {
		part.Warn(gofastcore.SeverityWarning, gofastcore.WarnPartNoText, "no text")
	}
}
