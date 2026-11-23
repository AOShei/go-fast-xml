# Profile `clml`: legislation.gov.uk CLML

**Selected by** the root `Legislation` in the namespace
`http://www.legislation.gov.uk/namespaces/legislation`.

## Mapping

| Element | Model |
|---|---|
| `Legislation` | transparent; `Extra` `clml.document_uri`, `clml.id_uri`, `clml.schema_version`, `clml.restrict_extent`, `clml.restrict_start_date`, `clml.number_of_provisions` |
| `ukm:Metadata`: `dc:title`, `dc:description`, other `dc:*`, `dct:valid`, `dct:alternative` | `Metadata.Title`, `description`, `Extra` `dc:*`, `dct:*` |
| `atom:link` | `Extra` `clml.link` (`@href`), `clml.link_rel`, `clml.link_title`, `clml.link_type`, `clml.link_hreflang` (one line per link; a link with no title adds no title line) |
| `ukm:*` classification, numbers and dates (`DocumentCategory`, `Year`, `Number`, `ISBN`, `Made`, `Laid`, `EnactmentDate`, `ComingIntoForce`, statistics, ...) | `Extra` `ukm:<Name>` from `@Value`/`@Date`/`@URI`, with `_time`, `_class`, `_title`, ... |
| `ukm:Subject` | `Metadata.Keywords` (`@Label`), `Extra` `ukm:Subject_uri` |
| `ukm:EURLexMetadata` | stored part, `custom_xml` |
| `PrimaryPrelims`, `SecondaryPrelims`, `EUPrelims` | part `chapter`; `Title` heading 1; `Number`, `LongTitle`, `DateText`, `Subtitle` paragraphs |
| `Body`, `EUBody` | part `body` |
| `Schedules`, `Appendix`, `ExplanatoryNotes`, `EarlierOrders`, `Commentaries`, `Footnotes` | part `chapter` named after the element |
| `Part`, `Chapter`, `Pblock`, `PsubBlock`, `P1group`, `Schedule`, `EUPart` ... `EUSubsection`, `SignedSection`, ... | section, anchor `@id`; `Attrs` `clml.element`, `clml.document_uri`, `clml.id_uri`, `clml.restrict_extent`, `clml.restrict_start_date`, `clml.status`, ... on its first block |
| `Number` of a `Part`, `Chapter`, `Schedule`, ... | label (the heading's marker: `Part 1`) |
| `Title` | heading by nesting (`heading_base` 2) |
| `P`, `P1` ... `P7` | transparent, anchor `@id` (`section-1-2-a`), the same `Attrs`; `P1para` ... `P7para` transparent |
| `Pnumber` | label |
| `Text` | paragraph |
| `Para`, `BlockText` | transparent |
| `BlockAmendment`, `BlockExtract` | transparent; every block inside is marked `clml.amendment_context`, `clml.amendment_format`, `clml.amendment_target_class` ... |
| `UnorderedList`, `OrderedList`, `ListItem` | list (`ordered` false; `clml.decoration`, `clml.type`), items with `Marker` from `@NumberOverride` |
| `Tabular` (anchor `@id`), XHTML `table` | table, `rows` model; `thead` rows and `th` header |
| `Commentary` | `footnote`, anchor `@id`, `clml.commentary_type` (`F`, `I`, `C`, ...) |
| `Footnote` | `footnote`, anchor `@id` |
| `Emphasis`, `Strong`, `Superior`, `Inferior` | italic, bold, superscript, subscript |
| `Citation`, `CitationSubRef`, `ExternalLink`, `SectionRange`, `Section` | links (`@URI`); `InternalLink` links to `@Ref` |
| `CommentaryRef`, `FootnoteRef`, `Addition`, `Substitution`, `Repeal`, `Term`, `Definition`, `Abbreviation`, `Acronym`, ... | inline, no style |

## Citing a block

Every provision carries its `id` (`section-172-1-a`) as `anchor_parent`,
and its first block the provision's `DocumentURI` and `IdURI`
(`Attrs["clml.document_uri"]`): the legislation.gov.uk address of that
exact provision. `Marker` is the printed number (`Pnumber`).

A section's number (`P1/Pnumber`) followed by a numbered subsection
(`P2/Pnumber`) is a paragraph of its own (`172`), since the subsection's
number is the marker of its text.

## Left to generic reading

- `ukm:UnappliedEffects` / `ukm:UnappliedEffect` and children
  (`AffectedTitle`, `InForceDates`, `InForce`, `CommencementAuthority`,
  `Savings`, `ukm:Section`, `ukm:SectionRange`): the effects not yet applied, attributes
  only (read as attribute notes) and titles (inferred paragraphs). A
  table would need cells from attributes and child text together.
- The CELLAR elements inside `ukm:EURLexMetadata` are counted as unmapped
  while being read into the stored part.

`CommentaryRef@Ref`, `Citation@Class`/`@Year`/`@Number`, `Addition@ChangeId`
and the other attributes of inline elements are not in the output.

## Versions

Dev samples: CLML schema 2.0 (`SchemaVersion`), primary, secondary and
EU retained legislation, whole acts and single provisions. Not tested:
`Image`/`Figure`, `Formula`, `Form` (no sample has them).
