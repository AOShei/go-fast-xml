# go-fast-xml

Extracts XML documents of any schema into the shared [go-fast-core](https://github.com/AOShei/go-fast-core) document model. Standard library only.

```go
import "github.com/AOShei/go-fast-xml" // package gofastxml

doc, err := gofastxml.ExtractFile(ctx, "fiche.xml", nil)
if err != nil { ... }
fmt.Println(doc.Text())
```

A document whose schema has a **profile** is read with it: the profile maps the schema's elements onto headings, paragraphs, lists, tables, links, notes and metadata (see [Profiles](#profiles)). Twenty-six profiles are built in: Akoma Ntoso, BOAMP (French procurement notices), Canadian federal laws (Justice Canada), HL7 CDA, Cross Industry Invoice (Factur-X, ZUGFeRD 2, XRechnung), CLML (legislation.gov.uk), DILA case law, DITA, DocBook, EU eForms procurement notices, web feeds (RSS and Atom), Formex (EUR-Lex), JATS, Légifrance, NewsML-G2 and NITF (news agencies), patents (USPTO and EPO), PubMed citations, S1000D, Service-Public.fr, TED notices before eForms, TEI, TMX translation memories, UBL invoices (Peppol) and XLIFF (1.x and 2.x). A profile can hand an element's content to a child extractor as an embedded document (escaped HTML in a feed, an invoice's attachments, the XHTML body of a news item, a BOAMP notice's HTML page; see [Embedded documents](#embedded-documents)). A profile is a JSON file, so other schemas can be given one without code, and `-draft-profile` drafts one from an XSD. Any other XML document is read generically: its text in document order, as inferred paragraphs, with its attributes and comments as notes.

The document is read as a stream, never as a tree: besides the document it returns, memory is bounded by the fixed bounds below (one tag, one block, the blocks waiting for an undecided element, the namespace declarations in scope), not by the file. For input not read as UTF-8, the map back to the file's bytes adds 16 bytes per 32 KiB of input. A generated 200 MiB document of small elements is read with a peak heap of about 1 MiB (`TestLargeStream`).

## API

- `Extract(ctx, r io.ReaderAt, size, opts)`, `ExtractFile(ctx, path, opts)`, `ExtractBytes(ctx, data, opts)`.
- `Extractor{Attributes, Profiles, Profile, Charset}` implements `gofastcore.Extractor`; its zero value is ready and safe for concurrent use. `Formats()` is `xml`, and `Source.Format` is `xml`, `Source.MIME` `application/xml`.
- `Attributes`: `AttributesNotes` (the default) makes each attribute a note (below); `AttributesNone` leaves attribute notes out, for data-style XML where attributes are keys and codes. Leaving them out raises no warning. It applies to generic reading.
- `Extractor.Profiles []*Profile`: profiles tried, in order, before the built-in ones. `Extractor.Profile string`: force the profile of that name (from `Profiles` or built in) whatever the root element; `ProfileNone` (`"none"`) forces generic reading; an unknown name is an error.
- `LoadProfile(io.Reader) (*Profile, error)` reads and validates a profile; `BuiltinProfiles()` lists the built-in ones in the order they are tried, `BuiltinProfile(name)` returns one. A `Profile` (`Name()`, `Title()`) is immutable and safe for concurrent `Extract` calls.
- `Extractor.Charset`: the encoding of the input given from outside the file (see [Encodings](#encodings)).
- `DraftProfile(ctx, path) (*Draft, error)` drafts a profile from an XSD (see [Drafting a profile](#drafting-a-profile-from-an-xsd)).
- Embedded documents: `AttrChildren` (`children`), `AttrMediaType` (`media_type`), and the keys of each child's `Metadata.Extra` `ExtraParentOffset` (`xml.parent_offset`), `ExtraParentLength` (`xml.parent_length`), `ExtraParentAnchor` (`xml.parent_anchor`), `ExtraParentElement` (`xml.parent_element`); `MaxChildren` (10,000), `MaxAttachmentBytes` (64 MiB). See [Embedded documents](#embedded-documents).

## Embedded documents

A profile's `document` rule (PROFILE_FORMAT.md section 3) says that an element's content is a document of its own: an RSS `description` holding escaped HTML, an Atom `content type="xhtml"`. The content is handed to `Options.ExtractChild` with the context of the reading, so the child shares the text and decode budgets and the cancellation, and the result is appended to `Document.Children`, in file order. Each text is handed over and dropped at once.

- **What is handed over.** `text/html; charset=utf-8`: the element's character data, unescaped (entities and CDATA resolved), as UTF-8. `application/xhtml+xml; charset=utf-8`: the markup between the start and end tag, the file's bytes when the file is UTF-8, else written back from the decoded text; namespace declarations made outside the element are not added. The bytes count against `Limits.MaxDecodedBytes` before the child reads; at most `MaxBlockBytes` (1 MiB) of one element is handed over, a longer content cut there with `limit.exceeded`.
- **The two links.** The heading of the section the element stands in (an item's title; else the section's first block) carries `Attrs["children"]`: the indexes in `Document.Children` of every document from that section, comma separated. Each child carries `xml.parent_offset` and `xml.parent_length` (the element's bytes in the file, start tag to end tag, as a block's `Loc`), `xml.parent_anchor` (the item's anchor, when it has one) and `xml.parent_element` (the element's path) in `Metadata.Extra`, and its `Source.Name` is short: `item/description#<guid>`. A section that ends with no block at all (an item whose only content was handed over) has no `children` attribute: then the child's own keys are the link.
- **Citing text of a child.** A block of a child is cited by the child's own `Loc` (offsets in the text handed over) within the element at `xml.parent_offset` of the feed file, and by `xml.parent_anchor`.
- **Without a child extractor** (no `Options.Child`, the depth limit, or the child extractor refused or failed), the child is still listed (the stub `ExtractChild` returns, with its `child.*` warning and the keys above), and the text is also kept in the body as a paragraph with `Attrs["media_type"]`: the markup as text for HTML, the text content for XHTML. No text is lost when go-fast-xml is used alone. When the child was extracted, the body has no block for the element: no text is in the output twice.
- Past `MaxChildren` (10,000) documents, the text stays in the body as that paragraph, with `limit.exceeded` once saying how many were not handed over. `Options.Parts` leaving a part out leaves its children out.
- **Base64 attachments** (a `document` rule with `"encoding": "base64"`: CII `AttachmentBinaryObject`, UBL `EmbeddedDocumentBinaryObject`). The text is decoded and the bytes handed over under the name and media type the file gives (`filename`, `mimeCode`; `application/octet-stream` when it gives none). The base64 is never written in the body: a child that was not extracted is listed, as core's stub, with the same keys and link. Text that is not valid base64 stays in the body as a paragraph with `Attrs["media_type"]`, with `input.stream_damaged`. At most `MaxAttachmentBytes` (64 MiB) of base64 is read for one attachment, and one attachment is held at a time; a longer one, or one past `MaxChildren`, is not handed over and not written, and `limit.exceeded` says how many.
- **Comments inside the element** are the child's: in the bytes handed over, and no note of the parent. When the text stays in the body, each is a note after its paragraph, as anywhere else.
- **A section that makes no block** (an RSS item with only a `link` or a `guid`), in a profile with a `document` rule, writes the values it read for its first block and its anchor as blocks of their own: one read from an element as the block that element's rule makes, one read from an attribute as an attribute note.
- **The command** reads `text/html` and `application/xhtml+xml` children with go-fast-html (its `Options.Child`); every other type is refused with `ErrUnsupported`, so an invoice's PDF attachment is listed, not read. This is why the module's `go.mod` requires go-fast-html; the library package imports only go-fast-core and the standard library. `-children=false` leaves children out of Markdown and text; JSON always has them.

## Command

`go install github.com/AOShei/go-fast-xml/cmd/gofastxml@latest`

```
gofastxml [flags] file-or-directory...
  -format json|markdown|text  -attributes=false  -o file  -quiet  -version
  -profile NAME|FILE|none  -profiles  -spans  -charset NAME
  -draft-profile schema.xsd
  -furniture  -stored[=kinds]  -part-markers  -children  -exclude-display[=reasons]
  -max-input-bytes N  -max-decoded-bytes N  -max-text-bytes N
```

A directory stands for the files in it and below it whose names end in `.xml`, `.dita`, `.ditamap`, `.docbook`, `.tei`, `.rss`, `.atom`, `.xlf`, `.xliff`, `.sdlxliff`, `.mxliff`, `.mqxliff` or `.tmx` (in any case). A file named on the command line is read whatever its name. With one input the JSON is indented; with several, each document is one line. Documents go to standard output; warnings of severity error go to standard error; the exit status is 1 if any input could not be read, 2 for a usage error.

The render flags are the series' shared ones (`COMMAND_FLAGS.md`), defined by core's `AddRenderFlags`, and affect Markdown and text only; JSON always has everything. This library writes one stored kind, `markup_comment` (the file's comments), so `-stored=markup_comment` puts comments in the text.

`-profile` takes the name of a built-in profile, a profile file, or `none`; `-profiles` lists the built-in profiles (name and title); `-spans` sets `Options.Spans` (inline styles and links in the JSON); `-charset` sets `Extractor.Charset` for every input; `-attributes=false` sets `AttributesNone`; `-max-input-bytes`, `-max-decoded-bytes` and `-max-text-bytes` set core's limits (0 the default, -1 no limit); `-quiet` says nothing about files that were read; `-version` prints the version. `-draft-profile schema.xsd` writes a draft profile (to `-o` or standard output) and reads no document.

## Profiles

A profile is a JSON file that says what a schema's elements mean. How to write one: [PROFILE_FORMAT.md](PROFILE_FORMAT.md).

**How one is chosen.** When the root element starts, the profiles of `Extractor.Profiles` are tried in order, then the built-in ones in the order `BuiltinProfiles()` and `gofastxml -profiles` list them (by name: `akn`, `boamp`, `canada-laws`, `cda`, `cii`, `clml`, `dila-juri`, `dita`, `docbook`, `eforms`, `feed`, `formex`, `jats`, `legifrance`, `newsml-g2`, `nitf`, `patent`, `pubmed`, `s1000d`, `service-public`, `ted`, `tei`, `tmx`, `ubl`, `xliff`, `xliff2`); the first whose `match` the root meets (local name, namespace, optionally a DOCTYPE public identifier prefix and attribute tests) is used, else the document is read generically. `Extractor.Profile` / `-profile` forces one, or `none`. `Metadata.Extra["xml.profile"]` names the profile used; it is absent for generic reading.

**What a profile gives.** Blocks of every kind core has for text (headings with levels from nesting, paragraphs, quotes, code, captions, footnotes, formulas, notes, list items with depth and `Marker`, tables in the rows or CALS model with header rows and merges, images), inline styles and links as `Spans` with `Options.Spans`, metadata, parts, and the stored part. Blocks a rule makes are `native`; what the profile does not map is read generically (`inferred`) and counted. A profile only chooses how the file's text is cut and labelled: it cannot add words, drop text or evaluate a condition.

**How to cite a block.** Every block has `Loc.Offset` and `Loc.Length`, the bytes of the element it was made from in the file, from its start tag to its end tag (a heading: its title element; a table: the table element; an item: the item element; stray text: the text). A table also says where each row came from: `Table.RowLocs` has one location per row, the row element's bytes and its anchor if the profile gives rows one (empty for a header row the profile supplies), so a single row can be cited. `Loc.Anchor` is the identifier the source gives the block (the rule's `anchor`, else `xml:id`); a block without one carries the nearest enclosing identified element's in `Attrs["anchor_parent"]` (`AttrAnchorParent`), so a citation can say "fiche F1199, chapter X" without offsets. The document itself is cited by `Source.Name` and the identifiers each profile copies to `Metadata.Extra` (below). Text moved out of the body (metadata, `Attrs` values, the stored part) has no offsets of its own; a block whose text is an attribute's value says so in `Attrs["attribute"]`. An inline rule's `value` (TEI `pb@n`, Formex `QUOT.START@CODE`) is written in the block's text where its element stands: that text is in the block's span as the attribute, not as character data. Tests check, for every block of every dev file of every profile's corpus, that its text is in the source at its offsets, and that every row location lies inside its table and starts at a tag.

**Warnings of profile reading** (all `info`, on the last part, names with counts, at most 100 names listed):

| Code | What happened |
|---|---|
| `xml.unmapped_elements` | Elements the profile has no rule for were read generically |
| `xml.unmapped_attributes` | Attributes of mapped elements that no rule reads (`element@attribute`); their values are not in the output |
| `xml.skipped_elements` | Elements a user profile's `skip` rules left out |

Core's `structure.heading_level_clamped` (headings nested deeper than 6), `input.unread_content` (MathML in a formula rule, kept as plain text) and `limit.exceeded` (below) are raised as core defines them.

### The built-in profiles

Each built-in profile has a full note in `profiles/<name>.md`: the mapping table, the `Extra` and `Attrs` keys, how to cite, what is left to generic reading and the versions tested. The sections below summarise them; the note is where the mapping is written down. Every profile's own keys carry its prefix, so they cannot collide with core's keys or another profile's. Every built-in profile reads all the text of its documents; what it has no rule for is read generically and counted in `xml.unmapped_elements`. Notes on each profile's dev set (unmapped elements and attributes) are in its note.

#### Akoma Ntoso (`akn`)

Selected by the root `akomaNtoso` in the namespace of Akoma Ntoso 3.0, its draft `3.0/CSD03`, or `akomantoso.org/2.0` (read with the 3.0 rules). Maps the document types, the hierarchy of legislation (parts, chapters, articles, paragraphs, points) with `num` as `Marker` and `heading` as heading, `authorialNote` as out-of-line footnotes, tables, quoted structures, the FRBR metadata of `meta` (`Extra` `akn.*`) and the `lifecycle`, `workflow` and `references` tables. Keys: `akn.*`. Cite: `Loc.Anchor` is `@eId` (or `@id` in 2.0), `anchor_parent` the enclosing provision's. Tested: 3.0 (legislation.gov.uk, Fedlex, OASIS examples), 3.0/CSD03 (Italian Senate), 2.0 (Bungeni); not tested: the 1.0 namespace and the `amendmentList`, `officialGazette`, `portion` and `statement` documents. [profiles/akn.md](profiles/akn.md)

#### BOAMP (`boamp`)

Selected by the root `ann` in `http://boamp.journal-officiel.gouv.fr/XML/3.2.5` (DILA's open data of French public procurement notices, schema 3.2.5) and by the root `ANNONCE` in no namespace (the archives of schema 2.3). `GESTION` and `DONNEES` are tables of names and values with the file's own names (`REFERENCE/IDWEB`, `INDEXATION/RESUME_OBJET`); an element whose presence is the value (`OUVERT`, `OPTIONS_NON`) is a row with an empty value. The notice's title is `Metadata.Title` and a level-1 heading, each lot a section. An eForms notice inside `DONNEES/EFORMS` is read as `eforms` reads one. `HTML` and `HTMLSYNTHESE`, the notice as a page, are embedded documents handed to the child extractor as `text/html`. Keys: `boamp.*`. Tested: 201 dev files (November 2026 and January 2019); the archives before 2015 are untested. [profiles/boamp.md](profiles/boamp.md)

#### Canadian federal laws (`canada-laws`)

Selected by the roots `Statute`, `Regulation` and `Bill` (no namespace) when the root declares the `lims` namespace `http://justice.gc.ca/lims` or the file has a `-//Justice Canada//` DOCTYPE (the annual statutes); a `Statute` with neither is read generically. Maps the identification (long title as `Metadata.Title` and a heading, the other fields to `Extra` `lims.*`, `xml:lang` as `Metadata.Language`), `Heading` at its declared `@level`, sections and their subdivisions with `Label` as `Marker`, marginal notes as headings, definitions with both defined terms inline, historical notes, footnotes, schedules, CALS tables, lists, formulas, images (`@source`, nothing fetched), amending text (marked) and recent amendments (a table). The `lims:` dates and ids are kept as written in `Attrs`; in-force state is reported as declared, never judged. Keys: `lims.*` (metadata), `lims:*` and `canada.*` (`Attrs`). Cite: `Loc.Anchor` from `lims:id`, `anchor_parent` the enclosing provision's. Tested: consolidated acts and regulations in English and French, annual statutes 2001 to 2024; not tested: MathML, `TableOfProvisions`. [profiles/canada-laws.md](profiles/canada-laws.md)

#### HL7 CDA (`cda`)

Selected by the root `ClinicalDocument` in `urn:hl7-org:v3` (a `ClinicalDocument` with no namespace is read generically). Maps the header (patient, author, custodian, participants, dates, codes) to `Metadata` and `Extra` `cda.*`, sections with their titles, their `code` and `templateId` as attribute notes, narrative blocks (paragraphs, lists, tables, `content` styles, `linkHtml`), and `nonXMLBody`. Structured entries are read generically, every value an attribute note, recorded as written, never interpreted. Keys: `cda.*`. Cite: `Loc.Anchor` from `@ID`. Tested: CDA R2 with C-CDA R1.1 and R2.1 templates, structured and unstructured; not tested: CDA R2.1 or R3, signature data. [profiles/cda.md](profiles/cda.md)

#### Cross Industry Invoice (`cii`)

Selected by the root `CrossIndustryInvoice` in `urn:un:unece:uncefact:data:standard:CrossIndustryInvoice:100` (Factur-X, ZUGFeRD 2, XRechnung CII, EN 16931). Number, type, currency, guideline in `Extra` `inv.*`; the issue date in `Metadata.Created` (format 102). Every value is written with the name the file gives it: parties, references and payment groups are tables of names and values (the file's element paths, `SellerTradeParty/PostalTradeAddress/CountryID`; attributes as `/@` rows) carrying `inv.party` or `inv.group`; the lines are one table (a row per `IncludedSupplyChainTradeLineItem`, `RowLocs`, a profile header row), and what a line holds beyond its columns is a table of line id, name and value after it; the VAT breakdown, allowances and charges and totals are tables. Amounts keep `currencyID`, codes their `schemeID`/`listID`. Attachments (`AttachmentBinaryObject`) are decoded and handed to `Options.ExtractChild` under their `filename` and `mimeCode`. ZUGFeRD 1.0 (`CrossIndustryDocument`) is read generically. Keys: `inv.*`. [profiles/cii.md](profiles/cii.md)

#### CLML (`clml`)

Selected by the root `Legislation` in the namespace `http://www.legislation.gov.uk/namespaces/legislation`. Maps primary and secondary prelims, the body's parts, chapters, cross-headings and provisions with `Pnumber` as `Marker`, footnotes out of line, tables (XHTML), schedules, explanatory notes and commentaries as parts (each `Commentary` a footnote), and the `ukm:` metadata. Keys: `clml.*`. Cite: `Loc.Anchor` from `@id` (`section-1`), `anchor_parent` the enclosing provision's. Tested: CLML schema 2.0, primary, secondary and EU retained legislation; not tested: `Image`/`Figure`, `Formula`, `Form`. [profiles/clml.md](profiles/clml.md)

#### DITA (`dita`)

Selected by the DITA topic and map roots with a DITA DOCTYPE public identifier or `ditaarch:DITAArchVersion`, and, with neither, by the root and the attribute DITA requires (rules go by element name: `@class` usually comes from DTD defaults, which are not read). Maps topics and their bodies, sections, steps, tables (CALS and `simpletable`), notes and maps (`topicref` items, their `href` as text when they have none). The filtering attributes on content (`platform`, `product`, `audience`, `otherprops`) are not read: they are counted in `xml.unmapped_attributes`, and nothing is filtered. Keys: `dita.*`. Cite: `Loc.Anchor` from `@id`. Tested: DITA 1.3 and DITA-OT documentation and tests; not tested: DITA 2.0, Lightweight DITA. [profiles/dita.md](profiles/dita.md)

#### DILA case law (`dila-juri`)

Selected by the roots in no namespace `TEXTE_JURI_JUDI` (Cour de cassation, CASS and INCA, and courts of appeal, CAPP), `TEXTE_JURI_ADMIN` (Conseil d'État and administrative courts, JADE) and `TEXTE_JURI_CONSTIT` (Conseil constitutionnel). Maps the decision's title to `Metadata.Title` and heading 1, its date to `Metadata.Created`, and jurisdiction, numbers, ECLI, formation, solution, publication, president, rapporteur, advocates as published to `Extra` `juri.*`, as written; the text (`CONTENU`, `br` as line breaks, paragraphs, tables, inline markup), the summary (`SCT` titrage and `ANA` analyses, one paragraph each, marked `juri.part`), the case law cited (`CITATION_JP`), referrals and observations (`SAISINES`, `OBSERVATIONS`), marked `juri.field`, and links (`LIEN`) one paragraph each with every attribute, as `legifrance` reads them. Keys: `juri.*`. Cite: the `ID` (`JURITEXT...`, `CETATEXT...`, `CONSTEXT...`) in `Extra["juri.id"]` and on every block as `anchor_parent`; the ECLI in `Extra["juri.ecli"]`. Tested: the 951 dev decisions of the 2026-11-08 dumps. [profiles/dila-juri.md](profiles/dila-juri.md)

#### DocBook (`docbook`)

Selected by DocBook 5 roots in `http://docbook.org/ns/docbook`, and DocBook 4 roots with no namespace and a DocBook DOCTYPE public identifier (or, with none, a few roots no other schema uses). Maps books, chapters and sections, `para`, lists, CALS and HTML tables, admonitions, figures and examples, `programlisting` as code, footnotes (split from their paragraph), glossaries, `refentry`, and `info` metadata. An `imageobject`'s `condition` is recorded in `Attrs["db.condition"]`, never evaluated. Keys: `db.*`. Cite: `Loc.Anchor` from `xml:id` (or `id` in DocBook 4). Tested: DocBook 5.0 to 5.2 and 4.1.2 to 4.5; not tested: SGML DocBook, `assembly`. `profiles/docbook.json` is generated by a test (`go test -run TestDocbookGenerated -docbook.update`). [profiles/docbook.md](profiles/docbook.md)

#### EU eForms notices (`eforms`)

Selected by the roots `ContractNotice`, `ContractAwardNotice` and `PriorInformationNotice` in their UBL namespaces (the public procurement notices TED publishes since 2023, eForms SDK 1.x). The notice's title is `Metadata.Title` and a level-1 heading, its description paragraphs; each lot is a section whose identifier (`LOT-0001`) is a level-2 heading and the anchor, its title a level-3 heading. Everything else (organisations, terms, criteria, results, tenders, contracts) is tables of names and values with the file's own element paths (`LotResult/TenderResultCode`, `.../@listName`), a row per value with `RowLocs`; codes are never interpreted. The notice's identifier, type, version and dates are in `Extra` `eforms.*`. The eForms field identifiers (`BT-21`) are not in the files and are not added. Tested: 522 dev files (SDK examples and one day of TED, 24 languages). [profiles/eforms.md](profiles/eforms.md)

#### Web feeds (`feed`)

Selected by the root `rss` (no namespace; RSS 0.91 to 2.0), `feed` in `http://www.w3.org/2005/Atom`, and `RDF` in the RDF namespace when it declares `http://purl.org/rss/1.0/` (RSS 1.0; any other RDF file is read generically). The channel or feed title is `Metadata.Title` and a level-1 heading; each `item` or `entry` is a section with its title as a level-2 heading. Escaped HTML is handed to the child extractor: RSS `description`, `content:encoded`, `itunes:summary` and `media:description` when they hold markup (else they are paragraphs), Atom `content`, `summary`, `subtitle` and `rights` by their `type` (`html`, `xhtml`; `text` or none is a paragraph as written). Links, dates, the guid and podcast data are attributes of the item's heading (`feed.link`, `feed.published`, `feed.itunes_duration`, ...) or paragraphs and notes marked `feed.field`; enclosures, Atom links and Media RSS content are attribute notes keeping every attribute. Keys: `feed.*`. Cite: the item's anchor is its `guid`, Atom `id` or `rdf:about`, on every block as `anchor_parent`. Tested: the 76 dev feeds (RSS 2.0, RSS 1.0, Atom, podcasts). [profiles/feed.md](profiles/feed.md)

#### Formex (`formex`)

Selected by the Formex 4 roots in no namespace (`ACT`, `ANNEX`, `DOC`, `PUBLICATION`, `CONS.ACT`, `CONS.ANNEX`, `CONS.OTH`, `GENERAL`). Maps the bibliographic record (`BIB.INSTANCE`) to `Metadata` and `Extra` `fx.*`, the preamble, enacting terms and final part as parts, recitals, articles and paragraphs with their numbers as `Marker`, lists, tables, notes out of line, and the quotation marks `QUOT.START`/`QUOT.END` written from their `CODE`. Keys: `fx.*`. Cite: `Loc.Anchor` from `IDENTIFIER` (`001.002`), notes from `NOTE.ID`; the Official Journal reference in `Extra`. Tested: Formex 05.20 to 05.59; not tested: consolidated texts, `GENERAL`, Formex 6. [profiles/formex.md](profiles/formex.md)

#### JATS (`jats`)

Selected by the root `article` with a NLM or NISO DOCTYPE public identifier or an `article-type` or `dtd-version` attribute, and by `pmc-articleset`. Maps `front` (title, authors, affiliations, dates, identifiers, abstract) to `Metadata` and `Extra` `jats.*`, `body` sections, paragraphs, lists, figures and tables (`table-wrap`, split from their paragraph), formulas, `back` (references one item each, footnotes, acknowledgements). Keys: `jats.*`. Cite: `Loc.Anchor` from `@id`; the DOI and PMID in `Extra`. Tested: JATS 1.1 Archiving, NLM 3.0, PMC article sets; not tested: JATS 1.2 to 1.4 Authoring, BITS. [profiles/jats.md](profiles/jats.md)

#### Légifrance (`legifrance`)

Selected by the LEGI and JORF roots in no namespace (`ARTICLE`, `TEXTE_VERSION`, `SECTION_TA`, `TEXTELR`, `JO`, `CONTENEUR`), KALI's (the same, and `TEXTEKALI`, `IDCC`) and CNIL's `TEXTE_CNIL`. Maps the metadata blocks (identifiers, dates, status, NOR) to `Metadata` and `Extra` `lf.*`, the title as heading 1, the article text (XHTML-like content with `br` as line breaks, and tables), and versions, links and structures as one paragraph each with their ids in `Attrs`. Keys: `lf.*`. Cite: the `ID` (`LEGIARTI...`) in `Extra`; sections of a `JO` summary by `Loc.Anchor` from `@id`. Tested: LEGI and JORF exports, KALI and CNIL (2026-11-08 dumps); not tested: `CONTENEUR`. Case law is `dila-juri`'s. [profiles/legifrance.md](profiles/legifrance.md)

#### NewsML-G2 (`newsml-g2`) and NITF (`nitf`)

`newsml-g2`: the roots `newsItem`, `packageItem`, `newsMessage`, `knowledgeItem`, `conceptItem`, `planningItem`, `catalogItem` and `catalog` in `http://iptc.org/std/nar/2006-10-01/` (news agency items). Each item is a section whose `guid` is a level-1 heading and the anchor; its headline is a level-2 heading and, for a file holding one item, `Metadata.Title`. The values of `itemMeta` and `contentMeta` (provider, dates, subjects, genres, located, creators, rights) are tables of names and values with the file's own names (`subject/@qcode`, `subject/name`); QCodes are written as the file writes them, never resolved. The content: XHTML in `inlineXML` is an embedded document handed to the child extractor; NITF in `inlineXML` is read by the NITF rules; any other vocabulary there (SportsML, EventsML-G2, an agency's own) is a table of names and values; `remoteContent` is a table per rendition. `nitf`: the root `nitf` in no namespace or in `http://iptc.org/std/NITF/2006-10-18/` (NITF 3.x). `hl1` is `Metadata.Title` and a level-1 heading; byline, dateline, distributor, rights, abstract and notes are paragraphs marked `nitf.field`; the body is paragraphs, headings, lists, tables, quotes and media; the `head` (document data, dates, keywords, `meta`) is one table of names and values; `date.issue@norm` is also `Metadata.Created` when it is an ISO 8601 date and time, basic (`20020225T143000-0500`) or extended, the latter also with a space in place of the `T` (`2022-07-11 18:33`). Keys: `newsml.*`, `nitf.*`. Tested: 340 NewsML-G2 dev files (IPTC's examples and agency test files of STT, AAP, Belga, NTB, CP, DPA) and 17 NITF dev files; NITF elements no dev file has are mapped from the standard's element set only. [profiles/newsml-g2.md](profiles/newsml-g2.md), [profiles/nitf.md](profiles/nitf.md)

#### Patents (`patent`)

Selected by the roots `us-patent-grant`, `us-patent-application`, `patent-application-publication`, `ep-patent-document`, `wo-patent-document`, `patent-document`. Maps the bibliographic data (numbers, dates, classifications, parties, citations) to `Metadata` and `Extra` `pat.*`, the abstract, description (headings, numbered paragraphs with `@num`), claims (one list item per claim, `claim-ref` as links), tables, chemistry, formulas and drawings (images; no file is loaded). Keys: `pat.*`. Cite: `Loc.Anchor` from `@id`, claims from `@num`. Tested: USPTO grant v4.0 to v4.3, application v4.3, pap-v16, EPO v1.1 to v1.5.1; not tested: USPTO v4.4 and later, WIPO ST.36 documents other than the EPO's, ST.96. [profiles/patent.md](profiles/patent.md)

#### PubMed (`pubmed`)

Selected by the root `PubmedArticleSet` in no namespace (E-utilities `efetch` results, the baseline and update files). Each record (`PubmedArticle`, `PubmedBookArticle`) is a section: its PMID is a level-1 heading and the anchor, its title a level-2 heading, each `AbstractText` a paragraph with its `Label` as `Marker`. Authors and investigators are a table (a row per author, `RowLocs`, every affiliation); keywords and references are lists; the journal, dates, pagination, identifiers (`ArticleId` with its `IdType`), MeSH headings, publication types, chemicals, grants and history are tables of names and values with the file's own names (`Journal/ISSN`, `MeshHeadingList/MeshHeading/DescriptorName/@UI`), never interpreted. `Metadata` is not filled from a record, since a file may hold many. Keys: `pubmed.*`. Cite: the PMID, `Loc.Anchor` of the record's heading and `anchor_parent` of its other blocks. Tested: the 146 dev files (1960s to 2026, English, French and other languages, books); elements the dev files lack are mapped from the DTD only. [profiles/pubmed.md](profiles/pubmed.md)

#### S1000D (`s1000d`)

Selected by the roots `dmodule`, `pm` and `dml` in no namespace. Maps the identification and status section to `Metadata` and `Extra` `s1000d.*`, descriptive content (levelled paragraphs, lists, tables, figures, warnings and cautions), procedures (steps), publication module entries and data management list entries. Applicability references (`applicRefId`) are recorded in `Attrs["s1000d.applic_ref"]`, never evaluated. Keys: `s1000d.*`. Cite: `Loc.Anchor` from `@id`; the data module code in `Extra`. Tested: issues 4.1 and 4.2; not tested: issues 4.0, 5.0 and 6, fault isolation, IPD, process, wiring. [profiles/s1000d.md](profiles/s1000d.md)

#### TED notices before eForms (`ted`)

Selected by the root `TED_EXPORT` in the namespaces of schema R2.0.8 and R2.0.9 (TED's XML until 2023). The coded data, the titles in every language (one table, a row per language) and each part of a form are tables of names and values with the file's own names; an element whose presence is the value (`PT_OPEN`, `NO_LOT_DIVISION`) is a row with an empty value. Each form (one per language) is a section headed by its `LG`. In the 2014 forms the title is `Metadata.Title` and a heading, each lot (`OBJECT_DESCR`) and award a section, descriptions paragraphs; notices written as text (`OTH_NOT`) are headings, paragraphs and lists. The earlier forms are tables only. Keys: `ted.*`. Tested: 544 dev files (2016 and 2022). [profiles/ted.md](profiles/ted.md)

#### TEI (`tei`)

Selected by the root `TEI` or `teiCorpus` in the TEI P5 namespace (P4 and older are read generically). Maps the `teiHeader` to `Metadata` and `Extra` `tei.*`, divisions with their heads, paragraphs, verse lines (one block each), speeches with their speakers as `Marker`, stage directions, notes, lists, tables, figures, dictionary entries, and editorial markup (`choice`, `app`, `add`, `del`) with every alternative in the text in file order and a stored record saying which it is. Page and line numbers inside a block (`pb@n`, `milestone@n`) are written in place. Keys: `tei.*`. Cite: `Loc.Anchor` from `xml:id`, and `tei.divN_n` and `tei.n` for the canonical reference. Tested: TEI P5 samples (ELTeC, Folger, DraCor, Perseus, papyri.info, Bodleian, FreeDict); not tested: `facsimile`, P4. [profiles/tei.md](profiles/tei.md)

#### TMX (`tmx`)

Selected by the root `tmx` in no namespace with a `version` attribute (TMX 1.1 to 1.4b). The `body` is one table: a row per `tu` with `RowLocs`, its `tuid` the anchor, and **a column per language**: a `tuv`'s text goes to the column of its `xml:lang` (else `lang`) as written, columns in order of first appearance, the header cells being the language codes from the file. Two `tuv` of one language in one `tu` are both in that column. `prop` and `note` are read (the row's first column, or the `tuv`'s cell); inline codes keep the native code they hold, where it stands. Header values in `Extra` `tmx.*`. Tested: the 40 dev files (Okapi, Weblate, OPUS), all TMX 1.4. [profiles/tmx.md](profiles/tmx.md)

#### XLIFF 1.x (`xliff`) and 2.x (`xliff2`)

`xliff`: the root `xliff` in `urn:oasis:names:tc:xliff:document:1.2` or `...:1.1`, or in no namespace with `version` 1.0, 1.1 or 1.2. `xliff2`: the root `xliff` in `urn:oasis:names:tc:xliff:document:2.0` (2.0 and 2.1) or `...:2.2`. Each `file` is a section with a heading from `original` (else `id`) and one table: a row per `trans-unit` (1.x) or per `segment` and `ignorable` (2.x) with `RowLocs`, the unit's `id` the anchor, columns for the ids, `state` and the other unit attributes, source, target and notes; a `group` is a row of its own from its attributes. The languages are kept as written. Inline elements that wrap text keep it; native codes held as character data are read where they stand; `alt-trans`, matches, glossary entries, metadata and original data are read and tied to their unit. A base64 `internal-file` is handed to the child extractor. Elements of tool namespaces (SDL, memoQ, Okapi, ITS) are read generically. Keys: `xliff.*`, `xliff2.*`. Tested: the 958 dev files (565 XLIFF 1.x, 393 XLIFF 2.x: Okapi's test files, the OASIS TC's test suite, and files written by Trados (`.sdlxliff`, 33), Phrase/Memsource (`.mxliff`, 26) and memoQ (`.mqxliff`, 11)). [profiles/xliff.md](profiles/xliff.md), [profiles/xliff2.md](profiles/xliff2.md)

#### Service-Public (`service-public`)

For the XML of Service-Public.fr published by DILA (schema Publication 3.5): the roots `Publication` (fiches F, dossiers and themes N, guided searches) and `ServiceComplementaire` (R: forms, online services, model letters, simulators, definitions), and the index files `Arborescence`, `Menu`, `Redirections`, `DatesModImp`, `CentresDeContact`. Conditions are recorded, never evaluated: every case of every condition is extracted, in file order.

| Source | Output |
|---|---|
| `dc:title` | `Metadata.Title`, and a level 1 heading at the top of the body |
| `dc:creator`; `dc:subject`; `dc:language`; `dc:description` | `Metadata.Authors`; `Keywords`; `Language` as written (`Fr`); `Extra["description"]` (no block) |
| `dc:date` and the other `dc:*` | `Extra["dc:date"]` etc. as written (`modified 2026-11-03`); `Metadata.Modified` is not set |
| root `@ID`, `@type`, `@spUrl`, `@dateDerniereModificationImportante`, `@profilsPro`, `@statut`, `@pictogramme` | `Extra["sp.id"]`, `sp.type`, `sp.url`, `sp.date_important`, `sp.profils_pro`, `sp.status`, `sp.pictogramme` |
| `SurTitre`, `Audience`, `Canal`, `NumeroCerfa`, `AutreNumero` (of the root) | `Extra["sp.surtitre"]`, `sp.audience`, `sp.canal`, `sp.numero_cerfa`, `sp.autre_numero` |
| `FilDAriane/Niveau` | `Extra["sp.breadcrumb"]` titles joined with ` > `; `sp.breadcrumb_ids`, `sp.breadcrumb_types` |
| `Theme`, `SousThemePere`, `DossierPere/Titre` | `Extra["sp.theme"]`, `sp.theme_id`, `sp.theme_audience`, `sp.sous_theme`, `sp.sous_theme_id`, `sp.dossier`, `sp.dossier_id` |
| `Chapitre`, `SousChapitre`, `Situation`, `Cas`, `OuSAdresser`, `Definition`, `Abreviation`, `Avertissement`, `Conclusion`, `Exemple`, `PartieLibre`, `CaracteristiqueLibre`, `Question`, `Problematique`, `Branche`, `Groupe`, `QuiPeutMAider`, `SousDossier`, `SousTheme`, `IntroductionSimple` | sections; their `Titre` a heading by nesting from level 2 |
| `Cas`/`Situation` `Titre` and `TitreRiche` | heading from `TitreRiche`; the plain `Titre` in `Attrs["sp.title_plain"]` only when its text differs |
| `TitreFlottant` | a heading one level below its section's, opening no section |
| `Chapitre@numerote`, `@type`, `TitreAlternatif` | `Attrs["sp.numbered"]`, `sp.type`, `sp.title_alt` on the chapter's heading; no number is invented |
| `Paragraphe`; `Liste@type`/`Item` | `paragraph`; `list_item`, ordered (and numbered) for `numero`, every item with `Attrs["sp.list"]` = the type (`puce`, `numero`, `caseACocher`) |
| `Tableau`/`Rangée@type`/`Cellule@fusionHorizontale\|fusionVerticale`; `TitreRiche`, `Resume` | `table` (rows model) with header rows and merges; `caption` blocks before it. `Colonne` is layout: its attributes are counted in `xml.unmapped_attributes` |
| `ASavoir`, `ANoter`, `Attention`, `Rappel` | their blocks carry `Attrs["sp.box"]` = the element name; the box's `Titre` is the `Marker` of its first block |
| `MiseEnEvidence`, `Exposant`, `Indice`; `Expression`, `TermeEtranger`, `Valeur`, `Variable`, `Date` | bold, superscript, subscript spans; plain text |
| `LienExterne@URL`, `LienInterne@LienPublication`, `LienIntra@LienID` | link spans whose address is the URL or the id as written (`F2384`); each link's other attributes (`commentaireLien`, the target's title, as `sp.comment`; `type`; `audience`; ...) are a record in the stored part (`note`, `stored`=`custom_property`, `href`) whose text is the link's own text. `TermeEtranger@langue` gives such a record too (`sp.langue`): core's `Span` has no language field |
| `BlocCas@affichage`, `ListeSituations@affichage` | `Attrs["sp.case_group"]` on every block inside |
| `Condition` (in `Cas`, `Situation`, `Chapitre`, `FragmentConditionne`, `Item`, `Reference`, ...) | `Attrs["sp.condition"]` on every block inside, as an outline: `ou(estVrai(var=T10266), estIndefini(var=T10267))` |
| `Question/Choix` with `SiSelectionne`; `Branche`/`Problematique` with `SiSelectionne` | a `list_item` with `Attrs["sp.sets"]` (`affecteVrai(var=T12852), affecteFaux(var=T12853)`); on the branch's heading; `IntituleSiSelectionne`, `TitreChoix` paragraphs |
| `Reference`, `PourEnSavoirPlus`, `ServiceEnLigne`, `NoticeLiee`, `ServiceEnLigneAnnexe`, `LienExterneCommente`, `Video`, `QuestionReponse`, `FicheConnexe`, `CommentFaireSi`, `RechercheGuidee(Pere)`, `FicheLiee`, `FichePrecedente`, `FicheSuivante`, `ServiceEnLigneAlternatif`, `Fiche`, `Dossier`, `Contact` | a `list_item` whose text is the `Titre` (linked to `@URL` or the id), `Loc.Anchor` the `@ID`, `Attrs["sp.group"]` = the element name, the other attributes under `sp.*` (`sp.url`, `sp.type`, `sp.audience`, `sp.format`, `sp.poids`, `sp.numerocerfa`, ...). No heading is invented for a group. `Complement`, `Connexion`, `Introduction`, `Source` follow as paragraphs |
| `Source` | a paragraph, `Loc.Anchor` its `@ID`, `Attrs["sp.field"]="Source"` |
| `OuSAdresser`, `QuiPeutMAider` with `PivotLocal`, `RessourceWeb@URL` | section; `Attrs["sp.pivot"]` on its heading; a paragraph whose text is the URL (`Attrs["attribute"]="URL"`) |
| `Image`, `Infographie`, `ImageIllustration` | `image` block: id from `@LienPublication`, alt from `TexteDeRemplacement`; `Legende` and `Infographie/Titre` captions; `Credits`, `Description` paragraphs |
| `Public`, `Langue`, `Cout`, `Calendrier`, `Etapes`, `Documents`, ... (R files) | their blocks carry `Attrs["sp.field"]` = the element name |
| `ItemArbo`, `ItemMenu`, `DatesModImp/Item` (index files) | a `list_item` at its depth with its `Titre`, anchor `@ID`, `sp.type`, `sp.date_modified`, `sp.file`, `sp.date_important` |
| `Redirections` with `Redirection`, `RedirectionExterne` | one `table` in file order: a header row `from`, `to`, `toUrl`, `date`, `audience`, `type`, `corpus` (the profile's text, marked `Attrs["header_source"]="profile"`; the table is `Origin: native`, as every row and cell is the file's), then a row per element from its attributes (`cells`), an absent attribute an empty cell; `Attrs["attribute"]` lists the attributes. Each row has its element's place in the file in `Table.RowLocs`; the table's `Loc` covers the root element |

On the public corpus (Service-Public, 4,194 dev and 1,362 held-out files), every element has a rule: `xml.unmapped_elements` is empty on both sets. The only unmapped attributes on dev are `Colonne@largeur` and `Colonne@type` (3,558 each, column layout); held-out has 2,728 unmapped attributes in all.

Keys: `sp.*`. Tested: Publication schema 3.5 (the dev and held-out sets above); not tested: earlier schema versions.

#### UBL invoices (`ubl`)

Selected by the roots `Invoice` (`urn:oasis:names:specification:ubl:schema:xsd:Invoice-2`) and `CreditNote` (`...:CreditNote-2`) only (Peppol BIS Billing 3.0, EN 16931); other UBL documents are read generically. The same mapping as `cii`: `Extra` `inv.*`, parties as tables of names and values with `inv.party`, one lines table (`InvoiceLine`, `CreditNoteLine`), tax subtotals, allowances and charges and the legal monetary total as tables. `EmbeddedDocumentBinaryObject` attachments are handed over likewise. Keys: `inv.*`. [profiles/ubl.md](profiles/ubl.md)

## Drafting a profile from an XSD

```
gofastxml -draft-profile schema.xsd [-o draft.json]
```

or `gofastxml.DraftProfile(ctx, path)`, which returns a `*Draft` (the JSON, the schema files read, and warnings), reads an XML Schema (XSD 1.0) and writes a **draft** profile for a person to finish. It is not a profile: every rule carries `"review": true`, and `LoadProfile` refuses a file that still has one (`a draft from -draft-profile, not a profile`). Check each rule, change or delete it, and remove the mark; then the file loads.

What it reads, with this module's own XML reader: global and local elements, named and anonymous complex types, simple types, `complexContent` and `simpleContent` extension (the base's content and attributes are added) and restriction, `group` and `attributeGroup` references, `sequence`, `choice`, `all`, `mixed`, `ref`, substitution groups (the members stand wherever the head may; an abstract head does not), `any`, attributes with their types (`xs:ID`, `xs:IDREF`, `xs:anyURI` and types derived from them, `xml:lang`, `xml:id`), the target namespace, `elementFormDefault` and `form`, chameleon includes. `xs:include`, `xs:import` and `xs:redefine` are followed **on disk only**, at locations relative to the file that names them. A location that is a URL, an absolute path, missing, or naming no file is not opened: nothing is fetched. A file is opened only after `os.Stat` finds it a regular file (a FIFO, device or directory is never opened); symbolic links are followed, and must lead to a regular file. The context is consulted before each file. Such parts are listed in the draft's `notes.not_read`, references to components they would have held in `notes.missing_components`, and the call returns an `input.external_reference` warning. Reading is bounded (512 files, includes 32 deep, 32 MiB in all, 400,000 schema elements, 100,000 element declarations); what a bound leaves out is a `limit.exceeded` warning and a note. Type cycles (`section` in `section`) are normal and followed once.

Not read: DTDs and RELAX NG schemas (use the XSD edition of a schema where there is one); identity constraints, facets and default values; XSD 1.1 (assertions, open content, `override`); `xs:notation`; `xsi:type` in documents.

What it writes, for each element the schema declares, from its content model first and its name only to break a tie:

| Shape | Suggestion |
|---|---|
| Elements only, a text first child (or a title-named one), and it nests or repeats | `section`, the child as `heading` with `"level": "nesting"` |
| Mixed content among elements-only content, or text content beside such siblings | `block`, `paragraph` |
| Mixed or text content inside mixed content | `inline`; with an `xs:anyURI` attribute, `link` |
| One child, repeated | `list`, the child as `item` |
| A repeated child whose children repeat (three levels), or CALS/XHTML names | `table`, `row`, `cell` |
| Empty with an `xs:anyURI` attribute | `image` with that attribute as `id` |
| An `xs:ID` attribute | `anchor` on the rule |
| Elements only, nothing more said | `transparent` |

Where one local element name has declarations that would get different suggestions, the draft writes one rule per parent (`Parent/name`). Elements with no suggestion are listed under `notes.undecided` with their content, parents, children and attributes; they are read generically. Each rule's `"note"` says why it was suggested; `rests on the name` marks a suggestion the name decided (title, titre, para, list, item, table, row, cell, fn, img and the like, in English and French). `match` is drafted from the global elements no other element contains and the target namespace. Metadata, labels, notes, parts and stored content are not drafted. Name guessing happens only here, never during extraction. The output is the same for the same schema.

On the public schemas (`go test -run TestDraft -v`, with `../.testdata/schemas`), the draft's rule kinds against the hand-written profiles, element by local name: Service-Public `Publication.xsd` same kind 55, different 60, no rule 2 of 117; Akoma Ntoso 3.0 179, 56, 14 of 249; JATS Publishing 1.3 150, 117, 13 of 280. Each draft is made in under 20 ms.

## How XML maps without a profile

One part of kind `body`, number 1.

- **Text elements.** An element that directly contains character data other than white space is a text element. It gives one `paragraph` block, `Origin: inferred`, whose text is its own character data with that of all its descendants, in document order. CDATA sections and entity replacements are character data.
- **Containers.** An element that holds only elements (and white space) is a container and gives no block of its own.
- **Inline elements.** A text element inside another text element is part of the outer block: `<p>Hello <b>world</b></p>` is one paragraph, `Hello world`. This holds also when the outer element's own text comes after its children (`<p><b>Bold</b> tail</p>`): the children's blocks are taken into it. To keep memory bounded, blocks wait for such a decision only up to 4 MiB or 8,192 blocks; past that, the open elements still undecided are taken for containers, and one that has text after that point gives a block from there on, with `limit.exceeded`. Nothing is lost either way; only where block boundaries fall differs.
- **White space.** Runs of XML white space (space, tab, line feed, carriage return) collapse to one space, and the text is trimmed. Under `xml:space="preserve"` the white space is kept as written, line breaks as `\n` and tabs included, and only the ends are trimmed; `xml:space="default"` turns it off again for a subtree. Other space characters (a no-break space) are content and kept, though not at the ends of a block.
- **`Attrs["element"]`** on each block made from an element is the path of element names from the root as written in the file, prefixes included, joined with `/` (`Publication/Texte/Paragraphe`), so a consumer can tell `<name>` from `<address>`.
- **Nothing is guessed from names.** No headings, lists, tables or titles are inferred from element names; a profile says what names mean.
- **Text outside the root element** (which a well-formed file does not have) is a paragraph of its own with no `element` path.
- **Locations.** Every block has `Loc.Offset` and `Loc.Length`: for a paragraph the element's span from its start tag to its end tag, for an attribute note its element's start tag, for a comment the comment. Offsets are bytes of the file, in every encoding (a byte order mark included): for a file not in UTF-8 the decoder keeps a checkpoint per 32 KiB and the offsets are mapped back after reading, re-reading only the stretches they fall in. A block read from an entity's replacement has the location of the reference. A paragraph or comment split by the block bound has the span of each piece's text; the pieces are in order and do not overlap. `Loc.Anchor` is the block's element's `xml:id`, else its `id`, else that of an element inside it.
- **Ligatures** U+FB00 to U+FB06 are written as their letters; other characters are kept as the file has them, invisible ones included.

### Attributes

Each attribute with a value that is not empty (after white space is collapsed) is a `note` block of type `attribute` (core's `NoteAttribute`), `Origin: inferred`, with the attribute's name as written under `Attrs["attribute"]` and its element's path under `Attrs["element"]`. It stands where the element starts: an element's attribute notes come before the element's own paragraph. Notes of an inline element's attributes come right after the paragraph they are inside. With `AttributesNone` there are none.

These attributes are not notes:

| Attribute | Goes to |
|---|---|
| `xmlns`, `xmlns:*` | nowhere: they declare namespaces, which are resolved |
| `xml:space` | how white space is read |
| `xml:lang` on the root element | `Metadata.Language` (elsewhere, or when longer than 64 bytes, it is a note) |
| `xsi:schemaLocation`, `xsi:noNamespaceSchemaLocation` on the root | `Metadata.Extra` (elsewhere they are notes); never loaded |

### Comments

A comment is a `note` block marked `Attrs["stored"] = "markup_comment"`, as go-fast-html writes HTML comments, at its place. A comment inside a text element follows that element's paragraph. Line breaks in a comment are kept; its ends are trimmed. The renderers leave comments out unless `RenderOptions.Stored` (or `StoredKinds` with `markup_comment`) asks for them.

### Metadata

| Field | From |
|---|---|
| `Language` | `xml:lang` on the root element, up to 64 bytes |
| `Extra["root"]`, `Extra["root_namespace"]` | the root element's name as written and its namespace URI |
| `Extra["schema_location"]`, `Extra["no_namespace_schema_location"]` | `xsi:schemaLocation` (white space collapsed), `xsi:noNamespaceSchemaLocation` on the root |
| `Extra["doctype"]`, `Extra["doctype_public"]`, `Extra["doctype_system"]` | the document type declaration's name and external identifiers |
| `Extra["encoding"]` | the encoding the file was read in |

## Encodings

The encoding is decided as XML 1.0 (section 4.3.3 and appendix F) says: a byte order mark (UTF-8, UTF-16LE, UTF-16BE), else UTF-16 recognised from the first bytes of the XML declaration, else the `encoding` the declaration names, else UTF-8. A declaration that names UTF-16 in a file that reads as ASCII is read as UTF-8; a byte order mark wins over a declaration that disagrees.

`Extractor.Charset` (the command's `-charset NAME`) gives the encoding from outside the file, as a MIME charset label does (`windows-1251`, `shift_jis`): it is used whatever the declaration names or the bytes suggest; a byte order mark still overrides it, as with go-fast-html's option of the same name. A label that names no encoding decoded here is `ErrUnsupported` before anything is read (go-fast-html ignores it with a warning).

UTF-8, UTF-16 and windows-1252 are decoded natively, and US-ASCII and ISO-8859-1 as windows-1252, as the WHATWG Encoding standard and browsers do (the bytes 0x80 to 0x9F are then the characters windows-1252 has there, not C1 controls). Every other label is looked up in go-fast-core's `charset`, the decoder all the go-fast libraries share (the other single-byte encodings, Shift_JIS, EUC-JP, ISO-2022-JP, GBK, GB18030, EUC-KR, Big5). A declared encoding not among them (ISO-2022-KR, UTF-7, EBCDIC), and UTF-32, is `ErrUnsupported`.

A file that declares no encoding and whose first MiB is not valid UTF-8 is read in the encoding recognised from its bytes where go-fast-html's detection is certain, with `text.encoding_detected` naming it; otherwise it is `ErrUnsupported`. Bytes that are not a character in the encoding are written as U+FFFD and counted in `text.unmapped_glyphs`, in UTF-8 files too.

## Entities and document type declarations

- **Never loaded.** No external DTD subset, external entity, parameter entity, schema, stylesheet or XInclude is read or fetched. The ones that would make a conforming processor load something are reported with `input.external_reference`, once per kind with a count: an external DTD subset, a reference to an external entity (or to an external parameter entity in the internal subset), an `xml-stylesheet` instruction, an XInclude `include` element. Schema location hints are only recorded in `Metadata.Extra`.
- **The internal subset** is read for general entity declarations only. Element, attribute-list (default attribute values included) and notation declarations are passed over; parameter entities are never expanded.
- **Internal entities** are expanded where referenced, their replacement read as markup (an entity may hold elements), in text and in attribute values. Nesting is bounded (`MaxEntityDepth`, 16), and so are the bytes all expansions produce together: `EntityExpansionBase` (1 MiB) plus `EntityExpansionRatio` (8) times the input size, and never more than the decode budget (`Limits.MaxDecodedBytes`, shared with embedding documents) has left. A reference past either bound is kept as written, and so is every expanding reference after the first one refused; the part carries `limit.exceeded`. A billion-laughs file and the quadratic-blowup variant finish in milliseconds. A reference to an entity inside its own replacement is kept as written and counted in `input.repaired`.
- **The five predefined entities and character references** always work. A character reference to a control character other than tab, line feed and carriage return is removed with `text.control_characters_removed`. One that names no character XML allows (`&#0;`, a surrogate, a number past U+10FFFF) is U+FFFD and counted in a `text.unmapped_glyphs` warning of its own, in text, in an attribute value and in the declared value of an entity the document uses.
- **Undeclared entities** (`&eacute;` in a DocBook 4 or JATS file whose DTD is not loaded): if the file has an external DTD subset and the name is one of the 252 character entities of HTML 4 (the names that mean the same in every vocabulary), the character is written and the part carries `xml.entities_assumed` (info) with the count. Otherwise the reference stays in the text as written (`&name;`) and the part carries `xml.entity_undeclared` (warning) with the count. A reference to an external entity also stays as written.

## Damaged input

The reader is lenient and counts what it mends in one `input.repaired` (warning) on the document: an end tag that matches an element further out closes the elements inside it; one that matches nothing is dropped; elements open at the end are closed; a `<` or `&` that starts no markup is text; an attribute without a value is passed over and one without quotes runs to white space or `>`; a quoted value that runs into a `<` lacks its closing quote and ends at the first `>` (which ends the tag) or else before the `<`, and so does a public or system identifier in a document type declaration; a tag or document type declaration cut off by the next `<` ends there; a comment or CDATA section open at the end runs to it; a second root element and text outside the root are read as they come; a second document type declaration, or one after the root element starts, is not read. Input with no element at all is `ErrMalformed`.

Processing instructions other than the XML declaration and `xml-stylesheet` are not text; they are counted in one `input.unread_content` (warning).

## Warnings

| Code | Severity | Where | What happened |
|---|---|---|---|
| `limit.exceeded` | error | part; document for `Limits.MaxParts` | a bound cut the output; the message says which and how much |
| `text.unmapped_glyphs` | warning | part | byte sequences not valid in the encoding were written as U+FFFD; in a warning of its own, character references that name no character (`&#0;`, a surrogate, a number past U+10FFFF) appear as U+FFFD |
| `text.control_characters_removed` | info | part | control characters other than tab and line feed were removed |
| `text.encoding_detected` | warning | document | no encoding declared, not UTF-8; read in the encoding named |
| `input.repaired` | warning | document | damaged markup read leniently, with the count |
| `input.external_reference` | warning | document | one per kind of reference a conforming processor would load, with the count; nothing was loaded |
| `input.unread_content` | warning | document; part | processing instructions, with the count (document); MathML in a profile's formula rule, kept as plain text, with the count (part) |
| `xml.entities_assumed` | info | part | undeclared entities read as the HTML 4 character entities of those names, with the count |
| `xml.entity_undeclared` | warning | part | undeclared entities kept as written, with the count |
| `part.no_text` | warning | part | the document has no block |
| `part.failed` | error | part | reading stopped on an I/O error; what was read is kept |
| `xml.unmapped_elements` | info | part | profile reading: elements with no rule read generically, names with counts |
| `xml.unmapped_attributes` | info | part | profile reading: attributes of mapped elements no rule reads, names with counts |
| `xml.skipped_elements` | info | part | profile reading: elements left out by a user profile's `skip` rules |
| `structure.heading_level_clamped` | info | part | profile reading: headings nested deeper than 6 have level 6 and their level in `Attrs["level"]` |

Part warnings are on the last part. `DraftProfile` returns its own warnings in `Draft.Warnings`: `input.external_reference` (warning; schema parts not read because they are not on disk) and `limit.exceeded` (error; a bound on schema reading cut the draft). No other code is raised.

## Errors

`ErrMalformed` (no element, or a negative size), `ErrUnsupported` (an encoding not read), `ErrLimitExceeded` (larger than `Limits.MaxInputBytes`, or a tag over `MaxTagBytes` before any element), `ErrNoPartsSelected` (`Options.Parts` excludes part 1), and the context's error when it is cancelled.

## Limits

Core's limits: `Limits.MaxInputBytes` (default 4 GiB, `ErrLimitExceeded`); `Limits.MaxDecodedBytes` (default 1 GiB), which here bounds entity expansion; `Limits.MaxTextBytes` (default 256 MiB): every block takes `BlockCost` plus its text plus its `element` path from it, and when it is spent the part is cut there with `limit.exceeded` saying how much input was left unread.

Fixed bounds of this library; those named are exported constants. Each cut is `limit.exceeded` (error) on the part, once per bound with a count:

| Bound | Value | What happens past it |
|---|---|---|
| `MaxDepth` | 512 | deeper elements are read as part of their ancestor: their text is kept, their tags and attributes are not read. Element nesting never recurses on the Go stack |
| `MaxAttributes` | 1,024 per element | later attributes are not read |
| `MaxTagBytes` | 16 MiB | reading stops at a larger tag, processing instruction, document type declaration or character reference |
| `MaxNameBytes` | 256 | an element or attribute name in `Attrs` is cut |
| `MaxPathBytes` | 4 KiB (256 × 16) | `Attrs["element"]` is cut at a `/` |
| `MaxValueBytes` | 1 MiB | an attribute value (decoded, entities expanded), and so its note or anchor, is cut; all the values of one tag together are held to `MaxTagBytes` |
| (namespace declarations) | 16 MiB in scope | a declaration past it is not read |
| `MaxBlockBytes` | 1 MiB | a longer text or comment is split into blocks marked `Continued`, between the pieces text is read in (at most 64 KiB each, cut after white space where there is some); white space at a cut is dropped, as at the ends of any block; no warning is raised |
| `MaxEntityDepth`, `EntityExpansionBase`, `EntityExpansionRatio` | 16; 1 MiB + 8 × input | see [Entities](#entities-and-document-type-declarations) |
| blocks waiting for a block | 8,192 blocks or 4 MiB | notes (attribute notes, comments) waiting for the block they stand in: the block's text so far is given as a piece and the notes follow; blocks waiting for an element not yet known to be a text element: see [generic reading](#how-xml-maps-without-a-profile) |
| table columns (profiles) | 1,024 | cells past it are left out |
| rows one cell spans (profiles) | 65,535 | the span is cut |
| spans in one block (profiles) | 100,000 | later spans are left out (the text stays) |
| `MaxChildren` (profiles) | 10,000 documents | the content of later `document` elements is not handed over and stays in the body as a paragraph with `Attrs["media_type"]` |
| one embedded document (profiles) | `MaxBlockBytes`, 1 MiB | the content handed over, and its fallback paragraph, are cut there |
| one base64 attachment (profiles) | `MaxAttachmentBytes`, 64 MiB of base64 | a longer attachment is not handed over and not written, with `limit.exceeded` |
| (decode budget, profiles) | `Limits.MaxDecodedBytes` | the bytes handed to children are taken from it before each child reads; once it is spent, a child is listed but not read, and its text stays in the body |
| a cell, label or value read from a child (profiles) | 1 MiB (`MaxBlockBytes`) | the text is cut there |
| parts started by `part` rules | 100,000, and `Limits.MaxParts` | the content stays in the last part; parts past `Limits.MaxParts` are left out |

The `xml.unmapped_*` and `xml.skipped_elements` warnings list at most 100 names, and count at most 1,000 distinct names, the rest together; this bounds the warning's message, not the output, and raises nothing.

With a profile, a block takes from `Limits.MaxTextBytes` its text, its `Marker`, its `Attrs` keys and values, its span addresses and, for a table, `CellCost` plus the text of each cell; a value a child gives later (`Attrs`, an image's alt text) is taken when it comes. The records of links in the stored part count the same way.

The context is checked every 1,024 tokens, and every 1,024 constructs passed over without a token (end tags that match nothing, elements past `MaxDepth`). Output is deterministic.

## What the file declares

XML itself declares nothing about display: whether an element is shown is up to each schema and its stylesheet. Generic reading therefore sets no `Display` reason. A profile may name a schema's switch that declares content not shown (`display`: an attribute and its values, giving `hidden` or `no_ink`, PROFILE_FORMAT.md section 3); the built-in profiles have no such rule, so no block read with them carries a `Display` reason. Conditions (Service-Public `Condition`, S1000D applicability, DocBook `condition`) are recorded, never evaluated, and every case is extracted.

Nothing about visibility is judged: no colour, size, position or overlap is looked at, and characters are kept as the file has them, invisible ones included. Unshown data (comments, attributes, stored data, metadata) is read as data, never run, evaluated or followed, and bounded by the limits.

## Nothing fetched or run

No external DTD subset, external entity, parameter entity, schema (`xsi:schemaLocation`), stylesheet (`xml-stylesheet`) or XInclude `include` is read, fetched or applied; the four kinds a conforming processor would load are reported with `input.external_reference` (below), and schema locations are recorded in `Metadata.Extra`. No XSLT, script or query in a document is run. A profile is data: nothing in it is run. `-draft-profile` (`DraftProfile`) follows `xs:include`, `xs:import` and `xs:redefine` on disk only, never a URL ([Drafting a profile](#drafting-a-profile-from-an-xsd)).

## Not read

- **Validation.** Documents are not validated against an XSD, a DTD or any other schema; default attribute values a DTD declares are not applied (so DITA's `@class` is usually absent).
- **XInclude.** An `xi:include` is not resolved: its `href` stays as text or an attribute note, with `input.external_reference`.
- **XSLT.** `xml-stylesheet` instructions are reported with `input.external_reference`, not applied; no stylesheet is run.
- **XBRL facts.** No profile reads XBRL: an instance is read generically, each fact's text a paragraph and its `contextRef`, `unitRef` and `decimals` attribute notes; contexts, units and the taxonomy are not interpreted.
- **Drafting from RELAX NG or a DTD.** `-draft-profile` reads XSD 1.0 only.
- **MathML** is kept as plain text, not converted (with `input.unread_content` where a profile's formula rule reads it).
- **Processing instructions** as text, and external anything (see [Nothing fetched or run](#nothing-fetched-or-run)).

## Tests

`go test ./...` runs everything; `-short` skips the large worst cases (a million attributes, a 64 MiB text node, a 200 MiB generated stream, a 17 MiB tag). An oracle test compares the character data with `encoding/xml`'s on 500 generated documents. `TestCorpus` reads `GOFAST_CORPUS` or `../.testdata/corpus` and runs every file of `dev/xml*/` and `heldout/xml*/` (counts only for held-out files); it skips when the corpus is absent. Fuzz targets: `FuzzExtract` and `FuzzProfile` (a profile with every rule kind) (.), `FuzzLoad` and `FuzzSelector` (`internal/profile`), `FuzzLoad` (`internal/xsd`), `FuzzDraft` (`internal/draft`), `FuzzParse` (`internal/dtd`). The scanner's and the decoders' own fuzz targets are in go-fast-core, with the packages (`xmlscan`, `charset`).

Each built-in profile has a corpus test (`Test<Profile>Corpus`: every dev and held-out file, aggregate counts only for held-out) and an oracle test (`Test<Profile>Oracles`) over its dev files: the body's text in order is the file's character data in order, apart from the listed exceptions (metadata, attribute-borne text such as TEI `pb@n` and Formex `QUOT.START@CODE`), each of which must be met in the dev set; and every block's text is in the source at its offsets. `TestCorpusRowLocs` checks every row location of every table of every dev file of the XML corpora.

Profiles: `TestProfile*` read small synthetic documents with one synthetic profile per shape of the format (headings by nesting, dispatch on attributes, CALS and rows tables with spans, labels, out-of-line footnotes, marks from conditions, parts, metadata, links, images, values from attributes); each checks `Validate` and that every block's text is at its offsets. `TestServicePublicCorpus` reads every Service-Public file of the dev and held-out sets (aggregate counts only for held-out) and fails on a panic, an error, a `Validate` failure or a file not read with the profile; it logs the unmapped elements and attributes. `TestServicePublicOracles` checks every dev file against two oracles: the body's block and cell text, in order, is the file's character data in order (white space aside) apart from a listed set of elements (metadata, the `Titre` that `TitreRiche` repeats, values read into `Attrs`, image alt text), each of which must occur; and every block's text is in the source at its `Loc.Offset`/`Length`.

`TestOutputDigests` (run with `-args -digests=FILE`, kept outside the repository) records a digest of the JSON of every dev file of every `xml-*` corpus, read with the profile chosen and generically, on its first run, and fails on any difference on later runs; it also checks that `ExtractBytes` gives what `ExtractFile` gives.

## Speed

Measured on a Ryzen 9 9900X (a shared machine), Go 1.27.3, one thread (`-cpu 1`), two to three runs each; user CPU (`user-s/op`) is reported by the batch benchmark because wall time is noisy there.

- `BenchmarkBatch` reads the whole Service-Public dev batch (4,194 files, 106 MB, one of 25 MB), one document at a time, the result dropped: `file` through `ExtractFile` with the profile chosen by the root element, 87 to 98 MB/s across runs (1.0 to 1.14 s user CPU, 920 MB allocated and 5.8 million allocations per batch); `bytes` through `ExtractBytes`, each file read into a reused buffer first, 103 to 106 MB/s; `generic` through `ExtractFile` with generic reading, about 75 MB/s; `encodingxml` tokenizes the same files with `encoding/xml`'s `RawToken` (tokens only, no document), about 108 MB/s.
- `BenchmarkServicePublic` reads a fixed list of 200 of those files from memory: about 91 MB/s and 993 allocations per file with the profile, 93 MB/s and 1,272 generically.
- `BenchmarkExtract` reads a synthetic 10 MiB document generically, about 85 MB/s; `BenchmarkEncodingXML` tokenizes the same bytes with `encoding/xml`, about 115 MB/s.

## Provenance

See `NOTICE`. The XML scanner, the character decoders and the table of HTML character names are go-fast-core's (`xmlscan`, `charset`, `entity`), shared by the go-fast libraries since v0.5.0; this library kept its own copies before. Their provenance is in go-fast-core's `NOTICE`.
