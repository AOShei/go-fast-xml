# Changelog

## Unreleased

- **`xml.unmapped_attributes` no longer counts attributes that are in the
  output.** An element a profile has no rule for is read generically, so
  its attributes are notes; they were also counted in this warning, whose
  meaning is "values not in the output". Now it counts only attributes of
  mapped elements that no rule reads. The element is still named in
  `xml.unmapped_elements`. No block or text changes; only this warning's
  message and whether it is raised.
- **`gofastxml` on a directory** also reads files named `.dita`,
  `.ditamap`, `.docbook` and `.tei` (in any case), as the `gofast` command
  does. It read `.xml` only.
- **Web feeds: a thirteenth built-in profile, `feed`**, for RSS 0.91 to
  2.0, RSS 1.0 and Atom 1.0 with their common extensions (Dublin Core,
  `content:`, iTunes, Podcasting 2.0, Media RSS, GeoRSS, Slash,
  syndication). Feeds were read generically. See `profiles/feed.md`.
- **An anchor that comes late reaches the blocks before it.** When a
  profile reads an element's anchor from a child (an RSS item's `guid`)
  and blocks of the element were made before it, those blocks get
  `Attrs["anchor_parent"]` too. In every profile, a user's own included.
  The output of the built-in profiles other than `feed` is unchanged on
  the corpus: none reads an anchor from a child.
- **Embedded documents: comments and empty sections.** A comment inside a
  `document` element is the child's, a note of the parent only when the
  text stays in the body. A section that makes no block writes what it
  read for its first block and anchor as blocks of its own (profiles with
  a `document` rule). The fallback text of HTML with element children
  closes its start tags. RSS `textInput` is mapped.
- **Embedded documents.** A new rule kind, `"as": "document"`, with
  `media_type` and `when`, hands an element's content to
  `Options.ExtractChild` and appends the result to `Document.Children`
  (escaped HTML in a feed was literal markup in a paragraph). The section's
  heading carries `Attrs["children"]`; each child carries
  `xml.parent_offset`, `xml.parent_length`, `xml.parent_anchor` and
  `xml.parent_element`. Without a child extractor the text stays in the
  body with `Attrs["media_type"]`. At most `MaxChildren` (10,000) per
  document. A `match` entry may test `declares`.
- **Dates in RFC 822 form** with a numeric offset or `GMT`, `UT`, `Z` fill
  `Metadata.Created` and `Modified` from a profile's `created` and
  `modified` fields; they stayed as written in `Extra`.
- **`gofastxml` reads HTML and XHTML children with go-fast-html.** The
  module now requires go-fast-core v0.1.6 and go-fast-html v0.4.4 (the
  command's dependency; the library package imports only go-fast-core).

## v0.1.0 (2026-11-24)

First release. Requires go-fast-core v0.1.4. Standard library only.

### Reading

- Streaming XML reader: the document is read as a stream of tokens,
  never as a tree; memory is bounded by fixed bounds, not by the file.
  Namespaces, offsets, comments, processing instructions and the
  document type declaration; lenient reading of damaged markup, counted
  in `input.repaired`.
- Encodings: byte order mark, the XML declaration, UTF-8, UTF-16,
  windows-1252 and the encodings of go-fast-html's `internal/charset`;
  detection when no encoding is declared and the text is not UTF-8
  (`text.encoding_detected`); `Extractor.Charset` and `-charset` give
  the encoding from outside the file. Offsets are bytes of the file in
  every encoding.
- Entities: the internal subset's general entities, expanded within
  bounds on nesting and on the bytes produced; undeclared entities in a
  file with an external DTD subset read as HTML 4's character entities
  (`xml.entities_assumed`), otherwise kept as written
  (`xml.entity_undeclared`). Nothing external is loaded or fetched:
  external DTDs and entities, stylesheets and XInclude are reported with
  `input.external_reference`.
- Generic reading for documents with no profile: text elements as
  inferred paragraphs with their element path, attributes as notes
  (`Extractor.Attributes`, `-attributes=false`), comments as
  `markup_comment` notes, metadata from the root element and the
  document type declaration.

### Profiles

- Profile format 1 (`PROFILE_FORMAT.md`): JSON files that map a
  schema's elements onto headings, paragraphs, lists, tables (rows and
  CALS models), links, notes, images, parts, metadata and the stored
  part, with value sources from attributes, ancestors and children,
  marks for conditional content, `split` and `out_of_line` for blocks
  met inside a paragraph, line breaks, and an inline's `value` written
  in place (including a hex code point). Loading is strict and errors
  name the rule. `LoadProfile`, `BuiltinProfiles`, `BuiltinProfile`,
  `Extractor.Profiles`, `Extractor.Profile`, `-profile`, `-profiles`.
- Twelve built-in profiles: Akoma Ntoso, HL7 CDA, CLML, DITA, DocBook,
  Formex, JATS, Légifrance, patents (USPTO and EPO), S1000D,
  Service-Public.fr and TEI, each with a note in `profiles/`.
- Citing: `Loc.Offset`/`Length` on every block, `Loc.Anchor`,
  `Attrs["anchor_parent"]`, `Attrs["attribute"]` for text from an
  attribute, and `Table.RowLocs` for every row of every table.
- Warnings `xml.unmapped_elements`, `xml.unmapped_attributes`,
  `xml.skipped_elements` (info); core's
  `structure.heading_level_clamped` and `input.unread_content` (MathML).
- `gofastxml -draft-profile schema.xsd` and `DraftProfile`: a draft
  profile from an XML Schema, read with the module's own reader
  (includes and imports on disk only), every rule marked for review.

### Limits and tests

- Core's limits, and fixed bounds on depth, attributes, tag size,
  names, paths, values, block size, entity expansion, waiting blocks,
  table columns, row spans, spans and parts; every cut is
  `limit.exceeded`.
- Command `gofastxml` with core's render flags.
- Tests: an oracle against `encoding/xml` on generated documents; for
  each built-in profile a corpus test and two oracles over its dev
  files (text in file order, text at its offsets); a corpus-wide check
  of row locations; worst cases; output digests; benchmarks; fuzz
  targets for the reader, the scanner, the DTD, the charsets, the
  profile loader and selectors, the rule engine, the XSD reader and the
  drafter.
