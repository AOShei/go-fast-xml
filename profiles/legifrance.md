# Profile `legifrance`: Légifrance LEGI, JORF, KALI and CNIL exports

**Selected by** the root element, in no namespace: `ARTICLE`,
`TEXTE_VERSION`, `SECTION_TA`, `TEXTELR`, `JO` (a Journal officiel
issue's summary), `CONTENEUR`; KALI's `TEXTEKALI` (a collective
agreement's structure, as `TEXTELR`) and `IDCC` (an agreement's
container, by IDCC number), KALI also using `ARTICLE`, `TEXTE_VERSION`,
`SECTION_TA`; CNIL's `TEXTE_CNIL`. DILA's case law (`TEXTE_JURI_*`) is
read by `dila-juri`.

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

KALI and CNIL add:

| Element | Model |
|---|---|
| `TEXTEKALI`, `IDCC`, `TEXTE_CNIL` root | transparent; `Extra["lf.root"]` |
| `IDCC/META/META_SPEC/META_CONTENEUR/TITRE` | `Metadata.Title` and heading 1 (elsewhere `TITRE` stays `lf.titre`) |
| `IDCC`'s `STRUCTURE_TXT/TM/TITRE_TM`, `LIEN_TXT` | as for `CONTENEUR`: a section and heading per `TM`, one paragraph per text (`@titretxt`, `lf.idtxt`), `lf.group=STRUCTURE_TXT` |
| `TXTS_ISO` | transparent; blocks inside carry `lf.group=TXTS_ISO` (empty on dev) |
| `ACTS_PRO/ACT_PRO`, `NUMS_BROCH/NUM_BROCH`, `CALIPSOS/CALIPSO` (KALI's subject), `HISTORIQUE`, `DATE_DEB_EXT`, `DATE_FIN_EXT` (extension dates), `APPLI_GEO`, `CODES_NOMENCLATURES/CODE`, `LIBELLE_EXTENSION`, `LIBELLE_ELARGISSEMENT` | `Extra` `lf.act_pro`, `lf.num_broch`, `lf.calipso`, `lf.historique`, `lf.date_deb_ext`, `lf.date_fin_ext`, `lf.appli_geo`, `lf.code_nomenclature`, `lf.libelle_extension`, `lf.libelle_elargissement`, as written, repeats one per line |
| `CONTEXTE/CONTENEUR` (attributes only) | `Extra` `lf.conteneur.cid`, `.nature`, `.num` (the IDCC), `.titre`, `.etat`, `.date_publi`, `.url` |
| `CONDITION_DIFFERE` | a paragraph, `lf.field=CONDITION_DIFFERE` |
| `INFORMATIONS_COMPLEMENTAIRES`, `DESCRIPTION_FUSION`, `INFOS_RESTRUCTURATION_BRANCHE` (each holding a `CONTENU`) | transparent; blocks inside carry `lf.field` |
| `SIGNATAIRES/SIGNPATRON`, `SIGNSYNDIC`, `ORGADHERE`, `ORGDENONCE`, `SIGNEXT`, `EXECUTION` | transparent; blocks inside carry `lf.signataires` (and `lf.field=SIGNATAIRES`) |
| `META_CNIL` | transparent |
| `META_CNIL/NUMERO`, `NATURE_DELIB`, `ETAT_JURIDIQUE` | `Extra` `lf.numero`, `lf.nature_delib`, `lf.etat_juridique`; its other children are LEGI's (`TITREFULL` the title, `TITRE`, `DATE_TEXTE`, `NOR`, ...) |

CNIL is read here rather than by `dila-juri`: `TEXTE_CNIL` has no
`META_JURI` and no `SOMMAIRE`, and its metadata and body are LEGI/JORF's.

## Citing a block

The document's own id is `Extra["lf.id"]` (`LEGIARTI...`), with
`lf.id_eli` where the file gives one, and `lf.num` the article number;
`lf.contexte` and `lf.contexte_ids` give its place in the code. Each
version, link and structure entry carries the target's id in
`Attrs["lf.id"]` and its validity dates in `lf.debut`/`lf.fin`.

## Left to generic reading

- Nothing: the dev set has no element with no rule
  (`xml.unmapped_elements` 0; it was 13,457, all `br`, before `br`
  became a `break` rule, a `\n` inside the block). Légifrance writes most of its body text as `div`
  text broken by `br`, so such a paragraph holds the printed lines, one
  per line.

`td@align`, `td@valign` are not in the output.

- KALI and CNIL (`xml-dila`, 485 dev files: KALI 298, CNIL 187):
  `xml.unmapped_elements` 0. Before the KALI rules, the 266 KALI files
  with LEGI roots left 1,238 elements unmapped (`CALIPSOS` 175,
  `CONTENEUR` 175, `DATE_DEB_EXT` 175, `DATE_FIN_EXT` 175, `HISTORIQUE`
  175, `INFORMATIONS_COMPLEMENTAIRES` 52, `APPLI_GEO`, `CODES_NOMENCLATURES`,
  `EXECUTION`, `LIBELLE_ELARGISSEMENT`, `LIBELLE_EXTENSION`, `ORGADHERE`,
  `ORGDENONCE`, `SIGNEXT`, `SIGNPATRON`, `SIGNSYNDIC` 28 each, `CALIPSO`
  11, `CONDITION_DIFFERE` 7, `CODE` 5, `DESCRIPTION_FUSION` 4,
  `INFOS_RESTRUCTURATION_BRANCHE` 4), and `TEXTEKALI` (29), `IDCC` (3)
  and `TEXTE_CNIL` (187) were read generically. Attributes no rule reads,
  as for LEGI: KALI `td@align` 507, `font@color` 16, `br@clear` 8, `p@dir`
  6, `font@size` 1, `th@align` 1; CNIL `br@clear` 106,
  `table@cellpadding` 9, `table@cellspacing` 8, `td@align` 4,
  `font@color` 1. They are left unread as in LEGI files (a rule reading
  `td@align`, which LEGI files have, would change their output).

## Versions

Dev samples: LEGI and JORF exports (articles, text versions, `TEXTELR`
structures, sections, one `JO` summary); KALI (2026-11-08 dump: 175
`ARTICLE`, 63 `SECTION_TA`, 28 `TEXTE_VERSION`, 29 `TEXTEKALI`, 3
`IDCC`) and CNIL (187 `TEXTE_CNIL`). Adding KALI and CNIL left the output
of every `xml-legifrance` file byte-identical. Not tested: `CONTENEUR`
(no file on dev, LEGI or KALI).
