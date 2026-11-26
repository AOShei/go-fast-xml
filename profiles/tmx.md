# Profile `tmx`: TMX translation memories

TMX 1.4b (LISA OSCAR, later ETSI GS LIS 002), and the 1.1 to 1.3 files
that write `tuv@lang` instead of `xml:lang`.

## Selection

Root `tmx` in no namespace **with a `version` attribute** (required by
every TMX DTD; TMX has no namespace, so the attribute is the second
test). A `tmx` root with no `version`, or in any namespace, stays
generic (`TestTMXSelection`). No other profile matches `tmx`.

All 40 dev files have `version="1.4"`; all their `tuv` carry `xml:lang`.

## Mapping

| Element | Output |
|---|---|
| `tmx` | `Extra["tmx.version"]` |
| `header` attributes | `Extra["tmx.srclang"]`, `tmx.adminlang`, `tmx.creationtool`, `tmx.creationtoolversion`, `tmx.datatype`, `tmx.segtype`, `tmx.o-tmf`, `tmx.o-encoding`, `tmx.creationdate`, `tmx.creationid`, `tmx.changedate`, `tmx.changeid`, as written |
| `header/note` | paragraph, `Attrs["tmx.field"]="note"`, `tmx.lang` from `xml:lang` |
| `header/prop` | paragraph, `tmx.field=prop`, `tmx.type` from `type`, `tmx.lang` |
| `header/ude` | a table, `tmx.field=ude`, `tmx.ude_name`, `tmx.ude_base`; one row per `map` from its attributes `unicode`, `code`, `ent`, `subst` (`Attrs["attribute"]`) |
| `body` | **one table**, `tmx.field=body` |
| `tu` | a row; `RowLocs` covers the `tu`, `Anchor` its `tuid` as written (none when absent) |
| `tuv` | a cell in the **column of its language** (`xml:lang`, else `lang`, as written; the engine's `column_key`): columns from the second, in order of first appearance in the body; the header row holds the language codes as written, its first cell empty (`header_source=attribute`) |
| two `tuv` of one language in one `tu` | both in that column, in file order, joined by `\n` |
| `tu/prop`, `tu/note` | the row's first column (`rest`), joined by `\n`; a `prop` is followed by its `type` after a space (`with`) |
| `tuv/prop`, `tuv/note`, `tuv/seg` | lines of the `tuv`'s cell, in file order; `prop` followed by its `type` |
| `bpt`, `ept`, `ph`, `it`, `ut` | inline code: the native code they hold is text where it stands (`<b>` in `It is <b>cold</b>`), style `code` in `Spans` |
| `hi`, `sub` | inline: their text in place |

A consumer cites one unit by the row's `RowLocs[i]` (bytes of the `tu`)
and `Anchor` (`tuid`); a language by the column whose header cell is its
code.

Attributes left as counts in `xml.unmapped_attributes`: the inline
codes' `i`, `x`, `type`, `pos`, `assoc` (pairing and kind of a code, not
text); `tu`/`tuv` administrative attributes (`creationdate`,
`creationid`, `changedate`, `changeid`, `usagecount`, `lastusagedate`,
`creationtool`, `o-tmf`, `datatype`, `segtype`, `srclang`,
`o-encoding`) and `note`/`prop` `xml:lang` and `o-encoding` inside a
`tu`. They are dates, counters and identifiers, not text for a reader;
the row has no field to hold them (proposal: a row `attrs`).

## Measured (dev: 40 files; held-out: 17)

- dev: 40 read with `tmx`, 0 errors, 0 `Validate` failures; blocks:
  table 42, paragraph 12, stored `markup_comment` 29;
  `xml.unmapped_elements` none; `xml.unmapped_attributes` 905 (the list
  above).
- held-out: 17 files, 0 errors, 0 `Validate` failures; table 17,
  paragraph 2; `limit.exceeded` 1 (the text budget on the largest file),
  `xml.unmapped_elements` none.
- Largest dev file (2.6 MB, 6,646 rows): 23 ms, 9 MiB allocated.
- Row locations are offsets in the file in every encoding (the 5 dev
  files not in UTF-8 had them in the decoded text before 1311e61).

## Oracles

(a) The body's text in order is the file's character data in order, with
each `prop`'s `type` after it, apart from: the header row of the body
table (attribute values); files encoding/xml stops in (6 on dev, compared
up to there); and the `tuv` of a `tu` whose language order differs from
the column order (2 on dev), where the text is the same characters and
(b) places each row. (b) Every block's text is in the source at its
offsets, and every row's cells are in the bytes of its `tu` (UTF-16
files, 5 on dev, are not compared byte for byte).
