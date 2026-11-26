# Profile `newsml-g2`: IPTC NewsML-G2

News agency items (AFP, Reuters, AP, DPA, NTB, STT, ...) in IPTC
NewsML-G2, namespace `http://iptc.org/std/nar/2006-10-01/`.

## What selects it

The roots `newsItem`, `packageItem`, `newsMessage`, `knowledgeItem`,
`conceptItem`, `planningItem`, `catalogItem` and `catalog`, each in the
NAR namespace. The same names in no namespace or another stay generic
(`TestNewsmlSelection`). `catalog` is read as one name and value table:
it cost one rule.

## Mapping

| Source | Output |
|---|---|
| The root item, or each item of `newsMessage/itemSet` | A `section`; its `guid` is its anchor and also its **heading, level 1**, at the start tag (`"heading": {"attr": "guid"}`, `Attrs["attribute"]=guid`), so the item's tables come after a heading of their item and the guid is a block's `Loc.Anchor`. Every other block of the item carries the guid in `anchor_parent`. `@version`, `@standard`, `@standardversion`, `@conformance`, `xml:lang` as `newsml.*` |
| `newsMessage/header` | One name and value table |
| `contentMeta/headline` | Heading, level 2; for the root item also `Metadata.Title`. In a `newsMessage` the headlines are headings only and `Metadata.Title` stays empty |
| `itemMeta/edNote`, `contentMeta/description`, `contentMeta/edNote` | Paragraphs marked `newsml.field`; `@role`, `xml:lang` as `newsml.role`, `newsml.xml_lang` |
| Every other child of `itemMeta` and `contentMeta` (`itemClass`, `provider`, `versionCreated`, `firstCreated`, `pubStatus`, `signal`, `link`, `slugline`, `dateline`, `by`, `creditline`, `language`, `located`, `creator`, `contributor`, `subject`, `genre`, `urgency`, `keyword`, `altId`, ...) | A **name and value table** each (`pairs: path`, header `name | value`, the profile's): `subject/@qcode`, `subject/name`, `subject/name/@lang`, `subject/broader/@qcode`, ... `RowLocs` on each value's element. QCodes as written, never resolved |
| `catalogRef`, `rightsInfo`, `assert`, `inlineRef`, `derivedFrom`, `derivedFromValue`, `partMeta`, `hopHistory`, `pubHistory`, `catalog`, `schemeMeta`, `concept`, `itemRef`, `newsCoverage`, the children of `group` | Name and value tables |
| `contentSet/inlineXML` with `@contenttype` `application/xhtml+xml`, `application/xhtml+html`, `xhtml/xml` or `text/html` (as written on dev); `inlineXML/html` (XHTML namespace) | A `document` rule: the bytes between the tags are handed to the child extractor (`Options.Child`) as `application/xhtml+xml`; with no child extractor, the engine's fallback paragraph (the text content) with `media_type` |
| `inlineXML/nitf` in the NITF namespace | Read by the `nitf` profile's rules, copied into this profile (the format has no sharing of rules between profiles), with `namespace_aliases` mapping the NITF namespace to no namespace so that they apply unchanged. The NITF `head` is its own name and value table; the NITF `hl1` is a heading only |
| `inlineXML/p` (NAR namespace) and NAR-namespace `p`, `ul`, `ol`, `li`, `section`, `div`, `a`, `em`, `i`, `strong`, `b`, `span`, `br` (met in `edNote`) | Paragraphs, lists, inline styles, links, breaks |
| `inlineData` | A paragraph; `@contenttype`, `@rendition`, `@encoding`, `xml:lang` as `newsml.*` |
| `remoteContent` | A name and value table per rendition (`remoteContent/@href`, `@residref`, `@contenttype`, `@size`, `@width`, `@height`, `@rendition`, ... and its children), `RowLocs` per value, not one table with a row per rendition: a rendition's attributes vary and a row of fixed columns would drop those not named |
| `groupSet/group` | Transparent; `@id` its anchor, `@role`, `@mode` mark its blocks; each `itemRef` a name and value table (`itemRef/@residref`, `itemRef/@href`, `itemRef/title`, ...) |

## Other vocabularies, and what is left to generic reading

Any other child of `inlineXML` (`sports-content`, 8 dev files; `events`,
8; STT's own upper-case vocabulary `BODY/TEXT/BRODTEXT/P`; a `nitf`
written in the NAR namespace, 2 dev files, which the copied NITF rules
do not match) is one name and value table (`pairs: path`, with
`pairs_empty`): every value and attribute with the file's own name
(`sports-content/sports-event/team/team-metadata/name`), so nothing of
it is unnamed, but its running text is rows, not paragraphs.

Left generic on dev (112 elements in all): XHTML-like elements inside an
`edNote` of DPA's files (`li` 23, `a` 14, `section` 12, `h4` 11, `ul` 11,
`p` 9, `h3` 4; their text stays in the note's paragraph), `ruby`, `rb`,
`rt`, `rp` (2 each), a `header` outside a `newsMessage` (4), NITF
`headline`/`hl1` in two agency files (6), and one file's own envelope
(`Header`, `Sent`, `sender`, ...).

Unread attributes on dev: `formatversion` 21, `inlineXML@wordcount` 20,
`lang` 14, `schemaLocation` 9, `edNote@pubconstraint` 7, `headline@rank`
5, and 11 others once each (`description@confidence`, `@relevance`,
`edNote@dir`, `headline@confidence`, `@dir`, `@relevance`,
`inlineData@hascontent`, `inlineXML@charcount`, `@hascontent`,
`@linecount`, `newsMessage@lang`).

## Dev set (340 files)

340 read, 0 errors, 0 `Validate` failures. Unmapped elements: 112 in
all (the list above). Unmapped attributes: 87 in all. Blocks: heading 497,
paragraph 358, table 5,421, note (attribute) 34; stored: markup_comment
466.

Oracles (`TestNewsmlOracles`): (a) the body's text in order equals the
file's character data in order, white space aside, the profile's header
rows and the name column and attribute rows of name and value tables
aside, with one tested exception, met in 1 dev file: a value of a name and
value table that starts with inline markup (`copyrightNotice` holding an
`a`) is read by the engine in another order (same characters). (b) Every
block's text is in the source at its offsets. Both pass.

## Held-out set (115 files)

115 read, 0 errors, 0 `Validate` failures. Unmapped elements 0, unmapped
attributes 28. Blocks: heading 180, paragraph 102, table 1,908; stored:
markup_comment 97.
