# Merge Documentation — Codex Quota Efficiency History (PR #441)

| Field | Value |
| --- | --- |
| Merge commit | `01c18839` ("Merge remote-tracking branch 'author/main'") |
| Compared against | `c37d48e` (prior author/main merge) |
| Upstream PR | [#441](https://github.com/Willxup/cpa-usage-keeper/pull/441) `feat(quota): add Codex quota efficiency history` |
| Commits merged | `543c63b7` → `8d2fcd9a` (feature + two correctness fixes) |
| DB migration | Yes — `20260820_codex_quota_history` (introduced in a prior merge; consumed by this PR) |
| Breaking API changes | None — additive endpoint |
| Net delta | 24 files, +3,048 / −98 |

---

## 1. Overview

This merge adds a **read-only Codex quota-efficiency history** surface to CPA Usage Keeper. When an admin opens a credential detail drawer for a Codex Auth File, a new **Quota History** tab shows the real quota-window series observed over the past 30 days, with the current active cycle displayed as a dual-axis bar+line chart (Token/1% and USD/1%) and completed cycles listed as detailed historical rows.

The key design principle is **dynamic cost computation at response time**: the persisted quota history tables store only quota observation metadata (remaining percent, window boundaries, timestamps) — never tokens or costs. Every request resolves matching `usage_events` against the current pricing snapshot, ensuring historical cost values always reflect the latest model prices.

---

## 2. New API Surface

### `GET /api/v1/quota/history/:auth_index`

**Access:** admin session required (registered under `adminProtected`).

**Path parameter:**

| Parameter | Required | Description |
| --- | --- | --- |
| `auth_index` | Yes | Stable identity of the selected Codex Auth File (trimmed, non-empty) |

**Query parameters (all optional):**

| Parameter | Type | Description |
| --- | --- | --- |
| `window_role` | `primary` or `secondary` | Select a specific window series; omits auto-selection |
| `window_seconds` | positive integer | Select a window by its real upstream duration in seconds |

**Response (200):**

```json
{
  "generated_at": "2026-08-23T06:30:00+08:00",
  "range_start": "2026-07-24T06:30:00+08:00",
  "windows": [
    {
      "window_role": "primary",
      "window_kind": "five_hour",
      "window_seconds": 18000,
      "has_current_cycle": true,
      "last_observed_at": "2026-08-23T06:20:00+08:00"
    }
  ],
  "selected_window": { "..." : "same shape as window entry" },
  "current_cycle": {
    "id": 42,
    "window_started_at": "2026-08-23T02:00:00+08:00",
    "reset_at": "2026-08-23T07:00:00+08:00",
    "first_observed_at": "2026-08-23T02:05:00+08:00",
    "last_observed_at": "2026-08-23T06:20:00+08:00",
    "usage": { "requests": 47, "successful_requests": 45, "failed_requests": 2, "input_tokens": 384200, "output_tokens": 121500, "reasoning_tokens": 52000, "cache_read_tokens": 98000, "cache_creation_tokens": 12000, "total_tokens": 670100, "total_cost_usd": 4.32, "cost_available": true },
    "transitions": [
      {
        "from_remaining_percent": 82,
        "to_remaining_percent": 81,
        "percentage_points": 1,
        "is_direct": true,
        "interval_started_at": "2026-08-23T04:12:00+08:00",
        "interval_ended_at": "2026-08-23T04:38:00+08:00",
        "usage": { "..." : "interval-scoped totals" },
        "tokens_per_point": 14280.5,
        "cost_per_point": 0.092,
        "cost_per_point_available": true
      }
    ]
  },
  "completed_cycles": [ { "..." : "same cycle shape, last 30 days" } ]
}
```

**Error responses:**

| Status | Condition |
| --- | --- |
| 400 | Empty `auth_index`, invalid `window_seconds`, unsupported identity type, or missing quota provider |
| 404 | `auth_index` not found among active Auth Files |
| 500 | Unexpected repository or pricing error |

---

## 3. Backend Architecture

### 3.1 Request flow

```
Browser → GET /api/v1/quota/history/:auth_index
  → api/quota.go handler (validates params, routes errors)
  → quota.Service.GetCodexQuotaHistory (validates identity + window params)
  → repository.BuildCodexQuotaEfficiencyHistory (the core computation)
      1. Load codex_quota_cycles (parent) for this auth_index in the last 30 days
      2. Derive window series from parent records (role+seconds composite key)
      3. Select window: current active → most-recently observed → explicit filter
      4. Load codex_quota_percent_segments (child) for selected window cycles
      5. Build transition boundaries from adjacent percent observations
      6. Stream usage_events via indexed range scan (auth_index + timestamp + id)
      7. Accumulate per-cycle and per-transition token totals
      8. Price each unique dimension group against the current pricing snapshot
      9. Compute tokens_per_point and cost_per_point for each transition
  → quota.Service maps repository DTO → API response DTO
```

### 3.2 Efficiency calculation semantics

- **Transitions** are derived from consecutive `codex_quota_percent_segments` within a cycle. Each transition captures the left-open, right-closed interval `(previous.first_observed_at, current.first_observed_at]`.
- **Multi-point drops** (e.g., 82% → 79% = 3 percentage points) produce a single averaged transition; intermediate percentages are **not fabricated**. The `is_direct` flag distinguishes single-point samples from averaged ones.
- **Usage matching** streams only the matching `usage_events` for the auth_index + time window, using `INDEXED BY idx_usage_events_auth_index_timestamp_id` to avoid full-table scans.
- **Pricing resolution** aggregates token counts by unique `(api_group_key, model, model_alias, service_tier, response_service_tier, reasoning_effort, endpoint, executor_type)` dimensions, then resolves cost via `pricing.Resolver`. If a group has tokens but the model is unrecognized, `cost_available` is set to `false`.
- **Single pricing snapshot** per response — the resolver is created once from the snapshot loaded at the method entry point, ensuring all cost values within the response are co-consistent.

### 3.3 Correctness fixes (commits `0c2a85ae` and `8d2fcd9a`)

The two post-feature commits addressed:

1. **Cycle boundary alignment** (`0c2a85ae`): corrected the active-cycle detection and transition interval endpoints so events at exact percent-boundary timestamps are attributed to the correct interval, not the next one.
2. **Aggregation and tooltip consistency** (`8d2fcd9a`): rewrote the streaming accumulator to separate cycle-level totals from transition-level totals (preventing double-counting when an event falls in a cycle but outside all transition intervals), added cost-availability handling for transitions where pricing cannot be resolved, and aligned the chart tooltip to display the correct per-interval values.

### 3.4 Schema consumption (no new migration)

This PR consumes the `20260820_codex_quota_history` migration (introduced in a prior merge), which created:

- **`codex_quota_cycles`** — parent table keyed by `(auth_index, window_role, window_seconds, reset_at)` with unique constraint `uniq_codex_quota_cycles_identity`.
- **`codex_quota_percent_segments`** — child table with unique constraint `uniq_codex_quota_percent_segments_cycle_percent`, foreign-keyed to the parent with `ON DELETE RESTRICT`.

The history tables store **only quota observation metadata** (percent, timestamps, window boundaries). No token, cost, or request counts are persisted in these tables — they are computed at read time.

### 3.5 Background history ingestion (pre-existing)

The quota history data is written by a background runner (`runCodexQuotaHistoryRunner`) that batches quota observations from two sources:

1. **Header snapshots** from the quota cache worker (passive, from quota check results).
2. **Active-check observations** from manual/automatic quota refresh tasks (after identity verification).

The runner uses a 10-second flush delay, bounded queue, monotonic-segment enforcement (new observations must never regress), database-tail recovery for cold starts, and best-effort shutdown flushing.

---

## 4. Frontend Changes

### 4.1 Quota History tab in credential drawer

**Files:** `CredentialDetailDrawer.tsx`, `CodexQuotaHistoryPanel.tsx`, `CodexQuotaHistoryPanel.module.scss`

The credential detail drawer (`CredentialDetailDrawer`) conditionally renders a **Quota History** tab when the selected credential is an Auth File whose `identity.type` is `codex`. Tab ordering: Overview → Quota History → Request Events → Errors.

When a non-Codex credential is selected or the drawer closes, state resets to the Overview tab.

### 4.2 CodexQuotaHistoryPanel

**Files:** `CodexQuotaHistoryPanel.tsx`, `CodexQuotaHistoryPanel.module.scss`

The panel renders:

- **Window switcher** (when multiple window series exist): segmented buttons with `aria-pressed`, primary-first, shortest-window-first ordering.
- **Current cycle card**: a Chart.js dual-axis chart:
  - Bar dataset (left axis): `tokens_per_point` per 1% drop. Direct (single-point) bars are blue; averaged bars are amber. Gradient fills via the shared `toUsageChartGradientFill` helper.
  - Line dataset (right axis): `cost_per_point` per 1% drop. Gaps (`spanGaps: false`) appear when cost is unavailable or isolated. Dashed border style.
  - Median summary line in the card header when data exists.
  - **Accessible summary** (`<ul>` with `screenReaderOnly`): full textual listing of all transitions with timestamps, types, and per-point values.
  - Chart legend: direct dot, averaged dot, cost line.
- **Completed cycles list**: cards showing window boundaries, transition rows with change/interval/usage/efficiency columns, and total-cycle summaries.
- **Error and empty states**: with retry button and localized messages.

### 4.3 Theme adaptation

The panel reads `resolvedTheme` from the theme store and passes `isDark` to the shared `getUsageChartTheme(isDark)` helper, ensuring grid lines, tick colors, text colors, and point border colors track the active light/dark theme. Chart animation is disabled for immediate rendering.

### 4.4 Shared chart utility refactor

**Files:** `chartConfig.ts`, `AnalysisPanel.tsx`

The Analysis panel was refactored to consume shared helpers (`buildUsageChartTooltipStyle`, `getUsageChartTheme`, `toUsageChartGradientFill`, `USAGE_CHART_REQUESTS_LINE_COLOR`) previously defined in Analysis-specific code, promoting them to reusable exports in `chartConfig.ts`. This reduces duplication and ensures visual consistency across the Analysis and Quota History charts.

### 4.5 i18n

New keys added in **all three locales** (en / zh / zh-TW):

| Key prefix | Coverage |
| --- | --- |
| `usage_stats.credentials_quota_history_*` | Panel titles, empty states, legend labels, tooltip text, window selectors, cycle boundaries, transition labels |
| `usage_stats.credentials_quota_history_role_primary/secondary` | Window role labels |
| `usage_stats.credentials_quota_history_window_*` | Window kind labels (five_hour, weekly, monthly) |

A dedicated test (`quotaHistoryTranslations.test.ts`) verifies that all translation keys referenced in the component exist in all three locale bundles.

### 4.6 API and type additions

**Files:** `api.ts`, `types.ts`

- `fetchCodexQuotaHistory(authIndex, options, signal)` — `GET /api/quota/history/{authIndex}` with optional query params.
- `FetchCodexQuotaHistoryOptions` — `windowRole?: 'primary' | 'secondary'`, `windowSeconds?: number`.
- Response types: `CodexQuotaHistoryResponse`, `CodexQuotaHistoryWindow`, `CodexQuotaHistoryCycle`, `CodexQuotaHistoryTransition`, `CodexQuotaHistoryUsage`.

---

## 5. Test Coverage

### Backend

| Area | Files | Key tests |
| --- | --- | --- |
| Route validation | `internal/api/quota_test.go` | Forward window selection, map validation/not-found errors |
| Service eligibility | `internal/quota/test/codex_quota_efficiency_test.go` | Selects current real window, rejects unsupported identity, rejects invalid params |
| Repository efficiency | `internal/repository/test/codex_quota_efficiency_test.go` | Window selection, gap handling, overlap handling, out-of-order timestamps, pricing availability, index usage verification |
| History runner | `internal/quota/test/codex_quota_history_runner_test.go` | Duplicate merge, batch sorting, cycle switch, state recovery, queue overflow, timer-based flush, write failure recovery, partial commit recovery |
| Migration | `internal/repository/migration/test/codex_quota_history_test.go` | Fresh database schema, idempotent migration, index verification, FK constraint, column exclusion |
| History writer | `internal/repository/test/codex_quota_history_test.go` | Monotonic enforcement, cycle switch writes, relative upgrade, absolute nearby, validation, thousand duplicates, writer cancel, child rollback |

### Frontend

| Area | Files |
| --- | --- |
| Panel rendering | `CodexQuotaHistoryPanel.test.tsx` — window switching, loading/error/empty states, current cycle chart data, completed cycles, accessibility summary, cost-availability warnings |
| Panel styles | `CodexQuotaHistoryPanel.styles.test.ts` — data-attribute hooks present |
| Drawer integration | `CredentialDetailDrawer.test.tsx` — tab visibility, tab reset on close/non-Codex selection |
| API serialization | `api.test.ts` — URL construction, query parameter handling |
| Translation parity | `quotaHistoryTranslations.test.ts` — all keys exist in en/zh/zh-TW |

---

## 6. Verification

Run the standard baseline:

```bash
make verify
```

Focused suites covering this merge:

```bash
# Backend
go test ./internal/api/... ./internal/quota/... ./internal/repository/...

# Frontend
npm --prefix ./web run test -- CodexQuotaHistoryPanel
npm --prefix ./web run test -- CredentialDetailDrawer
npm --prefix ./web run test -- quotaHistoryTranslations
npm --prefix ./web run lint
npm --prefix ./web run typecheck
```

---

## 7. Backward Compatibility & Operational Notes

- **No breaking changes.** The endpoint is purely additive.
- **No new environment variables.** All tuning is internal to the quota subsystem.
- **No new migration.** The `20260820_codex_quota_history` schema was introduced in a prior merge; this PR only writes to and reads from those tables.
- **Performance:** the efficiency query scans at most 30 days of `usage_events` for a single `auth_index` using an existing composite index. The streaming accumulator avoids loading the full result set into memory.
- **Security:** the endpoint requires an admin session. The query is bounded to the caller-specified `auth_index` — empty values are rejected to prevent cross-account scans. User-Agent and IP data noted in prior merge security notes still apply.
- **Data retention:** existing `usage_events` archival rules (90 days hot → cold archive) continue to apply. Quota history tables grow unbounded with cycle count but are naturally bounded by the refresh rate and the 30-day query window.
- **Background runner:** the history ingestion runner starts automatically when the quota service boots, consuming quota snapshots from header cache and active checks. A bounded queue prevents memory growth under load; overflows are logged and do not block the quota subsystem.

---

## 8. Documentation Sync Status

This merge was documented **after the fact** in a dedicated session (no code changes in this repository beyond documentation). The following permanent docs are updated in this session:

| Document | Change |
| --- | --- |
| `README.md` | Feature bullet added: Codex quota efficiency history |
| `README.zh.md` | Chinese feature bullet added |
| `docs/SPECIFICATION.md` | Endpoint added to §5.2 table, quota history behavior added to §6.5 |
| `docs/ARCHITECTURE.md` | Quota history computation path documented in §2 and §7 |
| `docs/ROADMAP.md` | New roadmap items from this merge's recommendations |
| `session.md` | Merge documented with provenance and decisions |

---

## 9. Future Work Recommendations

Candidates for `docs/ROADMAP.md` triage:

1. **Quota history freshness indicator** — Surface the `generated_at` timestamp and `last_observed_at` in the credential drawer header so operators can tell whether the history data is stale (e.g., no new observations for > 2 hours).
2. **Cost-availability drill-down** — When `cost_available = false` or a transition has `cost_per_point_available = false`, add a tooltip or footer note identifying which pricing dimensions were unresolved (model name, service tier, etc.) and a link to the pricing settings panel.
3. **Configurable history retention** — The 30-day window is currently hardcoded (`codexQuotaHistoryRange`). Consider making it configurable via `AppSetting` or an environment variable to accommodate operators who want longer or shorter retention.
4. **Per-window efficiency comparison** — Allow side-by-side comparison of primary vs. secondary window series to help operators understand the consumption rate difference between window types.
5. **Export quota history** — Add CSV/JSON export for the current window's cycle data, enabling spreadsheet analysis and compliance reporting.
6. **Quota history cache for concurrent requests** — The efficiency computation involves a streaming DB scan and pricing resolution on each request. For dashboards with multiple concurrent viewers, consider a short-lived in-memory cache keyed by `(auth_index, window_role, window_seconds)` with a 60-second TTL.
7. **Claude and other provider quota history** — Extend the quota history framework to Claude and other providers once their quota observation tables are mature, reusing the same cycle/segment/transition model.
8. **Chart accessibility audit** — Verify the Chart.js dual-axis chart is usable with keyboard navigation and screen readers; consider adding a data table toggle or print-friendly export alongside the visual chart.