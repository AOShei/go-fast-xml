# Profile `tei`: TEI P5

## What selects it

The root `TEI` or `teiCorpus` in the namespace
`http://www.tei-c.org/ns/1.0` (TEI P5). TEI P4 and older (no namespace)
are not matched and are read generically.

## Parts

`teiHeader`, `front`, `body`, `back`, `standOff` and `facsimile` are
`body` parts named after the element; a `teiCorpus` gives these parts
for each `TEI` in turn. `text` and `group` are transparent.

## Mapping

| Element | Model |
|---|---|
| `titleStmt/title` | `Metadata.Title` (the first; typed `sub`, `alt`, `short`, `desc`, `parallel` to Extra) |
| `titleStmt/author` | `Metadata.Authors` |
| `titleStmt/editor`, `respStmt`, `funder`, `sponsor`, `principal`; `editionStmt`, `extent`, `publicationStmt/*`, `seriesStmt`, `notesStmt/note`, `sourceDesc/bibl`, `biblFull`, `biblStruct`, `langUsage/language`, `textClass`, `creation`, `textDesc`, `revisionDesc` | `Metadata.Extra` `tei.*` (below); `keywords/term` to `Metadata.Keywords` |
| `encodingDesc/refsDecl`, `tagsDecl`, `classDecl`, `listPrefixDef`, `schemaRef` | stored part (`custom_xml`); `appInfo` stored as `generator` |
| `encodingDesc/projectDesc`, `editorialDecl`, `sourceDesc/p` | paragraphs in the teiHeader part, marked `tei.in` |
| `particDesc/listPerson`, `person`, `personGrp` | list; a person is an item (its `persName` the text, birth, death, occupation... after it, set off by spaces), anchor `xml:id` |
| `msDesc` | section in the teiHeader part: `msIdentifier`, `msItem/title`, `author`, `locus`, `note`, `incipit`, `explicit`, `rubric`, `support`, `layout`, `handNote`, `decoNote`, `origin`, `provenance`, `acquisition`, `binding`... each a paragraph with `tei.field` the element; `msItem@n` marks its blocks `tei.ms_item` |
| `div`, `div1`–`div7` | section; `head` the heading (level by nesting); every block inside carries `tei.divN_n`, `tei.divN_type`, `tei.divN_subtype` for each enclosing level N (1 to 4), the citation of Perseus and Folger texts (book 1, chapter 2, section 3 = `tei.div1_n=1`, `tei.div2_n=2`, `tei.div3_n=3`) |
| `p`, `ab` | paragraph, anchor `xml:id`, `tei.n` |
| `l` | **one paragraph per verse line**, anchor `xml:id`, `tei.n` the line number, `tei.field=l`; `lg` marks its lines `tei.lg` (its type) |
| `sp` | its blocks marked `tei.who`; `speaker` is the `Marker` of the speech's first block |
| `stage` | paragraph, `tei.field=stage`, `tei.type` |
| `castList`, `castGroup`, `castItem` | lists and items |
| `note` | footnote, `Marker` from `@n`, `tei.type`, `tei.place`; inside a `p` or `l` it ends the block there (`split`) and text after it is a new block |
| `pb`, `cb`, `milestone` | between blocks: a `note` of type `attribute` whose text is `@n` (`Attrs["attribute"]=n`, `tei.milestone=pb`); inside a `p`, `l`, `ab`, `head`, `stage`, `note`, `item` or `quote`: a record in the stored part (`custom_property`, name `pb`, `href` = `@n`, `tei.milestone`, `tei.facs`) at the pb's offset. Never block text |
| `lb` | a line break (`break`: `\n` in the block; a space in a value); `lb[@break='no']`, a break inside a word, adds nothing; `lb@n` not read |
| `choice` | every alternative stays in the text, in the file's order (`teh the`); each of `sic`, `corr`, `orig`, `reg`, `abbr`, `expan` also gives a record in the stored part saying which it is (`tei.choice`) |
| `app`, `lem`, `rdg` | in the text; records with `tei.app` = `lem`/`rdg` and `tei.wit` |
| `add`, `del` | in the text; records with `tei.edit` = `add`/`del` |
| `list`, `item`, `list/label` | list (ordered when `@type` is `ordered` or `numbered`), items, label as `Marker` |
| `table`, `row`, `cell` | table; `role="label"` rows and cells are headers; `cols`/`rows` spans |
| `figure`, `graphic`, `figDesc` | image (id = `url`), `figDesc` a paragraph |
| `quote` (outside a paragraph), `epigraph` | quote |
| `entry` (dictionaries) | one paragraph per entry, anchor `xml:id`; `form`, `orth`, `gramGrp`, `sense`, `cit/quote`... set off by spaces |
| `hi` | span by `@rend` (`italic`, `bold`, `sup`, `sub`); `emph` italic; names, dates, `w`, `c`, `pc`, `seg` and the like in the text |
| `ref`, `ptr` | link spans to `@target` |
| `titlePage`, `docTitle` (level-1 heading), `docAuthor`, `docImprint`, `byline`, `trailer`, `opener`, `closer` | headings and paragraphs |

## Metadata.Extra keys

`tei.id`, `tei.lang`, `tei.n` (of `TEI`), `tei.text_lang`,
`tei.text_type`, `tei.subtitle`, `tei.title_alt`, `tei.title_short`,
`tei.title_desc`, `tei.title_parallel`, `tei.author_key`,
`tei.author_ref`, `tei.editor` (+ `_role`), `tei.resp`, `tei.funder`,
`tei.sponsor`, `tei.principal`, `tei.edition`, `tei.extent`,
`tei.publisher`, `tei.distributor`, `tei.authority`, `tei.pub_place`,
`tei.pub_address`, `tei.pub_date` (+ `_when`), `tei.idno` (+ `_type`),
`tei.availability` (+ `_status`), `tei.publication`, `tei.pub_ref` (+
`_target`), `tei.series`, `tei.notes`, `tei.source`, `tei.language` (+
`_ident`, `_usage`), `tei.class_code` (+ `_scheme`), `tei.cat_ref`,
`tei.creation`, `tei.text_desc`, `tei.revision` (+ `_status`). Repeated
values are joined (`; ` or a line break); parallel keys follow the same
order. Dates are as written.

## Attrs keys

`tei.in`, `tei.field`, `tei.n`, `tei.divN_n`/`_type`/`_subtype`,
`tei.who`, `tei.lg`, `tei.lg_n`, `tei.part`, `tei.type`, `tei.place`,
`tei.target`, `tei.list_type`, `tei.box`, `tei.ms_item`,
`tei.milestone`, `tei.facs`, `tei.ed`, `tei.choice`, `tei.app`,
`tei.wit`, `tei.edit`, `tei.sex`, `tei.role`.

## How to cite a block

`Loc.Offset`/`Length`; `Loc.Anchor` = the element's `xml:id` (Folger
gives every line and speech one); the `tei.divN_n` attributes and
`tei.n` give the canonical reference (Hamlet 1.2 line 1, John 1.1). Page
numbers are `pb` notes between blocks, or stored records with their
offset inside a block.

## Limits of the format met here

- A `note` or `figure` inside a `p` or `l` ends the block there
  (`split`): the line or paragraph is in two blocks, the note between.
- A `pb` inside a block can be recorded only through a link record (its
  `href` is the page number); a key for "a record of an empty element's
  attribute" would say it plainly.
- `titleStmt/title`: only the first untyped title fills `Metadata.Title`;
  a second untyped title is not kept.

## Left to generic reading

`gap` (an empty element: generic reading writes its `reason`, `extent`,
`unit` as attribute notes, which is what it says); `standOff` contents
(`listEvent/event`, `desc`, `listRelation/relation`); ELTeC's
`textDesc` children in the ELTeC namespace (`authorGender`, `size`,
`reprintCount`, `timeSlot`); stray HTML-like `b`, `i`, `sub`, `sup`,
`smcap`, `ul` in one sample. Not read (counted): the linguistic
annotation of `w` (`@lemma`, `@ana`, `@n`), `lb@n`, `cit@type`,
`sense@level`, `form@type`, `@key`/`@ref` of names.

## Versions

Samples: TEI P5 (ELTeC, Folger TEI Simple, DraCor, Perseus/EpiDoc,
papyri.info EpiDoc, Bodleian manuscript descriptions, FreeDict
dictionaries, TEI test files). Not seen: `teiCorpus`, `facsimile`,
`div5`–`div7`, P4 files (not matched).
