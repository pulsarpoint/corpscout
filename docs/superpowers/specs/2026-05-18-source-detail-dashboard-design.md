# Source Detail Dashboard Design

## Goal

Replace the existing minimal `sources_.$name.tsx` page with a four-tab dashboard that gives operators full visibility and control over each data source: its schedule, editable configuration, pull run history, and the raw input queue with per-row inspection.

## Architecture

Extend the existing source detail route (`ui/app/routes/sources_.$name.tsx`) into a tabbed page. All reads use PostgREST views via the existing `/api/v1/db/*` proxy. Mutations go through the scheduler's Go API. The scheduler's periodic enqueue logic gains a `schedule_enabled` gate so automated runs can be paused without disabling the source entirely.

## Tech Stack

- **UI**: React Router v7, shadcn/ui (Tabs, Sheet, Badge, Button, Input, Textarea), TanStack Table, sonner toasts
- **Reads**: PostgREST via `/api/v1/db/v_source_raw_inputs` and `/api/v1/pull-runs`
- **Writes**: scheduler Go API (`PATCH /api/v1/sources/{name}`, two new POST endpoints)
- **DB**: PostgreSQL migrations + sqlc regeneration

---

## Section 1 — Data Model

### Migration 1: `data_sources.config`

```sql
ALTER TABLE data_sources ADD COLUMN config JSONB NOT NULL DEFAULT '{}';
```

Stores editable non-secret configuration per source: `base_url`, `rate_limit_rps`, `page_size`, and any other source-specific parameters. API keys, tokens, and secrets are never stored here — they remain as environment variables on the crawler. The `PATCH` handler rejects any config key matching the pattern `key|secret|token|password` with HTTP 422.

### Migration 2: `data_sources.schedule_enabled`

```sql
ALTER TABLE data_sources ADD COLUMN schedule_enabled BOOLEAN NOT NULL DEFAULT TRUE;
```

Controls whether automated timed runs are enqueued. When `false`, the scheduler skips enqueuing the next periodic job but the source remains fully operational — manual triggers still work and the processor still runs. Independent of the `enabled` column (which gates all activity for the source).

### Migration 3: `v_source_raw_inputs` view

A unified view that UNIONs all three raw input tables with a common set of columns. Used by the Raw Inputs tab via PostgREST.

```sql
CREATE OR REPLACE VIEW v_source_raw_inputs AS
  SELECT
    id,
    'gleif'            AS source_name,
    lei                AS source_native_id,
    processing_status,
    attempts,
    processing_error,
    first_seen_at,
    last_seen_at,
    payload_hash,
    raw_payload,
    EXISTS (
      SELECT 1 FROM suggestion_source_links ssl
      WHERE ssl.source_input_table = 'gleif_company_raw_inputs'
        AND ssl.source_input_key   = id::text
    ) AS has_suggestion
  FROM gleif_company_raw_inputs

  UNION ALL

  SELECT
    id,
    'companies_house'  AS source_name,
    company_number     AS source_native_id,
    processing_status,
    attempts,
    processing_error,
    first_seen_at,
    last_seen_at,
    payload_hash,
    raw_payload,
    EXISTS (
      SELECT 1 FROM suggestion_source_links ssl
      WHERE ssl.source_input_table = 'companies_house_company_raw_inputs'
        AND ssl.source_input_key   = id::text
    ) AS has_suggestion
  FROM companies_house_company_raw_inputs

  UNION ALL

  SELECT
    id,
    'brreg'            AS source_name,
    organization_number AS source_native_id,
    processing_status,
    attempts,
    processing_error,
    first_seen_at,
    last_seen_at,
    payload_hash,
    raw_payload,
    EXISTS (
      SELECT 1 FROM suggestion_source_links ssl
      WHERE ssl.source_input_table = 'brreg_company_raw_inputs'
        AND ssl.source_input_key   = id::text
    ) AS has_suggestion
  FROM brreg_company_raw_inputs;
```

**Status groups for filtering:**
- `live`: `processing_status IN ('pending', 'processing', 'failed')`
- `archive`: `processing_status IN ('processed', 'ignored', 'superseded')`

---

## Section 2 — API Layer

### Extended: `PATCH /api/v1/sources/{name}`

Accepts a JSON body with any combination of these optional fields:

```json
{
  "enabled": true,
  "schedule_enabled": false,
  "schedule_kind": "periodic",
  "schedule_expression": "every 12h",
  "config": { "base_url": "https://api.gleif.org/api/v1", "rate_limit_rps": 5 }
}
```

All fields are optional (pointer types in Go). Config is merged into the existing JSONB (not replaced). Any config key matching the regex `(?i)(key|secret|token|password)` is rejected with HTTP 422 and a descriptive error.

The scheduler's periodic enqueue logic reads `schedule_enabled` before scheduling the next run. If `false`, it skips enqueuing without marking the source as failed.

### New: `POST /api/v1/sources/{name}/raw-inputs/{id}/retry`

Resets a raw input row back to `pending` so it will be claimed on the next processor run. The handler resolves the correct underlying table from `source_name`. Only rows with `processing_status = 'failed'` or `'ignored'` can be retried; other statuses return HTTP 422.

```json
// response
{ "status": "retried" }
```

### New: `POST /api/v1/sources/{name}/raw-inputs/{id}/ignore`

Sets `processing_status = 'ignored'` on the row. Only `pending` and `failed` rows can be ignored; other statuses return HTTP 422.

```json
// response
{ "status": "ignored" }
```

### Existing (unchanged): pull run history

`GET /api/v1/pull-runs?source={name}&page=1&limit=20` — already exists, used by the Logs tab.

---

## Section 3 — UI Structure

### Route

`ui/app/routes/sources_.$name.tsx` — extended from its current minimal state into a full tabbed dashboard. The existing source detail page is replaced entirely.

### Page header

Always visible above the tabs:
- Source name (large) + adapter type + `enabled` status badge
- The enabled/schedule status is also reflected in the Schedule tab header row

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

**Two status banner rows** at the top of the tab (not the page header):

- Row 1 (green / red): `● Source enabled` with **Disable source** button, or `⊘ Source disabled` with **Enable source** button
- Row 2 (blue / amber): `⏱ Schedule active — runs every Xh` with **Pause schedule** button, or `⏸ Schedule paused` with **Resume schedule** button. Paused state includes a note: "Manual trigger still works."

**Schedule configuration card** (full width):
- Editable `schedule_kind` input (text)
- Editable `schedule_expression` input (text)
- Save / Reset buttons
- Helper text: supported formats (`every Nh`, cron `* * * * *`)
- Changes take effect on the next scheduled enqueue

**Next run card**: countdown + absolute UTC timestamp + Skip next run + Cancel queued job buttons

**Last run card**: result (succeeded / failed), timestamp, rows seen/new/updated

**Manual trigger card**: "Run immediately regardless of schedule state" + Trigger now button

### Config tab

Renders the current `data_sources.config` JSONB as a list of editable key-value rows. Each key and value is an input field. Buttons: Add field, Save, Reset. Secret-looking keys are highlighted with an error state and blocked on save. Empty config shows a placeholder with an Add field prompt.

### Logs tab

TanStack table with columns: Started At, Duration, Rows Seen, New, Updated, Unchanged, Result (success badge / error message). Paginated (20 per page), newest first. Data from `GET /api/v1/pull-runs?source={name}`.

### Raw Inputs tab

**Sub-tabs**: Live Queue (badge with count of pending+processing+failed rows) | Archive

**Table columns**: Status (coloured badge) · Native ID · First Seen · Attempts · Error (truncated) · Suggestion (✓ / —)

Data from PostgREST: `/api/v1/db/v_source_raw_inputs?source_name=eq.{name}&processing_status=in.(pending,processing,failed)` for live, `in.(processed,ignored,superseded)` for archive. Paginated (50 per page).

**Sheet drawer** (2× standard width, opens on row click):

- Header: native ID, table name, status badge, attempts count
- Action buttons (failed/ignored rows only): **Retry** (POST retry endpoint) + **Ignore** (POST ignore endpoint)
- Metadata cards: First Seen, Last Seen, Payload Hash, Suggestion (linked UUID or "None produced")
- Last Error block (red background, monospace, only shown if `processing_error` is set)
- Raw Payload section: syntax-highlighted JSON rendered from `raw_payload` JSONB

After Retry or Ignore, the sheet closes and the table refreshes.

---

## Error Handling

- All mutation toasts: success (green) and error (red) via sonner
- Config save with rejected secret keys: inline error on the field, no toast
- Retry/Ignore on wrong-status row: 422 response → red toast "Cannot retry a processed row"
- Sheet drawer load: if PostgREST returns an error, show error state inside the drawer (not a page crash)

---

## Security Constraint

Config keys matching `key|secret|token|password` (case-insensitive) are rejected at the API layer with HTTP 422. The UI highlights these keys in red before the user even saves. Raw payloads are displayed read-only; they must never contain credentials (enforced by the existing ingestion constraint: secrets are never written to raw inputs).
