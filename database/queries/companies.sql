-- name: UpsertCompanyByLEI :one
INSERT INTO companies (lei, name, country_id, registration_number, status, primary_source_id)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (lei) DO UPDATE SET
    name = EXCLUDED.name,
    status = EXCLUDED.status,
    updated_at = now()
RETURNING *;

-- name: UpsertCompanyByRegNumber :one
INSERT INTO companies (name, country_id, registration_number, status, primary_source_id)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (country_id, registration_number)
    WHERE registration_number IS NOT NULL AND lei IS NULL
DO UPDATE SET
    name = EXCLUDED.name,
    status = EXCLUDED.status,
    updated_at = now()
RETURNING *;

-- name: UpsertCompanyAlias :exec
INSERT INTO company_aliases (company_id, alias, alias_type, source_id)
VALUES ($1, $2, $3, $4)
ON CONFLICT (company_id, alias, alias_type) DO NOTHING;

-- name: UpsertCompanySource :exec
INSERT INTO company_sources (company_id, source_id, external_id, pull_run_id, raw_data, fetched_at)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (company_id, source_id) DO UPDATE SET
    external_id = EXCLUDED.external_id,
    pull_run_id = EXCLUDED.pull_run_id,
    raw_data    = EXCLUDED.raw_data,
    fetched_at  = EXCLUDED.fetched_at;

-- name: GetCompany :one
SELECT * FROM companies WHERE id = $1;

-- name: ListCompanies :many
SELECT * FROM companies c
WHERE (sqlc.narg('status')::text IS NULL OR status = sqlc.narg('status'))
  AND (sqlc.narg('country_id')::uuid IS NULL OR country_id = sqlc.narg('country_id'))
  AND (sqlc.narg('q')::text IS NULL OR name ILIKE '%' || sqlc.narg('q') || '%')
  AND (sqlc.narg('source_id')::uuid IS NULL OR EXISTS (
    SELECT 1 FROM company_sources cs WHERE cs.company_id = c.id AND cs.source_id = sqlc.narg('source_id')
  ))
ORDER BY name
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: UpdateCompanyParentLEI :exec
UPDATE companies
SET parent_lei          = $2,
    ultimate_parent_lei = $3,
    updated_at          = now()
WHERE id = $1;

-- name: ListCompaniesForGLEIFEnrich :many
SELECT id, lei FROM companies
WHERE lei IS NOT NULL
  AND parent_lei IS NULL
ORDER BY created_at
LIMIT $1 OFFSET $2;

-- name: CountCompanies :one
SELECT COUNT(*) FROM companies c
WHERE (sqlc.narg('status')::text IS NULL OR status = sqlc.narg('status'))
  AND (sqlc.narg('country_id')::uuid IS NULL OR country_id = sqlc.narg('country_id'))
  AND (sqlc.narg('q')::text IS NULL OR name ILIKE '%' || sqlc.narg('q') || '%')
  AND (sqlc.narg('source_id')::uuid IS NULL OR EXISTS (
    SELECT 1 FROM company_sources cs WHERE cs.company_id = c.id AND cs.source_id = sqlc.narg('source_id')
  ));

-- name: GetCompanyBySlug :one
SELECT * FROM companies WHERE canonical_slug = $1;

-- name: UpdateCompanySlug :exec
UPDATE companies
SET canonical_slug = $2,
    display_name   = $3,
    updated_at     = now()
WHERE id = $1;
