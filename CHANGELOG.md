# Changelog

All notable changes to this project are documented here.

## [Unreleased]

## [v1.16.1-arsydoni4326-alt] — 2026-09-24

### Changed
- **Merged upstream `v1.15.7`** into `develop` (git flow). All custom
  features are preserved: Provider Model GNN domain (`internal/gnn` +
  `web/src/features/model-gnn/`), IP geo/enrichment for Session Settings
  (`internal/enrichgeo`), workspace docs & deployment files. The merge was
  clean (no file overlap with upstream changes).
- Upstream fixes now included:
  - `fix(ui): align controls and improve quota error styling` (#570)
  - `fix(usage): normalize empty parent sessions` (#569)
  - `fix(credentials): align table header weight` (#568)
  - `fix(credentials): improve credential subtitle layout` (#567)

### Validation
- `go build ./...` and `go vet ./internal/...` clean.
- `go test ./cmd/... ./internal/...` all packages pass.
- Frontend `tsc --noEmit` clean; `eslint` clean.
- `vitest run` full suite: 1287 tests; the handful of timeouts observed
  under heavy system load (load avg ~11, memory pressure) all pass when
  re-run in isolation — environmental flakiness, not merge-related.

## [v1.16.0-arsydoni4326-alt] — 2026-09-21

### Changed
- **Provider Model GNN domain isolation** (`feature/model-gnn-domain`): the
  entire GNN feature moved into a dedicated, self-contained domain so future
  upstream merges cannot remove or replace it. Backend: `internal/service/
  provider_model_gnn.go` → `internal/gnn/gnn.go` (package `gnn`, constructor
  renamed to `gnn.NewService`), route registration moved out of `internal/api`
  into `internal/gnn/httpapi` (following the `internal/ranking/httpapi`
  pattern; `GET /api/v1/provider-model-gnn` and the
  `/provider-model-graph` alias unchanged). Frontend: panel, both renderers,
  and the layout/join helper moved from `web/src/components/usage/` to
  `web/src/features/model-gnn/` with feature-local `api.ts` and `types.ts`
  (after the `features/ranking` convention); the shared
  `ProviderModelGraphResponse` type block and `fetchProviderModelGNN` were
  removed from `web/src/lib/types.ts` / `web/src/lib/api.ts`. Wiring is now
  limited to four integration points: `gnn.NewService` in `internal/app/
  app.go`, `OptionalProviders.ProviderModelGraph` +
  `gnnhttpapi.RegisterRoutes` in `internal/api/router.go`, the
  `@/features/model-gnn` import in `web/src/pages/UsagePage.tsx`, and the
  `usage_stats.provider_model_graph.*` i18n keys. API contract, response
  shape, and UI behavior are unchanged.

### Documentation
- `docs/ARCHITECTURE.md`: module map now lists `internal/gnn`; the §7
  permanence callout records the domain-isolation contract and the allowed
  integration points.
- `docs/SPECIFICATION.md`: UC-12 permanence note extended with the domain
  isolation (backend `internal/gnn`, frontend `web/src/features/model-gnn/`).
- `README.md` / `README.zh.md`: permanent-feature notes mention the domain
  isolation.
- `AGENTS.md`: frontend GNN guidance points at the dedicated domain.

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
- Domain-isolation validation (`feature/model-gnn-domain`): `go build ./...`
  clean; `go test ./cmd/... ./internal/...` all packages pass (incl. the
  relocated `internal/gnn` tests); frontend `typecheck` clean; `vitest run`
  185 files / 1506 tests pass; `eslint` clean; `vite build` succeeds with the
  Reagraph WebGL chunk still lazy-loaded separately
  (`ProviderModelReagraphPanel` chunk, ~378 kB gzip).
