# Profile `xliff2`: XLIFF 2.0 to 2.2

XLIFF Version 2.0 (OASIS 2014), 2.1 (2018) and 2.2 core, with the
Translation Candidates (`mtc:`), Glossary (`gls:`), Metadata (`mda:`)
and Change Tracking (`ctr:`) modules mapped; the other modules are read
generically.

## Selection

Root `xliff` in `urn:oasis:names:tc:xliff:document:2.0` (XLIFF 2.0 and
2.1, which keeps the 2.0 namespace) or `urn:oasis:names:tc:xliff:document:2.2`
(any `version`). Both are aliases of no namespace (`namespace_aliases`),
so one set of rules reads them. An `xliff` root in another namespace
(`urn:x`, a `document:2.1` namespace no version uses), or in none with
`version` `2.0`, stays generic; XLIFF 1.x stays with `xliff`
(`TestXLIFF2Selection`, `TestXLIFFSelection`).

Dev roots: 2.0 namespace 229 files (`version` 2.0: 228, 2.1: 1), 2.2
namespace 164 (`version` 2.2). Most are the OASIS 2.1 and 2.2 test
suites and Okapi's test files, valid and invalid on purpose.

## Mapping

Keys of `Extra` and of `Attrs` carry the prefix `xliff2.`.

| Element | Output |
|---|---|
| `xliff` | `Extra["xliff2.version"]`; `srcLang`, `trgLang` as `mark` `xliff2.source_language`, `xliff2.target_language` on every block (as `xliff` keeps the 1.x `file` languages) |
| `file` | a `section` whose heading is `original`, else `file@id` (`Attrs["attribute"]` names which), `Loc.Anchor` `file@id`, `mark` `xliff2.original`; and **the file's table** (`table_header`), header row the profile's `unit, name, type, segment, state, source, target, other` (`header_source=profile`), its block where the file ends, after the file's other blocks |
| `file/notes/note`, `unit/notes/note`, `group/notes/note` | paragraph, `xliff2.field=note`, `xliff2.note_id`, `xliff2.category`, `xliff2.priority`, `xliff2.applies_to` |
| `originalData/data` | paragraph (the original code, as written), `xliff2.field=data`, `xliff2.data_id` |
| `skeleton` | transparent, `mark` `xliff2.field=skeleton`: its content read generically |
| `group` | a row from its start tag's `id`, `name`, `type` (columns 1 to 3), `RowLocs` the start tag, `Anchor` its `id`; with none of them, no row |
| `unit` | transparent; `Loc.Anchor`/`anchor_parent` its `id`; `mark` `xliff2.unit_name`, `xliff2.unit_type` on the blocks made in it (its notes, original data, module paragraphs) |
| `segment`, `ignorable` | **a row each**: columns 1 to 3 the unit's `id`, `name`, `type` (`cells` with `of`: outside the row's span), 4 its own `id`, 5 `segment@state` as written, 6 `source`, 7 `target`, 8 anything else in it read generically; `RowLocs` the element, `Anchor` the unit's `id` (unique in a file; segment ids are optional and unique only in a unit) |
| `pc`, `mrk` | inline: their text in place; `mrk@value` (a comment's text) written after it (`with`) |
| `sm` | inline: nothing but its `value` (`with`) |
| `em`, `ph`, `sc`, `ec` | inline: nothing (placeholders of codes; their attributes counted) |
| `cp` | the character `hex` names (`codepoint`); a control character (category Cc, what `cp` is for), surrogate, or value past U+10FFFF writes nothing and `cp@hex` is counted |
| `mtc:matches/mtc:match` | its `source` and `target` paragraphs with `xliff2.part`, `mark` `xliff2.field=match`, `xliff2.match_id`, `xliff2.match_ref`, `xliff2.similarity`, `xliff2.match_quality`, `xliff2.origin`, `xliff2.match_type`, `xliff2.match_subtype`, `xliff2.reference` |
| `gls:glossEntry` | `gls:term`, `gls:translation`, `gls:definition` paragraphs (`xliff2.part`, `xliff2.gls_source`, `xliff2.gls_translation_id`, `xliff2.gls_translation_ref`), `mark` `xliff2.field=glossary`, `xliff2.gls_id`, `xliff2.gls_ref` |
| `mda:metadata/mda:metaGroup/mda:meta` | paragraph, `xliff2.mda_type`, `mark` `xliff2.field=metadata`, `xliff2.mda_id`, `xliff2.mda_category`, `xliff2.mda_group_id`, `xliff2.mda_applies_to` |
| `ctr:changeTrack/ctr:revisions/ctr:revision/ctr:item` | paragraph (an earlier text), `xliff2.ctr_property`, `mark` `xliff2.field=change_track`, `xliff2.ctr_applies_to`, `xliff2.ctr_ref`, `xliff2.ctr_current_version`, `xliff2.ctr_author`, `xliff2.ctr_datetime`, `xliff2.ctr_version` |

A consumer cites one segment by its row's `RowLocs[i]` (bytes of the
`segment`) and `Anchor` (its unit's `id`); the unit's notes and modules
by their blocks' `anchor_parent`; the file by `xliff2.original` or the
heading's anchor. Blocks in a unit come before the file's table (a
table holds rows, not paragraphs).

## Left generic or counted

- `xml.unmapped_elements` on dev (393 files): 415, read generically
  (text as paragraphs, in the `other` column inside a segment;
  attributes as notes): Size and Length Restriction (`slr:profiles` 33,
  `normalization` 29, `data` 11, `sizeData`/`sub_sizeData` 11 each),
  Validation (`val:validation` 25, `rule` 27, `validationBad` 2),
  Resource Data (`res:resourceData` 14, `resourceItem` 22,
  `resourceItemRef` 11, its `source` 27, `target` 26, `reference` 3:
  mostly `href`s), a TBX `martif` and a TEI document inside a `file`
  (`termNote` 10 and others), the test suites' extension elements
  (`elem2` 30, `elem` 18, `elem1` 16, `badElem` 7, `okElem` 6, ...),
  elements where the schema has none (`segment` in `target` 3, `meta` in
  the core namespace 2, `item` 2), ITS `locQualityIssue(s)` 3, Okapi
  `additionalTagData`, `tag`, `tagId`, `type` 1 each.
- `xml.unmapped_attributes` on dev: 10,263. Inline-code bookkeeping:
  `pc@id` 1,327, `dataRefStart` 1,228, `dataRefEnd` 1,226, `canCopy`
  1,211, `canDelete` 1,211; `ph@id` 770, `canDelete` 657, `canCopy` 651,
  `dataRef` 611, `canReorder` 85; `mrk@id` 94, `type` 90, `ref` 40;
  `sc`, `ec`, `sm`, `em` ids and flags; `ph@disp` 42, `pc@dispStart`/
  `dispEnd` 16 each, `equivStart`/`equivEnd` 2 (the text a tool shows
  for a code: not written, as the task set); `cp@hex` 16 (controls);
  `segment@canResegment` 22, `subState` 21, `target@order` 28;
  `translate`, `srcDir`, `trgDir`, `canResegment` of file, group, unit;
  `xml:lang` 49, `xml:space`; `slr:` and `fs:` attributes; the test
  suites' extension attributes.

## Measured

- dev (921 files of `xml-xliff`): 528 read with xliff, 393 with xliff2;
  0 errors, 0 Validate failures, 0 panics; blocks over the whole set:
  heading 1,145, table 1,103, paragraph 61,901, note (attribute)
  182,339, stored 374 (heading 749 and table 708 before: 396 headings
  and 395 tables from the 2.x files); `xml.unmapped_elements` 415,
  `xml.unmapped_attributes` 10,263 (files read with xliff2).
- held-out (325 files): 192 xliff, 133 xliff2; 0 errors, 0 Validate
  failures; heading 434, table 405, paragraph 48,705, note 170,079,
  stored 135; `xml.unmapped_elements` 137, `xml.unmapped_attributes`
  719 (files read with xliff2).
- Output of every other file unchanged (`TestOutputDigests`: 393 of
  15,344 differ, the 2.x files).

## Oracles

(a) The body's text in order is the file's character data in order,
with `mrk@value`, `sm@value` where the profile writes them and the
character of `cp@hex` (none for a control), apart from: the profile's
header row, the five attribute columns (unit `id`, `name`, `type`,
segment `id`, `state`) and blocks whose text is an attribute (the
`file` heading); and files where the order differs (25: the blocks of
a unit, such as notes, original data and matches, come before the file's
table, and a `target` may stand before its `source`), where
the text is the same characters and (b) places every row. (b) Every
block's text is in the source at its offsets, and every row's cells
from column 4 on in the bytes of its element (columns 1 to 3 are the
unit's attributes, outside it).
