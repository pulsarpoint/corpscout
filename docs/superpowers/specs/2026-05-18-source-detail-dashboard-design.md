# Source Detail Dashboard Design

## Goal

Replace the existing minimal `sources_.$name.tsx` page with a four-tab dashboard that gives operators full visibility and control over each data source: its schedule, editable configuration, pull run history, and the raw input queue with per-row inspection.

## Architecture

Extend the existing source detail route (`ui/app/routes/sources_.$name.tsx`) into a tabbed page. All reads use PostgREST views via the existing `/api/v1/db/*` proxy. Mutations go through the scheduler's Go API. The scheduler's periodic enqueue logic gains a `schedule_enabled` gate so automated runs can be paused without disabling the source entirely.

## Tech Stack

- **UI**: React Router v7, shadcn/ui (Tabs, Sheet, Badge, Button, Input), TanStack Table, sonner toasts
- **Reads**: PostgREST via `/api/v1/db/v_source_raw_inputs`, `/api/v1/db/suggestion_source_links`, and `/api/v1/pull-runs`
- **Writes**: scheduler Go API (`PATCH /api/v1/sources/{name}`, two new POST endpoints)
- **DB**: PostgreSQL migrations + sqlc regeneration

---

## Section 1 — Data Model

### Migration 1: `data_sources.schedule_enabled`

`data_sources.config JSONB` already exists (added in `000017_source_ingestion_mvp.up.sql`). No config migration is needed — the API and UI add support for the existing column.

The only new column is:

```sql
ALTER TABLE data_sources ADD COLUMN schedule_enabled BOOLEAN NOT NULL DEFAULT TRUE;
```

Controls whether automated timed runs are enqueued. When `false`, the scheduler skips enqueuing the next periodic job but the source remains fully operational — manual triggers still work and the processor still runs. Independent of the `enabled` column (which gates all source activity).

### Migration 2: `v_source_raw_inputs` view

A unified view that UNIONs all three raw input tables (GLEIF, Companies House, Brreg) with a common set of columns. **Does not include `raw_payload`** — the list view only needs metadata; the Sheet drawer fetches full payload separately via PostgREST on the base table.

```sql
CREATE OR REPLACE VIEW v_source_raw_inputs AS
  SELECT
    id,
    'gleif'              AS source_name,
    'gleif_company_raw_inputs' AS source_input_table,
    lei                  AS source_native_id,
    processing_status,
    processing_attempts,
    processing_error,
    first_seen_at,
    last_seen_at,
    payload_hash,
    EXISTS (
      SELECT 1 FROM suggestion_source_links ssl
      WHERE ssl.source_input_table = 'gleif_company_raw_inputs'
        AND ssl.source_input_key   = id::text
    ) AS has_suggestion
  FROM gleif_company_raw_inputs

  UNION ALL

  SELECT
    id,
    'companies_house'    AS source_name,
    'companies_house_company_raw_inputs' AS source_input_table,
    company_number       AS source_native_id,
    processing_status,
    processing_attempts,
    processing_error,
    first_seen_at,
    last_seen_at,
    payload_hash,
    EXISTS (
      SELECT 1 FROM suggestion_source_links ssl
      WHERE ssl.source_input_table = 'companies_house_company_raw_inputs'
        AND ssl.source_input_key   = id::text
    ) AS has_suggestion
  FROM companies_house_company_raw_inputs

  UNION ALL

  SELECT
    id,
    'brreg'              AS source_name,
    'brreg_company_raw_inputs' AS source_input_table,
    organization_number  AS source_native_id,
    processing_status,
    processing_attempts,
    processing_error,
    first_seen_at,
    last_seen_at,
    payload_hash,
    EXISTS (
      SELECT 1 FROM suggestion_source_links ssl
      WHERE ssl.source_input_table = 'brreg_company_raw_inputs'
        AND ssl.source_input_key   = id::text
    ) AS has_suggestion
  FROM brreg_company_raw_inputs;
```

**Status groups for filtering:**
- `live`: `processing_status=in.(pending,processing,failed)`
- `archive`: `processing_status=in.(processed,ignored,superseded)`

**Sheet drawer detail**: the drawer queries the base table directly via PostgREST using `source_input_table` + `id` to fetch `raw_payload`. Suggestion links are fetched from `suggestion_source_links` filtered by `source_input_table=eq.{table}&source_input_key=eq.{id}`.

---

## Section 2 — API Layer

### Extended: `PATCH /api/v1/sources/{name}`

Accepts a JSON body with any combination of these optional fields:

```json
{
  "enabled": true,
  "schedule_enabled": false,
  "schedule_kind": "interval",
  "schedule_expression": "24h",
  "config": { "base_url": "https://api.gleif.org/api/v1", "rate_limit_rps": 5 }
}
```

All fields are optional (pointer types in Go). `config` is merged into the existing JSONB (not replaced). Any config key matching the regex `(?i)(key|secret|token|password)` is rejected with HTTP 422.

**Schedule contract for MVP**: only `schedule_kind = "interval"` is executed by the scheduler (`app.go` line 115 — `scheduleOnce` skips non-interval sources). `schedule_expression` must be a valid Go duration string (`24h`, `12h`, `168h`). The UI editor enforces this: one input for the interval value, validated client-side as a Go duration.

The scheduler's `scheduleOnce` function is extended to also check `schedule_enabled`: if `false`, skip enqueuing regardless of interval.

### New: `POST /api/v1/sources/{name}/raw-inputs/{id}/retry`

Resets a raw input row back to `pending` and enqueues a `source_process` River job so the row is picked up immediately without waiting for the next pull. The handler:
1. Resolves the correct underlying table from the source's `input_table_name`
2. Resets `processing_status = 'pending'`, `processing_error = NULL` on the row
3. Inserts a River job: `SourceProcessArgs{SourceName: name}` on the `source_process` queue
4. Only rows with `processing_status IN ('failed', 'ignored')` can be retried; others return HTTP 422

```json
// response
{ "status": "retried" }
```

### New: `POST /api/v1/sources/{name}/raw-inputs/{id}/ignore`

Sets `processing_status = 'ignored'` on the row. Only `pending` and `failed` rows can be ignored; others return HTTP 422.

```json
// response
{ "status": "ignored" }
```

### Existing (unchanged): pull run history

`GET /api/v1/pull-runs?source={name}&page=1&limit=20` — already exists, used by the Logs tab.

---

## Section 3 — UI Structure

### Route

`ui/app/routes/sources_.$name.tsx` — extended from its current minimal state into a full tabbed dashboard.

### Page header

Always visible above the tabs:
- Source name (large) + `source_group` badge + `pull_task_type` label
- `enabled` status badge (green/red)

### Tabs

| Tab | Content |
|---|---|
| **Schedule** | Schedule config editor, status banners, next/last run cards, manual trigger |
| **Config** | JSONB config key-value editor |
| **Logs** | Pull run history table, paginated |
| **Raw Inputs** | Live Queue / Archive sub-tabs, TanStack table, Sheet drawer |

---

## Section 4 — Tab Designs

### Schedule tab

**Two status banner rows** at the top of the tab:

- Row 1 (green / red): `● Source enabled` with **Disable source** button, or `⊘ Source disabled` with **Enable source** button. Calls `PATCH` with `{ "enabled": bool }`.
- Row 2 (blue / amber): `⏱ Schedule active` with **Pause schedule** button, or `⏸ Schedule paused — manual trigger still works` with **Resume schedule** button. Calls `PATCH` with `{ "schedule_enabled": bool }`.

**Schedule configuration card** (full width, only shown when `schedule_kind = "interval"`):
- Single input: interval duration (e.g. `24h`, `12h`). Validated as a Go duration string before save.
- Save / Reset buttons
- Note: "Only interval schedules are supported. Changes take effect on the next scheduler tick (≤5 min)."

**Next run card**: estimated next run time based on `last_started_at + interval`. No skip/cancel controls in MVP (no API contract for queued job cancellation).

**Last run card**: result (succeeded / failed), timestamp, rows seen/new/updated.

**Manual trigger card**: "Run immediately regardless of schedule state. Works even when schedule is paused." + **Trigger now** button (existing `POST /api/v1/sources/{name}/trigger`).

### Config tab

Renders the current `data_sources.config` JSONB as a list of editable key-value rows. Each key and value is an input field. Buttons: Add field, Save, Reset. Keys matching `key|secret|token|password` are highlighted with an inline error and blocked on save (422 from API). Empty config shows a placeholder with an Add field prompt.

### Logs tab

TanStack table sourced from `GET /api/v1/pull-runs?source={name}`. Columns: Started At, Duration, Rows Seen, New, Updated, Unchanged, Result (success badge / error message truncated). Paginated (20 per page), newest first.

### Raw Inputs tab

**Sub-tabs**: Live Queue (badge count) | Archive

**Table columns**: Status (coloured badge) · Native ID · First Seen · Attempts · Error (truncated) · Suggestion (✓ / —)

Data from PostgREST `v_source_raw_inputs` filtered by `source_name` and `processing_status` group. Paginated (50 per page).

**Sheet drawer** (2× standard width, opens on row click):

- Header: native ID, `source_input_table`, status badge, `processing_attempts` count
- Action buttons (failed/ignored rows only): **Retry** → `POST .../retry` + **Ignore** → `POST .../ignore`
- Metadata cards: First Seen, Last Seen, Payload Hash
- Suggestion links section: fetched from `suggestion_source_links?source_input_table=eq.{table}&source_input_key=eq.{id}` — shows `suggestion_table` + `suggestion_id` per link, or "None produced"
- Last Error block (red, monospace, only if `processing_error` is set)
- Raw Payload section: fetched on open from PostgREST base table (`/api/v1/db/{source_input_table}?id=eq.{id}&select=raw_payload`); syntax-highlighted JSON

After Retry or Ignore: sheet closes, table row refreshes.

---

## Error Handling

- All mutation toasts: success (green) and error (red) via sonner
- Config save with rejected secret key: inline field error, save blocked before API call
- Retry/Ignore on wrong-status row: 422 response → red toast with message
- Sheet drawer payload load error: inline error state inside the drawer, not a page crash
- Duration string validation: inline error on the schedule input before save

---

## Security Constraint

Config keys matching `key|secret|token|password` (case-insensitive) are rejected at the API layer with HTTP 422. The UI validates and highlights these before the user saves. Raw payloads are displayed read-only and are fetched only on demand (not in the list view). Payloads must never contain credentials — enforced by the existing ingestion constraint.
