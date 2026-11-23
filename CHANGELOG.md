# Changelog

## Unreleased

- Streaming XML scanner (`internal/xmlscan`), adapted from go-fast-odf's:
  namespace URIs, token offsets, comments, processing instructions and the
  document type declaration as tokens, general entities expanded within a
  bound on nesting and on the bytes produced, recovery from damaged markup.
- `internal/charset` and `internal/entity` copied from go-fast-html.
- Encoding decided from the byte order mark, the first bytes of the XML
  declaration and its `encoding`; streaming decoder to UTF-8.
- Document type declaration: external identifiers and the internal
  subset's general entities; nothing is loaded. Undeclared entities in a
  file with an external DTD subset read from the W3C/HTML list
  (`xml.entities_assumed`), otherwise kept as written
  (`xml.entity_undeclared`).
- Generic reading: text elements as inferred paragraphs with their
  element path, attributes as notes (`Extractor.Attributes`,
  `-attributes=false`), comments as `markup_comment` notes, metadata from
  the root element, `Loc.Offset`/`Length`/`Anchor`.
- Limits: core's, plus fixed bounds on depth, attributes, tag size, names,
  paths, values, block size (split, `Continued`) and entity expansion.
- Command `gofastxml` with core's render flags.
- Tests: oracle against `encoding/xml`, worst cases, fuzz targets, corpus
  test, benchmark.
