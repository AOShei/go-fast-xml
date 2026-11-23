# Profile `formex`: EUR-Lex Formex 4

**Selected by** the root element, in no namespace: `ACT`, `ANNEX`, `DOC`
(the `.doc.xml` bibliographic record of a publication), `PUBLICATION`
(an Official Journal issue's table of contents), and the consolidated
roots `CONS.ACT`, `CONS.ANNEX`, `CONS.OTH`, `GENERAL`.

## Mapping

| Element | Model |
|---|---|
| root | transparent; `Extra["fx.root"]`, `fx.nnc` |
| `BIB.INSTANCE`: `DOCUMENT.REF` (`@FILE`, `COLL`, `NO.OJ`, `YEAR`, `LG.OJ`, `PAGE.FIRST`, `PAGE.SEQ`, `VOLUME.REF`), `DATE`, `LG.DOC`, `NO.SEQ`, `PAGE.*`, `NO.DOC`, `NO.CELEX` | `Extra` `fx.document_ref`, `fx.oj.*`, `fx.date` (+ `fx.date_iso`), `fx.no.seq`, `fx.page.*`; `LG.DOC` is `Metadata.Language` |
| `BIB.DOC` (`NO.DOC`, `PROD.ID`, `FIN.ID`, `DURAB`, `EEA`), `AUTHOR` | `Extra` `fx.no.doc.*`, `fx.prod_id`, `fx.fin_id`, `fx.durab`, `fx.eea`; `Metadata.Authors` (the institution codes as written: `PE`, `CS`) |
| `PUBLICATION.REF`, `FMX/DOC.MAIN.PUB`, `DOC.SUB.PUB` (and their `REF.PHYS@FILE`), `PAPER`/`PDF` `ITEM.REF` | `Extra` `fx.publication_ref`, `fx.oj.*`, `fx.main.*`, `fx.sub.*` (one line per sub-publication), `fx.page`, `fx.pdf`, `fx.doi` |
| the root's `TITLE/TI`, `ITEM.VOLUME/TITLE/TI` | heading 1 |
| `GR.SEQ`, `DIVISION`, `SECTION` | section; `TITLE/TI` heading by nesting, `STI` paragraph (`fx.element`) |
| `PREAMBLE`, `ENACTING.TERMS`, `FINAL` of the root | parts `chapter` `PREAMBLE`, `body` `ENACTING.TERMS`, `chapter` `FINAL` |
| `PREAMBLE.INIT`, `GR.VISA/VISA`, `GR.CONSID.INIT`, `PREAMBLE.FINAL`, `PL.DATE` | paragraph (`fx.element`) |
| `GR.CONSID`/`CONSID` | list and items; `NO.P` is the recital's marker (`(1)`) |
| `ARTICLE` | section, anchor `@IDENTIFIER` (`001`) |
| `TI.ART` | heading by nesting (`Article 1`) |
| `STI.ART` | paragraph, `fx.element=STI.ART` |
| `PARAG` | transparent, anchor `@IDENTIFIER` (`001.002`) |
| `NO.PARAG`, `NO.P` | label: the marker (`1.`, `(a)`) |
| `ALINEA`, `P`, `TXT` | paragraph |
| `LIST`/`ITEM`, `NP` | list (`ordered` false: the marker is the file's `NO.P`; `fx.list_type` holds `@TYPE`) and items; `NP` transparent |
| `DLIST`/`DLIST.ITEM`, `TERM`, `DEFINITION` | list and items, `TERM` bold |
| `QUOT.S` | `quote` block |
| `TBL` (`CORPUS`, `ROW`, `CELL`) | table, `rows` model, anchor `@NO.SEQ`; `ROW@TYPE=HEADER` and `CELL@TYPE=HEADER` are header; `COLSPAN`, `ROWSPAN`; `TBL/TITLE` caption |
| `NOTE` | `footnote`, `out_of_line`, anchor `@NOTE.ID`; `fx.note_type`, `fx.numbering`, `fx.note_ref` |
| `ANNOTATION` | paragraph (`fx.element`) |
| `INCL.ELEMENT` | image, id `@FILEREF`, `fx.type` |
| `HT@TYPE` `BOLD`, `ITALIC`, `SUP`, `SUB`; `FT@TYPE=CODE` | inline styles; other `HT`, `FT`, `DATE`, `REF.DOC.OJ`, `IE`, `EXPR`, `IND`, `QUOT.START`, `QUOT.END` inline |

## Citing a block

A paragraph carries the `IDENTIFIER` of its `PARAG` (`001.002`) in
`Attrs["anchor_parent"]`, the heading of an article that of its
`ARTICLE`; with `Extra["fx.no.celex"]` (in the `.doc.xml` record) or the
Official Journal reference (`fx.oj.coll`, `fx.oj.no.oj`, `fx.oj.year`,
page) that cites "Article 1(2)". Notes carry `NOTE.ID` in `Loc.Anchor`.

## Left to generic reading

- The issue-level elements of `PUBLICATION` (`BIB.OJ`, `BIB.VOLUME`,
  `OJ`, `NO.ISSN`, `PRINTER`, `INCLUSIONS`, ...): one sample; read as
  inferred paragraphs.

`QUOT.START`/`QUOT.END` stand for quotation marks by character code
(`@CODE="2018"`); the profile cannot write the mark (a profile adds no
text), so a quoted passage has no quote characters around it. `CELL@COL`
is not needed (cells are in order).

## Versions

Dev samples declare Formex 05.20 to 05.59 (`xsi:noNamespaceSchemaLocation`);
the schemas in `.testdata` are 05.59 and 06.02. Not tested: consolidated
texts (`CONS.ACT`, `CONS.ANNEX`), `GENERAL` documents, and Formex 6.
