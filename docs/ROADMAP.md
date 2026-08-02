# CPA Usage Keeper — Roadmap

> The "railmap" for future work: a sequence of **phases**, each broken into
> concrete **steps/parts**. Phases are ordered by value-to-risk; phases can
> overlap, but steps within a phase are roughly sequential. Nothing here is a
> commitment — items may be re-scoped as the project evolves.

Legend: ⬜ not started · 🟡 in progress · ✅ done

---

## Phase 0 — Documentation & Developer Tooling

**Goal:** make the project easy to understand, contribute to, and verify —
the foundation every later phase builds on.

- **0.1 Project documentation set.** ⬜ (this change)
  - SPECIFICATION, ARCHITECTURE, ROADMAP, CONFIGURATION, CONTRIBUTING docs
    under `docs/`, linked from README; `session.md` at the root for session
    durability.
- **0.2 OpenAPI specification.** ⬜
  - Hand-write (or generate) an OpenAPI 3 document for the `/api` surface;
    serve it at `/api/openapi.json`; keep it CI-checked for drift.
- **0.3 Postman/HTTP collections.** ⬜
  - Checked-in request collections for the main flows (auth, overview,
    events, quota, pricing) to speed up manual verification.
- **0.4 E2E smoke suite.** ⬜
  - Playwright (or equivalent) happy-path tests: login → overview renders →
    events table filters → quota page; run in CI against a disposable CPA
    fixture.
- **0.5 Architecture decision records (ADRs).** ⬜
  - Start `docs/adr/` for consequential decisions (single-writer SQLite,
    checkpoint aggregation, archive design, embed transports).

---

## Phase 1 — Reliability & Hardening

**Goal:** boring dependability — tighter failure handling around the parts
of the system that already work.

- **1.1 Transactional write retry policy.** ⬜
  - Bounded retry for SQLITE_BUSY on the writer pool (jittered backoff),
    applied uniformly via a repository-level helper.
- **1.2 Backup restore tooling.** ⬜
  - Documented + scripted restore flow (`make restore FILE=...`), optional
    "restore from backup" affordance in the UI guarded by auth.
- **1.3 Schema rebuild from archive.** ⬜
  - Offline command that rebuilds hot tables/aggregations from
    `usage_events_archive` (the archive already exists for this purpose);
    progress reporting + resume support.
- **1.4 Ingestion dead-letter surfacing.** ⬜
  - Admin UI panel listing failed `redis_usage_inboxes` rows with payload
    preview and one-click replay/discard.
- **1.5 Health & readiness depth.** ⬜
  - Extend `/api/healthz` with component checks (DB write probe, ingest
    source connectivity, last aggregation checkpoint age).
- **1.6 Chaos/recovery test matrix.** ⬜
  - Scripted kill/restart scenarios asserting exactly-once aggregation and
    no inbox loss (documented pass criteria per scenario).

---

## Phase 2 — Analytics & UI/UX Enhancements

**Goal:** deeper insight and a more polished dashboard experience.

- **2.1 Theme system polish.** ⬜
  - Extend the light/dark/auto `data-theme` system with additional theme
    variants (e.g., high-contrast, OLED dark) and per-user persistence.
- **2.2 Advanced analytics visualizations.** ⬜
  - Model/provider share over time, cost-per-identity leaderboards,
    anomaly markers on latency charts, exportable chart images.
- **2.3 Custom dashboard layouts.** ⬜
  - Pin/reorder/hide overview cards; per-admin layout persisted in
    `AppSetting`.
- **2.4 Richer exports.** ⬜
  - CSV/JSON export for overview, analysis, and identity views (not just raw
    events); scheduled export option to WORK_DIR.
- **2.5 Additional i18n locales.** ⬜
  - Community-contributed locales beyond en/zh (plural rules, RTL readiness
    check in the i18n layer).
- **2.6 Accessibility pass.** ⬜
  - Keyboard navigation, focus management in modals, ARIA labels across the
    custom UI kit; automated axe checks in the E2E suite (ties to 0.4).

---

## Phase 3 — Alerts, Metrics & Integrations

**Goal:** Usage Keeper stops being only a place you *look at* and starts
*telling you* when things happen.

- **3.1 Webhook notifications.** ⬜
  - Configurable outbound webhooks (generic JSON + Discord/Telegram
    templates) for: quota threshold crossed, error-rate spike, ingest source
    down, aggregate cost budget exceeded.
- **3.2 Prometheus metrics endpoint.** ⬜
  - `GET /metrics`: ingestion rate, inbox depth, checkpoint lag, roll-up
    durations, quota refresh outcomes, Go runtime metrics.
- **3.3 Alert rule engine.** ⬜
  - Threshold rules over overview/activity stats with hysteresis and
    cooldown, configured in the UI, evaluated in the maintenance runner.
- **3.4 OpenTelemetry tracing (opt-in).** ⬜
  - OTLP exporter configuration; span coverage for ingest→process→aggregate
    and HTTP handlers.
- **3.5 SSO / OAuth2 login.** ⬜
  - Optional external identity providers (OIDC) alongside password auth, with
    role mapping to admin.

---

## Phase 4 — Scale & Platform Growth

**Goal:** prepare for bigger footprints and a broader ecosystem.

- **4.1 Multi-database backend exploration.** ⬜
  - Postgres/MySQL behind the repository seam (analysis first: which stores
    must change, dbresolver mapping, migration strategy); gated by a config
    switch.
- **4.2 Horizontal read scaling.** ⬜
  - Read-replica routing for analytics endpoints; cache layer for hot
    overview queries with explicit invalidation on checkpoint advance.
- **4.3 Plugin system for custom metrics.** ⬜
  - Sandbox mechanism (YAML-defined aggregations or WASM) for user-defined
    rollups surfaced as new dashboard cards.
- **4.4 Quota provider expansion.** ⬜
  - New providers as they appear in the CPA ecosystem; checklist-driven
    provider template (client, normalizer, registry, tests, docs).
- **4.5 Multi-instance awareness.** ⬜
  - Optional federation: several Keeper instances report rollups to a parent
    dashboard (builds on the ranking client plumbing).
- **4.6 Public API token support.** ⬜
  - Scoped, revocable API tokens for programmatic access (distinct from CPA
    API keys), documented in the OpenAPI spec (ties to 0.2).

---

## Maintenance Track (continuous)

Runs alongside every phase; items are pulled as needed rather than ordered.

- Dependency upgrades (Go, Gin, GORM, React, Vite, Chart.js) and security
  patch cadence.
- `make verify` kept green on all supported platforms; CI matrix hygiene.
- README and `docs/*` kept in sync with every feature change (policy in
  [CONTRIBUTING.md](CONTRIBUTING.md)).
- Community support: issue triage, reproduction fixtures, release notes.
