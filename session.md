# Session Log

This file preserves working context between sessions. Update it whenever the project changes meaningfully. Format: newest session at the bottom of the log table, with the "Current state" section always refreshed.

## Current state

- **Latest milestone:** Provider ↔ model relationship graph shipped — a sanitized `/api/provider-model-graph` proxy surfaces the CPA `/v0/management/config` provider/model aliases (secrets never parsed), and the Usage overview tab renders them as an interactive React Flow diagram.

- **Codebase shape:** Go 1.26+ backend (`cmd/server`, `internal/*`), React 19 + TypeScript SPA in `web/` (Vite 8, SCSS modules, custom design system — no UI framework), embedded into the binary via `web/static.go`.
- **Docs policy:** When modifying features, update the relevant doc in `docs/` and keep `README.md` capabilities in sync (see Project Guidelines).
- **Roadmap:** `docs/ROADMAP.md` holds phased future work (P0 docs/tooling → P4 growth). Before starting roadmap work, re-read that file and tick off steps as they land.

## Key facts to remember

- Storage is **SQLite via GORM + dbresolver** (writer + reader pools). Single-writer discipline: all ingestion writes and derived aggregations are serialized through background runners.
- Ingestion priority: **Redis SUBSCRIBE → Redis list pull → HTTP usage-queue pull** (fallback chain, backoff on failure), landing in `redis_usage_inboxes` inbox before being processed into `usage_events`.
- Derived stats are maintained by one serial `UsageAggregationRunner` with three checkpoints (`overview`, `activity`, `latency`) in `usage_aggregation_checkpoints`; cursors only move monotonically inside transactions.
- Auth has two roles: **admin** (password) and **api-key viewer** (CPA API Key). Sessions may use HttpOnly cookies or per-tab header tokens (CPAMC embed fallback).
- Ranking is opt-in, signed with ed25519 identity, talks to official center `https://keepers.cc.cd` (`/api/v1/leaderboards*`). Failure of ranking must never affect usage collection.
- i18n runs on i18next with **3 locales: en, zh, zh-TW** — every user-facing string must be added to all three.
- Auth is **on by default**: `AUTH_ENABLED` defaults to `true`; unset means `LOGIN_PASSWORD` is required at startup, and the public example password value is rejected.
- Update checker hits the GitHub Releases API; version injected with `-ldflags -X cpa-usage-keeper/internal/version.Version=...`.
- Verification baseline before any PR: `make verify` (or `go test ./...` + npm test/lint/typecheck/build in `web/`).

## Session log

| Date (UTC+7) | Session summary | Artifacts touched |
| --- | --- | --- |
| 2026-08-02 | Scanned whole repository (README, `cmd/server/main.go`, `internal/app/app.go`, code-definition maps of `internal/api` + `internal/entities`, parallel deep-dives into HTTP layer, data pipeline, domain services, frontend, and build/ops). Planned and wrote the complete documentation set: overhauled README, SPEC, ARCHITECTURE, phased ROADMAP, CONFIGURATION reference, CONTRIBUTING. Created this session file. | `README.md`, `docs/*`, `session.md` |
| 2026-08-02 (cont.) | Added a provider ↔ model relationship graph. Backend: `providerconfig.ManagementConfig` DTO parses only whitelisted fields from CPA `/v0/management/config` (api-key/base-url/headers/priority never decoded), `FetchManagementConfig` client method, `NewProviderModelGraphService` + `GET /api/provider-model-graph` route, `provider_model_graph_test.go` (7 tests). Frontend: `fetchProviderModelGraph` helper, `buildProviderModelGraph` (shared models merge), `ProviderModelGraphPanel` using `@xyflow/react`, i18n in 3 locales, rendered in the Usage overview tab after `OverviewRealtimePanel`. Verified: go build/vet/test all green; web typecheck/lint/vitest (105 files / 969 tests) green. | `internal/cpa/dto/providerconfig/*`, `internal/cpa/dto/response/response.go`, `internal/cpa/{endpoints,client}.go`, `internal/service/provider_model_graph*.go`, `internal/api/provider_model_graph.go`, `internal/{api/router,app/app}.go`, `web/src/{lib,components/usage,pages}/*`, `session.md` |
| 2026-08-02 (cont.) | Post-merge documentation sync after reviewing merge `f1f7b0b` from `author/main`. SPEC: added `GET /api/provider-model-graph` to §5.2, new UC-12 provider-topology + renumbered UC-13, §7 whitelisted-fields security note, roles table aligned to AUTH_ENABLED default. ARCH: service list + wiring order now include ProviderModelGraph, OptionalProviders mention, frontend stack documents @xyflow/react graph panel and i18next with en/zh/zh-TW. CONTRIBUTING/ROADMAP: locale lists corrected en/zh → en/zh/zh-TW. CONFIG: fixed `AUTH_ENABLED` default `false` → `true` (matches code `getBool("AUTH_ENABLED", true)` and READMEs). Verified via `internal/config` tests and i18n source. README/README.zh needed no changes (already updated by the merge). | `docs/SPECIFICATION.md`, `docs/ARCHITECTURE.md`, `docs/CONTRIBUTING.md`, `docs/ROADMAP.md`, `docs/CONFIGURATION.md`, `session.md` |

