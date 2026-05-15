-- name: ListSources :many
SELECT * FROM data_sources ORDER BY name;

-- name: GetSourceByName :one
SELECT * FROM data_sources WHERE name = $1;

-- name: UpdateSourceCursor :exec
UPDATE data_sources
SET last_cursor = $2, last_crawled_at = $3, updated_at = now()
WHERE id = $1;

-- name: UpsertDataSource :one
INSERT INTO data_sources (name, source_type, adapter_type, country_id, enabled, crawl_interval_hours, config)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (name) DO UPDATE SET
    enabled = EXCLUDED.enabled,
    crawl_interval_hours = EXCLUDED.crawl_interval_hours,
    config = EXCLUDED.config,
    updated_at = now()
RETURNING *;

-- name: UpdateSourceEnabled :exec
UPDATE data_sources SET enabled = $2, updated_at = now() WHERE name = $1;
