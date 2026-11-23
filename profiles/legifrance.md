# Profile `legifrance`: Légifrance LEGI and JORF exports

**Selected by** the root element, in no namespace: `ARTICLE`,
`TEXTE_VERSION`, `SECTION_TA`, `TEXTELR`, `JO` (a Journal officiel
issue's summary), `CONTENEUR`.

## Mapping

| Element | Model |
|---|---|
| root | transparent; `Extra["lf.root"]` |
| `META`, `META_COMMUN`, `META_SPEC`, `META_ARTICLE`, `META_TEXTE_VERSION`, `META_TEXTE_CHRONICLE`, `META_CONTENEUR` | transparent |
| `ID`, `ID_ELI`, `ANCIEN_ID`, `ORIGINE`, `URL`, `NATURE`, `NUM`, `ETAT`, `DATE_DEBUT`, `DATE_FIN`, `TYPE`, `CID`, `NOR`, `NUM_SEQUENCE`, `DATE_PUBLI`, `DATE_TEXTE`, `DERNIERE_MODIFICATION`, `ORIGINE_PUBLI`, `PAGE_DEB_PUBLI`, `PAGE_FIN_PUBLI`, `NUM_PARUTION`, `AUTORITE`, `MINISTERE`, `TITRE` | `Extra` `lf.<name in lower case>`, dates as written (`2999-01-01` is Légifrance's "no end") |
| `TITREFULL`, `SECTION_TA/TITRE_TA` | `Metadata.Title` and heading 1 |
| `MCS_ART/MC`, `MCS_TXT/MC` | `Metadata.Keywords` |
| `VERSION_A_VENIR`, `ENTREPRISE` (`DATES_EFFET`, `DOMAINES`) | `Extra` `lf.version_a_venir`, `lf.texte_entreprise`, `lf.date_effet`, `lf.domaine` |
| `CONTEXTE/TEXTE` (attributes), `TITRE_TXT`, the `TM/TITRE_TM` chain | `Extra` `lf.texte.cid`, `lf.texte.nature`, ..., `lf.texte.titre` (+ `_id`, `_court`, `_debut`, `_fin`), `lf.contexte` (`Partie législative > Livre Ier > ...`) with `lf.contexte_ids` |
| `TM`/`TITRE_TM` elsewhere (the `JO` summary) | section and heading by nesting, anchor `@id` |
| `VERSIONS`/`VERSION`/`LIEN_ART`, `LIEN_TXT` | one paragraph per version, its text the article number (`@num`) or the text's title (`@titretxt`) (`Attrs["attribute"]`); `lf.id`, `lf.debut`, `lf.fin`, `lf.etat`, `lf.origine`; `lf.group=VERSIONS`, `lf.version_etat` |
| `LIENS`/`LIEN` | one paragraph per link, its text the link's text; `lf.typelien`, `lf.sens` (direction: `source`/`cible`), `lf.id`, `lf.cidtexte`, `lf.naturetexte`, `lf.num`, `lf.numtexte`, `lf.nortexte`, `lf.datesignatexte`; `lf.group=LIENS` |
| `STRUCT`, `STRUCTURE_TA`, `STRUCTURE_TXT` with `LIEN_ART`, `LIEN_SECTION_TA` | one paragraph per article (`@num`) or section (its title), with ids, `lf.niv`, `lf.debut`, `lf.fin`, `lf.etat`, `lf.url`; `lf.group` names the list |
| `BLOC_TEXTUEL`, `NOTA`, `SM`, `VISAS`, `SIGNATAIRES`, `TP`, `ABRO`, `RECT`, `NOTICE`, `COMMENTAIRE` | transparent; every block inside carries `lf.field` |
| `CONTENU`, `div` | transparent (text directly in a `div` is an inferred paragraph) |
| `p`, `blockquote`, `table`/`tr`/`th`/`td`, `ul`/`ol`/`li` | paragraph, quote, table (`rows`), lists |
| `sup`, `sub`, `b`, `i`, `u`, `font`, `span`, `a` | inline |

## Citing a block

The document's own id is `Extra["lf.id"]` (`LEGIARTI...`), with
`lf.id_eli` where the file gives one, and `lf.num` the article number;
`lf.contexte` and `lf.contexte_ids` give its place in the code. Each
version, link and structure entry carries the target's id in
`Attrs["lf.id"]` and its validity dates in `lf.debut`/`lf.fin`.

## Left to generic reading

- Nothing now: `br` (13,457 in the dev set) is a `break` rule, a `\n`
  inside the block. Légifrance writes most of its body text as `div`
  text broken by `br`, so such a paragraph holds the printed lines, one
  per line.

`td@align`, `td@valign` are not in the output.

## Versions

Dev samples: LEGI and JORF exports (articles, text versions, `TEXTELR`
structures, sections, one `JO` summary). Not tested: `CONTENEUR` and
the `TEXTEKALI`/CNIL/JADE families, which have other roots.
