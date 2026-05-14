# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this repository is

`corpscout` is a standalone OSINT service that discovers companies registered in world countries and finds their associated internet domains. It is **not** part of the broader PulsarPoint scanning platform (`pulsarprotectpro2`, `pulsarprotectrunner2`, `pulsarprotectproweb`); it owns its own slice of data and runs against its own Postgres database (port `5435`, DB `corpscout`).

It uses a **aggregator-first, domain-pipeline** architecture:
- Company data is ingested from free global aggregators (GLEIF/LEI, OpenCorporates free tier, Wikidata) and purpose-built per-country registry crawlers.
- Domain association is resolved through a multi-signal pipeline in priority order: registry website field → Wikidata official website → certificate transparency (crt.sh org search) → WHOIS org field → search engine fallback.
- A scheduler drives continuous updates, re-running source crawlers and re-checking stale domain associations on a configurable cadence.

## Common commands

All commands are driven by the root `Makefile` and expect `DATABASE_URL` in the environment (see `.env`).

```bash
# Stack
make up            # docker compose up -d --build (postgres + migrate + corpscout)
make down
make logs
make rebuild       # rebuild just the corpscout worker

# Migrations (golang-migrate against $DATABASE_URL)
make migrate-up
make migrate-down  # rolls back one step

# Code generation
make sqlc-generate # reads database/sqlc.yaml, writes to internal/db/gen

# Build
make build         # builds into ./bin/worker (GOWORK=off required — see below)

# Tests
make test          # all packages
GOWORK=off go test ./internal/...
GOWORK=off go test -run TestScheduler ./internal/scheduler
```

### GOWORK=off is deliberate

Every `go build`/`go test` target passes `GOWORK=off`. This repo lives inside the `ppoint/` monorepo which may contain a parent `go.work`. `corpscout` is a **standalone** module (`github.com/pulsarpoint/corpscout`) and must not be pulled into that workspace. Always preserve `GOWORK=off` in any new build or test invocations.

### Module layout

The Go module lives at `corpscout/` (the repo root of this service). Always run Go tooling from here, or use the Makefile.

## Architecture

### Layering (top to bottom)

```
cmd/worker/main.go                    (entrypoint, flag parsing, signals)
    ↓
internal/app                          Server wiring — builds pgx pool + scheduler
    ↓
internal/scheduler                    Job scheduling and worker pool orchestration
    ↓
internal/sources/<name>               Per-source crawlers (GLEIF, OpenCorporates, per-country)
internal/domain/<signal>              Domain resolution signals (certsh, whoisorg, wikidata, search)
    ↓
internal/db/gen                       sqlc-generated code (DO NOT EDIT — use make sqlc-generate)
    ↓
PostgreSQL (pgx/v5)                   Schema in database/migrations, queries in database/queries
```

Supporting packages:
- `internal/config` — env loading. `CORPSCOUT_DATABASE_URL` is preferred; falls back to `DATABASE_URL`.
- `internal/logging` — `log/slog` JSON handler setup (mirrors backoffice-v2 pattern).

### Core interfaces

```go
// Every source crawler implements this.
type SourceAdapter interface {
    Name() string
    Crawl(ctx context.Context, since time.Time) ([]CompanyRecord, error)
}

// Every domain signal implements this.
type DomainSignal interface {
    Name() string
    Confidence() int
    Resolve(ctx context.Context, company CompanyRecord) ([]string, error)
}
```

### Source adapters (pluggable)

Each source adapter implements `SourceAdapter`. Current adapters:
- `sources/gleif` — full LEI (Legal Entity Identifier) database download and delta sync.
- `sources/opencorporates` — OpenCorporates API free tier (rate-limited).
- `sources/wikidata` — SPARQL queries for companies and their `official website (P856)` property.
- `sources/countries/<cc>` — per-country registry crawlers added incrementally (e.g., `uk`, `ee`, `dk`, `no`, `fr`, `nz`).

### Domain resolution pipeline

For each company, domain signals are tried in priority order and stored with confidence and source metadata:

1. Website field from registry/aggregator data
2. Wikidata `P856` official website
3. crt.sh — certificate transparency search by `O=` (organisation) field
4. WHOIS — registrant org name lookup
5. Search engine — DuckDuckGo/Bing as last resort

### Scheduler

The scheduler tracks per-source crawl schedules and per-company domain-resolution freshness. It uses a job table in Postgres (not an in-memory queue) so restarts are safe and progress survives crashes.

## Database workflow

Schema lives in `database/migrations/` (numbered `NNNNNN_<name>.up.sql` / `.down.sql`, applied by `golang-migrate`). Queries for sqlc live in `database/queries/`. `database/sqlc.yaml` points the generator at `../internal/db/gen`.

Workflow when adding a query:

1. Add/modify SQL in `database/queries/<file>.sql` with a `-- name: FooBar :one|:many|:exec` annotation.
2. If schema changes, add a new migration pair under `database/migrations/`.
3. Run `make sqlc-generate`.
4. Consume the generated method from `internal/db/gen.Queries` in the relevant service package.
5. `make migrate-up` to apply locally.

## Error handling

Follow the project-wide Go error-handling conventions from `AGENTS.md` at the monorepo root.

```
repository / external client  →  errors.Wrap(err, "context message")
service / adapter layer        →  wrap and add business context
scheduler / worker boundary    →  log once with slog.Error, do not re-wrap
client response / job result   →  store safe message, never expose stack traces
```

- Use `github.com/cockroachdb/errors` for all error wrapping and stack traces.
- Use `log/slog` (JSON handler via `internal/logging`) for structured logging.
- Do not log the same error in multiple layers — wrap and return upward; log once at the boundary.
- Never log secrets, tokens, passwords, API keys, or sensitive response bodies.
- Never store raw stack traces in job result columns visible to operators.

Example:

```go
// repository layer — wrap only
func (r *gleifRepo) FetchDelta(ctx context.Context, since time.Time) ([]LEIRecord, error) {
    resp, err := r.client.Get(ctx, since)
    if err != nil {
        return nil, errors.Wrap(err, "fetch GLEIF delta")
    }
    ...
}

// scheduler boundary — log once
func (s *Scheduler) runGLEIFJob(ctx context.Context, job Job) {
    records, err := s.gleif.FetchDelta(ctx, job.Since)
    if err != nil {
        slog.Error("GLEIF delta job failed", "job_id", job.ID, "error", err)
        s.markFailed(ctx, job.ID, err.Error())
        return
    }
    ...
}
```

## Environment variables

Copy `.env.example` to `.env`. Key variables:

- `DATABASE_URL` / `CORPSCOUT_DATABASE_URL` — Postgres DSN. Inside Docker the host is `postgres`; locally `localhost:5435`.
- `CORPSCOUT_LISTEN_ADDR` — defaults to `:8090` (metrics/health only, no public API).
- `CORPSCOUT_OPENCORPORATES_API_KEY` — optional; enables higher rate limits on OpenCorporates.
- `CORPSCOUT_GLEIF_DATA_DIR` — local cache directory for GLEIF bulk files (default `./data/gleif`).
- `CORPSCOUT_CRAWL_CONCURRENCY` — number of parallel crawler workers (default `5`).
- `CORPSCOUT_DOMAIN_CONCURRENCY` — number of parallel domain resolution workers (default `10`).
- `CORPSCOUT_CERTSH_RATE_LIMIT` — requests/second to crt.sh (default `2`).
