# Contributing to CPA Usage Keeper

Thanks for your interest! This guide covers setup, the verification baseline,
and the conventions that keep the codebase coherent.

---

## 1. Prerequisites

| Tool | Version |
|---|---|
| Go | 1.26+ (CGO enabled — SQLite via `mattn/go-sqlite3` needs a C toolchain: `gcc`, or MSYS2 on Windows) |
| Node.js | 24+ with npm |
| CLIProxyAPI | a running instance to point the dev server at (or fixtures for tests) |

---

## 2. Run Locally

```bash
# 1. Configure
cp .env.example .env         # set CPA_BASE_URL and CPA_MANAGEMENT_KEY

# 2. Backend (serves API on :8080, embedded SPA if web/dist exists)
go run ./cmd/server/main.go

# 3. Frontend dev server (hot reload; proxies /api → 127.0.0.1:8080)
cd web
npm ci
npm run dev                  # override target with VITE_API_PROXY_TARGET
```

Useful variations:

```bash
go run ./cmd/server/main.go --env ./my.env --host 127.0.0.1
go run ./cmd/server/main.go -v        # print version
```

---

## 3. Verification Baseline

Everything below must pass before opening a PR — it is exactly what CI runs.

```bash
make verify
```

This runs, in order:

```bash
go test ./cmd/... ./internal/...        # backend tests
npm --prefix ./web run test             # frontend tests
npm --prefix ./web run lint             # eslint
npm --prefix ./web run typecheck        # tsc (strict)
npm --prefix ./web run build            # vite production build
```

You can run any of them individually while iterating.

---

## 4. Backend Conventions (Go)

- **Layering.** Handlers in `internal/api` are thin (parse → validate → call
  service → map DTO). Business logic in `internal/service`. All SQL lives in
  `internal/repository` — never above it.
- **Repository pattern for data access.** Define/extend repositories per
  entity family; the router/handlers never see GORM types.
- **Composition over inheritance.** Services and runners are assembled in
  `internal/app/app.go` via constructor injection; keep the wiring order
  documented in [ARCHITECTURE.md](ARCHITECTURE.md) §3 accurate.
- **Entities** are the schema source of truth (`internal/entities`);
  migrations go in `internal/repository/migration`.
- **Single writer.** Writes go through the writer pool; reads use the reader
  pool. Aggregation checkpoint updates must stay inside the caller's
  transaction with the name+expected-cursor optimistic-concurrency check.
- **Failure isolation.** Optional subsystems (ranking, update check, quota)
  must never break ingestion; follow the existing "inert when disabled" and
  failure-tolerant-cache patterns.
- **Redaction.** Never log secrets — route through
  `internal/helper/redact.go`.
- **Tests.** Co-locate `*_test.go`; table-driven where practical. See
  `internal/api/*_test.go` and `internal/app/*_test.go` for patterns.

## 5. Frontend Conventions (`web/`)

- **TypeScript strict** — no `any` without justification; run `typecheck`.
- **No UI framework.** Extend the bespoke kit in `src/components/ui`; don't
  add a component-library dependency without discussion.
- **Styling:** CSS Modules per component; global layers via SCSS
  (dart-sass). Theming through the `data-theme` attribute — components must
  look right in light, dark, and auto.
- **i18n:** all user-facing strings go through the i18next-based `src/i18n`
  (locales `en`, `zh`, and `zh-TW` must all be updated).
- **Charts:** Chart.js via `src/lib/chartjs.ts` registration only.
- **Tests** under `src/test` / `src/components/...test...` with the existing
  runner setup.

## 6. Commits & PRs

- **Conventional Commits** with scopes, e.g.:
  `feat(quota): add kimi provider normalization`,
  `fix(ranking): toolbar overflow on narrow screens`,
  `docs: refresh configuration reference`.
- Keep PRs focused; reference issue/PR numbers in the body.
- Update or add tests covering the change; keep `make verify` green on your
  platform (CI covers the rest of the matrix).

## 7. Documentation Policy

Docs are part of the feature:

- Behavior change → update [SPECIFICATION.md](SPECIFICATION.md).
- Wiring/topology change → update [ARCHITECTURE.md](ARCHITECTURE.md).
- New/changed env var or flag → update [CONFIGURATION.md](CONFIGURATION.md)
  **and** `.env.example`.
- User-visible capability → update `README.md` (and `README.zh.md` if you
  can; otherwise flag it in the PR).
- Planned work → mark/adjust items in [ROADMAP.md](ROADMAP.md).
- Sizable decisions → consider an ADR (see ROADMAP item 0.5).

## 8. Reporting Issues

- Use `/reportbug` in Cline for tool issues; for project issues, open a
  GitHub issue with: version (`-v`), deployment mode (Docker/binary/source),
  relevant `.env` keys (values redacted), logs from `WORK_DIR/logs`, and
  reproduction steps.

---

## 9. Related Documents

- [README.md](../README.md) — project entry point.
- [ARCHITECTURE.md](ARCHITECTURE.md) — module map and data flow.
- [SPECIFICATION.md](SPECIFICATION.md) — functional contract.
- [ROADMAP.md](ROADMAP.md) — where the project is heading.
