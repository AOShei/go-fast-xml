# go-fast-xml

Extracts XML documents of any schema into the shared [go-fast-core](https://github.com/AOShei/go-fast-core) document model. Standard library only.

```go
import "github.com/AOShei/go-fast-xml" // package gofastxml

doc, err := gofastxml.ExtractFile(ctx, "fiche.xml", nil)
if err != nil { ... }
fmt.Println(doc.Text())
```

**Status: in development (phase 1 of `XML_PLAN.md`).** Any XML document is read generically: its text in document order, as inferred paragraphs. Schema profiles, which map a known schema's elements onto headings, lists and tables, are not built yet (see [Not read](#not-read)).

The document is read as a stream, never as a tree: memory is bounded by the largest block, not by the file. A generated 200 MiB document of small elements is read with a peak heap of about 1 MiB (`TestLargeStream`).

## API

- `Extract(ctx, r io.ReaderAt, size, opts)`, `ExtractFile(ctx, path, opts)`, `ExtractBytes(ctx, data, opts)`.
- `Extractor{Attributes}` implements `gofastcore.Extractor`; its zero value is ready and safe for concurrent use. `Formats()` is `xml`, and `Source.Format` is `xml`, `Source.MIME` `application/xml`.
- `Attributes`: `AttributesNotes` (the default) makes each attribute a note (below); `AttributesNone` leaves attribute notes out, for data-style XML where attributes are keys and codes. Leaving them out raises no warning.

## Command

`go install github.com/AOShei/go-fast-xml/cmd/gofastxml@latest`

```
gofastxml [flags] file-or-directory...
  -format json|markdown|text  -attributes=false  -o file  -quiet  -version
  -furniture  -stored[=kinds]  -part-markers  -children  -exclude-display[=reasons]
  -max-input-bytes N  -max-decoded-bytes N  -max-text-bytes N
```

A directory stands for the files in it and below it whose names end in `.xml` (in any case). A file named on the command line is read whatever its name. With one input the JSON is indented; with several, each document is one line. Documents go to standard output; warnings of severity error go to standard error; the exit status is 1 if any input could not be read, 2 for a usage error.

The render flags are the series' shared ones (`COMMAND_FLAGS.md`), defined by core's `AddRenderFlags`, and affect Markdown and text only; JSON always has everything. This library writes one stored kind, `markup_comment` (the file's comments), so `-stored=markup_comment` puts comments in the text.

## How XML maps

One part of kind `body`, number 1.

- **Text elements.** An element that directly contains character data other than white space is a text element. It gives one `paragraph` block, `Origin: inferred`, whose text is its own character data with that of all its descendants, in document order. CDATA sections and entity replacements are character data.
- **Containers.** An element that holds only elements (and white space) is a container and gives no block of its own.
- **Inline elements.** A text element inside another text element is part of the outer block: `<p>Hello <b>world</b></p>` is one paragraph, `Hello world`. This holds also when the outer element's own text comes after its children (`<p><b>Bold</b> tail</p>`): the children's blocks are taken into it. To keep memory bounded, blocks wait for such a decision only up to 4 MiB or 8,192 blocks; past that, the open elements still undecided are taken for containers, and one that has text after that point gives a block from there on. Nothing is lost either way; only where block boundaries fall differs.
- **White space.** Runs of XML white space (space, tab, line feed, carriage return) collapse to one space, and the text is trimmed. Under `xml:space="preserve"` the white space is kept as written, line breaks as `\n` and tabs included, and only the ends are trimmed; `xml:space="default"` turns it off again for a subtree. Other space characters (a no-break space) are content and kept, though not at the ends of a block.
- **`Attrs["element"]`** on each block made from an element is the path of element names from the root as written in the file, prefixes included, joined with `/` (`Publication/Texte/Paragraphe`), so a consumer can tell `<name>` from `<address>`.
- **Nothing is guessed from names.** No headings, lists, tables or titles are inferred from element names. That is what profiles will be for.
- **Text outside the root element** (which a well-formed file does not have) is a paragraph of its own with no `element` path.
- **Locations.** Every block has `Loc.Offset` and `Loc.Length`: for a paragraph the element's span from its start tag to its end tag, for an attribute note its element's start tag, for a comment the comment. Offsets are bytes in the text decoded to UTF-8: for a UTF-8 file, with or without a byte order mark, that is the file itself; for a file in another encoding they are offsets in its UTF-8 form. A block read from an entity's replacement has the location of the reference. A paragraph split by the block bound has the span of each piece's text. `Loc.Anchor` is the block's element's `xml:id`, else its `id`, else that of an element inside it.
- **Ligatures** U+FB00 to U+FB06 are written as their letters; other characters are kept as the file has them, invisible ones included.

### Attributes

Each attribute with a value that is not empty (after white space is collapsed) is a `note` block of type `attribute` (core's `NoteAttribute`), `Origin: inferred`, with the attribute's name as written under `Attrs["attribute"]` and its element's path under `Attrs["element"]`. It stands where the element starts: an element's attribute notes come before the element's own paragraph. Notes of an inline element's attributes come right after the paragraph they are inside. With `AttributesNone` there are none.

These attributes are not notes:

| Attribute | Goes to |
|---|---|
| `xmlns`, `xmlns:*` | nowhere: they declare namespaces, which are resolved |
| `xml:space` | how white space is read |
| `xml:lang` on the root element | `Metadata.Language` (elsewhere it is a note) |
| `xsi:schemaLocation`, `xsi:noNamespaceSchemaLocation` on the root | `Metadata.Extra` (elsewhere they are notes); never loaded |

### Comments

A comment is a `note` block marked `Attrs["stored"] = "markup_comment"`, as go-fast-html writes HTML comments, at its place. A comment inside a text element follows that element's paragraph. Line breaks in a comment are kept; its ends are trimmed. The renderers leave comments out unless `RenderOptions.Stored` (or `StoredKinds` with `markup_comment`) asks for them.

### Metadata

| Field | From |
|---|---|
| `Language` | `xml:lang` on the root element |
| `Extra["root"]`, `Extra["root_namespace"]` | the root element's name as written and its namespace URI |
| `Extra["schema_location"]`, `Extra["no_namespace_schema_location"]` | `xsi:schemaLocation` (white space collapsed), `xsi:noNamespaceSchemaLocation` on the root |
| `Extra["doctype"]`, `Extra["doctype_public"]`, `Extra["doctype_system"]` | the document type declaration's name and external identifiers |
| `Extra["encoding"]` | the encoding the file was read in |

## Encodings

The encoding is decided as XML 1.0 (section 4.3.3 and appendix F) says: a byte order mark (UTF-8, UTF-16LE, UTF-16BE), else UTF-16 recognised from the first bytes of the XML declaration, else the `encoding` the declaration names, else UTF-8. A declaration that names UTF-16 in a file that reads as ASCII is read as UTF-8; a byte order mark wins over a declaration that disagrees.

UTF-8, UTF-16 and windows-1252 are decoded natively, and US-ASCII and ISO-8859-1 as windows-1252, as the WHATWG Encoding standard and browsers do (the bytes 0x80 to 0x9F are then the characters windows-1252 has there, not C1 controls). Every other label is looked up in a copy of go-fast-html's `internal/charset` (the other single-byte encodings, Shift_JIS, EUC-JP, GBK, GB18030, EUC-KR, Big5). A declared encoding not among them (ISO-2022-JP, UTF-7, EBCDIC), and UTF-32, is `ErrUnsupported`.

A file that declares no encoding and whose first MiB is not valid UTF-8 is read in the encoding recognised from its bytes where go-fast-html's detection is certain, with `text.encoding_detected` naming it; otherwise it is `ErrUnsupported`. Bytes that are not a character in the encoding are written as U+FFFD and counted in `text.unmapped_glyphs`, in UTF-8 files too.

## Entities and document type declarations

- **Never loaded.** No external DTD subset, external entity, parameter entity, schema, stylesheet or XInclude is read or fetched. The ones that would make a conforming processor load something are reported with `input.external_reference`, once per kind with a count: an external DTD subset, a reference to an external entity (or to an external parameter entity in the internal subset), an `xml-stylesheet` instruction, an XInclude `include` element. Schema location hints are only recorded in `Metadata.Extra`.
- **The internal subset** is read for general entity declarations only. Element, attribute-list (default attribute values included) and notation declarations are passed over; parameter entities are never expanded.
- **Internal entities** are expanded where referenced, their replacement read as markup (an entity may hold elements), in text and in attribute values. Nesting is bounded (`MaxEntityDepth`, 16), and so are the bytes all expansions produce together: `EntityExpansionBase` (1 MiB) plus `EntityExpansionRatio` (8) times the input size, and never more than the decode budget (`Limits.MaxDecodedBytes`, shared with embedding documents) has left. A reference past either bound is kept as written, and so is every expanding reference after the first one refused; the part carries `limit.exceeded`. A billion-laughs file and the quadratic-blowup variant finish in milliseconds. A reference to an entity inside its own replacement is kept as written and counted in `input.repaired`.
- **The five predefined entities and character references** always work. A character reference to a control character other than tab, line feed and carriage return is removed with `text.control_characters_removed`; one to a surrogate or beyond U+10FFFF is U+FFFD.
- **Undeclared entities** (`&eacute;` in a DocBook 4 or JATS file whose DTD is not loaded): if the file has an external DTD subset and the name is in the W3C/HTML character entity list (go-fast-html's table), the character is written and the part carries `xml.entities_assumed` (info) with the count. Otherwise the reference stays in the text as written (`&name;`) and the part carries `xml.entity_undeclared` (warning) with the count. A reference to an external entity also stays as written.

## Damaged input

The reader is lenient and counts what it mends in one `input.repaired` (warning) on the document: an end tag that matches an element further out closes the elements inside it; one that matches nothing is dropped; elements open at the end are closed; a `<` or `&` that starts no markup is text; an attribute without a value is passed over and one without quotes runs to white space or `>`; a tag cut off by the next `<` ends there; a comment or CDATA section open at the end runs to it; a second root element and text outside the root are read as they come; a second document type declaration, or one after the root element starts, is not read. Input with no element at all is `ErrMalformed`.

Processing instructions other than the XML declaration and `xml-stylesheet` are not text; they are counted in one `input.unread_content` (warning).

## Warnings

| Code | Severity | Where | What happened |
|---|---|---|---|
| `limit.exceeded` | error | part | a bound cut the output; the message says which and how much |
| `text.unmapped_glyphs` | warning | part | byte sequences not valid in the encoding were written as U+FFFD |
| `text.control_characters_removed` | info | part | control characters other than tab and line feed were removed |
| `text.encoding_detected` | warning | document | no encoding declared, not UTF-8; read in the encoding named |
| `input.repaired` | warning | document | damaged markup read leniently, with the count |
| `input.external_reference` | warning | document | one per kind of reference a conforming processor would load, with the count; nothing was loaded |
| `input.unread_content` | warning | document | processing instructions, with the count |
| `xml.entities_assumed` | info | part | undeclared entities read from the W3C/HTML list, with the count |
| `xml.entity_undeclared` | warning | part | undeclared entities kept as written, with the count |
| `part.no_text` | warning | part | the document has no block |
| `part.failed` | error | part | reading stopped on an I/O error; what was read is kept |

## Errors

`ErrMalformed` (no element), `ErrUnsupported` (an encoding not read), `ErrLimitExceeded` (larger than `Limits.MaxInputBytes`), `ErrNoPartsSelected` (`Options.Parts` excludes part 1), and the context's error when it is cancelled.

## Limits

Core's limits: `Limits.MaxInputBytes` (default 4 GiB, `ErrLimitExceeded`); `Limits.MaxDecodedBytes` (default 1 GiB), which here bounds entity expansion; `Limits.MaxTextBytes` (default 256 MiB): every block takes `BlockCost` plus its text plus its `element` path from it, and when it is spent the part is cut there with `limit.exceeded` saying how much input was left unread.

Fixed bounds of this library, exported as constants. Each cut is `limit.exceeded` (error) on the part, once per bound with a count:

| Bound | Value | What happens past it |
|---|---|---|
| `MaxDepth` | 512 | deeper elements are read as part of their ancestor: their text is kept, their tags and attributes are not read. Nesting never recurses on the Go stack |
| `MaxAttributes` | 1,024 per element | later attributes are not read |
| `MaxTagBytes` | 16 MiB | reading stops at a larger tag, processing instruction or document type declaration |
| `MaxNameBytes` | 256 | an element or attribute name in `Attrs` is cut |
| `MaxPathBytes` | 4 KiB (256 × 16) | `Attrs["element"]` is cut at a `/` |
| `MaxValueBytes` | 1 MiB | an attribute note or anchor is cut |
| `MaxBlockBytes` | 1 MiB | a longer text is split at white space into paragraphs marked `Continued`; nothing is cut and no warning is raised |
| `MaxEntityDepth`, `EntityExpansionBase`, `EntityExpansionRatio` | 16; 1 MiB + 8 × input | see [Entities](#entities-and-document-type-declarations) |

The context is checked every 1,024 tokens. Output is deterministic.

## What the file declares

Generic reading reports no `Display` reason: XML in general has no declaration of display that holds whatever the schema. Nothing about visibility is judged: no colour, size, position or overlap is looked at, and characters are kept as the file has them. Unshown data (comments, attributes) is read as data, never run or followed.

## Not read

- **Profiles** (schema-specific mappings onto headings, lists, tables, metadata, with `native` blocks): not yet; later phases of `XML_PLAN.md`.
- XSD validation, DTD validation, default attribute values from a DTD, XSLT.
- Processing instructions as text.
- External anything (see above).

## Tests

`go test ./...` runs everything; `-short` skips the large worst cases (a million attributes, a 64 MiB text node, a 200 MiB generated stream, a 17 MiB tag). An oracle test compares the character data with `encoding/xml`'s on 500 generated documents. `TestCorpus` reads `GOFAST_CORPUS` or `../.testdata/corpus` and runs every file of `dev/xml*/` and `heldout/xml*/` (counts only for held-out files); it skips when the corpus is absent. Fuzz targets: `FuzzExtract` (.), `FuzzScanner` (`internal/xmlscan`), `FuzzParse` (`internal/dtd`).

`BenchmarkExtract` reads a synthetic 10 MiB document; `BenchmarkEncodingXML` tokenizes the same bytes with `encoding/xml` (tokens only, no document).

## Provenance

See `NOTICE`. The scanner is adapted from go-fast-odf's (itself from go-fast-ooxml); `internal/charset` and `internal/entity` are copies of go-fast-html's, with their tables' provenance.
