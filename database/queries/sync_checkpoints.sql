-- name: GetSyncCheckpoint :one
SELECT source_name, cursor, last_completed_at, updated_at
FROM source_sync_checkpoints
WHERE source_name = $1;

-- name: UpsertSyncCheckpoint :exec
INSERT INTO source_sync_checkpoints (source_name, cursor, last_completed_at)
VALUES ($1, $2, now())
ON CONFLICT (source_name) DO UPDATE
    SET cursor            = EXCLUDED.cursor,
        last_completed_at = EXCLUDED.last_completed_at,
        updated_at        = now();
