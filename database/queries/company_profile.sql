-- ── locations ─────────────────────────────────────────────────────────────────

-- name: UpsertCompanyLocation :one
INSERT INTO company_locations (
    company_id, location_type, label,
    address_line1, address_line2, city, region, postal_code,
    country, country_code, country_id, latitude, longitude,
    source, confidence, evidence
)
VALUES (
    $1, $2, $3,
    $4, $5, $6, $7, $8,
    $9, $10, (SELECT id FROM countries WHERE iso_alpha2 = $10), $11, $12,
    $13, $14, $15
)
ON CONFLICT (company_id, location_type, source)
    WHERE removed_at IS NULL AND location_type IN ('headquarters', 'registered_address')
DO UPDATE SET
    label         = EXCLUDED.label,
    address_line1 = EXCLUDED.address_line1,
    address_line2 = EXCLUDED.address_line2,
    city          = EXCLUDED.city,
    region        = EXCLUDED.region,
    postal_code   = EXCLUDED.postal_code,
    country       = EXCLUDED.country,
    country_code  = EXCLUDED.country_code,
    country_id    = EXCLUDED.country_id,
    latitude      = EXCLUDED.latitude,
    longitude     = EXCLUDED.longitude,
    confidence    = EXCLUDED.confidence,
    evidence      = EXCLUDED.evidence,
    removed_at    = NULL,
    updated_at    = now()
RETURNING *;

-- name: GetCompanyLocations :many
SELECT * FROM company_locations
WHERE company_id = $1 AND removed_at IS NULL
ORDER BY location_type, created_at;

-- ── phones ────────────────────────────────────────────────────────────────────

-- name: UpsertCompanyPhone :one
INSERT INTO company_phones (company_id, phone, description, purpose, source, confidence, evidence)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (company_id, phone, purpose) WHERE removed_at IS NULL
DO UPDATE SET
    description = EXCLUDED.description,
    source      = EXCLUDED.source,
    confidence  = EXCLUDED.confidence,
    evidence    = EXCLUDED.evidence,
    removed_at  = NULL,
    updated_at  = now()
RETURNING *;

-- name: GetCompanyPhones :many
SELECT * FROM company_phones
WHERE company_id = $1 AND removed_at IS NULL
ORDER BY purpose, phone;

-- ── emails ────────────────────────────────────────────────────────────────────

-- name: UpsertCompanyEmail :one
INSERT INTO company_emails (company_id, email, description, purpose, name, source, confidence, evidence)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
ON CONFLICT (company_id, lower(email), purpose) WHERE removed_at IS NULL
DO UPDATE SET
    description = EXCLUDED.description,
    name        = EXCLUDED.name,
    source      = EXCLUDED.source,
    confidence  = EXCLUDED.confidence,
    evidence    = EXCLUDED.evidence,
    removed_at  = NULL,
    updated_at  = now()
RETURNING *;

-- name: GetCompanyEmails :many
SELECT * FROM company_emails
WHERE company_id = $1 AND removed_at IS NULL
ORDER BY purpose, email;

-- ── industries ────────────────────────────────────────────────────────────────

-- name: UpsertCompanyIndustry :one
INSERT INTO company_industries (company_id, industry, source, confidence, evidence)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT ON CONSTRAINT uq_company_industries
DO UPDATE SET
    source     = EXCLUDED.source,
    confidence = EXCLUDED.confidence,
    evidence   = EXCLUDED.evidence
RETURNING *;

-- name: GetCompanyIndustries :many
SELECT * FROM company_industries
WHERE company_id = $1
ORDER BY industry;

-- ── markets ───────────────────────────────────────────────────────────────────

-- name: UpsertCompanyMarket :one
INSERT INTO company_markets (company_id, market, source, confidence, evidence)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT ON CONSTRAINT uq_company_markets
DO UPDATE SET
    source     = EXCLUDED.source,
    confidence = EXCLUDED.confidence,
    evidence   = EXCLUDED.evidence
RETURNING *;

-- name: GetCompanyMarkets :many
SELECT * FROM company_markets
WHERE company_id = $1
ORDER BY market;

-- ── services ──────────────────────────────────────────────────────────────────

-- name: UpsertCompanyService :one
INSERT INTO company_services (company_id, service, description, source, confidence, evidence)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT ON CONSTRAINT uq_company_services
DO UPDATE SET
    description = EXCLUDED.description,
    source      = EXCLUDED.source,
    confidence  = EXCLUDED.confidence,
    evidence    = EXCLUDED.evidence
RETURNING *;

-- name: GetCompanyServices :many
SELECT * FROM company_services
WHERE company_id = $1
ORDER BY service;

-- ── enrichment update ─────────────────────────────────────────────────────────

-- name: UpdateCompanyEnrichment :one
UPDATE companies SET
    short_name        = COALESCE(sqlc.narg('short_name')::text,        short_name),
    short_description = COALESCE(sqlc.narg('short_description')::text, short_description),
    description       = COALESCE(sqlc.narg('description')::text,       description),
    website           = COALESCE(sqlc.narg('website')::text,           website),
    founded_year      = COALESCE(sqlc.narg('founded_year')::int,       founded_year),
    employee_estimate = COALESCE(sqlc.narg('employee_estimate')::jsonb, employee_estimate),
    revenue_estimate  = COALESCE(sqlc.narg('revenue_estimate')::jsonb,  revenue_estimate),
    ownership         = COALESCE(sqlc.narg('ownership')::jsonb,         ownership),
    updated_at        = now()
WHERE id = sqlc.arg('id')::uuid
RETURNING *;
