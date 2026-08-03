# CPA Usage Keeper — Functional Specification

This document describes what CPA Usage Keeper does: its purpose, users, domain
model, HTTP API surface, and business rules. It is the functional contract for
the system; see [ARCHITECTURE.md](ARCHITECTURE.md) for how it is implemented.

---

## 1. Purpose & Scope

CPA Usage Keeper is a standalone persistence and analytics companion for
[CLIProxyAPI (CPA)](https://github.com/router-for-me/CLIProxyAPI). CPA itself
keeps usage data in memory; Usage Keeper ingests every usage event CPA
produces, stores it permanently in SQLite, and serves a web dashboard for
exploration, cost analysis, quota monitoring, and community ranking.

**In scope**

- Durable ingestion of CPA usage events (Redis pub/sub, Redis queue, or HTTP
  polling) with at-least-once delivery semantics.
- Long-term storage, retention (90 days hot + permanent cold archive), and
  automated SQLite backups.
- Analytics: overviews, time-series activity, token/cost/latency analysis,
  per-API-key and per-identity breakdowns.
- Request-level request logs (when enabled on the CPA side).
- Mirror and management of CPA metadata: Auth Files, API Keys, AI Providers,
  and model pricing rules.
- Quota tracking and refresh for AI providers (Claude, Codex, Gemini CLI,
  Kimi, xAI, Antigravity) attached to Auth Files.
- Optional, opt-in community ranking against the official center.
- Web UI (admin) and a read-only per-API-key viewer page.

**Out of scope**

- Proxying LLM traffic (that is CPA's job).
- Multi-tenant SaaS operation; Usage Keeper is a single-instance,
  single-admin tool.

---

## 2. Users & Roles

| Role | How they authenticate | Capabilities |
|------|-----------------------|--------------|
| **Admin** | Password login (`LOGIN_PASSWORD`), required when `AUTH_ENABLED=true` (default) | Full dashboard: all usage data, identities, pricing, quota, settings, auth files, API keys, update checks. |
| **Anonymous admin** | No login when `AUTH_ENABLED` is explicitly set to `false` | Same as admin. Intended only for trusted networks. |
| **API-key viewer** | One of the sync'd CPA API Keys (`/api/auth/api-key-login` or a per-tab key session) | Read-only, scoped view: `/api/key-overview` and `/api/key-activity` filtered to *their* key only. Rate-limited per viewer session. |

There is no user management beyond the single admin password and the CPA API
keys that are synchronized from CPA itself.

---

## 3. Use Cases / User Stories

### Admin

- **UC-1 — Monitor usage.** As an admin, I open the Overview page to see
  requests, tokens, cost, cache hit rate, success rate, RPM/TPM, and latency
  for a selectable time window, including a realtime view.
- **UC-2 — Explore events.** As an admin, I browse the request-level event
  stream, filter by API key, auth file, model, provider, status, and time
  range, and choose which columns to display.
- **UC-3 — Analyze trends.** As an admin, I open Analysis to see time-series
  trends, cost composition, usage heatmaps, and latency diagnostics per model
  or provider.
- **UC-4 — Track identities.** As an admin, I see usage grouped by identity
  (API key and/or auth file) with display names, so I can attribute cost to
  people or projects.
- **UC-5 — Manage quota.** As an admin, I view and refresh provider quotas
  for each Auth File, configure auto-refresh, inspect raw provider payloads,
  and reset credit counters after a billing-cycle reset.
- **UC-6 — Manage pricing.** As an admin, I view the model price catalog
  synced from CPA, override rules (multipliers, custom rates), preview the
  effect of a sync, and apply pricing changes at batch or single-model level.
- **UC-7 — Mirror CPA metadata.** As an admin, Auth Files, API Keys, and AI
  Providers are automatically synchronized from CPA so the dashboard stays
  current without manual edits.
- **UC-8 — Per-key sharing.** As an admin, I share an API key with a user so
  they can view their own usage on a read-only page.
- **UC-9 — Community ranking (opt-in).** As an admin, I opt in to the
  community ranking; my instance publishes an ed25519-signed identity and
  appears on the official leaderboard.
- **UC-10 — Stay current.** As an admin, I see a notification when a newer
  release is available (hidden for dev builds).
- **UC-11 — Data safety.** As an admin, daily backups and the 90-day →
  archive retention policy happen automatically; I can rely on the cold
  archive for future long-range rebuilds.
- **UC-12 — Provider topology via GNN.** As an admin, I view the provider ↔
  model relationship as an interactive **Graph Neural Network (GNN)** diagram
  (Usage overview tab), built from a sanitized snapshot of the CPA management
  config. This GNN supports rich analytics (learned node/edge features,
  structural learning, and prediction).

### API-key viewer

- **UC-13 — View my usage.** As a key holder, I authenticate with my CPA API
  key and see overview + activity statistics scoped strictly to my key.



---

## 4. Domain Model

Entities live in `internal/entities/` (GORM models).

| Entity | Purpose | Key fields / notes |
|--------|---------|--------------------|
| `UsageEvent` | One ingested CPA usage record (hot storage). | Request/response token counts (input, output, cache read/write, reasoning), model, provider, API key, auth file, status, latency, cost inputs, timestamps. Hot retention: 90 local calendar days. |
| `UsageEventArchive` | Cold storage for events aged out of `usage_events`. | Same shape; custom `TableName`; permanent; not queried by APIs; reserved for future schema rebuilds. |
| `RedisUsageInbox` | Durable inbox between ingestion and processing. | `source` column records origin (redis subscribe / redis pull / http pull); retention: current day after success, 7 days after failure. |
| `UsageOverviewHourlyStat` / `UsageOverviewDailyStat` | Pre-aggregated overview counters per hour/day. | Produced by the aggregation runner. |
| `UsageActivityStat` (+`UsageActivityGrain`) | Time-series activity rollups at multiple grains. | Powers charts/heatmaps. |
| `UsageLatencyStat` (+`UsageLatencyBucketType`) | Latency rollups and sketch buckets (p50/p90/p99 diagnostics). | Bucketed by type. |
| `UsageAggregationCheckpoint` (+`UsageAggregationCheckpointName`) | Monotonic cursors for the three aggregation pipelines (overview, activity, latency). | `AdvanceUsageAggregationCheckpoint` applies a name+expected-cursor optimistic-concurrency update *inside the caller's transaction*; `RowsAffected == 1` is required. |
| `UsageIdentity` (+`UsageIdentityAuthType`) | Identity dimension for grouping usage (API key, auth file). | Display names resolved via `usage_identity_display_name` helper. |
| `CPAAPIKey` | Mirrored CPA API key with local alias/settings. | Drives the read-only viewer role. |
| `ModelPriceSetting` / `ModelPriceRule` | Persisted pricing snapshot and admin overrides. | Loaded once at boot into the pricing catalog. |
| `AuthSession` | Persistent login sessions (when auth enabled). | In-memory session manager is the non-persistent alternative. |
| `AppSetting` | Generic key/value settings store. | Holds quota auto-refresh settings, ranking identity, etc. |

---

## 5. HTTP API Reference

Base path: `/api` under the configured `APP_BASE_PATH`. Auth edge routes and
the SPA/embedding assets are public; everything else requires a session
(HttpOnly cookie or the per-tab header token used by the embed fallback).

### 5.1 Public routes (no session required)

| Method & path | Description |
|---|---|
| `GET /` (+ SPA assets) | Embedded web UI. |
| `GET /cpamc.css`, `/cpamc.js`, `/cpamc.html`, `/tab.html`, `/embed.js` | CPAMC embed transports for the iframe plugin. |
| `GET /api/healthz` | Liveness/readiness probe. |
| `GET /api/auth/status` | Whether auth is enabled and the caller has a session. |
| `GET /api/auth/api-keys` | Whether API-key login is available (key list metadata for the login screen). |
| `POST /api/auth/login` | Password login. Rate-limited (failed-attempt counters). |
| `POST /api/auth/api-key-login` | Viewer login using a CPA API key. Rate-limited. |
| `GET /api/auth/api-key-session` | Establish/refresh a per-tab viewer session. |
| `POST /api/auth/logout` | Destroy the caller's session. |

### 5.2 Authenticated admin routes (`/api`, session required)

| Method & path | Description |
|---|---|
| `GET /api/usage/events` | Paginated request-level usage events with query filters. |
| `GET /api/usage/events/filter-options` | Dimension values available for filtering. |
| `GET /api/usage/events/export` | Export filtered events. |
| `POST /api/usage/events/request-log-download-token` + token-gated download | Short-lived tokens guarding request-log downloads (only when `CPA_REQUEST_LOG_ACCESS_ENABLED`). |
| `GET /api/usage/overview` | Aggregated overview for a time window. |
| `GET /api/usage/overview/realtime` | Realtime overview snapshot. |
| `GET /api/usage/activity` | Time-series activity rollups. |
| `GET /api/usage/analysis` | Analysis: trends, cost composition, heatmaps. |
| `GET /api/usage/analysis/latency` | Latency diagnostics. |
| `GET /api/usage/identities`, `GET /api/usage/identities/page` | Usage grouped by identity. |
| `GET /api/api-keys`, `/settings`, `/options`; alias `PUT` | Mirrored CPA API keys, their settings and alias updates. |
| `GET/... /api/auth-files` | Auth Files inventory, details, and management. |
| `GET /api/models/used` | Models observed in usage. |
| `GET /api/pricing` (+`/rules`, `/sync/preview`, `/batch/:model`) | Price catalog, rule CRUD, sync preview, batch/single-model mutation. |
| `GET/POST /api/quota/...` | Quota reads, refresh, auto-refresh settings/cache, raw payload inspection, per-provider and global credit resets. |
| `GET /api/provider-model-gnn` | Provider ↔ model relationship as a Graph Neural Network (GNN), proxied from the CPA `/v0/management/config` endpoint through a whitelisted-field DTO (secrets never parsed — see §7). The GNN exposes current structure and optional learned node/edge state. |
| `GET /api/update/check` | GitHub release check (suppressed for dev builds). |

### 5.3 Viewer routes (API-key session only)

| Method & path | Description |
|---|---|
| `GET /api/key-overview`, `GET /api/key-overview/realtime` | Overview scoped to the viewer's key. |
| `GET /api/key-activity` | Activity scoped to the viewer's key. |

Viewer endpoints are rate-limited per viewer session and expose no other
key's data.

---

## 6. Business Rules

### 6.1 Ingestion pipeline

1. **Source priority.** If `REDIS_QUEUE_ADDR` is configured, Usage Keeper
   prefers a Redis **SUBSCRIBE** source; failing that, it pulls the Redis
   list queue (`cpa:usage:queue`); the HTTP usage-queue pull against
   `CPA_BASE_URL` is the fallback when Redis is not configured.
2. **Durable inbox.** Every received event is first written to
   `redis_usage_inboxes` (at-least-once). A separate process runner consumes
   the inbox into `usage_events`, so ingestion never blocks on analytics
   writes.
3. **Control messages.** Non-usage control messages from CPA are routed to
   the Metadata Sync runner (Auth Files / API Keys / providers changed).
4. **Backoff.** Redis consumption retries with exponential backoff from 1s up
   to 30s on errors.
5. **Inbox retention.** Successful inbox rows are kept for the current day;
   failed rows are retained 7 days for diagnosis.

### 6.2 Aggregation

- A **single serial** aggregation runner advances three independent
  monotonic checkpoints (overview, activity, latency). Serial execution plus
  transactional checkpoint advance (optimistic concurrency on
  name+expected-cursor) guarantees each event is rolled up exactly once even
  after restarts.
- Rollups feed `usage_overview_hourly_stats`,
  `usage_overview_daily_stats`, `usage_activity_stats`, and
  `usage_latency_stats` (including latency sketches for percentile
  diagnostics).

### 6.3 Retention & maintenance

- Daily maintenance window: **04:30 local time**.
- `usage_events` older than **90 local calendar days** are moved to
  `usage_events_archive` (cold, permanent, not queried by APIs).
- Backups: `BACKUP_ENABLED=true` by default; default interval **24h**,
  retention **7 days**; written using a dedicated store on the writer pool.
- Log retention: `LOG_RETENTION_DAYS` (default 7; error logs 30 days).

### 6.4 Pricing & cost

- A default pricing catalog ships with the binary; at boot it is overlaid
  with the persisted snapshot (`ModelPriceSetting`/`ModelPriceRule`).
- Cost is computed from token fields: input, output, cache-read, cache-write,
  and reasoning tokens, multiplied by the resolved per-model rates and admin
  multipliers.
- Pricing syncs from CPA metadata; a preview mode shows the diff before
  applying; batch and single-model mutations are supported.

### 6.5 Quota

- Providers: **claude, codex (incl. codex_header), gemini_cli, kimi, xai,
  antigravity**.
- Manual refresh, scheduled auto-refresh (settings stored in SQLite), raw
  payload inspection, and credit resets (per provider or global).
- Concurrency: `QUOTA_REFRESH_WORKER_LIMIT` (default 10, hard cap 100);
  header-based providers use a snapshot/cache worker.

### 6.6 Ranking (opt-in)

- Disabled unless explicitly enabled. When on, the instance generates an
  **ed25519** identity persisted via `AppSetting` (saved transactionally with
  deleted-state protection).
- Talks to the official center `https://keepers.cc.cd/`
  (`/api/v1/leaderboards`, `/api/v1/leaderboards/metadata`); leaderboard
  reads are cached in memory for 30s.
- **Ranking failures must never affect usage collection.**

### 6.7 Update checks

- Queries the GitHub Releases API (`https://api.github.com`) for the upstream
  repository; suppressed for `dev` builds. Release binaries inject the
  version with
  `-ldflags -X cpa-usage-keeper/internal/version.Version=${VERSION}`.

---

## 7. Security Requirements

- **Sessions.** HttpOnly cookie transport; a per-tab header token is
  supported only as the CPAMC iframe embed fallback. Sessions are in-memory,
  or persisted in SQLite (`auth_sessions`) when password auth is enabled.
- **Login hardening.** Failed-attempt counters and rate limiting on both
  admin and API-key login endpoints; viewer API endpoints are rate-limited
  per viewer session.
- **CSP / embedding.** The `frame-ancestors` CSP is built only from the
  `CPA_PUBLIC_URL` origin; no wildcard framing.
- **TLS.** Optional direct TLS (`TLS_ENABLED`, `TLS_CERT_FILE`,
  `TLS_KEY_FILE`) or termination at a reverse proxy; `TLS_SKIP_VERIFY`
  applies to the outbound CPA client only.
- **Secrets handling.** Management keys and access tokens are redacted in
  logs and API responses (`internal/helper/redact.go`). The
  provider-model-GNN proxy decodes only whitelisted fields from the CPA
  management config (`internal/cpa/dto/providerconfig`); secret-bearing
  fields such as `api-key`, `base-url`, or `headers` are never parsed,
  stored, or forwarded.
- **Password storage.** `LOGIN_PASSWORD` is provided via environment; it is
  never returned by any endpoint.
- **Backup safety.** Backups contain the full database and must be stored
  with filesystem-level protection.

---

## 8. Related Documents

- [ARCHITECTURE.md](ARCHITECTURE.md) — internals, data flow, deployment.
- [CONFIGURATION.md](CONFIGURATION.md) — every env var and CLI flag.
- [ROADMAP.md](ROADMAP.md) — phased future work.
- [CONTRIBUTING.md](CONTRIBUTING.md) — development setup and conventions.
