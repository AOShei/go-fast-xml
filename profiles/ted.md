# Profile `ted`: TED notices before eForms (`TED_EXPORT`)

The XML of the Supplement to the Official Journal as TED published it
until 2023: schema R2.0.9 (the 2014 standard forms `F01_2014` to
`F25_2014`) and R2.0.8 (the earlier forms `CONTRACT`, `CONTRACT_AWARD`,
`PRIOR_INFORMATION`, ..., and notices written as text, `OTH_NOT`).

## Selection

Root `TED_EXPORT` in one of
`http://publications.europa.eu/resource/schema/ted/R2.0.9/publication`,
`http://publications.europa.eu/resource/schema/ted/R2.0.8/publication`,
`http://publications.europa.eu/TED_schema/Export`,
`http://formex.publications.europa.eu/ted/schema/export/R2.0.9.S01.E01`
(the last one is in no dev file: it is matched on the schema's name
only). The rules are written for the first; the others are its aliases.
`TED_EXPORT` in another namespace or none stays generic
(`TestTedSelection`).

## Mapping

Every value that is not running text is a row of a table of names and
values (`pairs`), the name being the file's own element path, attributes
as rows of their own. The schema says many things by the presence of an
empty element (`PT_OPEN`, `NO_LOT_DIVISION`, `SME`): such an element is a
row with an empty value (`pairs_empty`). Nothing is interpreted.

| Element | Output |
|---|---|
| `TED_EXPORT@DOC_ID`, `@EDITION`, `@VERSION` | `Extra["ted.doc_id"]`, `ted.edition`, `ted.version` |
| `TECHNICAL_SECTION`, `LINKS_SECTION`; `REF_OJS`, `NOTICE_DATA`, `CODIF_DATA` of `CODED_DATA_SECTION` | a table each (`CODIF_DATA/PR_PROC` = `Open procedure`, `CODIF_DATA/PR_PROC/@CODE` = `1`) |
| `ML_TITLES` | one table, a row per `ML_TI_DOC`: its `LG`, `TI_CY`, `TI_TOWN`, `TI_TEXT` (header the profile's: language, country, town, title) |
| `ML_AA_NAMES` | one table, a row per `AA_NAME`: its `LG`, the name |
| each form in `FORM_SECTION` (one per language the notice is published in) | a section; its `LG` is a level-1 heading (`Attrs["attribute"]="LG"`), with `ted.form`, `ted.category`, `ted.version` |
| **2014 forms.** `OBJECT_CONTRACT/TITLE` | `Metadata.Title` (the first form's) and a level-2 heading |
| `OBJECT_CONTRACT/SHORT_DESCR` | paragraphs, `Attrs["ted.field"]="SHORT_DESCR"` |
| `OBJECT_DESCR` (a lot) | a section, `ted.item`; its `TITLE` a level-3 heading, `SHORT_DESCR` and `INFO_ADD` paragraphs, its other children a table each, names from `OBJECT_DESCR/` |
| `AWARD_CONTRACT` | a section, `ted.item`; its `TITLE` a level-3 heading, its other children a table each |
| `COMPLEMENTARY_INFO/INFO_ADD` | paragraphs; its other children a table each |
| `LEGAL_BASIS`, `CONTRACTING_BODY`, `PROCEDURE`, `LEFTI`, and any other part of a form | one table each; the text they hold (`LEFTI/SUITABILITY/P`) is a row |
| **Earlier forms** (`CONTRACT/FD_CONTRACT`, ...) | one table per form, names from `FD_CONTRACT/`; their titles and descriptions are rows (`.../TITLE_CONTRACT/P`), and `Metadata.Title` is not filled |
| **Text notices** (`OTH_NOT/FD_OTH_NOT`) | `TI_DOC` a level-2 heading, `STI_DOC` paragraphs (`ted.field`), `CONTENTS/P` paragraphs, `GR_SEQ` a section with `TI_GRSEQ` its heading, `MARK_LIST` a list (`NO_MARK` the item's marker), `ADDRESS_NOT_STRUCT` read as the sentence it is written as (its `COUNTRY@VALUE` in place); other children tables |
| `P` | a paragraph; `FT` inline (`TYPE="SUP"`/`"SUB"` superscript and subscript; another `TYPE` kept as `ted.type`) |

## Measured (dev: 544 files; held-out: 173)

A twelfth of one day of 2022 (364 dev files in the R2.0.9 namespace, 2
in R2.0.8) and an eighth of one day of 2016 (178 dev files in
`http://publications.europa.eu/TED_schema/Export`, whose forms are
marked R2.0.8).

- dev: 544 read with `ted`, 0 errors, 0 `Validate` failures; blocks:
  heading 4,028, paragraph 10,231, list item 984, table 20,415;
  `xml.unmapped_elements` none, `xml.unmapped_attributes` none.
- held-out: 173 read with `ted`, 0 errors, 0 `Validate` failures;
  heading 996, paragraph 2,355, table 5,637; nothing unmapped.

## Oracles (`TestTedOracles`, 544 dev files)

(a) and (b) as for the other profiles pass on every file; the one
attribute written as text is `ADDRESS_NOT_STRUCT/COUNTRY@VALUE`.

## Limits

- The earlier forms (2016 and before) are tables only, with long names
  (`FD_CONTRACT_AWARD/OBJECT_CONTRACT_INFORMATION_CONTRACT_AWARD_NOTICE/...`).
  Their titles and descriptions could be headings and paragraphs as in
  the 2014 forms; each form family names them differently.
- In the 2014 forms the text outside `OBJECT_CONTRACT`, `AWARD_CONTRACT`
  and `COMPLEMENTARY_INFO` (selection criteria in `LEFTI`, review
  procedures) is rows, not paragraphs.
