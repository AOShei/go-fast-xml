# Profile `dila-juri`: DILA case law (JADE, CASS, INCA, CAPP, CONSTIT)

**Selected by** the root element, in no namespace: `TEXTE_JURI_JUDI`
(Cour de cassation, published `CASS` and unpublished `INCA`, and courts
of appeal `CAPP`), `TEXTE_JURI_ADMIN` (Conseil d'État and administrative
courts, `JADE`), `TEXTE_JURI_CONSTIT` (Conseil constitutionnel,
`CONSTIT`). CNIL deliberations (`TEXTE_CNIL`) and KALI are read by
`legifrance` (see its note): `TEXTE_CNIL` has no `META_JURI`, no
`SOMMAIRE`, and its metadata are LEGI/JORF's (`TITREFULL`, `DATE_TEXTE`,
`NOR`, `NUM_SEQUENCE`, `DATE_PUBLI`, `ORIGINE_PUBLI`, `PAGE_DEB_PUBLI`).

The elements this profile shares with `legifrance` (`META_COMMUN`,
`BLOC_TEXTUEL`, `CONTENU`, `LIENS`/`LIEN`, the XHTML-like content) are
read the same way; the keys carry this profile's prefix, `juri.`.

## Mapping

| Element | Model |
|---|---|
| root | transparent; `Extra["juri.root"]`; its `Loc` anchor is `META/META_COMMUN/ID`, so every block carries `Attrs["anchor_parent"]` = the decision's id |
| `META`, `META_COMMUN`, `META_SPEC`, `META_JURI`, `META_JURI_JUDI`, `META_JURI_ADMIN`, `META_JURI_CONSTIT`, `NUMEROS_AFFAIRES` | transparent |
| `META_JURI/TITRE` | `Metadata.Title` and heading 1 |
| `DATE_DEC` | `Metadata.Created` (a date the field does not take stays as written in `Extra["created"]`) |
| `ID`, `ANCIEN_ID`, `ORIGINE`, `URL`, `NATURE`, `JURIDICTION`, `NUMERO`, `SOLUTION`, `ECLI`, `FORMATION`, `NUMERO_AFFAIRE` (one per line), `PUBLI_BULL` (+ `@publie`), `FORM_DEC_ATT`, `DATE_DEC_ATT`, `SIEGE_APPEL`, `JURI_PREM`, `LIEU_PREM`, `DEMANDEUR`, `DEFENDEUR`, `PRESIDENT`, `AVOCAT_GL`, `AVOCATS`, `RAPPORTEUR`, `COMMISSAIRE_GVT`, `PUBLI_RECUEIL`, `TYPE_REC`, `NOR`, `NATURE_QUALIFIEE`, `LOI_DEF` (+ `@date`, `@nor`, `@num`), `TITRE_JO`, `URL_CC` | `Extra` `juri.<name in lower case>` (`juri.publi_bull_publie`, `juri.loi_def_date`, ...), as written: dates as written (`2999-01-01` is DILA's "none", as in LEGI), empty elements give nothing |
| `TEXTE` | transparent |
| `BLOC_TEXTUEL`, `SOMMAIRE`, `CITATION_JP`, `SAISINES`, `OBSERVATIONS` | transparent; every block inside carries `juri.field` |
| `SOMMAIRE/SCT` (titrage), `SOMMAIRE/ANA` (analysis) | one paragraph each, `juri.part` = `SCT` or `ANA`, `juri.id` its `@ID` (an `SCT` and the `ANA` it heads share one), `juri.type` (`@TYPE`, `PRINCIPAL`/`REFERENCE`) |
| `LIENS`/`LIEN` | one paragraph per link, its text the link's text; `juri.typelien`, `juri.sens`, `juri.id`, `juri.cidtexte`, `juri.naturetexte`, `juri.num`, `juri.numtexte`, `juri.nortexte`, `juri.datesignatexte`; `juri.group=LIENS` |
| `CONTENU`, `div` | transparent: text directly in them is an inferred paragraph |
| `p`, `blockquote`, `table`/`tr`/`th`/`td`, `ul`/`ol`/`li` | paragraph, quote, table (`rows`), lists |
| `br` | `break`: a `\n` in the block |
| `sup`, `sub`, `b`, `strong`, `i`, `em`, `u`, `font`, `span`, `a` | inline |

Old decisions are given as written: all capitals, page headers, a whole
decision as one run of `br`-broken lines in `CONTENU` (one inferred
paragraph, one line per printed line; split into `Continued` pieces past
the block size). Nothing is inferred: no heading is made from a line
that looks like one. Characters the file holds as C1 controls (some
decisions hold windows-1252's `€` as U+0080) are removed by core's text
convention, with `text.control_characters_removed`.

## Citing a block

The decision's id is `Extra["juri.id"]` (`JURITEXT...`, `CETATEXT...`,
`CONSTEXT...`) and every block's `Attrs["anchor_parent"]`; its ECLI, where
the file gives one, is `Extra["juri.ecli"]`. Each link carries the
target's id in `Attrs["juri.id"]`.

## Left to generic reading

- Nothing: on dev (1,436 files, 951 read with this profile)
  `xml.unmapped_elements` is 0.
- `br@clear` (6,389 on dev: CAPP 5,154, INCA 875, CASS 360) is counted
  in `xml.unmapped_attributes`, as in `legifrance`; its value is not in
  the output.

## Versions

Dev samples (2026-11-08 dumps): CASS 185, INCA 184, CAPP 193, JADE 200,
CONSTIT 189. `ECLI` is present in 71 of 562 `TEXTE_JURI_JUDI`, 11 of 200
`TEXTE_JURI_ADMIN` and all 189 `TEXTE_JURI_CONSTIT` files.
