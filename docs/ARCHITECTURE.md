# CPA Usage Keeper — Architecture

This document describes how CPA Usage Keeper is built: the runtime topology,
module layering, data flow, persistence design, frontend, and deployment
models. For *what* the system does, see [SPECIFICATION.md](SPECIFICATION.md).

---

## 1. System Context

```
                 ┌─────────────────────┐
                 │   CLIProxyAPI (CPA) │
                 │  (LLM proxy, in-    │
                 │   memory usage)     │
                 └─────────┬───────────┘
                           │ usage events (Redis pub/sub,
                           │  Redis list queue, or HTTP pull)
                           │ control messages (metadata changes)
                           ▼
┌────────┐        ┌──────────────────────────┐        ┌────────────────┐
│ Browser│◄──────►│      CPA Usage Keeper    │◄──────►│  Official       │
│ (SPA)  │  HTTP  │  Go server + SQLite      │ HTTPS  │  Ranking Center │
└────────┘        │  (this project)          │        │  keepers.cc.cd  │
                  └──────────┬───────────────┘        └────────────────┘
                             │ file backups / logs / archive
                             ▼
                        WORK_DIR (./data)
```

Usage Keeper is a single Go process embedding the web UI. It owns all of its
state in one SQLite database directory; the only hard external dependency is
CPA itself (Redis or HTTP), plus optional egress to GitHub (update checks)
and the ranking center (opt-in).

---

## 2. Module Layout (backend)

```
cmd/server/main.go ── flags (--env, --host, -v/--version), bootstrap logging
        │
        ▼
internal/app ────── application assembly (app.go) + background runners
        │            (maintenance, metadata sync, backups)
        ▼
internal/api ────── Gin router, handlers, auth middleware, DTO mapping
        │
        ├── internal/service ───── business services (usage, request logs,
        │                          identities, CPA API keys, auth files,
        │                          pricing, metadata sync, token processor,
        │                          provider-model Graph Neural Network (GNN))
        ├── internal/poller ────── ingestion: subscribe/pull sources, inbox
        │                          writer, ingest/process runners, the
        │                          serial usage aggregation runner
        ├── internal/repository ── GORM data access (+ activitystore,
        │                          latencystore, overviewstore, migration),
        │                          writer/reader pool wiring (dbresolver)
        ├── internal/quota ─────── provider quota clients & refresh engine
        ├── internal/ranking ───── opt-in community ranking client/service
        ├── internal/pricing ───── price catalog, resolver, snapshot
        ├── internal/cpa ───────── CPA management API client (HTTP + Redis queue client)
        ├── internal/auth ──────── session management (memory/persistent)
        ├── internal/backup ────── scheduled SQLite backups
        ├── internal/updatecheck ─ GitHub release check
        ├── internal/entities ──── GORM models (single source of schema truth)
        ├── internal/config ────── env/flag loading (godotenv), TZ data
        ├── internal/logging ───── logrus setup, Gin recovery, GORM logger
        └── internal/helper, internal/timeutil ── shared helpers
```

**Layering rules**

- Handlers in `internal/api` are thin: parse/validate → call service → map to
  DTO. SQL never appears above `internal/repository`.
- Services compose repositories (composition over inheritance); repositories
  own all GORM queries for their entity family.
- `internal/entities` is the schema source of truth; migrations live under
  `internal/repository/migration`.

---

## 3. Startup Wiring Order

`main()` → `app.NewWithOptions` → `config.Load` → `NewWithConfig`. The
constructor builds components in this strict order (see
`internal/app/app.go`):

1. `logging.Configure` (logrus, file rotation, GORM logger).
2. `repository.OpenDatabasePools` — **writer + reader** SQLite pools via
   GORM dbresolver (single writer, concurrent readers).
3. `ranking.NewService` / `NewRunner` (inert unless enabled).
4. `UsageRecentEventCache` — failure-tolerant; falls back to DB reads.
5. `LoadPricingSnapshot` → `pricing.NewCatalog`.
6. `cpa.NewClient` — CPA management API client.
7. `quota.NewServiceWithOptions`.
8. `poller.NewUsageAggregationRunner` (serial, 3 checkpoints).
9. `service.NewSyncServiceWithOptions` (recent-events hook + notifier).
10. `MetadataSyncRunner` (Auth Files / API Keys / providers / pricing sync).
11. Ingestion sources: `RedisPullSource` / `HTTPPullSource` /
    `RedisSubscribeSource` → `ControlAwareRedisInboxWriter` →
    `RedisIngestRunner` (backoff 1s→30s) → `RedisProcessRunner`, exposed via
    the `poller.NewRedisPoller` facade.
12. Optional `DatabaseBackupRunner`.
13. Domain services: Usage, RequestLog, UsageIdentity, CPAAPIKey,
    AuthFilesManagement, Pricing, ProviderModelGNN (sanitized proxy of the
    CPA `/v0/management/config` — the DTO in
    `internal/cpa/dto/providerconfig` decodes only whitelisted fields;
    `api-key`/`base-url`/`headers` are never parsed; internally, this
    service constructs a Graph Neural Network structure for providers
    and models with node/edge features).
14. `SessionManager` (persistent GORM store when auth is enabled, else
    in-memory) → auth handler → `api.NewRouter(webui.Static, ...)`.

`Run()` starts background tasks — **RedisIngest, RedisProcess,
UsageAggregation, Ranking, Maintenance, MetadataSync, QuotaAutoRefresh,
BackupMaintenance** — then serves HTTP (`ListenAndServe` or
`ListenAndServeTLS`). `Close()` shuts down in strict reverse order: stop
tasks → quota stop → cache close → reader pool → writer pool → logs.

---

## 4. Data Flow — Event Ingestion & Aggregation

```
CPA ──► IngestSource (subscribe | redis pull | http pull)
           │  raw payload (at-least-once)
           ▼
     redis_usage_inboxes  ── control msgs ──► MetadataSyncRunner
           │  (source column: redis_sub/redis_pull/http_pull)
           ▼
     RedisProcessRunner ──► usage_events ──► UsageAggregationRunner (serial)
                                                │  per pipeline, in tx:
                                                │  read batch → roll up →
                                                │  advance checkpoint
                                                ▼
                       overview hourly/daily · activity · latency stats
                                                │
                                                ▼
                                       API handlers ──► React SPA
```

Key invariants:

- **Durable inbox first.** Ingestion never blocks on analytics writes; a
  crash mid-pipeline loses nothing already acknowledged.
- **Single-writer discipline.** All writes go through the writer pool;
  analytics reads use the reader pool.
- **Exactly-once rollup.** `AdvanceUsageAggregationCheckpoint` does an
  optimistic-concurrency update (`WHERE name=? AND cursor=?`) **inside the
  caller's transaction** and requires `RowsAffected == 1`. Combined with the
  serial runner, every event is aggregated exactly once, even across
  restarts.
- **Warm reads.** A recent-events cache serves hot queries; on failure the
  system degrades to DB reads (availability over novelty).

---

## 5. Retention, Archive & Backups

- Daily maintenance at **04:30 local** moves `usage_events` older than **90
  local calendar days** into `usage_events_archive` (cold, permanent,
  reserved as the basis for future schema rebuilds; never queried by APIs).
- Inbox rows: kept for the current day on success, 7 days on failure.
- `backup.DatabaseBackupRunner` snapshots the SQLite file (default: enabled,
  every 24h, 7-day retention) using a dedicated store on the writer pool.
- Timezone (`TZ`, default `Asia/Shanghai`) anchors all calendar-day
  boundaries; `internal/config/tzdata.go` embeds tz data so containers work
  without system zoneinfo.

---

## 6. HTTP & Auth Architecture

- Gin router (`internal/api/router.go`) with `OptionalProviders` (quota,
  status, provider-model GNN, …) so the binary still boots when optional
  subsystems are off.
- Public edge routes (auth status/login/logout, embed transports, healthz)
  register **before** the session middleware; everything else sits behind it.
- Session transport: HttpOnly cookie, or a per-tab header token as the CPAMC
  iframe fallback. `frame-ancestors` CSP derives only from `CPA_PUBLIC_URL`.
- Persistent sessions (`auth_sessions` table) when password auth is on;
  otherwise an in-memory manager.
- Static SPA served from embedded FS (`web/static.go`: `go:embed all:dist`),
  including base-path rewriting when `APP_BASE_PATH` is set.
- Structured recovery via `internal/logging/gin_recovery.go`.

---

## 7. Frontend Architecture (`web/`)

- **React 19 + TypeScript (strict) + Vite**; no UI framework — a bespoke
  component kit in `src/components/ui` (Button, Card, Input, Select, Modal,
  PortalTooltip, LoadingSpinner, EmptyState, LanguageSwitcher,
  MainActionButton, icons).
- Styling: SCSS (dart-sass) global layers + **CSS Modules** per component;
  theming via `data-theme` (light "white" / dark / auto).
- Charts: **Chart.js 4** via `react-chartjs-2` (`src/lib/chartjs.ts`).
- Graphs: **@xyflow/react** (React Flow) renders the provider ↔ model
  relationship as an interactive **Graph Neural Network (GNN)** diagram on
  the Usage overview tab (`ProviderModelGNNPanel`), fed by
  `GET /api/provider-model-gnn`; labels prefer the model alias, Gemini
  entries merge into a single node, oauth aliases are sorted. Node and edge
  features support future advanced analytics (embeddings, structural
  learning, prediction). The panel surfaces GNN state directly — provider
  nodes are tinted by hue from the `kind_hash` feature, models flagged with
  `is_shared=1` get a badge, and per-node feature vectors and embeddings
  appear in tooltips. The summary chip shows provider/model/dim counters
  sourced from the GNN response meta block.
- i18n: **i18next**-backed system (`src/i18n`), locales **en + zh + zh-TW**
  (Traditional Chinese) selected via `LanguageSwitcher`.
- Structure: `assets`, `components/{ui,usage,test}`, `embed` (CPAMC iframe
  mode), `features/ranking`, `hooks`, `i18n`, `lib`, `pages`, `stores`,
  `styles`, `types`, `utils/usage`.
- Build: Vite alias `@ → src`, `base: './'`; dev proxy `/api → 127.0.0.1:8080`
  (`VITE_API_PROXY_TARGET`). Output lands in `web/dist` and is embedded into
  the Go binary.

---

## 8. Deployment Topologies

1. **Docker (recommended).** Multi-stage `Dockerfile`; data persisted via a
   volume at `WORK_DIR`. `docker-compose.example.yml` runs Keeper alone;
   README shows a combined CPA + Keeper stack.
2. **Prebuilt binaries.** GitHub `binary-release.yml` (tag `v*`): linux
   amd64/arm64, macOS amd64/arm64, Windows amd64/arm64 (MSYS2 UCRT64 /
   CLANGARM64), tar.gz/zip bundling `.env.example`, READMEs, LICENSE, and the
   systemd unit.
3. **Homebrew** (`Willxup/cpa-usage-keeper` tap), **GHCR** image.
4. **systemd** unit template (`deploy/linux/cpa-usage-keeper.service`,
   `__CPA_USAGE_KEEPER_DIR__` placeholder).
5. **From source:** `go run ./cmd/server/main.go` + `npm run dev` for the UI.

Container/start script: `docker-entrypoint.sh`; version injected via
`-ldflags -X cpa-usage-keeper/internal/version.Version=${VERSION}`.

---

## 9. Build & Verification Pipeline

- `make verify` (the baseline, also in CI `ci.yml`):
  `go test ./cmd/... ./internal/...` plus `npm --prefix ./web run
  test | lint | typecheck | build`.
- Release: `binary-release.yml` on `v*` tags (CGO builds per platform).

---

## 10. Extensibility Points

| Extension | Hook |
|---|---|
| New quota provider | Add a client under `internal/quota`, register in `registry.go`. |
| New ingest source | Implement the `IngestSource` interface in `internal/poller`. |
| New analytics | Add a store under `internal/repository` + extend the aggregation runner with a new checkpoint name. |
| New API surface | Thin handler in `internal/api` + service in `internal/service`. |
| Storage backends | Repository layer isolates GORM/dialect specifics (single seam for a future multi-DB port). |
| UI features | Pages under `web/src/pages`, shared widgets in `components/ui`, feature folders like `features/ranking`. |

---

## 11. Related Documents

- [SPECIFICATION.md](SPECIFICATION.md) — functional contract.
- [CONFIGURATION.md](CONFIGURATION.md) — env/flag reference.
- [ROADMAP.md](ROADMAP.md) — future phases.
- [CONTRIBUTING.md](CONTRIBUTING.md) — dev workflow.
