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
SELECT * FROM companies
WHERE ($1::text IS NULL OR status = $1)
  AND ($2::uuid IS NULL OR country_id = $2)
ORDER BY name
LIMIT $3 OFFSET $4;

-- name: CountCompanies :one
SELECT COUNT(*) FROM companies
WHERE ($1::text IS NULL OR status = $1)
  AND ($2::uuid IS NULL OR country_id = $2);
