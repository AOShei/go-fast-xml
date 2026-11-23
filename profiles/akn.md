# Profile `akn`: Akoma Ntoso

**Selected by** the root `akomaNtoso` in the namespace of Akoma Ntoso 3.0
(`http://docs.oasis-open.org/legaldocml/ns/akn/3.0`), its draft
`.../akn/3.0/CSD03`, or the older `http://www.akomantoso.org/2.0`. The two
older namespaces are read with the 3.0 rules (`namespace_aliases`).

## Mapping

| Element | Model |
|---|---|
| `akomaNtoso`, the document type (`act`, `bill`, `doc`, `debate`, `judgment`, `amendment`, `documentCollection`, ...) | transparent; type, `@name`, `@contains` to `Extra` (nested documents: `Attrs`) |
| `meta/identification`: `FRBRWork`, `FRBRExpression`, `FRBRManifestation` children | `Extra` `akn.work.*`, `akn.expression.*`, `akn.manifestation.*` (`this`, `uri`, `date` + `date_name`, `author` + `author_as`, `country`, `subtype`, `number`, `name`, `language`, `format`, `component` + `component_name`, ...); `FRBRExpression/FRBRlanguage` is also `Metadata.Language` |
| `FRBRManifestation/preservation` | stored part, `custom_xml` |
| `meta/publication` | `Extra` `akn.publication_*` |
| `meta/references` (`TLC*`, `original`, `hasAttachment`, ...), `lifecycle/eventRef`, `workflow/step`, `classification/keyword` | a table each, cells from the attributes (`eId`, `id`, `href`, `showAs`, ...), fixed header (`header_source=profile`), `Attrs["akn.meta"]` names the list |
| `meta/notes/note` | `footnote` block |
| `meta/proprietary`: `dc:title`, `dc:description`, other `dc:*`; `ukm:*` (legislation.gov.uk) | `Metadata.Title`, `description`, `Extra` `dc:*`; `Extra` `ukm:*` from `@Value`/`@Date`/`@URI` |
| `coverPage`, `preface`, `preamble`, `conclusions` of the top document | part `chapter` named after the element |
| `body`, `mainBody`, `judgmentBody`, `debateBody`, `amendmentBody`, `collectionBody`, `portionBody` of the top document | part `body` |
| `attachments/attachment`, `attachments/doc` (2.0), `components/component`, `collectionBody/component` | part `chapter` (`attachment`, `component`); the same elements inside are read as sections or transparent |
| hierarchy: `part`, `chapter`, `section`, `article`, `paragraph`, `subparagraph`, `point`, `list`, `level`, `hcontainer`, `tblock`, `blockContainer`, `container`, `recital(s)`, `citation(s)`, `longTitle`, `formula`, the debate, judgment and amendment containers (full list in the JSON) | `section`, anchor `@eId` (else `@id`), `Attrs` `akn.element`, `akn.class`, `akn.name`, `akn.wid`, `akn.guid`, `akn.refers_to`, `akn.status`, `akn.period` on its first block |
| `num`, `from` | `label`: the `Marker` of the heading or first block |
| `heading`, `crossHeading` | heading, level by nesting (`heading_base` 2) |
| `subheading`, `block`, `listIntroduction`, `listWrapUp`, `narrative`, `summary`, `scene`, `other`, `p` | paragraph (`akn.element` names the element where it is not `p`) |
| `content`, `intro`, `wrapUp`, `quotedStructure`, `embeddedStructure`, `subFlow`, `foreign` | transparent |
| `blockList`/`item`, `ul`, `ol`/`li`, `toc`/`tocItem` | list and items, anchor `@eId` |
| `authorialNote` | `footnote` block, `out_of_line`, `Marker` from `@marker`, anchor `@eId` |
| `table`/`tr`/`th`/`td`, the same in XHTML in `foreign` | table, `rows` model, `colspan`/`rowspan` |
| `img` | image (`@src`, `@alt`) |
| MathML `math` | `formula`, `notation` `plain` (its text), `Attrs["akn.formula_markup"]="MathML"` |
| `b`, `i`, `sup`, `sub` | inline styles |
| `ref`, `a`, `noteRef`, `documentRef`, `componentRef`, `rref` | links (`@href`, `@src`, `@from`) |
| the other inline elements (`term`, `def`, `docTitle`, `docNumber`, `person`, `date`, `mod`, `quotedText`, `ins`, `del`, ...) | inline, no style |

## Citing a block

`Loc.Offset`/`Length` cover the block's element; a paragraph inside a
numbered unit carries the unit's `eId` in `Attrs["anchor_parent"]` (for
example `art_1__para_1`), items and notes their own `eId` in `Loc.Anchor`.
With the work's `Extra["akn.work.this"]` (or the expression's) that is an
Akoma Ntoso reference: `/akn/xx/act/2026/1/eng@/!main~art_1__para_1`. The
`Marker` holds the number as printed (`Art. 1`, `(2)`, `(a)`).

A footnote (`authorialNote`) is a block before the paragraph it stands in;
the paragraph's text is its element's text without the note.

## Left to generic reading

- `br`, `eol`, `eop`: no rule kind writes a line break; the text either
  side is joined with no space (`EDI<br/>über` gives `EDIüber`).
- `analysis` and its children (`activeModifications`, `textualMod`,
  `source`, `destination`, `otherAnalysis`, ...), `temporalData`,
  `presentation`, vendor elements in `proprietary` (MODS, `confersPower`,
  `match`, Uruguay's `asunto`, `carpeta`, ...): attributes only, read as
  attribute notes; text, where there is any, as inferred paragraphs.
- The page furniture of Bungeni 2.0 samples (`firstPage`,
  `evenPageHeading`, ...), debate leftovers (`quorum`, `voting`, `recount`).

Attributes of inline elements (`term@refersTo`, `person@refersTo`, `@eId`
of inline elements, quoted structures' `@startQuote`) are not in the
output: an inline rule with `attrs` would make a stored record for every
term.

## Versions

Dev samples: 3.0 (20 files: legislation.gov.uk, Fedlex, OASIS examples),
3.0/CSD03 (7, Italian Senate), 2.0 (4, Bungeni examples). Not tested: the
AKN 1.0 namespace, and the 3.0 `amendmentList`, `officialGazette`,
`portion` and `statement` documents, whose rules are written from the
schema only.
