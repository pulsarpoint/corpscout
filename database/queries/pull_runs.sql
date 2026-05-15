-- name: CreatePullRun :one
INSERT INTO source_pull_runs (source_id, river_job_id, started_at, cursor_start)
VALUES ($1, $2, now(), $3)
RETURNING *;

-- name: CompletePullRun :exec
UPDATE source_pull_runs
SET status = 'completed', completed_at = now(),
    cursor_end = $2, records_fetched = $3, records_upserted = $4
WHERE id = $1;

-- name: FailPullRun :exec
UPDATE source_pull_runs
SET status = 'failed', completed_at = now(), error_message = $2
WHERE id = $1;

-- name: InsertSourceSnapshot :one
INSERT INTO source_snapshots (source_id, pull_run_id, payload_hash, payload)
VALUES ($1, $2, $3, $4)
ON CONFLICT (source_id, payload_hash) DO NOTHING
RETURNING *;
