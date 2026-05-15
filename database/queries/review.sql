-- name: CreateDomainReview :one
INSERT INTO company_domain_reviews (company_domain_id, action, reviewed_by, review_note)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: ListReviewsForClaim :many
SELECT * FROM company_domain_reviews
WHERE company_domain_id = $1
ORDER BY created_at DESC;
