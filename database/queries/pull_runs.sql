-- name: CreatePullRun :one
INSERT INTO source_pull_runs (source_id, river_job_id, task_type, trigger_type)
VALUES (
    (SELECT id FROM data_sources WHERE name = $1),
    $2, $3, $4
)
RETURNING *;

-- name: SucceedPullRun :exec
UPDATE source_pull_runs
SET status = 'succeeded',
    finished_at = now(),
    rows_seen = $2,
    raw_rows_inserted = $3,
    raw_rows_updated = $4,
    raw_rows_unchanged = $5
WHERE id = $1;

-- name: FailPullRun :exec
UPDATE source_pull_runs
SET status = 'failed',
    finished_at = now(),
    error_message = $2
WHERE id = $1;

-- name: InterruptStalePullRuns :exec
UPDATE source_pull_runs SET status = 'failed', error_message = 'interrupted on startup'
WHERE status = 'running';

-- name: ListPullRuns :many
SELECT r.*, d.name AS source_name
FROM source_pull_runs r
JOIN data_sources d ON d.id = r.source_id
WHERE ($1::text IS NULL OR d.name = $1)
ORDER BY r.started_at DESC
LIMIT $3 OFFSET $2;
