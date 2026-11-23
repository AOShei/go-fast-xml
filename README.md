# go-fast-xml

Extracts XML documents of any schema into the shared [go-fast-core](https://github.com/AOShei/go-fast-core) document model. Standard library only.

```go
import "github.com/AOShei/go-fast-xml" // package gofastxml

doc, err := gofastxml.ExtractFile(ctx, "fiche.xml", nil)
if err != nil { ... }
fmt.Println(doc.Text())
```

**Status: in development (phase 2 of `XML_PLAN.md`).** A document whose schema has a **profile** is read with it: the profile maps the schema's elements onto headings, paragraphs, lists, tables, links and metadata (see [Profiles](#profiles)). One profile is built in, Service-Public.fr (DILA, Publication 3.5). Any other XML document is read generically: its text in document order, as inferred paragraphs.

The document is read as a stream, never as a tree: besides the document it returns, memory is bounded by the fixed bounds below (one tag, one block, the blocks waiting for an undecided element, the namespace declarations in scope), not by the file. For input not read as UTF-8, the map back to the file's bytes adds 16 bytes per 32 KiB of input. A generated 200 MiB document of small elements is read with a peak heap of about 1 MiB (`TestLargeStream`).

## API

- `Extract(ctx, r io.ReaderAt, size, opts)`, `ExtractFile(ctx, path, opts)`, `ExtractBytes(ctx, data, opts)`.
- `Extractor{Attributes}` implements `gofastcore.Extractor`; its zero value is ready and safe for concurrent use. `Formats()` is `xml`, and `Source.Format` is `xml`, `Source.MIME` `application/xml`.
- `Attributes`: `AttributesNotes` (the default) makes each attribute a note (below); `AttributesNone` leaves attribute notes out, for data-style XML where attributes are keys and codes. Leaving them out raises no warning. It applies to generic reading.
- `Extractor.Profiles []*Profile`: profiles tried, in order, before the built-in ones. `Extractor.Profile string`: force the profile of that name (from `Profiles` or built in) whatever the root element; `ProfileNone` (`"none"`) forces generic reading; an unknown name is an error.
- `LoadProfile(io.Reader) (*Profile, error)` reads and validates a profile; `BuiltinProfiles()` lists the built-in ones in the order they are tried, `BuiltinProfile(name)` returns one. A `Profile` (`Name()`, `Title()`) is immutable and safe for concurrent `Extract` calls.

## Command

`go install github.com/AOShei/go-fast-xml/cmd/gofastxml@latest`

```
gofastxml [flags] file-or-directory...
  -format json|markdown|text  -attributes=false  -o file  -quiet  -version
  -profile NAME|FILE|none  -profiles  -spans
  -furniture  -stored[=kinds]  -part-markers  -children  -exclude-display[=reasons]
  -max-input-bytes N  -max-decoded-bytes N  -max-text-bytes N
```

A directory stands for the files in it and below it whose names end in `.xml` (in any case). A file named on the command line is read whatever its name. With one input the JSON is indented; with several, each document is one line. Documents go to standard output; warnings of severity error go to standard error; the exit status is 1 if any input could not be read, 2 for a usage error.

The render flags are the series' shared ones (`COMMAND_FLAGS.md`), defined by core's `AddRenderFlags`, and affect Markdown and text only; JSON always has everything. This library writes one stored kind, `markup_comment` (the file's comments), so `-stored=markup_comment` puts comments in the text.

`-profile` takes the name of a built-in profile, a profile file, or `none`; `-profiles` lists the built-in profiles (name and title); `-spans` sets `Options.Spans` (inline styles and links in the JSON).

## Profiles

A profile is a JSON file that says what a schema's elements mean. How to write one: [PROFILE_FORMAT.md](PROFILE_FORMAT.md).

**How one is chosen.** When the root element starts, the profiles of `Extractor.Profiles` are tried in order, then the built-in ones; the first whose `match` the root meets (local name, namespace, optionally a DOCTYPE public identifier prefix and attribute tests) is used, else the document is read generically. `Extractor.Profile` / `-profile` forces one, or `none`. `Metadata.Extra["xml.profile"]` names the profile used; it is absent for generic reading.

**What a profile gives.** Blocks of every kind core has for text (headings with levels from nesting, paragraphs, quotes, code, captions, footnotes, formulas, notes, list items with depth and `Marker`, tables in the rows or CALS model with header rows and merges, images), inline styles and links as `Spans` with `Options.Spans`, metadata, parts, and the stored part. Blocks a rule makes are `native`; what the profile does not map is read generically (`inferred`) and counted. A profile only chooses how the file's text is cut and labelled: it cannot add words, drop text or evaluate a condition.

**How to cite a block.** Every block has `Loc.Offset` and `Loc.Length`, the bytes of the element it was made from in the file, from its start tag to its end tag (a heading: its title element; a table: the table element; an item: the item element; stray text: the text). `Loc.Anchor` is the identifier the source gives the block (the rule's `anchor`, else `xml:id`); a block without one carries the nearest enclosing identified element's in `Attrs["anchor_parent"]` (`AttrAnchorParent`), so a citation can say "fiche F1199, chapter X" without offsets. The document itself is cited by `Source.Name` and, for Service-Public, `Metadata.Extra["sp.id"]` and `["sp.url"]`. Text moved out of the body (metadata, `Attrs` values, the stored part) has no offsets of its own; a block whose text is an attribute's value says so in `Attrs["attribute"]`. Tests check, for every block of every Service-Public dev file, that its text is in the source at its offsets.

**Warnings of profile reading** (all `info`, on the last part, names with counts, at most 100 names listed):

| Code | What happened |
|---|---|
| `xml.unmapped_elements` | Elements the profile has no rule for were read generically |
| `xml.unmapped_attributes` | Attributes of mapped elements that no rule reads (`element@attribute`); their values are not in the output |
| `xml.skipped_elements` | Elements a user profile's `skip` rules left out |

Core's `structure.heading_level_clamped` (headings nested deeper than 6), `input.unread_content` (MathML in a formula rule, kept as plain text) and `limit.exceeded` (below) are raised as core defines them.

### Service-Public (`service-public`)

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
| `LienExterne@URL`, `LienInterne@LienPublication`, `LienIntra@LienID` | link spans whose address is the URL or the id as written (`F2384`); each link's other attributes (`commentaireLien`, the target's title; `type`; `audience`; ...) are a record in the stored part (`note`, `stored`=`custom_property`, `href`) |
| `BlocCas@affichage`, `ListeSituations@affichage` | `Attrs["sp.case_group"]` on every block inside |
| `Condition` (in `Cas`, `Situation`, `Chapitre`, `FragmentConditionne`, `Item`, `Reference`, ...) | `Attrs["sp.condition"]` on every block inside, as an outline: `ou(estVrai(var=T10266), estIndefini(var=T10267))` |
| `Question/Choix` with `SiSelectionne`; `Branche`/`Problematique` with `SiSelectionne` | a `list_item` with `Attrs["sp.sets"]` (`affecteVrai(var=T12852), affecteFaux(var=T12853)`); on the branch's heading; `IntituleSiSelectionne`, `TitreChoix` paragraphs |
| `Reference`, `PourEnSavoirPlus`, `ServiceEnLigne`, `NoticeLiee`, `ServiceEnLigneAnnexe`, `LienExterneCommente`, `Video`, `QuestionReponse`, `FicheConnexe`, `CommentFaireSi`, `RechercheGuidee(Pere)`, `FicheLiee`, `FichePrecedente`, `FicheSuivante`, `ServiceEnLigneAlternatif`, `Fiche`, `Dossier`, `Contact` | a `list_item` whose text is the `Titre` (linked to `@URL` or the id), `Loc.Anchor` the `@ID`, `Attrs["sp.group"]` = the element name, the other attributes under `sp.*` (`sp.url`, `sp.type`, `sp.audience`, `sp.format`, `sp.poids`, `sp.numerocerfa`, ...). No heading is invented for a group. `Complement`, `Connexion`, `Introduction`, `Source` follow as paragraphs |
| `Source` | a paragraph, `Loc.Anchor` its `@ID`, `Attrs["sp.field"]="Source"` |
| `OuSAdresser`, `QuiPeutMAider` with `PivotLocal`, `RessourceWeb@URL` | section; `Attrs["sp.pivot"]` on its heading; a paragraph whose text is the URL (`Attrs["attribute"]="URL"`) |
| `Image`, `Infographie`, `ImageIllustration` | `image` block: id from `@LienPublication`, alt from `TexteDeRemplacement`; `Legende` and `Infographie/Titre` captions; `Credits`, `Description` paragraphs |
| `Public`, `Langue`, `Cout`, `Calendrier`, `Etapes`, `Documents`, ... (R files) | their blocks carry `Attrs["sp.field"]` = the element name |
| `ItemArbo`, `ItemMenu`, `DatesModImp/Item` (index files) | a `list_item` at its depth with its `Titre`, anchor `@ID`, `sp.type`, `sp.date_modified`, `sp.file`, `sp.date_important` |
| `Redirections` with `Redirection`, `RedirectionExterne` | one `table` in file order: a header row `from`, `to`, `toUrl`, `date`, `audience`, `type`, `corpus` (the profile's text: `Origin: inferred`, `Attrs["header_source"]="profile"`), then a row per element from its attributes (`cells`), an absent attribute an empty cell; `Attrs["attribute"]` lists the attributes. Core's `Table` has no location per row: the table's `Loc` covers the root element |

On the public corpus (Service-Public, 4,194 dev and 1,362 held-out files), every element has a rule: `xml.unmapped_elements` is empty on both sets. The only unmapped attributes on dev are `Colonne@largeur` and `Colonne@type` (3,558 each, column layout); held-out has 2,728 unmapped attributes in all.

The `sp.*` keys are the profile's own (decision S9), so they cannot collide with core's keys.

## How XML maps without a profile

One part of kind `body`, number 1.

- **Text elements.** An element that directly contains character data other than white space is a text element. It gives one `paragraph` block, `Origin: inferred`, whose text is its own character data with that of all its descendants, in document order. CDATA sections and entity replacements are character data.
- **Containers.** An element that holds only elements (and white space) is a container and gives no block of its own.
- **Inline elements.** A text element inside another text element is part of the outer block: `<p>Hello <b>world</b></p>` is one paragraph, `Hello world`. This holds also when the outer element's own text comes after its children (`<p><b>Bold</b> tail</p>`): the children's blocks are taken into it. To keep memory bounded, blocks wait for such a decision only up to 4 MiB or 8,192 blocks; past that, the open elements still undecided are taken for containers, and one that has text after that point gives a block from there on. Nothing is lost either way; only where block boundaries fall differs.
- **White space.** Runs of XML white space (space, tab, line feed, carriage return) collapse to one space, and the text is trimmed. Under `xml:space="preserve"` the white space is kept as written, line breaks as `\n` and tabs included, and only the ends are trimmed; `xml:space="default"` turns it off again for a subtree. Other space characters (a no-break space) are content and kept, though not at the ends of a block.
- **`Attrs["element"]`** on each block made from an element is the path of element names from the root as written in the file, prefixes included, joined with `/` (`Publication/Texte/Paragraphe`), so a consumer can tell `<name>` from `<address>`.
- **Nothing is guessed from names.** No headings, lists, tables or titles are inferred from element names. That is what profiles will be for.
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

UTF-8, UTF-16 and windows-1252 are decoded natively, and US-ASCII and ISO-8859-1 as windows-1252, as the WHATWG Encoding standard and browsers do (the bytes 0x80 to 0x9F are then the characters windows-1252 has there, not C1 controls). Every other label is looked up in a copy of go-fast-html's `internal/charset` (the other single-byte encodings, Shift_JIS, EUC-JP, GBK, GB18030, EUC-KR, Big5). A declared encoding not among them (ISO-2022-JP, UTF-7, EBCDIC), and UTF-32, is `ErrUnsupported`.

A file that declares no encoding and whose first MiB is not valid UTF-8 is read in the encoding recognised from its bytes where go-fast-html's detection is certain, with `text.encoding_detected` naming it; otherwise it is `ErrUnsupported`. Bytes that are not a character in the encoding are written as U+FFFD and counted in `text.unmapped_glyphs`, in UTF-8 files too.

## Entities and document type declarations

- **Never loaded.** No external DTD subset, external entity, parameter entity, schema, stylesheet or XInclude is read or fetched. The ones that would make a conforming processor load something are reported with `input.external_reference`, once per kind with a count: an external DTD subset, a reference to an external entity (or to an external parameter entity in the internal subset), an `xml-stylesheet` instruction, an XInclude `include` element. Schema location hints are only recorded in `Metadata.Extra`.
- **The internal subset** is read for general entity declarations only. Element, attribute-list (default attribute values included) and notation declarations are passed over; parameter entities are never expanded.
- **Internal entities** are expanded where referenced, their replacement read as markup (an entity may hold elements), in text and in attribute values. Nesting is bounded (`MaxEntityDepth`, 16), and so are the bytes all expansions produce together: `EntityExpansionBase` (1 MiB) plus `EntityExpansionRatio` (8) times the input size, and never more than the decode budget (`Limits.MaxDecodedBytes`, shared with embedding documents) has left. A reference past either bound is kept as written, and so is every expanding reference after the first one refused; the part carries `limit.exceeded`. A billion-laughs file and the quadratic-blowup variant finish in milliseconds. A reference to an entity inside its own replacement is kept as written and counted in `input.repaired`.
- **The five predefined entities and character references** always work. A character reference to a control character other than tab, line feed and carriage return is removed with `text.control_characters_removed`; one to a surrogate or beyond U+10FFFF is U+FFFD.
- **Undeclared entities** (`&eacute;` in a DocBook 4 or JATS file whose DTD is not loaded): if the file has an external DTD subset and the name is in the W3C/HTML character entity list (go-fast-html's table), the character is written and the part carries `xml.entities_assumed` (info) with the count. Otherwise the reference stays in the text as written (`&name;`) and the part carries `xml.entity_undeclared` (warning) with the count. A reference to an external entity also stays as written.

## Damaged input

The reader is lenient and counts what it mends in one `input.repaired` (warning) on the document: an end tag that matches an element further out closes the elements inside it; one that matches nothing is dropped; elements open at the end are closed; a `<` or `&` that starts no markup is text; an attribute without a value is passed over and one without quotes runs to white space or `>`; a quoted value that runs into a `<` lacks its closing quote and ends at the first `>` (which ends the tag) or else before the `<`, and so does a public or system identifier in a document type declaration; a tag or document type declaration cut off by the next `<` ends there; a comment or CDATA section open at the end runs to it; a second root element and text outside the root are read as they come; a second document type declaration, or one after the root element starts, is not read. Input with no element at all is `ErrMalformed`.

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
| `xml.unmapped_elements` | info | part | profile reading: elements with no rule read generically, names with counts |
| `xml.unmapped_attributes` | info | part | profile reading: attributes of mapped elements no rule reads, names with counts |
| `xml.skipped_elements` | info | part | profile reading: elements left out by a user profile's `skip` rules |
| `structure.heading_level_clamped` | info | part | profile reading: headings nested deeper than 6 have level 6 and their level in `Attrs["level"]` |

## Errors

`ErrMalformed` (no element, or a negative size), `ErrUnsupported` (an encoding not read), `ErrLimitExceeded` (larger than `Limits.MaxInputBytes`, or a tag over `MaxTagBytes` before any element), `ErrNoPartsSelected` (`Options.Parts` excludes part 1), and the context's error when it is cancelled.

## Limits

Core's limits: `Limits.MaxInputBytes` (default 4 GiB, `ErrLimitExceeded`); `Limits.MaxDecodedBytes` (default 1 GiB), which here bounds entity expansion; `Limits.MaxTextBytes` (default 256 MiB): every block takes `BlockCost` plus its text plus its `element` path from it, and when it is spent the part is cut there with `limit.exceeded` saying how much input was left unread.

Fixed bounds of this library, exported as constants. Each cut is `limit.exceeded` (error) on the part, once per bound with a count:

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
| table columns (profiles) | 1,024 | cells past it are left out |
| rows one cell spans (profiles) | 65,535 | the span is cut |
| a cell, label or value read from a child (profiles) | 1 MiB (`MaxBlockBytes`) | the text is cut there |
| parts started by `part` rules | 100,000, and `Limits.MaxParts` | the content stays in the last part; parts past `Limits.MaxParts` are left out |

With a profile, a block takes from `Limits.MaxTextBytes` its text, its `Marker`, its `Attrs` keys and values, its span addresses and, for a table, `CellCost` plus the text of each cell; a value a child gives later (`Attrs`, an image's alt text) is taken when it comes. The records of links in the stored part count the same way.

The context is checked every 1,024 tokens, and every 1,024 constructs passed over without a token (end tags that match nothing, elements past `MaxDepth`). Output is deterministic.

## What the file declares

Generic reading reports no `Display` reason: XML in general has no declaration of display that holds whatever the schema. Nothing about visibility is judged: no colour, size, position or overlap is looked at, and characters are kept as the file has them. Unshown data (comments, attributes) is read as data, never run or followed.

## Not read

- **Profiles** other than Service-Public (JATS, DocBook, TEI, DITA, Akoma Ntoso, ...): later phases of `XML_PLAN.md`.
- XSD validation, DTD validation, default attribute values from a DTD, XSLT.
- Processing instructions as text.
- External anything (see above).

## Tests

`go test ./...` runs everything; `-short` skips the large worst cases (a million attributes, a 64 MiB text node, a 200 MiB generated stream, a 17 MiB tag). An oracle test compares the character data with `encoding/xml`'s on 500 generated documents. `TestCorpus` reads `GOFAST_CORPUS` or `../.testdata/corpus` and runs every file of `dev/xml*/` and `heldout/xml*/` (counts only for held-out files); it skips when the corpus is absent. Fuzz targets: `FuzzExtract` and `FuzzProfile` (a profile with every rule kind) (.), `FuzzLoad` and `FuzzSelector` (`internal/profile`), `FuzzScanner` (`internal/xmlscan`), `FuzzParse` (`internal/dtd`).

Profiles: `TestProfile*` read small synthetic documents with one synthetic profile per shape of the format (headings by nesting, dispatch on attributes, CALS and rows tables with spans, labels, out-of-line footnotes, marks from conditions, parts, metadata, links, images, values from attributes); each checks `Validate` and that every block's text is at its offsets. `TestServicePublicCorpus` reads every Service-Public file of the dev and held-out sets (aggregate counts only for held-out) and fails on a panic, an error, a `Validate` failure or a file not read with the profile; it logs the unmapped elements and attributes. `TestServicePublicOracles` checks every dev file against two oracles: the body's block and cell text, in order, is the file's character data in order (white space aside) apart from a listed set of elements (metadata, the `Titre` that `TitreRiche` repeats, values read into `Attrs`, image alt text), each of which must occur; and every block's text is in the source at its `Loc.Offset`/`Length`.

`BenchmarkExtract` reads a synthetic 10 MiB document; `BenchmarkEncodingXML` tokenizes the same bytes with `encoding/xml` (tokens only, no document). `BenchmarkServicePublic` reads a fixed list of 200 Service-Public dev files with the profile and generically (about 96 and 103 MB/s, 1,050 and 1,390 allocations per file, on a Ryzen 9 9900X).

## Provenance

See `NOTICE`. The scanner is adapted from go-fast-odf's (itself from go-fast-ooxml); `internal/charset` and `internal/entity` are copies of go-fast-html's, with their tables' provenance.
