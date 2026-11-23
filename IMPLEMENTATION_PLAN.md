# go-fast-xml: implementation plan

The approved plan is `XML_PLAN.md` in the workspace root (2026-11-23).
This file tracks its phases (section 7) in this repository.

| # | Phase | Status |
|---|---|---|
| 0 | Design check: `PROFILE_FORMAT.md`, the Service-Public mapping, corpus samples | with the orchestrator and Andrew |
| 1 | Reader and generic reading (plan sections 1 to 3) | done, merged |
| 1r | Fresh-eyes review of 1, then fixes | not started |
| 2 | Profile engine and Service-Public | done on `phase-2-profiles`, awaiting review |
| 3 | Profiles of rounds 2, 3 and 4 | not started |
| 4 | `-draft-profile` | not started |
| 5 | Review rounds 2 and 3, hardening, speed | not started |
| 6 | Finish: README complete, CHANGELOG dated | not started |
| 7 | Tag, core README row, umbrella wiring | not started |

## Phase 1

| Step | Status |
|---|---|
| Scanner (`internal/xmlscan`): namespaces, offsets, comments, PIs, DOCTYPE, recovery, bounded entity expansion | done |
| Encoding: BOM, XML declaration, UTF-8/16, windows-1252, `internal/charset` | done |
| DOCTYPE internal subset, entities, external references | done |
| Generic reader: blocks, attributes, comments, metadata; oracle test against `encoding/xml` | done |
| Limits and worst cases | done |
| Command `cmd/gofastxml` | done |
| Fuzz targets: `FuzzExtract`, `FuzzScanner`, `FuzzParse` | done |
| README | done |

Phase 1 is complete on `phase-1-reader`, awaiting review (phase 1r).

### Seam for phase 2

- `internal/xmlread.Reader.Run(h Handler)` drives any consumer. The
  reader owns the element stack (`Element`: qualified name, local name,
  namespace URI, attributes, `Preserve`, start-tag offsets, a `Data` slot
  for the consumer), the DTD and entities, and the counts that become
  warnings. `Element` values are reused after `EndElement`: a consumer
  copies what it keeps.
- `internal/generic.Generic` is one `Handler`. `generic.Out` is the part
  writer (budget, counts of cuts) a profile consumer shares. A profile
  consumer would implement `Handler` and pass the events of an element it
  has no rule for, and of its subtree, to a `Generic` writing to the same
  `Out`; `Generic.Finish` writes out what is pending.
- `Extract` builds reader, `Out` and consumer separately; the consumer is
  chosen in one place.

## Phase 2

| Step | Status |
|---|---|
| `PROFILE_FORMAT.md` (part A as user documentation) | done |
| Loader and selectors (`internal/profile`), fuzzed | done |
| Rule engine (`internal/engine`): every rule kind, both table models, spans, parts, citing | done |
| Profile choice: built-in (`profiles/*.json`), `Extractor.Profiles`, `Extractor.Profile`, command flags | done |
| Service-Public profile, corpus test, oracles of C6 and A6 | done |
| Benchmark, README, CHANGELOG | done |
