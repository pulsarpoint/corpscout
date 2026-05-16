-- database/queries/organizations.sql

-- name: InsertOrganization :one
INSERT INTO organizations (
    canonical_slug, display_name, organization_type, website,
    short_description, description, country_code, governance, metadata, evidence
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
RETURNING *;

-- name: GetOrganizationByID :one
SELECT * FROM organizations WHERE id = $1;

-- name: GetOrganizationBySlug :one
SELECT * FROM organizations WHERE canonical_slug = $1;

-- name: ListOrganizations :many
SELECT * FROM organizations
WHERE (sqlc.narg('q')::text IS NULL OR display_name ILIKE '%' || sqlc.narg('q') || '%')
ORDER BY display_name
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: CountOrganizations :one
SELECT COUNT(*) FROM organizations
WHERE (sqlc.narg('q')::text IS NULL OR display_name ILIKE '%' || sqlc.narg('q') || '%');

-- name: UpdateOrganizationStatus :exec
UPDATE organizations SET status = $2, updated_at = now() WHERE id = $1;
