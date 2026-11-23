# Changelog

## Unreleased

### Phase 5: fixes after review

- A table is charged core's cell cost for every cell of its padded grid
  as its rows end and as it widens; one the text budget cannot take
  stops with `limit.exceeded` before its rows are padded (20,000 empty
  rows under one 1,024-column row no longer allocate 1.2 GB).
- Repeated values of one `Metadata.Extra` key are joined in linear time.
- Text after a block, list or item read as lines of the paragraph (or
  item, or cell) it stands in starts a new line: `<p>lead<list>...</list>
  tail</p>` no longer runs the last item into "tail".
- An unclosed comment, processing instruction or literal in a DOCTYPE's
  internal subset ends at the subset's `]>` (else at the end of the
  declaration's window, the token limit), with `input.repaired`; the
  document after it is read instead of refused as `ErrMalformed`.
- A paragraph over `MaxBlockBytes` is cut at the last white space already
  read, not where an inline tag starts in the middle of a word (profile
  and generic reading); only a run with no white space is cut where it
  stands.
- Choosing a rule under ancestors with long tested attribute values no
  longer costs time in proportion to those values for every element.

### Phase 4: drafting a profile from an XSD

- `gofastxml -draft-profile schema.xsd [-o draft.json]` and
  `DraftProfile(ctx, path) (*Draft, error)`: read an XML Schema with the
  module's own reader (`internal/xsd`; includes, imports and redefines
  on disk only, nothing fetched, bounded, fuzzed) and write a draft
  profile whose rules are suggestions from the content model, each with
  `"review": true` and a `"note"` saying why. Undecided elements and the
  parts of the schema not read are listed in the draft's `notes`.
- Profile format: top-level `notes` and rule `note` (unread);
  `LoadProfile` refuses a rule with `"review": true`.

### Phase 2: profiles

- Fixes after review: a row's `cells` from its element's attributes and
  a table's fixed `header` (`Attrs["header_source"]="profile"`; the
  table is `Origin: native` since phase 3, every row and cell being
  declared by the file); Service-Public `Redirections` read
  as one table (no unmapped element on dev). Spans kept on each piece of
  a split block. A `part` inside an open block finishes the block first.
  A link's stored record has the link's text, its title in
  `Attrs["title"]` (Service-Public: `sp.comment`).
- Profiles (`PROFILE_FORMAT.md`): JSON files mapping a schema's elements
  to the model. Loader (`internal/profile`) with strict JSON, validation
  whose errors name the rule, selectors compiled once and indexed by the
  last step's name; `LoadProfile`, `BuiltinProfiles`, `BuiltinProfile`,
  `Extractor.Profiles`, `Extractor.Profile` (`ProfileNone`).
- Rule engine (`internal/engine`): every rule kind of the format, value
  sources (attribute, ancestor's attribute, child text, child outline),
  marks, labels, `duplicate_of`, both table models (rows and CALS),
  spans with `Options.Spans`, parts and the stored part, generic fallback
  for unmapped elements; `Loc.Offset`/`Length` on every block,
  `Loc.Anchor` and `Attrs["anchor_parent"]` for citing.
- Keys added to the signed-off format, each for one decision of the
  Service-Public mapping: `duplicate_of`/`key` (S4), `title` and `attrs`
  on `inline` (S6), `extra` (root attributes), `join` (breadcrumb),
  `value` on `block` and `note` (`RessourceWeb@URL`).
- Built-in profile `service-public` (Publication 3.5, both roots and the
  five index files).
- Warnings `xml.unmapped_elements`, `xml.unmapped_attributes`,
  `xml.skipped_elements` (info); `structure.heading_level_clamped`.
- `Metadata.Extra["xml.profile"]` names the profile used.
- Command: `-profile NAME|FILE|none`, `-profiles`, `-spans`.
- Tests: one synthetic profile per shape, `FuzzProfile`, `FuzzLoad`,
  `FuzzSelector`, the Service-Public corpus test and the two oracles over
  every dev file, `BenchmarkServicePublic`.

### Phase 1: reading

- Streaming XML scanner (`internal/xmlscan`), adapted from go-fast-odf's:
  namespace URIs, token offsets, comments, processing instructions and the
  document type declaration as tokens, general entities expanded within a
  bound on nesting and on the bytes produced, recovery from damaged markup.
- `internal/charset` and `internal/entity` copied from go-fast-html.
- Encoding decided from the byte order mark, the first bytes of the XML
  declaration and its `encoding`; streaming decoder to UTF-8.
- Document type declaration: external identifiers and the internal
  subset's general entities; nothing is loaded. Undeclared entities in a
  file with an external DTD subset read as HTML 4's 252 character entities
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
