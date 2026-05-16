-- database/queries/open_source_projects.sql

-- name: InsertOpenSourceProject :one
INSERT INTO open_source_projects (
    canonical_slug, display_name, website, repository_url, license,
    short_description, description, lifecycle_status, metadata, evidence
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
RETURNING *;

-- name: GetOpenSourceProjectByID :one
SELECT * FROM open_source_projects WHERE id = $1;

-- name: GetOpenSourceProjectBySlug :one
SELECT * FROM open_source_projects WHERE canonical_slug = $1;

-- name: ListOpenSourceProjects :many
SELECT * FROM open_source_projects
WHERE (sqlc.narg('q')::text IS NULL OR display_name ILIKE '%' || sqlc.narg('q') || '%')
ORDER BY display_name
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: CountOpenSourceProjects :one
SELECT COUNT(*) FROM open_source_projects
WHERE (sqlc.narg('q')::text IS NULL OR display_name ILIKE '%' || sqlc.narg('q') || '%');

-- name: UpdateOpenSourceProjectStatus :exec
UPDATE open_source_projects SET status = $2, updated_at = now() WHERE id = $1;
