-- name: GetCompany :one
SELECT * FROM companies WHERE id = $1;

-- name: GetCompanyBySlug :one
SELECT * FROM companies WHERE canonical_slug = $1;

-- name: UpdateCompanySlug :exec
UPDATE companies
SET canonical_slug = $2,
    display_name   = $3,
    updated_at     = now()
WHERE id = $1;

-- name: ListCompanies :many
SELECT * FROM companies c
WHERE (sqlc.narg('status')::text IS NULL OR status = sqlc.narg('status'))
  AND (sqlc.narg('country_id')::uuid IS NULL OR country_id = sqlc.narg('country_id'))
  AND (sqlc.narg('q')::text IS NULL OR name ILIKE '%' || sqlc.narg('q') || '%')
ORDER BY name
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: CountCompanies :one
SELECT COUNT(*) FROM companies c
WHERE (sqlc.narg('status')::text IS NULL OR status = sqlc.narg('status'))
  AND (sqlc.narg('country_id')::uuid IS NULL OR country_id = sqlc.narg('country_id'))
  AND (sqlc.narg('q')::text IS NULL OR name ILIKE '%' || sqlc.narg('q') || '%');

-- name: GetCompanyByLEI :one
SELECT * FROM companies WHERE lei = $1;

-- name: GetCompanyByRegistrationAndCountry :one
SELECT c.*
FROM companies c
JOIN countries co ON co.id = c.country_id
WHERE c.registration_number = $1
  AND co.iso_alpha2 = $2;

-- name: InsertCompany :one
INSERT INTO companies (canonical_slug, name, country_id, status)
VALUES ($1, $2, $3, coalesce($4, 'active'))
RETURNING *;

-- name: GetCompanyByExactName :one
SELECT * FROM companies WHERE lower(name) = lower($1) LIMIT 1;
