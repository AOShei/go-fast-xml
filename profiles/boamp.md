# Profile `boamp`: BOAMP, French public procurement notices

DILA's open data of the Bulletin officiel des annonces des marchés
publics (`echanges.dila.gouv.fr/OPENDATA/BOAMP`).

## Selection

- Root `ann` in `http://boamp.journal-officiel.gouv.fr/XML/3.2.5` (the
  current files, and the 2022 to 2025 archives).
- Root `ANNONCE` in no namespace (the archives of schema 2.3, 2015 to
  2021). The name is a common French word and the files carry no
  namespace or DOCTYPE, so nothing else can be tested: another
  vocabulary with that root would be named `boamp` and read mostly
  generically, since the rules name BOAMP's `GESTION` and `DONNEES`.
  The archives before 2015 (schemas 0.1 and 1.1, yearly ZIP files) were
  not fetched; if their root is `ANNONCE` they are read with the same
  rules, untested.

`ann` in no namespace, `ANNONCE` in a namespace and `annonce` stay
generic (`TestBoampSelection`).

## Mapping

As `eforms` and `ted`: values are rows of tables of names and values, the
name the file's own element path; an element whose presence is the value
(`OUVERT`, `JOUE`, `OPTIONS_NON`) is a row with an empty value.

| Element | Output |
|---|---|
| `GESTION/REFERENCE`, `INDEXATION`, `NOM_HTML`, `MARCHE` | a table each (`REFERENCE/IDWEB`, `INDEXATION/RESUME_OBJET`, `INDEXATION/DESCRIPTEURS/DESCRIPTEUR/LIBELLE`) |
| `DONNEES/OBJET/TITRE_MARCHE` | `Metadata.Title` and a level-1 heading |
| `OBJET/OBJET_COMPLET` | a paragraph, `Attrs["boamp.field"]` |
| `OBJET/LOTS/LOT` | a section: `INTITULE` a level-2 heading, `DESCRIPTION` and `INFO_COMPL` paragraphs, its other children a table each, names from `LOT/` |
| `RENSEIGNEMENTS_COMPLEMENTAIRES/RENS_COMPLEMENT` | a paragraph |
| other children of `DONNEES` (`IDENTITE`, `PROCEDURE`, `CONDITION_*`, `ATTRIBUTION`, `RECTIF`, ...) | a table each |
| `DONNEES/FNSimple`, `MAPA` (the national simplified forms) | a table per part (`organisme`, `initial`, `attribution`, `rectificatif`); their title (`natureMarche/intitule`) and description are rows, and `Metadata.Title` is not filled |
| `DONNEES/EFORMS/ContractNotice`, `ContractAwardNotice` (an eForms notice, with BOAMP's own namespaces for the UBL components) | as `eforms` reads a notice: title a level-1 heading and `Metadata.Title`, each lot a section with its identifier as heading and anchor, the rest tables; keys `boamp.*`. The notice's own header values (`cbc:ID`, `IssueDate`) are tables here, not `Extra` |
| `HTML`, `HTMLSYNTHESE` (the notice as a page and its summary, escaped HTML) | embedded documents handed to `Options.Child` as `text/html`, listed in `Document.Children`; with no child extractor each is a paragraph of its markup, `Attrs["media_type"]`, as for a feed's HTML. They repeat the notice's data as prose |

## Measured (dev: 201 files; held-out: 70)

Notices of three days of November 2026 (schema 3.2.5, 158 dev files:
107 with an eForms notice, 42 `FNSimple`, 9 `MAPA`) and two archives of
January 2019 (`ANNONCE`, 43 dev files).

- dev: 201 read with `boamp`, 0 errors, 0 `Validate` failures; blocks:
  heading 941, paragraph 1,108, table 8,957; nothing unmapped.
  `limit.exceeded` on 1 file: an HTML page longer than 1 MiB is handed
  over cut there.
- held-out: 70 read with `boamp`, 0 errors, 0 `Validate` failures;
  heading 308, paragraph 365, table 2,752; nothing unmapped.

## Oracles (`TestBoampOracles`, 201 dev files)

(a) and (b) pass on every file; `HTML` and `HTMLSYNTHESE` are left out
of (a) (embedded documents).
