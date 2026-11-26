# Profile `nitf`: News Industry Text Format

Wire copy and archives in IPTC's NITF 3.x (the DTD and the XSD), as AP,
AAP, PA, EFE, NTB, ANSA and others deliver it.

## What selects it

The root `nitf` in no namespace, or in `http://iptc.org/std/NITF/2006-10-18/`
(the namespace NewsML-G2 puts NITF in): `"namespace_aliases"` maps that
namespace to no namespace, so one set of rules reads both. A `nitf` root
in any other namespace (including the same URI without its final `/`)
stays generic, as does NITF without its root (`TestNitfSelection`).

## Mapping

| Source | Output |
|---|---|
| `nitf` | Its `doc-id@id-string` (`head/docdata/doc-id`) is the document's identifier: every block carries it in `anchor_parent`. `@version`, `@change.date`, `@change.time`, `@baselang`, `@uno`, `@class` as `nitf.version`, ... on the first block |
| `head` | One **name and value table** (`pairs`, header `name | value`, the profile's), the names the file's paths from `head` with attributes as `@name`: `head/title`, `head/meta/@name` followed by `head/meta/@content`, `head/tobject/tobject.subject/@tobject.subject.refnum`, `head/docdata/doc-id/@id-string`, `.../date.issue/@norm`, `.../date.release/@norm`, `.../urgency/@ed-urg`, `.../doc.copyright/@holder`, `.../key-list/keyword/@key`, `.../identified-content/...`, `head/pubdata/@...`, `head/revision-history/@...`, and anything else the head holds. `RowLocs` on each value's element. Codes and dates as written |
| `hedline/hl1` | `Metadata.Title` and a heading, level 1 (inline markup kept) |
| `hl2` (in `hedline`, `block` or `body.content`) | Heading, level 2; `@id` its anchor, `@class` as `nitf.class` |
| `byline` (`person`, `byttl`), `dateline` (`location`, `city`, `state`, `story.date`), `distributor` (`org`), `rights` (`rights.owner`, ...), `body.end/tagline`, `body.end/bibliography` | A paragraph each, marked `nitf.field` with the element's name |
| `abstract`, `body.head/note` | Their paragraphs marked `nitf.field=abstract` / `note`; `note@noteclass`, `@type` as `nitf.noteclass`, `nitf.note_type` |
| `body.head/series` | A name and value table (`series/@series.name`, ...) |
| `p` | A paragraph; `@id` its anchor; `@lede`, `@class`, `@lang`, `xml:lang`, `@summary`, `@optional-text`, `@quote-source` (and `@lead`, met on dev) as `nitf.*` |
| `block` | Transparent; `@id` its anchor, `@class` marks every block inside as `nitf.class` |
| `lang` | Transparent; `@lang` marks the blocks inside as `nitf.lang` |
| `bq` | A quote block; its `block`/`p` and `credit` are lines of it |
| `credit` (elsewhere) | A paragraph, `nitf.field=credit` |
| `ul`, `ol`, `li` | Lists |
| `dl`, `dt`, `dd` | A list, an item each, `nitf.element=dt` / `dd` |
| `pre` | A code block |
| `fn` | A footnote block; met inside a paragraph it splits it there (`split`) |
| `hr`, `br` | A line break |
| `media` | Transparent; `@media-type`, `@style` mark its blocks |
| `media-reference` | An image: `@source` its address, `@alternate-text` its alt and text; `@mime-type`, `@height`, `@width`, `@units`, `@name`, `@data-location`, `@coding`, `@time`, `@time-unit-of-measure`, `@outcue`, `@source-credit`, `@copyright` in `Attrs` under their own names |
| `media-caption` | A caption block |
| `media-producer` | A paragraph, `nitf.field=media-producer` |
| `media-metadata`, `media-object` | Name and value tables |
| `nitf-table` | Transparent; `nitf-table-metadata` is a name and value table (`nitf-table-metadata/nitf-col/@value`, the summary's paragraph as `nitf-table-metadata/nitf-table-summary/p`) |
| `table`, `tr`, `th`, `td`, `caption`, `thead`, `tbody`, `tfoot`, `col`, `colgroup` | A table (rows model, `RowLocs` per row); `thead/tr` and all-`th` leading rows are header rows; `colspan`, `rowspan` kept; `table/caption` a caption block |
| `em`, `strong`, `sub`, `sup`, `a@href` | Inline styles and links; `a@rel`, `@rev`, `@name` in the link's record |
| `person`, `org`, `function`, `location`, `sublocation`, `city`, `state`, `region`, `country`, `chron`, `money`, `num`, `frac`, `event`, `object.title`, `alt-code`, `virtloc`, `classifier`, `copyrite`, `q`, `pronounce`, `name.given`, `name.family` | Inline: their text stays in the paragraph. Their attributes (`@idsrc`, `@value`, `chron@norm`, `money@unit`, `num@units`, `location@location-code`, `country@iso-cc`, ...) go, under their own names, in the record in the stored part an inline with `attrs` makes (as Service-Public's `TermeEtranger@langue`) |

`Metadata.Created` is the first `docdata/date.issue@norm` the engine
reads as a date: ISO 8601 basic date and time (`20020225T143000-0500`,
with `Z`, a numeric offset or no zone, which is read as UTC) or ISO 8601
extended. The value is also a row of the head table, as written. A date
written another way (`2022-07-11 18:33`, with a space) is only the row.
`date.release` and `date.expire` are rows only. On the dev set 15 of the
17 files have `Created`; the 2 others write the date with a space. NITF
inside a NewsML-G2 item fills it the same way (1 dev file has one, with
a space).

`Loc.Anchor`: `doc-id@id-string` is an attribute of a child of `head`;
the engine's `"heading": "anchor"` takes only a child's text and a
`metadata` rule (the title) takes no `anchor`, so no block has it as its
own `Loc.Anchor`: every block has it as `anchor_parent`. Blocks with an
`id` of their own (`p`, `hl2`, `block`, `fn`, `table`) have it as anchor.

## Left to generic reading

On dev: `hl7` (1, an element that is not NITF, in IPTC's sample) is read
generically (an inferred paragraph). Attributes counted as unread:
`td@class` 360, `th@class` 66 (presentation classes of AP's agate
tables; a cell rule takes no `attrs`), `hl1@id` 4 (the title is a
`metadata` rule, which takes no anchor), `nitf@xsi:schemaLocation` 1.

Not met on dev, written from the NITF 3.x element set: `ol`, `dl`/`dt`/`dd`,
`fn`, `hr`, `q`, `sub`, `sup`, `chron`, `money`, `num`, `frac`, `event`,
`function`, `object.title`, `virtloc`, `copyrite`, `pronounce`,
`media-object`, `tfoot`, `col`, `colgroup`, `table/caption`, `bibliography`,
`rights.*` other than `rights.owner`, `body.head/series`. `media-object`
(encoded data) is read as a name and value table: its text as written.

## Dev set (17 files)

17 read, 0 errors, 0 `Validate` failures. Unmapped elements: `hl7` 1.
Unmapped attributes: `td@class` 360, `th@class` 66, `hl1@id` 4,
`nitf@schemaLocation` 1. Blocks: caption 2, code 1, heading 50, image 3,
list_item 10, paragraph 277, table 27; stored: custom_property 6,
markup_comment 70.

Oracles (`TestNitfOracles`): (a) the body's block and cell text in order
equals the file's character data in order, white space aside, with no
exception; the profile's header rows and the name column and attribute
rows of name and value tables are not compared, being profile or
attribute text. (b) Every block's text is in the source at its offsets
(for the files declared ISO-8859-1, in the span decoded from it). Both
pass.

## Held-out set (10 files)

10 read, 0 errors, 0 `Validate` failures, unmapped elements 0, unmapped
attributes 1. Blocks: caption 3, heading 19, image 3, list_item 9,
paragraph 122, table 17; stored: custom_property 18, markup_comment 99.
Warnings: `input.external_reference` 3.
