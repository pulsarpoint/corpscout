# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this repository is

`corpscout` is a standalone OSINT service that discovers companies registered in world countries and finds their associated internet domains. It is **not** part of the broader PulsarPoint scanning platform; it owns its own PostgreSQL database (port `5435`, DB `corpscout`).

The system is split into three services:

| Service | Language | Responsibility |
|---|---|---|
| `scheduler/` | Go | Job orchestration, data storage, REST API |
| `crawler/` | Python | Crawling (API adapters + Crawl4AI fallback), domain signal pipeline |
| `ui/` | React Router v7 | Data browser + operations dashboard |

The **scheduler** is the only service that writes to PostgreSQL. The **crawler** is stateless — receives requests, returns JSON. The **ui** reads everything through the scheduler's REST API.

## Common commands

```bash
# Full stack
make up            # docker compose up -d --build
make down
make logs

# Scheduler (Go) — run from scheduler/
make build         # GOWORK=off go build → bin/worker
make test          # GOWORK=off go test ./...
make sqlc-generate # regenerate DB code from database/queries/
make migrate-up    # apply migrations via golang-migrate
make migrate-down  # roll back one step

# Crawler (Python) — run from crawler/
pip install -r requirements.txt
uvicorn main:app --reload   # dev server on :8000

# UI (React) — run from ui/
pnpm install
pnpm dev           # dev server on :5173 (use :9999 for browser testing via proxy)
pnpm typecheck
pnpm build
```

### GOWORK=off is deliberate

The scheduler Go module (`github.com/pulsarpoint/corpscout/scheduler`) lives inside the `ppoint/` monorepo, which may have a parent `go.work`. Always pass `GOWORK=off` for any Go build or test invocation, or use the Makefile.

## Architecture

### Layering

```
ui (React Router v7)
    ↓ REST API
scheduler (Go)
    ├── internal/app/           wiring: pgx pool, River client, Chi router
    ├── River (riverqueue/river) job queue backed by Postgres
    ├── internal/workers/       SourceCrawlWorker, DomainResolveWorker
    │       ↓ HTTP
    └── crawler (Python/FastAPI)
            ├── sources/        per-source extraction handlers
            └── domain_resolver.py  domain signal pipeline
    ↓
PostgreSQL
    ├── application schema      (database/migrations/)
    └── River schema            (river_job, river_queue, river_leader)
```

### Core interfaces

```go
// River worker for source crawling
type SourceCrawlArgs struct {
    SourceName string    `json:"source_name"`
    Since      time.Time `json:"since"`
}
func (SourceCrawlArgs) Kind() string { return "source_crawl" }

// River worker for domain resolution
type DomainResolveArgs struct {
    CompanyID string `json:"company_id"`
}
func (DomainResolveArgs) Kind() string { return "domain_resolve" }
```

```python
# Canonical output from every source handler
class CompanyRecord(BaseModel):
    name: str
    country_iso2: str
    registration_number: str | None = None
    lei: str | None = None
    status: str = "active"
    website: str | None = None
    aliases: list[str] = []        # trading names, former names, normalized variants
    raw_data: dict
    snapshot_hash: str             # SHA-256 of raw_data JSON
    # Enrichment fields
    locations: list[CompanyLocation] = []
    phones: list[CompanyPhone] = []
    emails: list[CompanyEmail] = []
    industries: list[str] = []
    founded_year: int | None = None
    employee_estimate: dict = {}
```

### Two crawler paths

Crawl4AI is **not the default**. Each `data_sources` row declares `adapter_type`:

- `api` — direct HTTP with `httpx`, deterministic JSON/XML parsing. Used for all structured sources (GLEIF, Companies House, Brreg, CVR, OpenCorporates, etc.). Fast, cheap, no LLM.
- `crawl4ai` — Playwright + LLM extraction via Crawl4AI. Fallback only for JS-heavy or unstructured HTML sources with no API.

`CRAWLER_OPENAI_API_KEY` is only required when at least one source uses `adapter_type = crawl4ai`.

### Crawler API

```
POST /crawl/{source_name}   { "since": "...", "page": 1 }
     → { "records": [...], "has_more": bool, "total": int }

POST /resolve/domain        { "company_name": "...", "lei": "...", "country": "GB" }
     → { "candidates": [{ "domain": "...", "signal": "...", "confidence": 60, "evidence": {} }] }
```

### Scheduler REST API

```
GET  /api/v1/stats
GET  /api/v1/companies          ?page, limit, country, source, status, q
GET  /api/v1/companies/:id
GET  /api/v1/domains            ?page, limit, min_confidence, signal
GET  /api/v1/countries
GET  /api/v1/sources
PATCH /api/v1/sources/:name     { "enabled": bool, "crawl_interval_hours": int }
POST  /api/v1/sources/:name/trigger
GET  /api/v1/jobs               ?page, limit, status, source
```

## Database workflow

Schema in `database/migrations/`, queries in `database/queries/`, sqlc config in `database/sqlc.yaml`. Generated code written to `scheduler/internal/db/gen/` — do not edit generated files.

Workflow when adding a query:
1. Add SQL with `-- name: FooBar :one|:many|:exec` annotation to `database/queries/`.
2. Add migration pair if schema changes.
3. Run `make sqlc-generate` from `scheduler/`.
4. Consume new method from `scheduler/internal/db/gen.Queries`.
5. `make migrate-up` to apply locally.

## Error handling

Follows the project-wide Go convention from `AGENTS.md` at the monorepo root.

```
crawlerclient / db layer   →  errors.Wrap(err, "context")
River workers              →  log once with slog.Error, return err (River handles retries)
REST handlers              →  log once, return safe JSON error { "error": "..." }
```

- `github.com/cockroachdb/errors` for wrapping and stack traces.
- `log/slog` JSON handler via `internal/logging`.
- River workers are the single logging boundary — source adapters and the crawler client wrap and return, never log.
- Never store stack traces in the database or expose them in API responses.

Example:

```go
// crawlerclient — wrap only
func (c *Client) Crawl(ctx context.Context, source string, since time.Time, page int) (*CrawlResponse, error) {
    resp, err := c.http.Post(ctx, "/crawl/"+source, req)
    if err != nil {
        return nil, errors.Wrap(err, "crawler POST /crawl/"+source)
    }
    return resp, nil
}

// River worker — log once, return error for River to handle retries
func (w *SourceCrawlWorker) Work(ctx context.Context, job *river.Job[SourceCrawlArgs]) error {
    resp, err := w.client.Crawl(ctx, job.Args.SourceName, job.Args.Since, 1)
    if err != nil {
        slog.Error("source crawl failed", "source", job.Args.SourceName, "job_id", job.ID, "error", err)
        return err
    }
    ...
}
```

## Environment variables

### scheduler
- `CORPSCOUT_DATABASE_URL` / `DATABASE_URL` — Postgres DSN. Docker host: `postgres`; locally: `localhost:5435`.
- `CORPSCOUT_LISTEN_ADDR` — defaults to `:8090`.
- `CORPSCOUT_CRAWLER_URL` — defaults to `http://crawler:8000`.
- `CORPSCOUT_CRAWL_CONCURRENCY` — River `source_crawl` MaxWorkers (default `5`).
- `CORPSCOUT_DOMAIN_CONCURRENCY` — River `domain_resolve` MaxWorkers (default `10`).
- `CORPSCOUT_TEMPORAL_HOST` — Temporal gRPC address (e.g. `companycollect:7233`). Required for Temporal-backed sources.
- `CORPSCOUT_TEMPORAL_UI_URL` — Base URL of the Temporal UI shown in the Jobs page (e.g. `http://100.85.212.113:8089`).

### crawler
- `CRAWLER_LISTEN_ADDR` — defaults to `:8000`.
- `CRAWLER_OPENAI_API_KEY` — required for LLM extraction fallback.
- `CRAWLER_LLM_MODEL` — defaults to `gpt-4o-mini`.
- `CRAWLER_CERTSH_RATE_LIMIT` — req/s to crt.sh (default `2`).
- `CRAWLER_OPENCORPORATES_API_KEY` — optional; enables higher rate limits.
- `COMPANIES_HOUSE_API_KEY` — required for Companies House source.

### ui
- `BACKEND_URL` — scheduler base URL for server-side loaders (default `http://localhost:8090`).
- Client-side fetches use relative URLs through the nginx/dev proxy.

---

## Temporal data-pipeline integration

Some sources bypass the legacy River/crawler path and run Temporal workflows instead. The mapping is in `scheduler/internal/workers/data_task.go`:

| Source | Workflow | Country |
|---|---|---|
| `companies_house` | `PullCompaniesHouse` | GB |
| `brreg` | `PullBrreg` | NO |

### Trigger flow

```
POST /api/v1/sources/:name/trigger
  → DataTaskArgs River job  (queue: data_task)
  → DataTaskWorker.Work()
  → Temporal: ExecuteWorkflow(PullCompaniesHouse, ...)
  → temporal_executions row created (status: running)
  → [Temporal workflow runs independently]
  → Go activity WriteRawInputs  → companies_house_company_raw_inputs
  → Go activity MarkExecutionComplete
      → temporal_executions row updated (status: completed)
      → source_process River job enqueued
  → SourceProcessWorker
      → company_suggestions (pending review)
  → User bulk-approves via Review page
      → companies table populated
```

### Data pipeline services (remote, on companycollect)

The Temporal workers run on the `companycollect` server (Tailscale IP `100.85.212.113`). They are **not** part of the corpscout Docker Compose stack.

**SSH access:** `ssh graovic@100.85.212.113`

**Worker locations on server:**
```
/home/graovic/temporal/services/go-worker/    # Go worker (WriteRawInputs, MarkExecutionComplete)
/home/graovic/temporal/services/python-worker/ # Python worker (FetchPage activities)
```

**Worker commands (on server):**
```bash
cd /home/graovic/temporal/services/go-worker
docker compose up -d --build    # start / rebuild
docker compose logs -f          # stream logs

cd /home/graovic/temporal/services/python-worker
docker compose up -d --build
docker compose logs -f
```

**Deploying code changes from local Mac:**
```bash
# Always exclude .env — the server uses localhost, the Mac uses companycollect
rsync -av --exclude='.env' \
  ppoint/data-pipelines/services/go-worker/ \
  graovic@100.85.212.113:/home/graovic/temporal/services/go-worker/

rsync -av --exclude='.env' \
  ppoint/data-pipelines/services/python-worker/ \
  graovic@100.85.212.113:/home/graovic/temporal/services/python-worker/

# Then rebuild on server
ssh graovic@100.85.212.113 "cd /home/graovic/temporal/services/go-worker && docker compose up -d --build"
ssh graovic@100.85.212.113 "cd /home/graovic/temporal/services/python-worker && docker compose up -d --build"
```

**IMPORTANT — .env differences:**

| Variable | Mac (local dev) | Server (companycollect) |
|---|---|---|
| `TEMPORAL_HOST` | `companycollect:7233` | `localhost:7233` |
| `CORPSCOUT_DB_URL` (go-worker) | `...@companycollect:5432/...` | `...@localhost:5432/...` |

The server uses `network_mode: host` so services reach each other via `localhost`. The Mac reaches the server via Tailscale hostname `companycollect`. Never overwrite the server `.env` files with the Mac `.env` files.

### Temporal server (on companycollect)

| Endpoint | Address |
|---|---|
| gRPC (workers connect here) | `100.85.212.113:7233` |
| Web UI | `http://100.85.212.113:8089` |
| Namespace | `corpscout` |
| Task queue (Go worker) | `corpscout-pipelines` |
| Task queue (Python worker) | `corpscout-pipelines-python` |

**Local dev Temporal** (for testing without the server): run `make temporal-up` from `ppoint/data-pipelines/` — starts Temporal + UI on `localhost:7233` / `localhost:8089`.

### Database (on companycollect)

```
Host:     100.85.212.113 (or localhost when on server)
Port:     5432
DB:       corpscout
User:     corpscout
Password: password123
DSN:      postgres://corpscout:password123@100.85.212.113:5432/corpscout?sslmode=disable
```

**Query from Mac:**
```bash
docker run --rm postgres:16-alpine psql \
  "postgres://corpscout:password123@100.85.212.113:5432/corpscout?sslmode=disable" \
  -c "SELECT COUNT(*) FROM companies;"
```

**Apply migrations:**
```bash
# From corpscout/ on Mac — the docker-compose migrate service points to companycollect
docker compose run --rm migrate
```

### Workflow history / ContinueAsNew

Large bulk pulls (Companies House has 5M+ active companies) use `ContinueAsNew` after every 50 pages to prevent Temporal's history size limit from being hit. Each run processes 50 × 100 = 5,000 records, then restarts with a fresh history carrying the cursor and accumulated totals forward. `MarkExecutionComplete` is only called on the final run (when `has_more = false`).

### Raw-input processing flow

After a Temporal workflow completes, `MarkExecutionComplete` automatically enqueues a `source_process` River job. The `SourceProcessWorker` in the scheduler claims pending rows from `*_raw_inputs` tables and converts them into `company_suggestions`. These suggestions require manual (or bulk) approval in the Review page before they appear in the `companies` table.

**Bulk-approve all pending suggestions via API:**
```bash
IDS=$(curl -s http://localhost:8092/api/v1/suggestions/companies/ids | jq -r '.ids')
curl -s -X POST http://localhost:8092/api/v1/suggestions/companies/bulk \
  -H "Content-Type: application/json" \
  -d "{\"ids\": $IDS, \"action\": \"approve\"}"
```
