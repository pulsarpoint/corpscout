-- name: CreateDomainReview :one
INSERT INTO company_domain_reviews (company_domain_id, action, reviewed_by, review_note)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: CreateDomainReviewAndUpdateStatus :one
-- Atomically records the review decision and updates the domain candidate status
-- in a single statement so the audit trail can never diverge from the domain state.
WITH upd AS (
    UPDATE company_domains
    SET status = $5, relationship_type = $6
    WHERE id = $1
)
INSERT INTO company_domain_reviews (company_domain_id, action, reviewed_by, review_note)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: ListReviewsForClaim :many
SELECT * FROM company_domain_reviews
WHERE company_domain_id = $1
ORDER BY created_at DESC;
