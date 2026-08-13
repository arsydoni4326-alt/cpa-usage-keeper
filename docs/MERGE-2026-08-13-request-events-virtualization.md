# Merge Documentation — Request Events Virtualization, Cursor Pagination & Pricing Timeout Reporting

**Merge date:** 2026-08-13
**Source branch:** `author/main`
**Commits merged (relative to `647fb5c`):** `53ea4ff` → `41977f66`
**Upstream PRs:** [#421](https://github.com/Willxup/cpa-usage-keeper/pull/421) (perf: request-events virtualization + cursor pagination), [#422](https://github.com/Willxup/cpa-usage-keeper/pull/422) (fix: report Models.dev sync timeouts)

> Status: The merge is currently **staged** in the working repository (conflicts resolved, awaiting commit).

---

## 1. Overview

This merge introduces two user-visible and one infrastructure improvement to CPA Usage Keeper:

1. **Request Events table performance rework** — replaces offset-based pagination with **keyset (cursor) pagination** and adds **row virtualization** for large result sets, together with an **infinite/streaming "load more"** interaction.
2. **Request Events reward/export pipeline improvements** — the Request Events view no longer exposes page-size chooser or previous/next page buttons; instead it loads the first page and streams subsequent pages via cursor continuation.
3. **Models.dev pricing-sync timeout reporting** — the backend now distinguishes upstream timeouts (gateway 504) from general errors (500), and the frontend surfaces a specific, actionable message when the Models.dev sync times out.

Both changes are backward-compatible at the API surface level: the old `page`/`page_size` parameters and response pagination fields still exist, but the Request Events flow now prefers `cursor_mode` + `cursor` continuation. No database migration is required.

---

## 2. Backend Changes

### 2.1 Keyset (cursor) pagination for Request Events

**Files:** `internal/api/usage_events.go`, `internal/api/usage_filter.go`, `internal/service/usage.go`, `internal/service/dto/usage.go`, `internal/repository/usage.go`, `internal/repository/dto/usage_events.go`, `internal/repository/dto/usage_query_filter.go`

The previous offset-based pagination (`LIMIT ... OFFSET ...`) performed a full-table `COUNT` on every page and could degrade to deep offset scans for large event tables. This merge adds cursor-based continuation:

- **Client-supplied pagination is unchanged** (`page`, `page_size`); **new optional query parameters** are introduced:
  - `cursor_mode=true` — activates cursor mode.
  - `cursor=<opaque base64url token>` — a continuation cursor encoding `timestamp|id`.
- **Cursor encoding** (`encodeUsageEventsCursor`/`decodeUsageEventsCursor` in `internal/api/usage_filter.go`): the cursor is the base64url-encoded string `RFC3339Nano(timestamp)|<id>`; both values are normalized to storage time. Decoding validates format and rejects invalid cursors.
- **Cursor continuation query** (`internal/repository/usage.go`): when `CursorMode` and `CursorTimestamp` are set, the repository applies:
  ```sql
  WHERE (timestamp < :cursorTimestamp OR (timestamp = :cursorTimestamp AND id < :cursorId))
  ```
  with `ORDER BY timestamp DESC, id DESC`.
- **Efficiency:** In cursor mode, the repository fetches `pageSize + 1` rows to detect "has more" without a `COUNT`, and **skips the `COUNT` query entirely** (returns `-1` for `TotalCount`) to avoid deep pagination full-table scans. `TotalPages` computation is preserved where the count exists.
- **Response additions:** `usageEventsResponse` now includes `next_cursor` (omitted when empty) and `has_more` (boolean). The service DTO `UsageEventsPage` and repository record gain `HasMore`.
- **Semantics:** The first load (no `cursor`) returns the most recent `pageSize` events and, when more remain, `has_more=true` + `next_cursor`. Subsequent loads pass `cursor` to fetch older events. `cursor_mode` forces `Page=1`, `Offset=0`.

### 2.2 Pricing sync timeout classification

**Files:** `internal/api/pricing.go`, `internal/api/test/pricing_test.go`

Previously, any `PreviewPricingSync` failure returned a generic `500 internal server error`, hiding upstream connectivity problems. This merge adds `writePricingSyncPreviewError`, which classifies the error:

- If the error is **`context.DeadlineExceeded`** **or** any error implementing `net.Error` with `Timeout() == true` → returns **`504 Gateway Timeout`** with body `{"error":"Models.dev request timed out"}`, and logs the underlying error.
- Otherwise → falls back to the existing generic `500` handler.

**Tests added:**
- `TestPricingSyncPreviewRouteReturnsGatewayTimeoutForUpstreamTimeout` — `context.DeadlineExceeded` → 504 + time-out body.
- `TestPricingSyncPreviewRouteReturnsGatewayTimeoutForNetworkTimeout` — a stub `net.Error` with `Timeout()==true` → 504 + time-out body.
- `TestPricingSyncPreviewRouteKeepsNonTimeoutErrorsInternal` — a plain decode error → 500 + internal body.

### 2.3 Page-size cleanup

**Files:** `internal/api/usage_filter.go`, related test files

The allowed usage-events page sizes were reduced from `[20, 50, 100, 500, 1000]` to `[20, 50, 100]`, aligning with the new fixed 50-row streaming default and discouraging heavy page sizes now that cursor continuation is the preferred access pattern.

---

## 3. Frontend Changes (React + TypeScript)

### 3.1 Infinite-scroll / "Load more" Request Events panel

**Files:** `web/src/components/usage/RequestEventsDetailsCard.tsx`, `web/src/pages/UsagePage.tsx`, `web/src/pages/UsagePage.module.scss`, `web/src/lib/api.ts`, `web/src/lib/types.ts`

- **Removed:** the page-size chooser, previous/next page buttons, and the `page`/`pageSize`/`totalPages` props from `RequestEventsDetailsCard`.
- **Added:** `hasMore`, `loadingMore`, `autoLoadMore`, `onLoadMore`. The panel now shows:
  - `Loaded {loaded} / {total}` (with `aria-live="polite"`).
  - A **"Load more"** button (shown when `hasMore`) that calls `onLoadMore`; disabled while loading.
- **Auto-load-more:** the table scroll container triggers `onLoadMore` when the scroll position is within a `1200px` threshold of the bottom (`shouldLoadMoreRequestEvents`), gated by `autoLoadMore`, `hasMore`, and not already loading. An effect re-checks on mount and when the loaded row count changes.
- **Appending logic (`appendUniqueUsageEvents`):** incoming pages are merged by event `id`, deduplicating against already-loaded rows.
- **Error handling (`handleUsageEventLoadMoreError`):** pauses auto-load, recovers range-bounds conflicts, and routes 401 to `onAuthRequired`.

### 3.2 Row virtualization

**Files:** `web/src/components/usage/RequestEventsDetailsCard.tsx`, `web/src/pages/UsagePage.module.scss`

- When there are **> 50 rows**, the table switches to **TanStack Virtual** row virtualization:
  - Constant `REQUEST_EVENT_VIRTUAL_ROW_HEIGHT = 44`, `overscan = 8`, `initial viewport height = 760`.
  - Memoized `RequestEventTableRow` component with `measureElement` refs and `data-index`/`aria-rowindex` attributes.
  - Virtual spacer `<tr>` rows (`requestEventsVirtualSpacerRow`, `pointer-events: none`) provide top/bottom padding.
  - `useLayoutEffect` re-measures when visible columns change; `useAnimationFrameWithResizeObserver` smooths measurement.
- **ARIA:** the table sets `aria-rowcount={totalCount + 1}` and each virtual row exposes `aria-rowindex`.
- A **performance micro-optimization**: token/cost/latency label strings are precomputed once per row (`latencyLabel`, `ttftLabel`, `speedLabel`, `*TokensLabel`, `costLabel`) and reused across renders, avoiding repeated `Intl` formatting during virtualization.

### 3.3 USD & duration formatting / caching

**Files:** `web/src/utils/usage.ts`, `web/src/utils/usage/latency.ts`

- `formatUsd` now uses **two pre-created `Intl.NumberFormat` instances** (precise 4-decimals for `< $1`, standard 2-decimals otherwise) instead of constructing a formatter per call — important now that cost labels are rendered by the virtualizer.
- `formatDurationNumber` now **caches `Intl.NumberFormat` instances** by `(locale, options)` key in a module-level `Map`, avoiding repeated formatter construction during virtualization.

### 3.4 Pricing sync timeout UX

**Files:** `web/src/components/usage/PriceSettingsCard.tsx`, `web/src/i18n/index.ts`

- `notifyPricingSyncUnexpectedError` checks for `ApiError` with **status 504** and shows the localized message `model_price_sync_timeout` ("Models.dev connection timed out...") instead of the generic sync-failure message.
- New i18n keys added in **all three locales (en / zh / zh-TW)**:
  - `usage_stats.model_price_sync_timeout`
  - `usage_stats.request_events_load_more`
  - `usage_stats.request_events_loaded_count`
  - (legacy `request_events_page_*` strings retained for backward compatibility)

### 3.5 UsagePage state rework

**Files:** `web/src/pages/UsagePage.tsx`, `web/src/pages/UsagePage.module.scss`, `web/src/pages/test/UsagePage.logic.test.ts`

- Replaced `eventsPageSize`/`eventsTotalPages` state with `eventsNextCursor` + derived `eventsHasMore`, plus `eventsLoadingMore` and `eventsAutoLoadMore`.
- The initial load uses `pageSize: 50` + `cursorMode: true`; `loadMoreEvents` passes the stored `cursor`.
- `RequestEventsPreferences` no longer persists `pageSize`; the saved-preference version and normalizer drop the page-size field.
- Added/updated tests covering the new cursor appending, load-more error handling, and virtualization behavior (`web/src/components/usage/test/RequestEventsDetailsCardVirtualization.test.tsx`).

---

## 4. Test Coverage Added

| Area | Files |
| --- | --- |
| Backend pricing timeout classification | `internal/api/test/pricing_test.go` |
| Backend usage filter / cursor parsing | `internal/api/usage_filter_test.go`, `internal/api/test/usage_events_test.go` |
| Backend repository cursor continuation | `internal/repository/usage_events_test.go` |
| Frontend virtualization | `web/src/components/usage/test/RequestEventsDetailsCardVirtualization.test.tsx` (new), consolidated `RequestEventsDetailsCard.test.tsx`, `RequestEventsColumnSettings.test.tsx`, `RequestEventsDetailsCard{CacheTokens,ClientMetadata,ModelAlias,RequestLog,SpeedMode}.test.tsx` |
| Frontend logic | `web/src/pages/test/UsagePage.logic.test.ts`, `web/src/lib/test/api.test.ts`, `web/src/components/usage/test/PriceSettingsCard.test.tsx` |

---

## 5. Verification

Before committing/merging, run:

```bash
make verify
```

or individually:

```bash
go test ./cmd/... ./internal/...
npm --prefix ./web run test
npm --prefix ./web run lint
npm --prefix ./web run typecheck
npm --prefix ./web run build
```

---

## 6. Backward Compatibility & Migration Notes

- **No schema change** — no SQLite migration required.
- The **HTTP API remains backward compatible**: `page`/`page_size` still work for callers that continue using offset pagination; cursor parameters are additive.
- New response fields (`next_cursor`, `has_more`) are additive.
- The frontend no longer exposes page-size selection or prev/next controls for Request Events; if third-party clients relied on the removed saved `pageSize` preference, that's a UI-only change (the preference is no longer persisted).
- Page-size validation now caps at 100 (500/1000 no longer allowed).

---

## 7. Future Work Recommendations

The following are recommended follow-ups surfaced by this merge and the author's own analysis, for consideration in `docs/ROADMAP.md`:

1. **Cursor-pagination generalization** — The keyset pattern is currently specific to usage events. Consider exposing the repository cursor-continuation primitive (`timestamp|id`) as a reusable helper for other high-volume lists (e.g., request logs, ranking, auth-file activity) to avoid deep-offset scans there too.
2. **`next_cursor` + WebSocket/SSE streaming** — The frontend already streams via repeated cursor requests. Evaluate whether an SSE or WebSocket push for new events (combining the auto-load-more with real-time inbox events) could eliminate polling from the request-events panel.
3. **Server-side cursor validation hardening** — Cursors are opaque but user-suppliable. Add unit coverage for malformed/base64-invalid cursors and confirm the normalization path (`NormalizeStorageTime`) behaves correctly for timezone/collation edge cases across stored rows.
4. **Metrics for page-size reduction** — Because 500/1000 pages are no longer allowed, document the recommended max page size for API consumers and add a test pinning the allowed set to prevent silent regressions.
5. **Virtualization accessibility audit** — The virtualizer introduces spacer rows and `aria-rowindex`. Run an accessibility review of keyboard navigation and screen-reader row counts on large Request Events tables (the `aria-rowcount` change from `totalCount+1` should be validated against the actual header + data rows).
6. **Deduplication semantics for live updates** — `appendUniqueUsageEvents` dedupes by `id`. If events are ever retracted/tombstoned, define how cursor continuation interacts with tombstone rows to avoid duplicate/overlap in infinite scroll.
7. **Pricing timeout parity** — The backend classifies timeout errors from Models.dev sync preview; verify the same error classification is applied to other upstream calls (e.g., the full sync / batch apply path) and that the frontend shows the 504-specific message everywhere the sync can fail.
8. **`Intl` formatter caching correctness** — Module-level `Intl.NumberFormat` caches assume stable locale/configuration for the process lifetime. If runtime locale switching is ever supported, clear the caches on locale change (as the i18n layer does for resolved language).

---

## 8. Related Prior Context

- Prior merge `647fb5c` (PR #417) introduced the **capacity benchmark suite** (`internal/benchmark/`) — a test-only, non-runtime addition. The current merge is independent of it, though both affect Request Events and Dashboard latency; the new cursor pagination is expected to reduce the deep-offset scans the benchmark measured for large `usage_events` tables.
- The 90-day custom day-range cap (`usageEventsCustomDayRangeMaxDays = 90`) and the request-events 50-row default are retained from earlier work; this merge keeps both intact.