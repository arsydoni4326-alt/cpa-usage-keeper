# Changelog

All notable changes to this project are documented here.

## [Unreleased]

### Fixed
- Restored the `cpaManagementURL` definition (`getBackToCPALinkURL(status)`)
  in `web/src/pages/UsagePage.tsx`, which was dropped during the merge that
  introduced the shared glass dashboard header (`a32e5523`). The dangling
  reference broke `tsc --noEmit` and therefore every fresh frontend build,
  which is why the Provider Model GNN diagram
  (`ProviderModelGNNPanel`) appeared missing from deployed builds.

### Documentation
- Added `AGENTS.md` at the project root: a consolidated orientation file for
  AI coding agents covering the project overview, backend layering rules,
  frontend conventions, the `make verify` baseline, the documentation
  workflow, security notes, runtime background components, and the installed
  antislop skills (`.agents/skills/`), with an antislop pointer block so the
  first-run install wizard does not re-trigger.
- `docs/ARCHITECTURE.md`: added a permanence requirement for the Provider
  Model GNN diagram (must stay mounted on the Usage overview tab; removal or
  replacement requires an explicit documented architectural decision).
- `docs/SPECIFICATION.md`: marked UC-12 (Provider topology via GNN) as
  permanent and mandatory.
- `README.zh.md`: synced the feature list with the GNN wording in
  `README.md` (dual renderer: React Flow grid + Reagraph WebGL).

### Validation
- `tsc --noEmit` clean; `vitest run` 177 files / 1450 tests pass.
- `go build ./...` verified.
