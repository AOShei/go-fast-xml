# Profile `xliff`: XLIFF 1.0 to 1.2

XLIFF 1.2 (OASIS 2008), 1.1, and 1.0; with the files SDL Trados
(`.sdlxliff`), memoQ (`.mqxliff`) and Okapi write in XLIFF 1.2.

## Selection

Root `xliff` in `urn:oasis:names:tc:xliff:document:1.2` or
`urn:oasis:names:tc:xliff:document:1.1` (any `version`), or in no
namespace with `version` `1.0`, `1.1` or `1.2`. Both namespaces are
aliases of no namespace (`namespace_aliases`), so one set of rules reads
all three. An `xliff` root in another namespace (XLIFF 2.x, `urn:x`), or
in none with another or no `version`, stays generic
(`TestXLIFFSelection`). XLIFF 2.0 to 2.2 are **not** read by this
profile: they stay generic (393 dev files).

Dev roots: 1.2 namespace 500 files (408 also carry `its:version="2.0"`,
the ITS version), 1.1 namespace 7, no namespace 12 (`version` 1.0: 1,
1.2: 11); 2.0 namespace 229, 2.2 namespace 164.

## Mapping

| Element | Output |
|---|---|
| `xliff` | `Extra["xliff.version"]` |
| `file` | a `section`; its heading is `original` (else `file@id`; with neither, none), from the start tag (`"heading"`, `Attrs["attribute"]="original"`); every block in it carries `mark` `xliff.original`, `xliff.source_language`, `xliff.target_language`, `xliff.datatype`, `xliff.tool`, `xliff.date`, `xliff.category`, `xliff.product_name`, `xliff.product_version`, `xliff.build_num`; `Loc.Anchor` from `file@id` |
| `header/note` | paragraph, `xliff.field=note`, `xliff.from`, `xliff.priority`, `xliff.annotates` |
| `header/tool` | paragraph whose text is `tool-name` (`Attrs["attribute"]`), `xliff.tool_id`, `xliff.tool_version`, `xliff.tool_company` |
| `phase-group/phase` | a table, a row per `phase` from its attributes (`phase-name`, `process-name`, `company-name`, `tool-id`, `date`, `job-id`, `contact-name`, `contact-email`, `contact-phone`), its `note` in a tenth column |
| `count-group/count`, `context-group/context` (outside a unit) | paragraph, `xliff.count_type`/`xliff.unit`, `xliff.context_type` |
| `glossary`, `reference`, `skl` | `external-file@href` a `note` of type `attribute` (`xliff.uid`, `xliff.crc`); `internal-file` a paragraph (`xliff.form`); `internal-file[@form='base64']` an embedded document (`"encoding": "base64"`, media type from `mime-type`, absent: `application/octet-stream`); the base64 is never body text |
| `body` | **one table per `file`**, header row the profile's `id, resname, restype, approved, translate, state, source, target, notes, other` (`header_source=profile`); `xliff.original` on the table |
| `group` | a row of the file's table from its start tag's `id`, `resname`, `restype`, `approved`, `translate` (columns 1 to 5, the rest empty), before its units' rows; `RowLocs` the start tag, `Anchor` its `id`. A group with none of them makes no row. Rows are in file order, so a unit follows its group's row; where a group ends, and nested groups, are not kept in the table (a table holds no tree), only in the file at the rows' offsets |
| `trans-unit`, `bin-unit` | a row; `RowLocs` covers the element, `Anchor` its `id` as written. Columns 1 to 5 its attributes `id`, `resname`, `restype`, `approved`, `translate` as written; 6 the `state` of its first `target` (`bin-target`), as written; 7 `source` (`bin-source`); 8 `target` (`bin-target`); 9 `note` followed by its `from`; 10 everything else in the unit in file order, one per line: `seg-source`, `alt-trans` (`source`, `target`, `note`, then a line of its `match-quality`, `origin`, `tool`), `context` followed by its `context-type`, `count` followed by `count-type` and `unit`, extension elements |
| `g`, `mrk`, `sub` | inline: their text in place |
| `bpt`, `ept`, `ph`, `it`, `ut` | inline code: the native code is text where it stands |
| `x`, `bx`, `ex` | their `equiv-text`, when present, written where they stand (`value`); otherwise nothing |

A consumer cites one unit by its row's `RowLocs[i]` (bytes of the
`trans-unit`) and `Anchor` (its `id`, unique within a `file`); the file
by the blocks' `xliff.original`.

## Left generic or counted

- `xml.unmapped_elements` on dev (files read with xliff, 163,814): all
  in extension namespaces: SDL (`sdl:value`, `tag`, `cxt`, `props`,
  `bpt`, `ept`, `fmt`, `node-def`, `seg-defs`, ...), memoQ, Trados
  "asset" metadata, ITS, Okapi, LISA metrics. They are read generically
  (text as inferred paragraphs, in the `other` column inside a unit;
  attributes as notes): nothing is dropped.
- `xml.unmapped_attributes` (306,706; 314,042 before the `state`
  column, the group rows and `alt-trans`'s attributes were read). What is
  left has no text for a reader, apart from `ph@equiv-text` (823) and
  `g@equiv-text` (93), see below:
  - inline-code bookkeeping, identifiers and codes: `mrk@mtype` 41,096,
    `mrk@mid` 41,082, `ph@id` 30,850, `bpt@id` 30,734, `ept@id` 30,734,
    `bpt@ctype` 30,519, `ph@x` 19,174, `x@id` 11,321, `ph@ctype` 10,367,
    `g@id` 7,238, `it@ctype`/`id`/`pos` 4,875 each, `x@xid`, `g@ctype`,
    `g@xid`, `bx@id`, `ex@id`, `g@href`, `g@target`;
  - `xml:lang` of `source`/`target` 31,616 (the file's languages are in
    `mark`), `xml:space`;
  - unit data with no text: `trans-unit@datatype` 3,267, `style`,
    `coord`, `ts`, `font`, `maxwidth`, `size-unit`, `minwidth`,
    `extradata`; `target@state-qualifier` 16, `target@phase-name` 3,
    `target@state` 3 (a second `target` of a unit: the column takes the
    first); `alt-trans@extype` 7, `alttranstype` 3, `mid` 2;
    `bin-unit@mime-type` 2; `context-group@name`, `count-group@name` 3
    each; `note@annotates` 3, `note@priority` 1 (in a unit);
  - extension attributes: SDL `file@inputEncoding` 598, ITS
    `its:version` 454, memoQ timestamps, `status`, `segmentguid`,
    `segpart` (120 each), Okapi `xliff2-state`, ITS localisation-quality
    references, and single attributes of single files.
  - `ph@equiv-text`, `g@equiv-text`: the text a tool shows for a code
    whose native code (`ph`) or content (`g`) is already written in
    place; writing both (`with`) put the equivalent next to the code
    inside the same words, which the two oracles cannot tell from the
    file's text. Left counted.

## Measured

- dev: 921 files, 528 read with xliff, 393 generic (XLIFF 2.x); 0
  errors, 0 Validate failures, 0 panics; blocks over the set: table 708,
  paragraph 63,060, note (attribute) 197,379, stored 374; `limit.exceeded`
  1.
- held-out: 325 files, 192 read with xliff, 133 generic; 0 errors, 0
  Validate failures; table 272, paragraph 49,023, note 172,138;
  `limit.exceeded` 1.
- Largest dev file (7.0 MB, 4,839 rows): 352 ms, 419 MiB allocated over
  the read, 148 MiB heap in use after.

## Oracles

(a) The body's text in order is the file's character data in order, with
`note@from`, `context@context-type`, `count@count-type unit` (in a unit),
`alt-trans@match-quality origin tool` and the `equiv-text` of `x`, `bx`,
`ex` where the profile writes them, apart from: the profile's header
row, the attribute columns 1 to 6 and blocks whose text is an attribute
(`Attrs["attribute"]`: the `file` heading, `tool`);
`internal-file form="base64"` (handed to the child extractor); files
encoding/xml stops in (12 on dev, compared up to there); and units
whose children are not in column order (31 files), where the text is
the same characters and (b) places every row. (b) Every block's text is
in the source at its offsets, and every row's cells in the bytes of its
unit (2 files not in UTF-8 are not compared byte for byte).
