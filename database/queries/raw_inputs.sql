-- GLEIF

-- name: UpsertGLEIFCompanyRawInput :one
INSERT INTO gleif_company_raw_inputs (
    source_pull_run_id, source_native_id, lei, legal_name,
    registration_status, headquarters_country_code, parent_lei, ultimate_parent_lei,
    source_updated_at, raw_payload, payload_hash
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
ON CONFLICT (lei, payload_hash) DO UPDATE SET last_seen_at = now()
RETURNING *;

-- name: ClaimPendingGLEIFRawInputs :many
UPDATE gleif_company_raw_inputs
SET processing_status = 'processing',
    processing_attempts = processing_attempts + 1,
    processing_lease_by = $1,
    processing_lease_until = now() + ($2 * interval '1 second'),
    updated_at = now()
WHERE id IN (
    SELECT id FROM gleif_company_raw_inputs
    WHERE processing_status = 'pending'
       OR (processing_status = 'processing' AND processing_lease_until < now())
    ORDER BY created_at
    LIMIT $3
    FOR UPDATE SKIP LOCKED
)
RETURNING *;

-- name: MarkGLEIFRawInputProcessed :exec
UPDATE gleif_company_raw_inputs
SET processing_status = 'processed', processed_at = now(), updated_at = now()
WHERE id = $1;

-- name: MarkGLEIFRawInputFailed :exec
UPDATE gleif_company_raw_inputs
SET processing_status = 'failed', processing_error = $2, updated_at = now()
WHERE id = $1;

-- Companies House

-- name: UpsertCompaniesHouseRawInput :one
INSERT INTO companies_house_company_raw_inputs (
    source_pull_run_id, source_native_id, company_number, company_name,
    company_status, company_type, source_updated_at, raw_payload, payload_hash
)
VALUES ($1, $2, $2, $3, $4, $5, $6, $7, $8)
ON CONFLICT (company_number, payload_hash) DO UPDATE SET last_seen_at = now()
RETURNING *;

-- name: ClaimPendingCompaniesHouseRawInputs :many
UPDATE companies_house_company_raw_inputs
SET processing_status = 'processing',
    processing_attempts = processing_attempts + 1,
    processing_lease_by = $1,
    processing_lease_until = now() + ($2 * interval '1 second'),
    updated_at = now()
WHERE id IN (
    SELECT id FROM companies_house_company_raw_inputs
    WHERE processing_status = 'pending'
       OR (processing_status = 'processing' AND processing_lease_until < now())
    ORDER BY created_at
    LIMIT $3
    FOR UPDATE SKIP LOCKED
)
RETURNING *;

-- name: MarkCompaniesHouseRawInputProcessed :exec
UPDATE companies_house_company_raw_inputs
SET processing_status = 'processed', processed_at = now(), updated_at = now()
WHERE id = $1;

-- name: MarkCompaniesHouseRawInputFailed :exec
UPDATE companies_house_company_raw_inputs
SET processing_status = 'failed', processing_error = $2, updated_at = now()
WHERE id = $1;

-- Brreg

-- name: UpsertBrregRawInput :one
INSERT INTO brreg_company_raw_inputs (
    source_pull_run_id, source_native_id, organization_number, organization_name,
    registration_status, website, source_updated_at, raw_payload, payload_hash
)
VALUES ($1, $2, $2, $3, $4, $5, $6, $7, $8)
ON CONFLICT (organization_number, payload_hash) DO UPDATE SET last_seen_at = now()
RETURNING *;

-- name: ClaimPendingBrregRawInputs :many
UPDATE brreg_company_raw_inputs
SET processing_status = 'processing',
    processing_attempts = processing_attempts + 1,
    processing_lease_by = $1,
    processing_lease_until = now() + ($2 * interval '1 second'),
    updated_at = now()
WHERE id IN (
    SELECT id FROM brreg_company_raw_inputs
    WHERE processing_status = 'pending'
       OR (processing_status = 'processing' AND processing_lease_until < now())
    ORDER BY created_at
    LIMIT $3
    FOR UPDATE SKIP LOCKED
)
RETURNING *;

-- name: MarkBrregRawInputProcessed :exec
UPDATE brreg_company_raw_inputs
SET processing_status = 'processed', processed_at = now(), updated_at = now()
WHERE id = $1;

-- name: MarkBrregRawInputFailed :exec
UPDATE brreg_company_raw_inputs
SET processing_status = 'failed', processing_error = $2, updated_at = now()
WHERE id = $1;
