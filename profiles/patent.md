# Profile `patent`

Patent documents: USPTO grant and application XML (`us-patent-grant`,
`us-patent-application`, v4.x DTDs; the older `patent-application-publication`,
pap-v16) and EPO `ep-patent-document` (WIPO ST.36 family; also selects
`wo-patent-document` and `patent-document` by root, untested).

## Selection

By root element (no namespace): `us-patent-grant`, `us-patent-application`,
`patent-application-publication`, `ep-patent-document`, `wo-patent-document`,
`patent-document`. `Metadata.Extra["pat.doc_type"]` records which.

## Mapping

| Element | Model |
|---|---|
| root | transparent; its attributes to `Extra` (`pat.id`, `pat.lang`, `pat.dtd_version`, `pat.file`, `pat.status`, `pat.country`, `pat.date_produced`, `pat.date_publ`; EPO also `pat.doc_number`, `pat.kind`) |
| `invention-title`, `title-of-invention` | `Metadata.Title` and a level 1 heading where it stands (`pat.title_id`) |
| USPTO bibliographic data (`publication-reference`, `application-reference`, `priority-claim`, classifications, `us-term-of-grant`, `number-of-claims`, `figures`, parties, examiners, related documents, PCT data) | `Extra` under `pat.*` (table below); text as written, dates as written (`YYYYMMDD`) |
| EPO `SDOBI` (`B100`...`B800`) | `Extra` under `pat.*`, one key per INID code (table below); `eptags` (`B001EP`, `B005EP`, `B007EP`: the EPO's own processing tags) stored, kind `custom_xml` |
| `references-cited/citation`, `us-references-cited/us-citation`, EPO `B560/B561`, `B562` | a list item each (`pat.section=references-cited`; EPO `pat.cit_type` `patent` / `non-patent`); `patcit`, `nplcit`, `document-id` and its fields, `category` stay in the item's text |
| `abstract`, `description` | sections (`pat.section`, `pat.lang`); the section's `id` is the `anchor_parent` of blocks with no anchor |
| `description-of-drawings` | nested section (`pat.part`) |
| `heading` | heading by nesting: level 2 in the description, 3 in `description-of-drawings`; source level in `pat.level` (`level`) or `pat.lvl` |
| `p` | paragraph; **anchor = `num`** (the paragraph number cited as "[0023]"); `id` in `pat.id` |
| `ul`, `ol`, `li` | list, items (`li` anchor = `num`, `id` in `pat.id`). Both are unordered in the model: ST.36 and USPTO write the item labels (`(i)`, `a)`) in the text, so no number is added. `pat.list_style`, `pat.list=ol` |
| `dl/dt`, `dl/dd` | list items with `pat.dl` `term` / `definition` |
| `b`, `i`, `sup`, `sub`, `sup2`, `sub2` | inline styles; `u`, `o`, `smallcaps` inline with no style (core has no underline); `br` a line break (`break`) |
| `figref`, `crossref`, `claim-ref`, pap `cross-reference` | links to `idref` (`target`) |
| `claims`, pap `subdoc-claims` | list (`pat.section=claims`, `pat.lang`) |
| `claim` | one list item per claim; **anchor = `num`**; `id` in `pat.id` (the `claim-ref` link target). Nested `claim-text` stays in the claim's text (one item per claim) |
| `us-claim-statement`, `claim-statement` | paragraph (`pat.part=claim-statement`) |
| `drawings/figure/img` | image (`Image.ID` = `file`, alt text = `alt`, anchor = `img@id`, `pat.num`, `pat.width`, `pat.height`, `pat.img_content`, `pat.img_format`, `pat.orientation`) |
| `chemistry/img`, `maths/img` | image with `pat.kind`, `pat.parent_id` (the `chemistry`/`maths` id) and `pat.num` |
| `us-chemistry` | image whose `Image.ID` is the `cdx-file`; `pat.mol_file`, `pat.idref` (no file is loaded) |
| `us-math` | its `img` an image with `pat.nb_file`, `pat.idref` |
| `maths/math` (MathML, no namespace in the samples) | formula block, `notation=plain`, text content of the MathML, `pat.notation=mathml`; inside a paragraph it is a line of that paragraph (engine rule for blocks in blocks) |
| `tables/table` (CALS) | table, anchor = `tables@id`, `pat.num`; `table/title` caption. A table inside a `p` ends the paragraph and the text after it is a new paragraph with the same anchor |
| `ep-reference-list` | section (`pat.section=reference-list`) |
| `search-report-data`, `doc-page` | transparent (`pat.section=search-report`), the page images as images |
| pap-v16 `subdoc-abstract`, `subdoc-description`, `section`, `paragraph` (anchor = `id`, `number` as `Marker`), `subdoc-drawings`, `image` | as their v4 counterparts |

### Extra keys

USPTO: `pat.pub_country`, `pat.pub_number`, `pat.pub_kind`, `pat.pub_date`;
`pat.app_type`, `pat.app_country`, `pat.app_number`, `pat.app_kind`,
`pat.app_date`, `pat.app_series_code`; `pat.priority` (each claim's text,
`; `-joined), `pat.priority_kind`, `pat.priority_sequence`;
`pat.term_of_grant`; `pat.class_locarno`, `pat.class_national`,
`pat.class_ipcr`, `pat.class_ipc`, `pat.class_cpc`, `pat.class_cpc_main`,
`pat.class_cpc_further`, `pat.search_field`; `pat.number_of_claims`,
`pat.exemplary_claim`, `pat.drawing_sheets`, `pat.number_of_figures`;
`pat.related_documents`; `pat.applicant` (+ `_type`, `_designation`),
`pat.inventor`, `pat.agent` (+ `_type`), `pat.assignee`,
`pat.examiner_primary`, `pat.examiner_assistant`; `pat.pct_filing`,
`pat.pct_publishing`; `pat.sir_flag`. A party's value is the text of its
element, white space collapsed ("Doe Jane Austin US"). pap-v16 adds
`pat.filing_type`, `pat.app_filing`, `pat.correspondence_address`,
`pat.representative_figure`.

EPO (INID code in brackets): `pat.pub_number` (B110), `pat.pub_kind_text`
(B121), `pat.pub_kind` (B130), `pat.pub_date` (B140), `pat.pub_country`
(B190), `pat.app_number` (B210), `pat.app_date` (B220),
`pat.exam_request_date` (B241), `pat.exam_report_date` (B242),
`pat.filing_lang` (B250), `pat.procedure_lang` (B251EP), `pat.pub_lang`
(B260), `pat.priority_number`/`_date`/`_country` (B310/B320/B330),
`pat.correction_date`/`_bulletin` (B405), `pat.app_pub_date`/`_bulletin`
(B430), `pat.grant_date`/`_bulletin` (B450), `pat.grant_intention_date`
(B451EP), `pat.ipc_edition` (B516), `pat.class_ipc_main` (B511),
`pat.class_ipc_further` (B512), `pat.class_ipcr` (B510EP),
`pat.class_cpc` (B520EP), `pat.title` (B542, one line per language) with
`pat.title_lang` (B541), `pat.search_report_date` (B565EP),
`pat.figure_to_publish` (B598), `pat.related_number`/`_date` (B600 `cdoc`),
`pat.applicant`, `pat.inventor`, `pat.proprietor`, `pat.representative`
(B711/B721/B731/B741 `snm`) each with `_street`, `_city`, `_country`
(and `_iid`, `_ref`, `_suffix` where present), `pat.designated_states`
(B840), `pat.extension_states` (B845EP), `pat.validation_states`
(B849EP), `pat.pct_app_number`/`_date`/`pat.pct_filing_lang`
(B861/B862), `pat.pct_pub_number`/`_date`/`_bulletin` (B871),
`pat.search_report_pub_date`/`_bulletin` (B880), `pat.bib_lang`.
Repeated values are joined with `; ` (states with a space); a party
missing one field shifts the later values of that key, so the keys of
one role are aligned only when every party has every field.

EPO documents carry their title only in `pat.title` (all languages):
there is no `Metadata.Title` and no title heading, since the file does not
mark which of the B542 titles is the document's own.

### Attrs keys

`pat.section` (`abstract`, `description`, `claims`, `drawings`,
`references-cited`, `reference-list`, `search-report`), `pat.part`,
`pat.lang`, `pat.id`, `pat.num`, `pat.level`, `pat.lvl`, `pat.list`,
`pat.list_style`, `pat.dl`, `pat.cit_type`, `pat.kind`, `pat.parent_id`,
`pat.idref`, `pat.mol_file`, `pat.nb_file`, `pat.img_content`,
`pat.img_format`, `pat.width`, `pat.height`, `pat.orientation`,
`pat.notation`, `pat.display`, `pat.overflow`, `pat.frame`, `pat.colsep`,
`pat.rowsep`, `pat.pgwide`, `pat.tabstyle`.

## Citing a block

Document: `pat.pub_country` + `pat.pub_number` + `pat.pub_kind`
(`US 09999999 B2`, `EP 1000000 B1`). Paragraph: `Loc.Anchor` is its
number (`0023`, cited "[0023]"); claim: `Loc.Anchor` is the claim number
(`00001` USPTO, `0001` EPO; the `claim-ref` links point at `pat.id`).
EPO documents repeat the claims in three languages: tell them apart with
`pat.lang`. Tables and formulas: the `tables`/`maths` id. `Loc.Offset` and
`Length` give the bytes in every case.

## Left to generic reading

- EPO search report internals (`srep-*`, `patent-family`,
  `family-member`, `priority-application`, the `citation` inside
  `srep-citations`): the samples hold one; read generically, text kept.
- pap-v16 bibliographic leftovers (`first-named-inventor`,
  `classification-us-primary`, `uspc`, `parent-child`, ...): inside
  metadata values, counted as unmapped; their text is in the value.
- `sequence-list`, `us-sequence-list-doc` (an external file reference).
- `patcit@num`/`@dnum`/`@id` and `nplcit@num` of a citation are its
  item's `pat.patcit_num`, `pat.patcit_dnum`, `pat.patcit_id`,
  `pat.nplcit_num` (an attribute of a child, `{"attr": ..., "child": ...}`).
- Unmapped attributes kept out: the `sequence` of parties, MathML presentation attributes,
  `ul@compact`.

## Versions

Tested (dev, 37 files): `us-patent-grant` v4.0 (2004-12-02), v4.1
(2005-08-25), v4.2 (2006-08-23), v4.3 (2012-12-04); `us-patent-application`
v4.3 (2012-12-04); `patent-application-publication` pap-v16 (2002-01-01);
`ep-patent-document` v1.1, v1.3, v1.4, v1.5, v1.5.1. Not covered by the
samples: USPTO v4.4 and later (2013+), the 2001-2004 grant format
(`PATDOC`, v2.5, not selected), `wo-patent-document` and any WIPO ST.36
document other than the EPO's, ST.96. One held-out file has a root this
profile does not select.
