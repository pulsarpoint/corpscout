-- name: UpsertCompanyRelationship :one
INSERT INTO company_relationships (
    subject_company_id, related_company_id, relationship_type,
    source, confidence, evidence, ownership_percentage, valid_from, valid_to
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
ON CONFLICT ON CONSTRAINT uq_company_relationships_current
DO UPDATE SET
    source               = EXCLUDED.source,
    confidence           = EXCLUDED.confidence,
    evidence             = EXCLUDED.evidence,
    ownership_percentage = EXCLUDED.ownership_percentage,
    valid_from           = EXCLUDED.valid_from,
    valid_to             = EXCLUDED.valid_to,
    updated_at           = now()
RETURNING *;

-- name: ListCompanyRelationships :many
SELECT * FROM company_relationships
WHERE subject_company_id = $1
  AND removed_at IS NULL
  AND status IN ('active', 'needs_review')
ORDER BY relationship_type, created_at;

-- name: UpdateCompanyRelationshipStatus :exec
UPDATE company_relationships
SET status     = $2,
    updated_at = now()
WHERE id = $1;
