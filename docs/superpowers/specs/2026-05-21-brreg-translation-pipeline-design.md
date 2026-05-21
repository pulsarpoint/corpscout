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

### Translation + processing flow (operator-triggered)

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
         ├─ reads/writes translation_cache
         │
         ▼
  Local LLM (OpenAI-compatible, POST /v1/chat/completions)
  model: qwen3:6b at http://100.77.62.33:8080
         │
         ▼ (after each successful batch)
  EnqueueSourceProcess Go activity
  → inserts River job: source_process for brreg
         │
         ▼
  BrregProcessor (River worker in scheduler)
  → claims rows WHERE raw_payload_en IS NOT NULL
```

`source_process` is enqueued by the translation workflow after each successful `TranslateBrregBatch` — not once at the end, but after every batch. This means rows become processable incrementally as translation progresses rather than waiting for the entire backlog to finish.

`BrregProcessor` only claims rows where `raw_payload_en IS NOT NULL`. This is the hard gate ensuring Norwegian data never enters the suggestion/approval pipeline.

---

## Database Changes

### Migration: add translation columns to `brreg_company_raw_inputs`

```sql
ALTER TABLE brreg_company_raw_inputs
  ADD COLUMN raw_payload_en         JSONB,
  ADD COLUMN translation_status     TEXT NOT NULL DEFAULT 'pending',
  ADD COLUMN translation_attempts   INT  NOT NULL DEFAULT 0,
  ADD COLUMN translation_error      TEXT,
  ADD COLUMN translation_model      TEXT,
  ADD COLUMN translation_prompt_version TEXT,
  ADD COLUMN translated_at          TIMESTAMPTZ,
  ADD CONSTRAINT chk_brreg_translation_status CHECK (
    translation_status IN ('pending', 'translating', 'translated', 'failed', 'skipped')
  );

CREATE INDEX idx_brreg_raw_translation_status
  ON brreg_company_raw_inputs (translation_status, created_at);
```

`translation_status` values:
- `pending` — newly inserted, not yet translated
- `translating` — claimed by an active activity (lease)
- `translated` — `raw_payload_en` is populated via LLM
- `skipped` — no translatable fields found; `raw_payload_en` was built from structural renames and cache only, no LLM call needed
- `failed` — translation attempted and errored; `translation_error` is set

Both `translated` and `skipped` produce a populated `raw_payload_en` and satisfy the `IS NOT NULL` gate for `ClaimPendingBrregRawInputs`.

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
```

### Workflow logic

```
TranslateBrregRawInputs(input):
  pagesThisRun = 0
  loop:
    result = TranslateBrregBatch(input.IDs, input.PromptVersion, input.Model, batchSize=50)
    accumulated += result.Processed
    if result.Processed > 0:
      EnqueueSourceProcess("brreg")           // make this batch's rows available to processor
    if result.Processed == 0: break           // nothing left
    if input.IDs is non-empty: break          // specific IDs → single pass only
    pagesThisRun++
    if pagesThisRun >= 50:
      ContinueAsNew with accumulated updated  // bound history to 50 batches
  // done
```

`EnqueueSourceProcess` is a lightweight Go activity that inserts a `source_process` River job for brreg. It runs after every batch so the processor can start working on translated rows immediately without waiting for the full backlog to complete. River's `UniqueOpts` on the `source_process` job kind mean concurrent enqueues are collapsed — if a job is already pending or running, the duplicate insert is a no-op.

No `MarkExecutionComplete` call — this workflow has no corpscout run ID. The `translation_status` column on each row is the progress record.

### `TranslateBrregBatch` activity

1. **Claim rows**:
   - All-pending path: `WHERE translation_status = 'pending'`
   - Specific-IDs path: `WHERE (translation_status IN ('pending', 'failed')) AND id = ANY($ids)`
   
   Use `FOR UPDATE SKIP LOCKED LIMIT 50`. Immediately mark claimed rows `translating` and increment `translation_attempts`.

2. **Extract translatable strings**: for each row, build two collections:
   - `cached_lookups`: map of `{category → []unique_text}` for enum-like fields
   - `llm_required`: map of `{row_id → []unique_text}` for free-text fields

3. **Load cache**: `SELECT category, original_hash, translated_text FROM translation_cache WHERE (category, original_hash) IN (...)` — one query for all unique strings in the batch.

4. **Identify cache misses** per category. Collect all unique strings not in cache.

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
   - Update each row: set `raw_payload_en`, `translation_status` (`translated` if LLM was called, `skipped` if no LLM was needed), `translation_model`, `translation_prompt_version`, `translation_attempts`, `translated_at`

The DB transaction never opens until after all LLM calls complete.

### Error handling

| Scenario | Behaviour |
|---|---|
| LLM returns invalid JSON | Activity fails → Temporal retries (up to configured max attempts) |
| LLM omits a specific key | Keep original Norwegian text for that field; mark row `translated` with fallback |
| LLM call times out | Activity fails → Temporal retries |
| Single row fails to build `raw_payload_en` | Mark that row `failed` with `translation_error`; continue with rest of batch |
| All rows in batch fail | Activity returns `processed=0`; workflow continues loop (avoids infinite retry on bad data) |

---

## `MarkExecutionComplete` change

Add a `SkipSourceProcess bool` field to `MarkCompleteParams`. The `PullBrreg` workflow passes `SkipSourceProcess: true`. When set, the activity skips the River `source_process` job insertion entirely.

All other sources continue to work as before — `SkipSourceProcess` defaults to `false`.

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
| `TestBuildRawPayloadEn_KeyMapping` | All Norwegian keys produce correct English keys; no unknown keys in output |
| `TestBuildRawPayloadEn_LegalNameNotTranslated` | `navn` value is copied verbatim |
| `TestBuildRawPayloadEn_BooleansAndDatesPassThrough` | Booleans, dates, numbers, codes copied as-is |
| `TestExtractTranslatableStrings` | Only `aktivitet`, `vedtektsfestetFormaal`, etc. collected for LLM; enum fields collected for cache |
| `TestTranslateBrregBatch_CacheHitSkipsLLM` | When all strings are in cache, no LLM call is made |
| `TestTranslateBrregBatch_CacheMissSendsToLLM` | Cache miss strings are sent to LLM; results stored in cache |
| `TestTranslateBrregBatch_LLMInventedKeyDiscarded` | Key in LLM response not in input is silently dropped |
| `TestTranslateBrregBatch_InvalidLLMJSONFails` | Non-JSON LLM response causes activity error |
| `TestTranslateBrregBatch_PerTextFallback` | LLM omitting a key keeps original text; row still marked translated |
| `TestClaimPendingBrregRawInputs_ExcludesNullPayloadEn` | Query returns zero rows when `raw_payload_en IS NULL` |
| `TestTranslateBrregWorkflow_StopsWhenNoPendingRows` | Workflow exits cleanly when batch returns 0 processed |
| `TestTranslateBrregWorkflow_SpecificIDsSinglePass` | IDs-only run does not loop or ContinueAsNew |
| `TestTranslateBrregWorkflow_EnqueuesSourceProcessAfterEachBatch` | `source_process` River job inserted after every batch with processed > 0 |
| `TestMarkExecutionComplete_SkipSourceProcess` | When `SkipSourceProcess=true`, no River job is inserted |
