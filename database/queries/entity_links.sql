-- name: InsertCPELinkSuggestion :one
INSERT INTO cpe_entity_link_suggestions (
    cpe_vendor_token, target_entity_type,
    target_company_id, target_organization_id, target_open_source_project_id,
    proposed_entity_payload, suggested_by, confidence, evidence
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: ListPendingCPELinkSuggestions :many
SELECT * FROM cpe_entity_link_suggestions
WHERE status = 'pending'
ORDER BY created_at
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: CountPendingCPELinkSuggestions :one
SELECT COUNT(*) FROM cpe_entity_link_suggestions WHERE status = 'pending';

-- name: UpdateCPELinkSuggestionStatus :exec
UPDATE cpe_entity_link_suggestions
SET status      = $2,
    reviewed_by = $3,
    reviewed_at = now(),
    review_note = $4,
    updated_at  = now()
WHERE id = $1;

-- name: InsertCPEEntityLink :one
INSERT INTO cpe_entity_links (
    cpe_vendor_token, entity_type,
    company_id, organization_id, open_source_project_id,
    approved_suggestion_id
)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: GetCPEEntityLinkByToken :one
SELECT * FROM cpe_entity_links
WHERE cpe_vendor_token = $1 AND removed_at IS NULL;

-- name: InsertCVELinkSuggestion :one
INSERT INTO cve_entity_link_suggestions (
    cve_id, target_entity_type,
    target_company_id, target_organization_id, target_open_source_project_id,
    proposed_entity_payload, suggested_by, confidence, evidence
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: ListPendingCVELinkSuggestions :many
SELECT * FROM cve_entity_link_suggestions
WHERE status = 'pending'
ORDER BY created_at
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: UpdateCVELinkSuggestionStatus :exec
UPDATE cve_entity_link_suggestions
SET status      = $2,
    reviewed_by = $3,
    reviewed_at = now(),
    review_note = $4,
    updated_at  = now()
WHERE id = $1;

-- name: InsertCVEEntityLink :one
INSERT INTO cve_entity_links (
    cve_id, entity_type,
    company_id, organization_id, open_source_project_id,
    approved_suggestion_id
)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;
