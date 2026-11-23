# Changelog

## v0.1.0 (unreleased)

First release. Requires go-fast-core v0.1.3. Standard library only.

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
