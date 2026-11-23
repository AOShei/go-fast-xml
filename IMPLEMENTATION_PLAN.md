# go-fast-xml: implementation plan

The approved plan is `XML_PLAN.md` in the workspace root (2026-11-23).
This file tracks its phases (section 7) in this repository.

| # | Phase | Status |
|---|---|---|
| 0 | Design check: `PROFILE_FORMAT.md`, the Service-Public mapping, corpus samples | with the orchestrator and Andrew |
| 1 | Reader and generic reading (plan sections 1 to 3) | in progress on `phase-1-reader` |
| 1r | Fresh-eyes review of 1, then fixes | not started |
| 2 | Profile engine and Service-Public | not started |
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
| Limits and worst cases | |
| Command `cmd/gofastxml` | |
| Fuzz targets | |
| README | |
