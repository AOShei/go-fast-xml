# Profile `cda`

HL7 CDA Release 2 clinical documents, including C-CDA R1.1/R2.1
documents. Nothing here is medical judgement: the profile maps what the
document declares.

## Selection

Root `ClinicalDocument` in the namespace `urn:hl7-org:v3`. A
`ClinicalDocument` with no namespace is not CDA and is read generically
(one dev stub is).

## Mapping

| Element | Model |
|---|---|
| `realmCode`, `typeId`, `templateId`, `id`, `code` (+`translation`, `originalText`), `effectiveTime`, `confidentialityCode`, `setId`, `versionNumber`, `copyTime` (document level) | `Extra`: `cda.realm`, `cda.type_id_root`/`_extension`, `cda.template_id`/`cda.template_extension` (one line per `templateId`), `cda.id_root`/`_extension`/`_authority`, `cda.code`/`_system`/`_system_name`/`_display`, `cda.code_translation*`, `cda.code_original_text`, `cda.effective_time` (the HL7 TS value as written), `cda.confidentiality`/`_system`/`_display`, `cda.set_id_root`/`_extension`, `cda.version`, `cda.copy_time` |
| `title` | `Metadata.Title` and a level 1 heading |
| `languageCode` | `Metadata.Language` (the `code`) |
| `recordTarget`, `author`, `dataEnterer`, `informant`, `custodian`, `informationRecipient`, `legalAuthenticator`, `authenticator`, `participant`, `inFulfillmentOf`, `documentationOf`, `relatedDocument`, `authorization`, `componentOf` | `Extra["cda.record_target"]`, `cda.author`, ... : the text of the participant (names, addresses, organisation names), white space collapsed, `; ` between repeated participants; `cda.participant_type`, `cda.related_document_type` |
| `structuredBody`, `component` | transparent |
| `section` | section (anchor = `ID`); nested sections nest; headings from level 2 |
| `section/title` | heading by nesting |
| `section/code` | `note` of type `attribute` whose text is the `displayName`, with `cda.code`, `cda.code_system`, `cda.code_system_name`, `cda.field=code` |
| `section/templateId`, `section/id` | `note` of type `attribute` (text = `root`, `cda.extension`, `cda.field`) |
| `section/text` (the narrative block) | transparent (anchor = `ID`); every block in it carries `cda.part=narrative`; stray text in it is an inferred paragraph |
| `paragraph` | paragraph (anchor = `ID`, `cda.style`); `paragraph/caption` its `Marker` |
| `list`, `item` | list (ordered when `listType="ordered"`), items (anchor = `ID`); `list/caption` caption; `item/caption` label |
| `table`, `thead`, `tbody`, `tfoot`, `tr`, `th`, `td` | rows table (anchor = `ID`; `th` and `thead` rows are header; `colspan`, `rowspan`); `table/caption` caption |
| `content` | inline; `Bold`/`Italics` in `styleCode` as styles; a `content` with an `ID` also gives a record in the stored part (`custom_property`, `name=content`) whose text is the content's text and `cda.id` its `ID`: this is what an entry's `reference value="#ID"` points at |
| `linkHtml` | link to `href` (its `title` in the record) |
| `footnote` | footnote block (anchor = `ID`); `footnoteRef` superscript link to `IDREF` |
| `renderMultiMedia` | image whose `Image.ID` is `referencedObject`, alt text its `caption` |
| `sub`, `sup` | inline styles; `br` inline (no line break) |
| `section/entry` | **generic notes, for every entry**: the entry's structured data (acts, observations, codes, values, times) is read generically, so every attribute value is a `note` of type `attribute` at its place (`Origin: inferred`) and text (`originalText`) an inferred paragraph; every block carries `cda.part=entry` and `cda.entry_type` (`typeCode`). Chosen over `stored` because the stored part keeps only an element's text, and an entry's data is almost all in attributes |
| `section/author`, `section/informant`, `section/subject` | `Extra["cda.section_author"]` etc. |
| `nonXMLBody` | `Extra["cda.body"]="nonXMLBody"` |
| `nonXMLBody/text` | stored (`custom_xml`): the inline data as written, base64 not decoded (an embedded plain text body is stored too); `cda.nonxml_media_type`, `cda.nonxml_representation`, `cda.nonxml_compression` |

## Citing a block

Document: `cda.id_root` + `cda.id_extension` (with `cda.set_id_*` and
`cda.version` for the version). Block: `Loc.Anchor` (`section@ID`,
`paragraph@ID`, `item@ID`, `table@ID`, `footnote@ID`) or
`Attrs["anchor_parent"]`. The section's coded type is the `note` with
`cda.field=code` that opens it.

## Left to generic reading

Everything inside `entry` (by choice, above). Unmapped attributes kept
out: `td@ID`/`th@ID` (core has no anchor for a cell), `content@styleCode`
values other than Bold/Italics, `xsi:type`, the attributes inside header
participants (`administrativeGenderCode@code`, `birthTime@value`,
`telecom@value`, `time@value`, ids: the participant's text is kept, its
coded values are not; see "Needs" in the report), `reference@value` of a
non-inline `nonXMLBody`.

## Versions

Dev (32 files): CDA R2 with C-CDA R1.1 and R2.1 templates (HL7 C-CDA
examples, CDA core examples, vendor samples from the C-CDA samples
collections), structured and unstructured (`nonXMLBody`, base64 PDF,
plain text, reference). Not covered: CDA R2.1 or R3, `sdtc:` extension
elements beyond what the samples hold, documents with `section@nullFlavor`
semantics, signature data.
