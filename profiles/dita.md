# Profile `dita`: DITA 1.3 topics and maps

## What selects it

The roots `topic`, `concept`, `task`, `reference`, `glossentry`,
`glossgroup`, `troubleshooting`, `dita`, `map`, `bookmap`,
`subjectScheme`, `learningContent` with a DOCTYPE public identifier
starting `-//OASIS//DTD DITA` or a `ditaarch:DITAArchVersion` attribute;
and, with neither (DITA-OT test files have no DOCTYPE), a `topic`,
`concept`, `task` or `reference` with the `id` DITA requires, a `map`
with a `title` attribute, a `dita` with `xml:lang`.

Rules go by element name, not by `@class`: the `class` attribute is
usually supplied by the DTD's defaults, which are not read, so most
files do not carry it. Specialisations other than the base topic types
are read generically (and counted) unless their names are below.

## Mapping

| Element | Model |
|---|---|
| `topic`, `concept`, `task`, `reference`, `glossentry`, `troubleshooting` | section, anchor `id`; every block carries `dita.topic` = the topic's `id`; the first block `dita.type` |
| root `title` | `Metadata.Title` and a level-1 heading; other `title`s headings by nesting (table and figure titles: captions) |
| `shortdesc`, `navtitle`, `searchtitle`, `glossdef`, `desc` | paragraphs, `dita.field` |
| `prolog/author`, `topicmeta/author` | `Metadata.Authors`; `keywords/keyword` `Metadata.Keywords`; `source`, `publisher`, `copyright`, `critdates`, `prodinfo`, `category`, `audience`, `othermeta`, `resourceid` to Extra `dita.*` |
| `indexterm` | **stored** (`custom_xml`), not body text |
| `body`, `conbody`, `taskbody`, `refbody`, `glossBody`, `bodydiv`, `sectiondiv` | transparent |
| `section`, `example`, `refsyn`, `prereq`, `context`, `result`, `postreq`, `steps-informal` | sections (task parts marked `dita.in`) |
| `p`, `lq`, `codeblock` (language from `outputclass`), `pre`, `screen`, `lines`, `msgblock` | paragraph, quote, code |
| `note` | its blocks marked `dita.note` = `@type` (`note`, `tip`, `important`, `warning`...) |
| `ul`, `ol`, `sl`, `dl` (`dt` the item, `dd` after), `parml` | lists |
| `steps`, `substeps` | ordered lists; `cmd` the step's text; `info`, `stepxmp`, `stepresult` after it marked `dita.step` |
| `choices`, `choicetable` | list, table |
| `simpletable`, `properties` | table (`sthead`/`prophead` header rows) |
| `table` | CALS table |
| `fig`, `image` | caption from `title`/`desc`; image (id = `href`, alt from `alt`) |
| `xref` | link span to `href` as written, and a record in the stored part with `dita.keyref`, `dita.scope`, `dita.format` |
| `@conref`, `@conkeyref`, `@keyref` on `p`, `ph`, `keyword` | kept as written in `Attrs` (`dita.conref`...); **conref content is not resolved** |
| `fn` | footnote, `Marker` from `callout` |
| `draft-comment`, `required-cleanup` | `note` of type `comment` |
| inline programming and UI elements (`codeph`, `filepath`, `cmdname`, `uicontrol`...) | code or bold spans; `b`, `i`, `sup`, `sub`, `tt`, `term`, `cite` spans |
| map: `topicref`, `mapref`, `keydef`, `chapter`, `ditavalref`, `topichead`, `topicgroup` and the like | **list items nested as in the map**; `href` (`navtitle` for `topichead`) is the item's text when it has none of its own (`value`, with `Attrs["attribute"]`); `dita.href`, `dita.keys`, `dita.keyref`, `dita.navtitle`, `dita.scope`, `dita.format`, `dita.chunk`, `dita.processing_role` in `Attrs`. Nothing is fetched |
| `reltable` | table (`relheader` header row, `relcell` cells) |
| `related-links/link` | list items with `dita.href` |

## How to cite a block

`Loc.Offset`/`Length`; `Loc.Anchor` = the element's `id`; a block with
no `id` carries its topic's (section's...) in `Attrs["anchor_parent"]`
and its topic's in `dita.topic`, so `file.dita#topicid/elementid` (DITA's
own addressing) can be built from `Source.Name`, `dita.topic` and the
anchor.

## Decisions

- The `href` of a map reference is the item's `Marker`, so that a
  `topicref` with no title of its own is not lost (an item with no text
  makes no block). The format has no `value` for items; with one, the
  `href` would be the item's text with `Attrs["attribute"]`.

## Left to generic reading

None of the elements met in the dev set. Not read (counted):
`xref@type`, `simpletable@relcolwidth`, `@keycol`, `@outputclass`,
`@class`, `@domains`, `@rev`, filtering attributes (`@platform`,
`@product`, `@audience`, `@otherprops` on content: `mark` would record
them; left for a later pass).

## Versions

Samples: DITA 1.3 (OASIS specification sources), DITA-OT documentation
and test files (DITA 1.2/1.3, some with no DOCTYPE). Not seen: DITA 2.0,
Lightweight DITA (XDITA), learning and training topics' bodies,
`bookmap` contents, `subjectScheme` contents.
