-- name: CreateTemporalExecution :one
INSERT INTO temporal_executions (workflow_type, source_name, country, input_ids, river_job_id)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: UpdateTemporalExecutionStarted :exec
UPDATE temporal_executions
SET workflow_id     = $2,
    workflow_run_id = $3,
    status          = 'running'
WHERE id = $1;

-- name: UpdateTemporalExecutionFailed :exec
UPDATE temporal_executions
SET status        = 'failed',
    error_message = $2,
    completed_at  = now()
WHERE id = $1;

-- name: ListTemporalExecutions :many
SELECT * FROM temporal_executions
ORDER BY started_at DESC
LIMIT $1 OFFSET $2;

-- name: GetTemporalExecution :one
SELECT * FROM temporal_executions
WHERE id = $1;
