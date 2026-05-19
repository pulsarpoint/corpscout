-- name: GetSourceByName :one
SELECT * FROM data_sources WHERE name = $1;

-- name: ListSources :many
SELECT * FROM data_sources ORDER BY name;

-- name: UpdateSourceEnabled :exec
UPDATE data_sources SET enabled = $2, updated_at = now() WHERE name = $1;

-- name: UpdateSourceScheduleEnabled :exec
UPDATE data_sources SET schedule_enabled = $2, updated_at = now() WHERE name = $1;

-- name: UpdateSourceSchedule :exec
UPDATE data_sources
SET schedule_kind = $2, schedule_expression = $3, updated_at = now()
WHERE name = $1;

-- name: UpdateSourceConfig :exec
UPDATE data_sources SET config = $2, updated_at = now() WHERE name = $1;

-- name: UpdateSourcePullStarted :exec
UPDATE data_sources SET last_started_at = now(), updated_at = now() WHERE name = $1;

-- name: UpdateSourcePullSucceeded :exec
UPDATE data_sources
SET last_success_at = now(),
    last_source_marker_type = $2,
    last_source_marker = $3,
    last_source_modified_at = $4,
    consecutive_failures = 0,
    last_error = NULL,
    updated_at = now()
WHERE name = $1;

-- name: UpdateSourcePullFailed :exec
UPDATE data_sources
SET last_failed_at = now(),
    consecutive_failures = consecutive_failures + 1,
    last_error = $2,
    updated_at = now()
WHERE name = $1;

-- name: GetSourcesWithCapabilities :many
SELECT * FROM data_sources
WHERE array_length(capabilities, 1) > 0
ORDER BY name;

-- name: UpdateSourceCapabilities :exec
UPDATE data_sources SET capabilities = $2, updated_at = now() WHERE name = $1;
