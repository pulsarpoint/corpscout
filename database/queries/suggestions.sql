-- Company root suggestions


-- name: InsertCompanySuggestion :one
INSERT INTO company_suggestions (
    proposed_display_name, proposed_legal_name, proposed_website,
    proposed_canonical_slug, proposed_country_id, proposed_profile, confidence
)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: GetCompanySuggestionByID :one
SELECT * FROM company_suggestions WHERE id = $1;

-- name: ListPendingCompanySuggestions :many
SELECT * FROM company_suggestions
WHERE status = 'pending'
ORDER BY created_at DESC
LIMIT $2 OFFSET $1;

-- name: CountPendingCompanySuggestions :one
SELECT COUNT(*) FROM company_suggestions WHERE status = 'pending';

-- name: UpdateCompanySuggestionApproved :exec
UPDATE company_suggestions
SET status = 'approved',
    created_company_id = $2,
    reviewed_by = $3,
    reviewed_at = now(),
    review_note = $4,
    updated_at = now()
WHERE id = $1;

-- name: UpdateCompanySuggestionRejected :exec
UPDATE company_suggestions
SET status = 'rejected',
    reviewed_by = $2,
    reviewed_at = now(),
    review_note = $3,
    updated_at = now()
WHERE id = $1;

-- Company section suggestions

-- name: InsertCompanyDomainSuggestion :one
INSERT INTO company_domain_suggestions (
    company_id, company_suggestion_id, operation, domain,
    current_payload, proposed_payload, confidence
)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: InsertCompanyContactSuggestion :one
INSERT INTO company_contact_suggestions (
    company_id, company_suggestion_id, operation, contact_kind,
    current_payload, proposed_payload, confidence
)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: InsertCompanyLocationSuggestion :one
INSERT INTO company_location_suggestions (
    company_id, company_suggestion_id, operation, location_kind, country_code, city,
    current_payload, proposed_payload, confidence
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: InsertCompanyStatusSuggestion :one
INSERT INTO company_status_suggestions (
    company_id, company_suggestion_id, operation, status_field,
    current_value, proposed_value, current_payload, proposed_payload, confidence
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: InsertCompanyRelationshipSuggestion :one
INSERT INTO company_relationship_suggestions (
    company_id, company_suggestion_id, operation, relationship_type,
    related_company_id, related_company_suggestion_id,
    related_company_name, related_lei,
    current_payload, proposed_payload, confidence
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
RETURNING *;

-- Organization and open-source project root suggestions

-- name: InsertOrganizationSuggestion :one
INSERT INTO organization_suggestions (
    proposed_display_name, proposed_organization_type, proposed_website,
    proposed_canonical_slug, proposed_profile, confidence
)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: InsertOpenSourceProjectSuggestion :one
INSERT INTO open_source_project_suggestions (
    proposed_display_name, proposed_repository_url, proposed_website,
    proposed_license, proposed_lifecycle_status, proposed_canonical_slug,
    proposed_profile, confidence
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- Provenance links

-- name: InsertSuggestionSourceLink :one
INSERT INTO suggestion_source_links (
    suggestion_table, suggestion_id, source_id,
    source_input_table, source_input_key, source_pull_run_id,
    confidence, evidence_excerpt
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;
