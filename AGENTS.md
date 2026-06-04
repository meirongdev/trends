# AGENTS.md

Guidance for AI coding agents (and humans) working in this repo. Keep it accurate as the code evolves.

## What this is

`trends` — a trendshift.io-style GitHub trending-repo tracker. A single Go binary runs a cron worker (discover repos → snapshot star/fork/issue metrics → score momentum → materialize rankings) plus a read-only REST API, all over an embedded SQLite database. Product spec: [`spec.md`](./spec.md). A Next.js SSR frontend is planned (M2), not yet built.

## Build / test / run

Dependencies are vendored in the local module cache. **Always build and test offline** — the sandbox usually has no network. **Scope Go package patterns to `./cmd/... ./internal/...`** (NOT `./...`) — the repo contains a `web/` frontend whose `node_modules` ships stray `.go` files that `./...` would otherwise scan:

```bash
GOPROXY=off go build ./cmd/... ./internal/...
GOPROXY=off go test ./cmd/... ./internal/...
GOPROXY=off go vet ./cmd/... ./internal/...
```

Do NOT run `go get` or `go mod tidy` for uncached modules (needs network). Prefer the stdlib; add a third-party dep only when there's a real need.

Run the daemon (cron worker + API):

```bash
GITHUB_TOKENS=ghp_xxx DB_PATH=trends.db go run ./cmd/trends
```

Run a single job once and exit (handy for local checks; exits non-zero on failure):

```bash
RUN_ONCE=discovery GITHUB_TOKENS=ghp_xxx DB_PATH=/tmp/t.db go run ./cmd/trends
RUN_ONCE=snapshot  ... go run ./cmd/trends
RUN_ONCE=score     ... go run ./cmd/trends   # works offline on an empty DB (no-op, exit 0)
```

Env vars: `DB_PATH` (default `trends.db`), `GITHUB_TOKENS` (comma-separated, round-robin; empty = unauthenticated/low quota), `GITHUB_API_BASE_URL`, `GITHUB_GRAPHQL_URL`, `API_LISTEN_ADDR` (default `:8080`), `DISCOVERY_CRON`, `SNAPSHOT_CRON`.

## Architecture (packages)

| Package | Responsibility |
|---|---|
| `internal/config` | Load configuration from environment |
| `internal/store` | SQLite (WAL, single-writer) — embedded migrations, repos, daily snapshots, rankings, read queries |
| `internal/github` | GitHub client: REST search (discovery) + GraphQL `nodes(ids:)` batch (metrics); token rotation, rate-limit/partial-error handling |
| `internal/scoring` | Pure momentum scoring (no DB): EWMA + acceleration + window delta + relative growth, cohort min-max, configurable weights, `RankPeriod` |
| `internal/ingest` | Jobs: `RunDiscovery`, `RunSnapshot` (computes `star_delta`), `RunScoring` (materializes daily/weekly/monthly Top-N) |
| `internal/scheduler` | Thin `robfig/cron/v3` wrapper with graceful drain on stop |
| `internal/api` | Read-only REST API (Go 1.22+ `ServeMux`), 7 endpoints: `GET /healthz`, `/api/v1/trending`, `/api/v1/languages`, `/api/v1/repositories/{id}` (+ `/snapshots`, `/rankings`), `/api/v1/search`. |
| `cmd/trends` | Wires config → store → github → jobs → scheduler + HTTP server |

Dependency directions (no cycles): `github → store`; `scoring` is pure; `ingest → {github, store, scoring}`; `api → store`; `scheduler` standalone; `cmd → all`. Jobs depend on GitHub only through narrow interfaces (`ingest.Discoverer`, `ingest.Fetcher`) so they can be tested with fakes.

## Conventions

- **TDD, always.** Write the failing test first, see it fail, implement the minimum to pass, commit. Small, focused commits — one logical change each.
- **Migrations**: `internal/store/migrations/NNNN_*.sql`, applied in sorted order inside a transaction and tracked in `schema_migrations`. Never edit an already-applied migration — add a new numbered file.
- **SQLite**: opened with WAL + `busy_timeout` + `foreign_keys`, and `SetMaxOpenConns(1)` to serialize writers. Use **portable SQL only** (no Postgres-specific features) to keep the migration path open (spec §12). No array columns — use join tables.
- **Idempotent jobs**: re-running for the same day must be safe (`INSERT … ON CONFLICT` upserts; `ReplaceRankings` deletes-then-inserts per `(period, date)`). The scoring delta baseline uses `StarsBefore(repoID, date)` so same-day re-runs don't zero out `star_delta`.
- **Errors**: return them up; jobs fail fast. Surface silent no-ops (e.g. an update that matched zero rows returns an error).
- **Logging**: `log/slog` JSON to stdout.
- **Commit messages** end with:
  `Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>`

## Testing approach

- Store/job tests open a **temp SQLite DB** per test (`newTestDB(t)` + `t.TempDir()`) — no shared state, migrations applied fresh.
- The GitHub client is tested with `net/http/httptest`; jobs with in-memory fakes implementing the narrow interfaces — **no real network in tests**.
- API handlers are tested via `httptest` against `Server.Routes()`.
- Run everything with `GOPROXY=off`.

## Dev workflow & status

This codebase is built milestone-by-milestone with the superpowers flow: `brainstorming → spec.md → writing-plans → subagent-driven-development → finishing-a-development-branch`. Per-milestone implementation plans live in [`docs/superpowers/plans/`](./docs/superpowers/plans/). The product roadmap is `spec.md` §14.

- **Done (merged to `main`) — MVP + Phase 1 + developer rankings:** M0 (data), M1a (scoring), M1b (read API), M2 (React SPA), M3a (badge), M3b (submission), M3c (topics), M4a (developer rankings: `/api/v1/developers` + `/trending/developers`, live owner-appearance aggregation). **12 API endpoints.** All in one Go binary.
- **Next:** rest of Phase 2 — Yearly view, Insights/Stats, GitHub-trending history archive. Then Phase 3 (live mentions, accounts, sponsorship).

Deferred scoring tuning (fork signal, winsorize, decay, yearly period, per-period weights) is documented in the M1a plan and intentionally not yet implemented.
