-- name: UpsertDomain :one
INSERT INTO domains (domain)
VALUES ($1)
ON CONFLICT (domain) DO UPDATE SET last_verified_at = now()
RETURNING *;

-- name: UpsertCompanyDomain :one
INSERT INTO company_domains (company_id, domain_id, relationship_type, status, signal, confidence, evidence)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (company_id, domain_id, signal) DO UPDATE SET
    confidence   = EXCLUDED.confidence,
    evidence     = EXCLUDED.evidence,
    last_seen_at = now()
RETURNING *;

-- name: ListDomainsForCompany :many
SELECT d.domain, cd.*
FROM company_domains cd
JOIN domains d ON d.id = cd.domain_id
WHERE cd.company_id = $1
ORDER BY cd.confidence DESC;

-- name: ListCandidatesForReview :many
SELECT cd.*, c.name AS company_name, d.domain
FROM company_domains cd
JOIN companies c ON c.id = cd.company_id
JOIN domains   d ON d.id = cd.domain_id
WHERE cd.status = 'needs_review'
ORDER BY cd.first_seen_at
LIMIT $1 OFFSET $2;

-- name: UpdateCompanyDomainStatus :exec
UPDATE company_domains SET status = $2, relationship_type = $3 WHERE id = $1;
