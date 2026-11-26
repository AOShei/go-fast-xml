# Changelog

## Unreleased

- **A directory given to `gofastxml` stands for `.xlf`, `.xliff` and `.tmx`
  files too**, the names of XLIFF and TMX translation files. A file named
  on the command line is read whatever its name, as before.
- **Built-in profile `eforms`** for EU eForms procurement notices (UBL
  `ContractNotice`, `ContractAwardNotice`, `PriorInformationNotice` with
  the eForms extensions): the title and each lot as headings, descriptions
  as paragraphs, every other value in tables of names and values. See
  `profiles/eforms.md`.
- **Built-in profile `ted`** for TED notices before eForms (`TED_EXPORT`,
  schema R2.0.8 and R2.0.9): coded data, translated titles and form parts
  as tables; in the 2014 forms the title, lots and awards as headings and
  sections; text notices (`OTH_NOT`) as headings, paragraphs and lists.
  See `profiles/ted.md`.
- **Built-in profile `boamp`** for BOAMP, the French procurement bulletin
  (DILA open data: root `ann` of schema 3.2.5, and `ANNONCE` of the
  2015 to 2021 archives). The embedded eForms notice is read as `eforms`
  reads one; `HTML` and `HTMLSYNTHESE` are handed to the child extractor.
  See `profiles/boamp.md`.
- **Built-in profiles `nitf` and `newsml-g2`** for news agency formats:
  NITF 3.x (root `nitf`, in no namespace or the NITF 2006-10-18
  namespace) and IPTC NewsML-G2 (`newsItem`, `packageItem`,
  `newsMessage`, `knowledgeItem`, `conceptItem`, `planningItem`,
  `catalogItem`, `catalog`). An item's XHTML body is handed to the child
  extractor; NITF inside a NewsML-G2 item is read by the NITF rules. See
  `profiles/nitf.md` and `profiles/newsml-g2.md`.
- Profile format: `"pairs": "parent"` names a table's values from the
  element its own element stands in (`ProcurementProject/ID`);
  `"pairs_empty": true` writes an element with no content and no
  attributes as a row with an empty value (`PROCEDURE/PT_OPEN`). No
  earlier profile uses either; the output of every earlier corpus file
  is unchanged.

## v0.3.0 (2026-11-26)

- **New built-in profile `dila-juri`** for DILA's case law: roots
  `TEXTE_JURI_JUDI` (CASS, INCA, CAPP), `TEXTE_JURI_ADMIN` (JADE) and
  `TEXTE_JURI_CONSTIT` (CONSTIT). See `profiles/dila-juri.md`.
- **`legifrance` reads KALI and CNIL**: new roots `TEXTEKALI`, `IDCC` and
  `TEXTE_CNIL`, and rules for the KALI-only elements (`CALIPSO`,
  `HISTORIQUE`, extension dates, signatories' parts, `CONDITION_DIFFERE`,
  `CONTEXTE/CONTENEUR`, ...). The output of every LEGI/JORF file is
  unchanged.
- Engine: several value sources of one rule naming the same `child` all
  get its text; an anchor read from a grandchild reaches the blocks after
  the elements between. The output of every earlier corpus file is
  unchanged.
- **Built-in profile `canada-laws`** for Justice Canada's federal acts,
  regulations and annual statutes (`github.com/justicecanada/laws-lois-xml`,
  English and French): roots `Statute`, `Regulation` and `Bill` with the
  `lims` namespace declared or a Justice Canada DOCTYPE. See
  `profiles/canada-laws.md`.
- **A label written as a paragraph of its own takes its own container's
  attributes.** A waiting label that cannot be the marker of the next
  block (a section's number before a numbered subsection) is a paragraph;
  it took the `attrs`, `mark` values and `anchor_parent` of the element
  opened after it (the subsection), and now takes those of the element
  that holds it. Text is unchanged. On the dev corpus this corrects
  attributes in 32 `clml`, 14 `akn` and 1 `jats` files: a section number's
  `anchor_parent`, URIs and dates are now the section's, not those of its
  first subsection.
- **New built-in profiles `cii` and `ubl`** for electronic invoices
  (EN 16931). `cii`: root `CrossIndustryInvoice` in
  `urn:un:unece:uncefact:data:standard:CrossIndustryInvoice:100` (Factur-X,
  ZUGFeRD 2, XRechnung). `ubl`: roots `Invoice` and `CreditNote` of UBL 2
  (Peppol BIS Billing 3.0). Header values in `Extra` `inv.*`, every value
  written with the name the file gives it: parties and groups as tables
  of names and values (`SellerTradeParty/Name`; `inv.party`, `inv.group`),
  one lines table with `RowLocs` followed by a table of line id, name and
  value for what a line holds beyond its columns, tables for the VAT breakdown, allowances and
  charges, and totals. ZUGFeRD 1.0 (`CrossIndustryDocument`) is read
  generically. See `profiles/cii.md` and `profiles/ubl.md`.
- **Base64 attachments are handed to the child extractor.** A `document`
  rule with `"encoding": "base64"` decodes the element's text and hands
  the bytes to `Options.ExtractChild` under the file's `filename` and
  `mimeCode`; the base64 is not written in the body. Used by `cii` and
  `ubl`. New constant `MaxAttachmentBytes` (64 MiB of base64).
- Profile keys: `column`, `rest`, `one_row` and `with` (tables built from
  nested elements); on `document` rules `encoding` and `name`. A metadata
  date of eight digits (`YYYYMMDD`) fills `Created`/`Modified`. A table
  holding only the profile's header row makes no block.
- Profile keys `pairs` (a table of names and values: each value of an
  element written with the file's own element path, attributes as `/@`
  rows, `RowLocs` on the value's element) and `rest_table` (what a row
  holds beyond its column cells, as a table of id, name and value).
- **New built-in profiles `tmx`, `xliff` and `xliff2`** for translation
  files. `tmx`: root `tmx` with a `version` attribute; one table per
  body, a row per `tu`, a column per language as written. `xliff`: XLIFF
  1.0 to 1.2; `xliff2`: XLIFF 2.0 to 2.2; a section with a heading and a
  table per `file`, a row per `trans-unit` or per `segment` and
  `ignorable`, with `RowLocs` and the unit's id as anchor. See
  `profiles/tmx.md`, `profiles/xliff.md` and `profiles/xliff2.md`.
- **Fixed: table row locations in files not in UTF-8.** `Table.RowLocs`
  held offsets in the decoded text, not bytes of the file, when the file
  was in UTF-16, windows-1252 or another encoding; blocks' own locations
  were right. In every profile. On the dev corpus only TMX files are
  affected.
- Profile keys: `column_key` (a cell's column chosen by an attribute's
  value, the value being the header cell); `heading` on a `section` (a
  heading from an attribute of the section's element); `table_header` on
  a `section` (the section is also a table); a `cells` entry may read a
  child's or an ancestor's attribute; a row with `cells` and `rest` stays
  open for its content; a row of attributes with none present makes no
  row.
- **New built-in profile `pubmed`** for PubMed / MEDLINE citation sets:
  root `PubmedArticleSet`. A section per record with its PMID as heading
  and anchor, the title, the abstract's labelled parts, an authors table,
  keywords and references as lists, and every identifier, date and code
  in tables of names and values. See `profiles/pubmed.md`.
- Profile keys: `"heading": "anchor"` on a `section` (the child its
  anchor is read from is its heading too); the value source `anchor_of`;
  a row's own attributes are rows of its `rest_table`.

## v0.2.1 (2026-11-25)

- **A directory given to `gofastxml` stands for `.rss` and `.atom` files
  too** (in any case), as the `gofast` command's does since go-fast v0.1.2.
  The walk takes `.xml`, `.dita`, `.ditamap`, `.docbook`, `.tei`, `.rss`
  and `.atom`. A file named on the command line is read whatever its name,
  as before.

## v0.2.0 (2026-11-24)

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
