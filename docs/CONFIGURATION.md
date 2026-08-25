# CPA Usage Keeper — Configuration Reference

All configuration is via environment variables (optionally from a `.env` file
— see `.env.example` — loaded with godotenv; `--env <path>` selects a
different file). CLI flags are listed at the end, followed by the on-disk
layout of `WORK_DIR`.

---

## 1. Core / Connection

| Variable | Default | Description |
|---|---|---|
| `CPA_BASE_URL` | *(required)* | Base URL of the CLIProxyAPI instance to manage/poll. |
| `CPA_MANAGEMENT_KEY` | *(required)* | Management key authenticating against CPA. Treated as a secret; never logged. |
| `CPA_PUBLIC_URL` | unset | Public origin of this Keeper instance. Used to build the `frame-ancestors` CSP for embedding the UI in iframes. Must be a full origin, e.g. `https://keeper.example.com`. |
| `APP_HOST` | all interfaces | HTTP bind address. Overridden by the `--host` CLI flag. |
| `APP_PORT` | `8080` | HTTP listen port. |
| `APP_BASE_PATH` | `/` | Base path the app is mounted under (for reverse proxies). Static asset URLs are rewritten accordingly. |
| `TZ` | `Asia/Shanghai` | Timezone anchoring all calendar-day boundaries (retention, daily stats, maintenance window). tz data is embedded, so it works in minimal containers. |

## 2. CPA Client Behavior

| Variable | Default | Description |
|---|---|---|
| `REQUEST_TIMEOUT` | `30s` | HTTP timeout for calls to CPA (Go duration syntax). |
| `TLS_SKIP_VERIFY` | `false` | Skip TLS certificate verification for the **outbound** CPA client only. Development aid; do not use in production. |
| `CPA_REQUEST_LOG_ACCESS_ENABLED` | `false` | Enable access to CPA request logs (guards the request-log download token endpoints). |

## 3. Ingestion (Redis)

| Variable | Default | Description |
|---|---|---|
| `REDIS_QUEUE_ADDR` | unset | Redis address for queue/pub ingestion. **When unset, Keeper falls back to HTTP polling** of CPA. When set, SUBSCRIBE is preferred, then list pull of `cpa:usage:queue`. |
| `REDIS_QUEUE_TLS` | `false` | Use TLS for the Redis connection. |
| `REDIS_QUEUE_BATCH_SIZE` | `10000` | Max items pulled per Redis batch. |
| `REDIS_QUEUE_IDLE_INTERVAL` | `1s` | Poll interval when the queue is idle. |

## 4. Authentication

| Variable | Default | Description |
|---|---|---|
| `AUTH_ENABLED` | `true` | Require login. When unset, auth defaults to enabled, so `LOGIN_PASSWORD` becomes required (startup fails without it). Setting `AUTH_ENABLED=false` must be explicit — **anyone with network access is then an admin**, use only on trusted networks. |
| `LOGIN_PASSWORD` | unset | Admin password. Required when `AUTH_ENABLED=true` (including the default). Never returned by any endpoint; public example placeholder values are rejected at startup. |
| `AUTH_SESSION_TTL` | `168h` (7 days) | Session lifetime. Persistent sessions are stored in SQLite when auth is enabled. |

## 4a. IP Enrichment (opt-in)

| Variable | Default | Description |
|---|---|---|
| `IP_ENRICHMENT_ENABLED` | `false` | When `true`, the Session Settings panel enriches `login_ip` / `last_seen_ip` with optional privacy-preserving metadata (reverse-DNS hostname, private-network marker). Private/reserved addresses are classified locally and never queried. |
| `IP_ENRICHMENT_TTL` | `24h` | Lifetime of a resolved enrichment value in the in-memory cache. |
| `IP_ENRICHMENT_TIMEOUT` | `2s` | Timeout for a single background reverse-DNS lookup. |

## 5. Quota

| Variable | Default | Description |
|---|---|---|
| `QUOTA_REFRESH_WORKER_LIMIT` | `10` | Max concurrent quota refresh workers (hard cap 100). Auto-refresh schedules and per-provider settings are stored in SQLite (`AppSetting`). |

## 6. Storage / Working Directory

| Variable | Default | Description |
|---|---|---|
| `WORK_DIR` | `./data` | Directory holding the SQLite database, backups, and logs. Persist this (Docker volume / bind mount). |

## 7. Logging

| Variable | Default | Description |
|---|---|---|
| `LOG_LEVEL` | `info` | logrus level (`debug`, `info`, `warn`, `error`, …). |
| `LOG_FILE_ENABLED` | `true` | Also write logs to files under `WORK_DIR`. |
| `LOG_RETENTION_DAYS` | `7` | Days to retain rotated logs. Error logs are kept **30** days. |

## 8. Backups

| Variable | Default | Description |
|---|---|---|
| `BACKUP_ENABLED` | `true` | Scheduled SQLite backups. |
| `BACKUP_INTERVAL` | `24h` | Time between backups. |
| `BACKUP_RETENTION_DAYS` | `7` | Days to retain backup files. |

## 9. TLS (server)

| Variable | Default | Description |
|---|---|---|
| `TLS_ENABLED` | `false` | Serve HTTPS directly (`ListenAndServeTLS`). Usually terminated at a reverse proxy instead. |
| `TLS_CERT_FILE` | unset | Path to the TLS certificate (PEM). |
| `TLS_KEY_FILE` | unset | Path to the TLS private key (PEM). |

## 10. Metadata Sync

| Variable | Default | Description |
|---|---|---|
| `METADATA_SYNC_INTERVAL` | *(see `cfg.MetadataSyncInterval`)* | Interval for syncing Auth Files / API Keys / AI Providers / pricing metadata from CPA (in addition to control-message-triggered syncs). |

> Ranking participation and update checks have no dedicated env vars; ranking
> is opt-in from the UI, and update checks are automatic for release builds
> (suppressed for `dev` builds).

---

## 11. CLI Flags

| Flag | Description |
|---|---|
| `--env <path>` | Path to a `.env` file to load (alternative to `./.env`). |
| `--host <addr>` | Override `APP_HOST` for the HTTP bind address. |
| `-v`, `--version` | Print version and exit. Release builds inject the version via `-ldflags -X cpa-usage-keeper/internal/version.Version=${VERSION}`. |

---

## 12. On-Disk Layout (`WORK_DIR`)

```
<WORK_DIR>/
├── cpa-usage-keeper.db        # SQLite database (writer/reader pools point here)
├── cpa-usage-keeper.db-wal    # SQLite WAL files (when active)
├── cpa-usage-keeper.db-shm
├── backups/                   # scheduled backups (retention: BACKUP_RETENTION_DAYS)
└── logs/                      # file logs when LOG_FILE_ENABLED=true
                               #   error logs retained 30 days, others LOG_RETENTION_DAYS
```

Backups contain the full database — protect `WORK_DIR` at the filesystem
level (see the Security section of the README and
[SPECIFICATION.md](SPECIFICATION.md) §7).

---

## 13. Related Documents

- [README.md](../README.md) — quick start & deployment recipes.
- [SPECIFICATION.md](SPECIFICATION.md) — functional behavior.
- [ARCHITECTURE.md](ARCHITECTURE.md) — internals.
