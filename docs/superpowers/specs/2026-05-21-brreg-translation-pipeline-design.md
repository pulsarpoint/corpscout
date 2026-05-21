# Brreg Norwegian-to-English Translation Pipeline Design

## Goal

Translate Norwegian company data in `brreg_company_raw_inputs` to English before it can be processed into company suggestions or approved. Translation is manually triggered by the operator, either for all pending rows or a selected subset. Untranslated rows are invisible to the processor.

## Architecture

### PullBrreg flow (import only — no processing)

```
PullBrreg Temporal workflow
  → download_brreg_bulk (Python activity)
  → ImportBrregBulk (Go activity)
      writes brreg_company_raw_inputs.raw_payload
      translation_status = 'pending', raw_payload_en = NULL
  → MarkExecutionComplete (Go activity)
      marks temporal_executions row completed
      does NOT enqueue source_process
```

`source_process` is never enqueued by the import pipeline for brreg. Processing only happens after the operator explicitly runs translation.

### Translation flow (operator-triggered)

```
Operator (UI)
  │
  ├─ "Translate All"      → POST /api/v1/sources/brreg/translate
  ├─ "Translate Selected" → POST /api/v1/sources/brreg/translate { "ids": [...] }
  └─ "Translate" (detail) → POST /api/v1/sources/brreg/translate { "ids": [rowId] }
         │
         ▼
  corpscout scheduler
  → triggers TranslateBrregRawInputs Temporal workflow
         │
         ▼
  data-pipelines go-worker (task queue: corpscout-pipelines)
  → TranslateBrregBatch activity (loops with ContinueAsNew)
         │
         ├─ reads/writes brreg_company_raw_inputs (raw_payload_en, translation_status)
         └─ reads/writes translation_cache
```

Translation ends when all claimed rows have a populated `raw_payload_en`. No `source_process` job is enqueued automatically — the operator triggers company processing manually (see § Processing gate below).

### Processing gate (manual, operator-triggered)

When the operator is ready to process translated rows into company suggestions, they trigger `source_process` manually via the UI or directly via the River DB insert. `BrregProcessor` only claims rows where `raw_payload_en IS NOT NULL` — this is the hard gate ensuring Norwegian data never enters the suggestion/approval pipeline.

---

## Database Changes

### Migration: add translation columns to `brreg_company_raw_inputs`

```sql
ALTER TABLE brreg_company_raw_inputs
  ADD COLUMN raw_payload_en              JSONB,
  ADD COLUMN translation_status          TEXT NOT NULL DEFAULT 'pending',
  ADD COLUMN translation_attempts        INT  NOT NULL DEFAULT 0,
  ADD COLUMN translation_error           TEXT,
  ADD COLUMN translation_model           TEXT,
  ADD COLUMN translation_prompt_version  TEXT,
  ADD COLUMN translated_at               TIMESTAMPTZ,
  ADD COLUMN translation_lease_by        TEXT,
  ADD COLUMN translation_lease_until     TIMESTAMPTZ,
  ADD CONSTRAINT chk_brreg_translation_status CHECK (
    translation_status IN ('pending', 'translating', 'translated', 'failed', 'skipped')
  );

CREATE INDEX idx_brreg_raw_translation_status
  ON brreg_company_raw_inputs (translation_status, created_at);

CREATE INDEX idx_brreg_raw_translation_lease
  ON brreg_company_raw_inputs (translation_lease_until)
  WHERE translation_status = 'translating';
```

`translation_status` values:
- `pending` — newly inserted, not yet translated
- `translating` — claimed by an active activity; `translation_lease_by` and `translation_lease_until` are set
- `translated` — `raw_payload_en` is populated; covers both LLM-translated rows and cache-only rows (see § Skipped semantics below)
- `skipped` — record has no translatable fields at all (only structural renames apply); `raw_payload_en` is built without any LLM call or cache lookup
- `failed` — translation attempted and errored; `translation_error` is set

Both `translated` and `skipped` produce a populated `raw_payload_en` and satisfy the `IS NOT NULL` gate for `ClaimPendingBrregRawInputs`.

**Lease and stale-row recovery:** `translation_lease_by` is the Temporal workflow run ID; `translation_lease_until` is `now() + 10 minutes`. The `TranslateBrregBatch` claim step also reclaims stale leases — rows where `translation_status = 'translating' AND translation_lease_until < now()` are treated as `pending` and included in the next claim. This means a crashed activity can never strand rows longer than 10 minutes.

### Migration: `translation_cache` table

```sql
CREATE TABLE translation_cache (
  category           TEXT NOT NULL,
  original_hash      TEXT NOT NULL,
  source_lang        TEXT NOT NULL DEFAULT 'no',
  target_lang        TEXT NOT NULL DEFAULT 'en',
  prompt_version     TEXT NOT NULL,
  model              TEXT NOT NULL,
  original_text      TEXT NOT NULL,
  translated_text    TEXT NOT NULL,
  created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (category, original_hash, source_lang, target_lang, prompt_version, model)
);
```

`original_hash` is `sha256(trim(lower(original_text)))` hex-encoded. `category` is one of: `org_form`, `sector_code`, `industry_code`, `capital_type`, `activity`, `statutory_purpose`, `vat_description`, `other`.

Using `(category, original_hash, prompt_version, model)` as the key means changing the prompt or upgrading the model automatically produces new cache entries without invalidating old ones — old translations remain available as a fallback.

---

## Field Mapping: `raw_payload` → `raw_payload_en`

`raw_payload_en` is a **curated English-normalized payload**. It maps only the fields listed in this section, using English key names and translated values. Fields not listed are omitted. `raw_payload` remains the **authoritative lossless registry record** and is never modified — all Norwegian data is preserved there regardless of translation status.

### Structural rename (no translation, no LLM)

| Norwegian key | English key | Notes |
|---|---|---|
| `organisasjonsnummer` | `organization_number` | string, copy as-is |
| `navn` | `name` | company name, copy as-is — never translate legal names |
| `hjemmeside` | `website` | URL, copy as-is |
| `stiftelsesdato` | `founded_date` | date string, copy as-is |
| `registreringsdatoEnhetsregisteret` | `registered_date` | date string |
| `konkurs` | `is_bankrupt` | boolean |
| `underAvvikling` | `is_under_liquidation` | boolean |
| `underTvangsavviklingEllerTvangsopplosning` | `is_forced_dissolution` | boolean |
| `erIKonsern` | `is_in_group` | boolean |
| `registrertIMvaregisteret` | `in_vat_register` | boolean |
| `registrertIForetaksregisteret` | `in_business_register` | boolean |
| `harRegistrertAntallAnsatte` | `has_registered_employees` | boolean |
| `sisteInnsendteAarsregnskap` | `last_annual_report_year` | string/number |
| `kapital.belop` | `capital.amount` | number |
| `kapital.valuta` | `capital.currency` | ISO code |
| `kapital.antallAksjer` | `capital.shares` | number |
| `forretningsadresse` | `business_address` | see address mapping below |
| `postadresse` | `postal_address` | see address mapping below |

### Address sub-object mapping (structural, no translation)

| Norwegian key | English key |
|---|---|
| `adresse` | `street` |
| `poststed` | `city` |
| `postnummer` | `postal_code` |
| `kommune` | `municipality` |
| `kommunenummer` | `municipality_number` |
| `landkode` | `country_code` |
| `land` | `country` |

### Cached translation (LLM on first occurrence, cache thereafter)

These are finite enum-like values. The first time a value is seen it goes to the LLM; subsequently it's served from `translation_cache`.

| Norwegian field | Cache category | Example |
|---|---|---|
| `organisasjonsform.beskrivelse` | `org_form` | "Enkeltpersonforetak" → "Sole proprietorship" |
| `organisasjonsform.kode` | *(copy as-is)* | "ENK" |
| `institusjonellSektorkode.beskrivelse` | `sector_code` | "Personlig næringsdrivende" → "Self-employed individuals" |
| `institusjonellSektorkode.kode` | *(copy as-is)* | "8200" |
| `naeringskode1/2/3.beskrivelse` | `industry_code` | "Elektrisk installasjonsarbeid" → "Electrical installation work" |
| `naeringskode1/2/3.kode` | *(copy as-is)* | "43.210" |
| `kapital.type` | `capital_type` | "Aksjekapital" → "Share capital" |

### Always LLM (free text, unique per company)

| Norwegian field | English key | Notes |
|---|---|---|
| `aktivitet[]` | `activities[]` | array of activity descriptions |
| `vedtektsfestetFormaal[]` | `statutory_purpose[]` | array of statutory purpose lines |
| `frivilligMvaRegistrertBeskrivelser[]` | `vat_descriptions[]` | array |

If any of these arrays are empty or absent, the field is included as an empty array in `raw_payload_en` — no LLM call needed for that field.

### `raw_payload_en` example output

```json
{
  "organization_number": "831909242",
  "name": "CERI HOLDING AS",
  "website": null,
  "founded_date": "2023-11-01",
  "registered_date": "2024-01-04",
  "is_bankrupt": false,
  "is_under_liquidation": false,
  "is_forced_dissolution": false,
  "is_in_group": false,
  "in_vat_register": false,
  "in_business_register": true,
  "has_registered_employees": false,
  "last_annual_report_year": "2025",
  "organization_form": { "code": "AS", "description": "Limited company" },
  "sector_code": { "code": "2100", "description": "Private limited companies" },
  "industry_code_1": { "code": "00.000", "description": "Unspecified" },
  "industry_code_2": null,
  "industry_code_3": null,
  "activities": ["Holding company."],
  "statutory_purpose": ["Own and manage investments."],
  "vat_descriptions": [],
  "capital": { "amount": 30000.00, "currency": "NOK", "shares": 100, "type": "Share capital" },
  "business_address": {
    "street": ["Storengveien 50D"],
    "city": "STABEKK",
    "municipality": "BÆRUM",
    "municipality_number": "3201",
    "postal_code": "1368",
    "country_code": "NO",
    "country": "Norge"
  },
  "postal_address": null
}
```

---

## Translation Workflow: `TranslateBrregRawInputs`

### Input contract

```go
type TranslateBrregInput struct {
  IDs           []string `json:"ids,omitempty"`     // empty = all pending
  PromptVersion string   `json:"prompt_version"`    // e.g. "v1"
  Model         string   `json:"model"`             // e.g. "qwen3:6b"
  Accumulated   int      `json:"accumulated"`       // rows translated so far (ContinueAsNew)
}

type TranslateBrregBatchResult struct {
  Claimed    int `json:"claimed"`     // rows locked in this batch
  Translated int `json:"translated"`  // rows where raw_payload_en was written (translated + skipped)
  Failed     int `json:"failed"`      // rows where translation_status was set to failed
}
```

### Workflow logic

```
TranslateBrregRawInputs(input):
  pagesThisRun = 0
  loop:
    result = TranslateBrregBatch(input.IDs, input.PromptVersion, input.Model, batchSize=50)
    accumulated += result.Translated
    if result.Claimed == 0: break             // nothing left to work on
    if input.IDs is non-empty: break          // specific IDs → single pass only
    pagesThisRun++
    if pagesThisRun >= 50:
      ContinueAsNew with accumulated updated  // bound history to 50 batches
  // done
```

Exit condition is `claimed == 0`, not `translated == 0`. If a batch is all failures (`claimed=50, translated=0, failed=50`), the workflow continues because there may be more pending rows; it stops only when the claim query returns nothing. This prevents a batch of bad data from masking remaining work.

No `source_process` enqueue — processing is always triggered manually by the operator. No `MarkExecutionComplete` call — this workflow has no corpscout run ID. The `translation_status` column on each row is the progress record.

### `TranslateBrregBatch` activity

1. **Claim rows**:
   - All-pending path: `WHERE translation_status = 'pending' OR (translation_status = 'translating' AND translation_lease_until < now())`
   - Specific-IDs path: `WHERE (translation_status IN ('pending', 'failed') OR (translation_status = 'translating' AND translation_lease_until < now())) AND id = ANY($ids)`

   Both paths reclaim stale leases inline. Use `FOR UPDATE SKIP LOCKED LIMIT 50`. Immediately mark claimed rows `translating`, set `translation_lease_by = <workflow_run_id>`, `translation_lease_until = now() + 10 minutes`, and increment `translation_attempts`. Return `Claimed = len(rows)`; exit early with `Claimed = 0` if the query returns nothing.

2. **Extract translatable strings**: for each row, build two collections:
   - `cached_lookups`: map of `{category → []unique_text}` for enum-like fields
   - `llm_required`: map of `{row_id → []unique_text}` for free-text fields

3. **Load cache**: use the full primary key for all lookups:
   ```sql
   SELECT category, original_hash, translated_text
   FROM translation_cache
   WHERE source_lang = $source_lang
     AND target_lang = $target_lang
     AND prompt_version = $prompt_version
     AND model = $model
     AND (category, original_hash) IN (...)
   ```
   One query for all unique strings in the batch. Cache hits from a different model or prompt version are not used; they will result in a new LLM call and a new cache row keyed to the current (model, prompt_version) pair.

4. **Identify cache misses** per category. Collect all unique strings not in cache for the current (model, prompt_version).

5. **LLM call** (if any misses): send one request per category with cache misses:
   ```json
   {
     "model": "qwen3:6b",
     "messages": [
       { "role": "system", "content": "You are a Norwegian-to-English translator. Return a JSON object where each key is the original Norwegian text and the value is the English translation. Return only the JSON object, no explanation." },
       { "role": "user", "content": "{\"Enkeltpersonforetak\": \"\", \"Aksjeselskap\": \"\"}" }
     ]
   }
   ```
   For free-text fields (`aktivitet`, etc.) send per-row strings grouped together.

6. **Validate LLM response**: parse JSON; assert every returned key is a strict subset of sent keys. Discard any invented keys with a warning log. If JSON is unparseable → fail the activity (Temporal retries).

7. **Build `raw_payload_en`** for each row using: structural renames + cache hits + new LLM translations + originals for any LLM misses (per-text fallback).

8. **Write in one transaction**:
   - Upsert new translations into `translation_cache`
   - Update each row: set `raw_payload_en`, `translation_model`, `translation_prompt_version`, `translation_attempts`, `translated_at`, clear `translation_lease_by` and `translation_lease_until`
   - `translation_status` is set to:
     - `translated` — if the record had any cached or LLM-translated fields (enum or free-text); this includes cache-only rows where no LLM call was needed
     - `skipped` — only if the record contained no cached or LLM fields at all (all fields were structural renames); this is rare in practice
     - `failed` — single-row error path (see error table)

The DB transaction never opens until after all LLM calls complete.

### Error handling

| Scenario | Behaviour |
|---|---|
| LLM returns invalid JSON | Activity fails → Temporal retries (up to configured max attempts) |
| LLM omits a specific key | Keep original Norwegian text for that field; mark row `translated` with fallback |
| LLM call times out | Activity fails → Temporal retries |
| Single row fails to build `raw_payload_en` | Mark that row `failed` with `translation_error`; continue with rest of batch |
| All rows in batch fail | Activity returns `claimed=N, translated=0, failed=N`; workflow continues because `claimed > 0` — it stops only when the next claim returns nothing |
| Stale lease (activity crashed mid-batch) | Next batch's claim query reclaims rows where `translation_lease_until < now()`; rows are reset to `translating` with a new lease |

---

## `MarkExecutionComplete` change

Remove the River `source_process` job insertion from `MarkExecutionComplete` entirely. No source enqueues `source_process` automatically — all company processing is manually triggered by the operator. This is an intentional, global change: it applies to Companies House and any future source as well as brreg.

`MarkCompleteParams` requires no `SkipSourceProcess` flag — the behaviour is uniform.

### Manual process trigger: `POST /api/v1/sources/:name/process`

Since automatic enqueuing is gone, the operator needs a first-class UI action for every source. This endpoint inserts a `source_process` River job for the named source:

Request body: none.

Response:
```json
{ "job_id": 12345, "status": "enqueued" }
```

Returns 409 if a `source_process` job for that source is already `pending` or `running`. Returns 404 if the source name is unknown.

**UI placement:** A **Process** button is added to every source detail page (the existing `sources_.$name.tsx` route), regardless of source type. It is disabled and shows "Processing…" while a job is in flight. For brreg it is also disabled until `translated > 0` — the tooltip reads "Translate rows first" when both `translated == 0` and `pending > 0`.

---

## `ClaimPendingBrregRawInputs` query change

Add `AND raw_payload_en IS NOT NULL` to the existing WHERE clause. This is the only change to the processor — no other logic changes.

---

## API

### `POST /api/v1/sources/brreg/translate`

Request body (optional):
```json
{ "ids": ["uuid1", "uuid2"] }
```

- If `ids` is absent or empty: trigger `TranslateBrregRawInputs` for all pending rows
- If `ids` is present: trigger for those specific rows only

Response:
```json
{ "workflow_id": "translate-brreg-all", "status": "started" }
```

Workflow IDs:
- All-pending: fixed ID `translate-brreg-all` — Temporal rejects a duplicate `StartWorkflow` if one is already running, the API returns 409.
- Specific IDs: ID `translate-brreg-ids-{unix_timestamp}` — always a fresh run, no deduplication needed since the set is bounded and immediately consumed.

The specific-IDs path also accepts `failed` rows, allowing the operator to retry individual failed translations from the detail page.

### `GET /api/v1/sources/brreg/translation-stats`

Response:
```json
{
  "pending": 983241,
  "translating": 50,
  "translated": 12803,
  "skipped": 231,
  "failed": 17,
  "total": 996342
}
```

### Existing raw input detail endpoint

Add `raw_payload_en`, `translation_status`, `translation_attempts`, `translation_error`, `translation_model`, `translation_prompt_version`, `translated_at` to the response for brreg rows.

### Existing raw inputs list endpoint

Add `translation_status` to list response items. Add `translation_status` as an accepted filter query param.

---

## UI Changes

### Raw inputs list page (brreg source only)

**Table additions:**
- New `Translation` column showing a badge: `Translated` (green) / `Pending` (amber) / `Skipped` (muted) / `Failed` (red) / `Translating` (blue spinner)
- Filter dropdown alongside existing status filter: "Translation: All / Pending / Translated / Failed / Skipped"

**Header actions:**
- When `pending > 0`: banner — "N items need translation" with a **Translate All** button
- Row checkboxes: when ≥1 row selected, a **Translate Selected** button appears in the bulk action bar

Clicking either button calls `POST /api/v1/sources/brreg/translate` (with or without IDs), then shows a toast and refreshes the translation stats.

### Raw input detail page (brreg source only)

Below the existing raw payload viewer, add a **Translation** section:

**Stats row**: `translation_status` badge · model · prompt version · attempts · `translated_at` · error message (if failed)

**Payload panels** (two columns side by side):
- Left: **Norwegian (original)** — formatted JSON of `raw_payload`
- Right: **English (translated)** — formatted JSON of `raw_payload_en`, or a placeholder "Not yet translated" with a **Translate** button

The **Translate** button (single row) calls `POST /api/v1/sources/brreg/translate` with `{ "ids": [rowId] }` and polls the row until `translation_status` changes.

### "Move to companies" action (future gating)

When implemented, this action is only enabled for brreg rows where `raw_payload_en IS NOT NULL`. Rows with `translation_status = 'pending'` or `'failed'` show the action as disabled with tooltip "Translate first".

---

## LLM Client (data-pipelines go-worker)

A new `llm` package in `data-pipelines/services/go-worker/llm/client.go` — a lean copy of the corpscout scheduler's `internal/llm/client.go`, adapted for the go-worker's config pattern.

Config via env vars added to `data-pipelines/services/go-worker/.env` (and the server's `.env`):
- `LLM_BASE_URL` (default `http://100.77.62.33:8080`)
- `LLM_MODEL` (default `qwen3:6b`)
- `LLM_PROMPT_VERSION` (default `v1`)

The client exposes one method relevant here:

```go
// TranslateMap sends a map of {Norwegian: ""} to the LLM and returns {Norwegian: English}.
// Returns error if the response is not valid JSON or if any returned key was not in the input.
func (c *Client) TranslateMap(ctx context.Context, category string, inputs map[string]string) (map[string]string, error)
```

---

## Translation Prompt

**Prompt version: `v1`**

```
System:
You are a Norwegian-to-English business translator. You will receive a JSON object where
each key is a Norwegian business term and each value is an empty string. Return the same
JSON object with each value filled in with the accurate English translation. Return only
the JSON object — no explanation, no markdown, no extra keys.

User:
{"Enkeltpersonforetak": "", "Aksjeselskap": "", "Elektrisk installasjonsarbeid": ""}
```

Expected response:
```json
{"Enkeltpersonforetak": "Sole proprietorship", "Aksjeselskap": "Limited company", "Elektrisk installasjonsarbeid": "Electrical installation work"}
```

---

## Testing

| Test | What it verifies |
|---|---|
| `TestBuildRawPayloadEn_KeyMapping` | All Norwegian keys produce correct English keys; no unknown keys in output; omitted Norwegian-only fields are absent |
| `TestBuildRawPayloadEn_LegalNameNotTranslated` | `navn` value is copied verbatim |
| `TestBuildRawPayloadEn_BooleansAndDatesPassThrough` | Booleans, dates, numbers, codes copied as-is |
| `TestExtractTranslatableStrings` | Only `aktivitet`, `vedtektsfestetFormaal`, etc. collected for LLM; enum fields collected for cache |
| `TestTranslateBrregBatch_CacheHitSkipsLLM` | When all strings are in cache (full key match), no LLM call is made; row marked `translated` not `skipped` |
| `TestTranslateBrregBatch_CacheKeyMismatch_TreatedAsMiss` | Cache row for same text but different model/prompt_version is not used |
| `TestTranslateBrregBatch_CacheMissSendsToLLM` | Cache miss strings are sent to LLM; results stored in cache keyed by full (model, prompt_version) |
| `TestTranslateBrregBatch_LLMInventedKeyDiscarded` | Key in LLM response not in input is silently dropped |
| `TestTranslateBrregBatch_InvalidLLMJSONFails` | Non-JSON LLM response causes activity error |
| `TestTranslateBrregBatch_PerTextFallback` | LLM omitting a key keeps original text; row still marked `translated` |
| `TestTranslateBrregBatch_StructuralOnlyRow_MarkedSkipped` | Row with no enum or free-text fields is marked `skipped` |
| `TestTranslateBrregBatch_SetsLease` | Claimed rows have `translation_lease_by` and `translation_lease_until` set |
| `TestTranslateBrregBatch_ReclaimsStaleRows` | Rows with `translating` + expired lease are included in the next claim |
| `TestTranslateBrregBatch_ClearsLeaseOnSuccess` | Written rows have `translation_lease_by = NULL` and `translation_lease_until = NULL` |
| `TestTranslateBrregBatch_ReturnsCounts` | Result struct has correct `claimed`, `translated`, `failed` values |
| `TestClaimPendingBrregRawInputs_ExcludesNullPayloadEn` | Processor query returns zero rows when `raw_payload_en IS NULL` |
| `TestTranslateBrregWorkflow_StopsWhenClaimedIsZero` | Workflow exits cleanly when batch returns `claimed=0` |
| `TestTranslateBrregWorkflow_ContinuesWhenAllBatchFailed` | Workflow continues if `claimed > 0` even when `translated=0, failed=N` |
| `TestTranslateBrregWorkflow_SpecificIDsSinglePass` | IDs-only run does not loop or ContinueAsNew |
| `TestMarkExecutionComplete_NoRiverJobEnqueued` | `MarkExecutionComplete` completes without inserting any River job |
| `TestProcessSourceHandler_EnqueuesJob` | `POST /api/v1/sources/:name/process` inserts a `source_process` River job |
| `TestProcessSourceHandler_409WhenAlreadyRunning` | Returns 409 if a job for that source is already pending or running |
