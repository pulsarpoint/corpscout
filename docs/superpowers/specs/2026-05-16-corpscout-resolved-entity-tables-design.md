# Corpscout Resolved Entity Tables Design

## Goal

Restructure Corpscout's central identity model so it stores only resolved, trusted entities of the three types we care about:

- companies
- organizations
- open-source projects

Corpscout should not promote every imported vendor string, CPE token, GitHub name, or backoffice catalog vendor into a resolved entity. Uncertain inputs should stay in candidate/observation queues until they are confidently resolved or manually reviewed.

Backoffice-v2 will use Corpscout as the central source for resolved company, organization, and open-source project identity data. Backoffice-v2 remains responsible for catalog vendors, products, product-service relationships, CVE/CPE mappings, and product/service augmentation.

## Current Context

Corpscout currently has a `companies` table and company-specific child tables:

- `company_locations`
- `company_phones`
- `company_emails`
- `company_industries`
- `company_markets`
- `company_services`
- `company_domains`
- `company_aliases`
- `company_sources`
- `company_relationships`

The current schema is company-centric. That works for registry and GLEIF data, but it is not enough for catalog identity resolution from security/product sources. CPE vendor tokens can represent companies, foundations, open-source projects, brands, product families, or ambiguous labels.

The new model must support precise storage without creating fake companies.

## Design Decision

Do not add a generic `entries` parent table.

Use three resolved root tables instead:

- `companies`
- `organizations`
- `open_source_projects`

Each root type owns its own child tables. There will not be shared generic child tables for contacts, links, locations, forums, CPE tokens, or sources. The schema should stay explicit because each entity type has different profile data and different enrichment rules.

When consumers need a unified surface, Corpscout should provide a resolver API or SQL view over the three tables. The database does not need a shared root row just to make polymorphism convenient.

## Entity Types

### Company

A company is a commercial or legal business entity.

Examples:

- F5
- Microsoft
- NOVELIC
- Cloudflare

Companies can have legal identifiers, registration numbers, LEI values, ownership data, headquarters, financial estimates, markets, industries, and commercial services.

### Organization

An organization is a non-company institution, foundation, standards body, nonprofit, university, government agency, or similar entity.

Examples:

- Apache Software Foundation
- CNCF
- OWASP
- IETF
- MIT

Organizations can have governance data, foundation/nonprofit metadata, official sites, project relationships, forums, contacts, and maintained CPE vendor tokens.

### Open-Source Project

An open-source project is a project/community/repository/product-like OSS identity that is not itself the legal owner.

Examples:

- Nmap
- OpenSSL
- Kubernetes
- Apache HTTP Server

Open-source projects can have repositories, package names, maintainers, licenses, documentation, security contacts, forums, release metadata, and related organizations or companies.

## Non-Goals

- Do not create a universal identity graph in this phase.
- Do not model products in Corpscout. Products remain in backoffice-v2.
- Do not import every CPE vendor token as a resolved entity.
- Do not store product names as companies.
- Do not add a generic `entries` table unless a future phase introduces shared child tables or strong cross-type FK requirements.
- Do not force ambiguous inputs into one of the three resolved tables.
- Do not design or implement a separate field-update review queue in this phase.

## Resolved Tables

### `companies`

Keep the current `companies` table as the root table for company entities.

Existing company enrichment columns and child tables remain company-specific. Future company-specific additions should continue to use `company_*` tables.

Add these company columns before creating the unified read view:

```sql
ALTER TABLE companies
    ADD COLUMN IF NOT EXISTS canonical_slug TEXT,
    ADD COLUMN IF NOT EXISTS display_name TEXT,
    ADD COLUMN IF NOT EXISTS resolution_status TEXT NOT NULL DEFAULT 'resolved',
    ADD COLUMN IF NOT EXISTS confidence REAL,
    ADD COLUMN IF NOT EXISTS evidence JSONB NOT NULL DEFAULT '{}'::jsonb;
```

`name` can remain the legal or registry name where existing imports depend on it. `display_name` can become the product-facing name used by consumers.

For existing company rows, add `canonical_slug` as nullable first, backfill it with the rules below, resolve collisions, then enforce `NOT NULL` and uniqueness. New company writes must provide `canonical_slug`.

After backfill:

```sql
ALTER TABLE companies
    ALTER COLUMN canonical_slug SET NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS uq_companies_canonical_slug
    ON companies(canonical_slug);
```

### Canonical Slug Rules

`canonical_slug` is a stable handle for URLs, search, and operator-facing references. It is unique inside each resolved root table, not globally unique across all entity types. Consumers must treat `(entity_type, canonical_slug)` as the public lookup key when using the unified resolver surface.

Generate slugs with this algorithm:

1. Choose the base label from `display_name`; for companies, fall back to `name` when `display_name` is empty.
2. Trim whitespace, normalize Unicode with NFKD, transliterate when supported, drop remaining non-ASCII characters, lowercase the result, and replace `&` with `and`.
3. Replace every run of non-alphanumeric characters with one hyphen.
4. Collapse repeated hyphens and trim leading or trailing hyphens.
5. If the slug is empty, use `{entity_type}-{uuid8}`.
6. If the slug collides inside the same root table, append `-{uuid8}` from the entity ID instead of using an order-dependent numeric suffix.

Do not automatically rewrite `canonical_slug` when `display_name` changes. Slug changes should be explicit administrative operations so external references remain stable. Resolver matching must not treat slug equality alone as strong identity evidence.

### `company_relationships`

Add a company-specific relationship table for parent, ownership, acquisition, and group relationships between resolved companies.

This is not a generic entity relationship table. It only models company-to-company relationships where both sides are resolved company rows. Organization and open-source project relationships should use their own explicit tables later if needed.

```sql
CREATE TABLE company_relationships (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    subject_company_id UUID NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
    related_company_id UUID NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
    relationship_type TEXT NOT NULL,
    ownership_percentage NUMERIC(5,2),
    valid_from DATE,
    valid_to DATE,
    source TEXT NOT NULL DEFAULT 'manual',
    confidence REAL,
    evidence JSONB NOT NULL DEFAULT '{}'::jsonb,
    status TEXT NOT NULL DEFAULT 'active',
    removed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT chk_company_relationships_type CHECK (
        relationship_type IN (
            'direct_parent',
            'ultimate_parent',
            'subsidiary_of',
            'owned_by',
            'acquired_by',
            'merged_into',
            'same_group',
            'other'
        )
    ),
    CONSTRAINT chk_company_relationships_distinct CHECK (
        subject_company_id <> related_company_id
    ),
    CONSTRAINT chk_company_relationships_ownership_percentage CHECK (
        ownership_percentage IS NULL OR ownership_percentage BETWEEN 0 AND 100
    ),
    CONSTRAINT chk_company_relationships_confidence CHECK (
        confidence IS NULL OR confidence BETWEEN 0 AND 1
    ),
    CONSTRAINT chk_company_relationships_status CHECK (
        status IN ('active', 'needs_review', 'rejected', 'superseded')
    ),
    CONSTRAINT chk_company_relationships_evidence_object CHECK (
        jsonb_typeof(evidence) = 'object'
    ),
    CONSTRAINT chk_company_relationships_valid_range CHECK (
        valid_to IS NULL OR valid_from IS NULL OR valid_to >= valid_from
    )
);

CREATE UNIQUE INDEX uq_company_relationships_current
    ON company_relationships(subject_company_id, related_company_id, relationship_type)
    WHERE removed_at IS NULL
      AND status IN ('active', 'needs_review');

CREATE INDEX idx_company_relationships_subject
    ON company_relationships(subject_company_id, status, relationship_type)
    WHERE removed_at IS NULL;

CREATE INDEX idx_company_relationships_related
    ON company_relationships(related_company_id, status, relationship_type)
    WHERE removed_at IS NULL;
```

Direction rule:

- `subject_company_id` is the company being described.
- `related_company_id` is the parent, owner, acquirer, merged target, or group company.

Relationship type rule:

- `direct_parent` and `ultimate_parent` are registry-style parent-chain relationships, especially for GLEIF or corporate registry enrichment.
- `subsidiary_of` and `owned_by` are evidence-backed business relationships when the exact registry parent chain is not available.
- Only one current relationship of the same type between the same companies is allowed. Rejected, superseded, or removed rows can remain as history without blocking a corrected active row.

Source and precedence rule:

- `source` must identify the origin of the relationship, such as `gleif`, `company_registry`, `manual_review`, `ai_research`, or a named importer.
- `direct_parent` is authoritative over `subsidiary_of` for parent-chain consumers when both rows exist for the same subject and related company.
- `subsidiary_of` can coexist with `direct_parent` as business-research evidence, but operational parent lookup should prefer `direct_parent`.
- `ultimate_parent` rows are denormalized cache rows derived from the `direct_parent` chain. They exist to make queries fast, not as independently asserted facts. When the direct-parent chain changes, matching `ultimate_parent` rows must be rebuilt or superseded.

Examples:

```text
NOVELIC subsidiary_of Sona BLW Precision Forgings Ltd.
subject_company_id = NOVELIC
related_company_id = Sona BLW Precision Forgings Ltd.
relationship_type = subsidiary_of
```

```text
NOVELIC direct_parent Sona BLW Precision Forgings Ltd.
subject_company_id = NOVELIC
related_company_id = Sona BLW Precision Forgings Ltd.
relationship_type = direct_parent
```

`companies.parent_lei` and `companies.ultimate_parent_lei` can remain as imported GLEIF fields for compatibility, but `company_relationships` should become the reviewable relationship surface. When the parent company is resolved locally, GLEIF enrichment should upsert a relationship row in addition to storing the LEI values.

### `organizations`

Create a separate root table for resolved non-company organizations.

```sql
CREATE TABLE organizations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    canonical_slug TEXT NOT NULL UNIQUE,
    display_name TEXT NOT NULL,
    organization_type TEXT NOT NULL,
    website TEXT,
    short_description TEXT,
    description TEXT,
    country_code TEXT,
    governance JSONB NOT NULL DEFAULT '{}'::jsonb,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    confidence REAL,
    evidence JSONB NOT NULL DEFAULT '{}'::jsonb,
    status TEXT NOT NULL DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT chk_organizations_type CHECK (
        organization_type IN ('foundation', 'standards_body', 'nonprofit', 'government', 'university', 'community', 'other')
    ),
    CONSTRAINT chk_organizations_status CHECK (
        status IN ('active', 'inactive', 'unknown')
    ),
    CONSTRAINT chk_organizations_confidence CHECK (
        confidence IS NULL OR confidence BETWEEN 0 AND 1
    ),
    CONSTRAINT chk_organizations_metadata_object CHECK (jsonb_typeof(metadata) = 'object'),
    CONSTRAINT chk_organizations_governance_object CHECK (jsonb_typeof(governance) = 'object'),
    CONSTRAINT chk_organizations_evidence_object CHECK (jsonb_typeof(evidence) = 'object')
);
```

Organization child tables should be explicit:

- `organization_cpe_vendor_tokens`
- `organization_websites`
- `organization_contacts`
- `organization_locations`
- `organization_social_links`
- `organization_forums`
- `organization_sources`
- `organization_projects`

### `open_source_projects`

Create a separate root table for resolved open-source projects.

```sql
CREATE TABLE open_source_projects (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    canonical_slug TEXT NOT NULL UNIQUE,
    display_name TEXT NOT NULL,
    website TEXT,
    repository_url TEXT,
    license TEXT,
    short_description TEXT,
    description TEXT,
    lifecycle_status TEXT NOT NULL DEFAULT 'active',
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    confidence REAL,
    evidence JSONB NOT NULL DEFAULT '{}'::jsonb,
    status TEXT NOT NULL DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT chk_open_source_projects_lifecycle_status CHECK (
        lifecycle_status IN ('active', 'maintenance', 'deprecated', 'unknown')
    ),
    CONSTRAINT chk_open_source_projects_status CHECK (
        status IN ('active', 'inactive', 'unknown')
    ),
    CONSTRAINT chk_open_source_projects_confidence CHECK (
        confidence IS NULL OR confidence BETWEEN 0 AND 1
    ),
    CONSTRAINT chk_open_source_projects_metadata_object CHECK (jsonb_typeof(metadata) = 'object'),
    CONSTRAINT chk_open_source_projects_evidence_object CHECK (jsonb_typeof(evidence) = 'object')
);
```

Open-source project child tables should be explicit:

- `open_source_project_cpe_vendor_tokens`
- `open_source_project_repositories`
- `open_source_project_package_names`
- `open_source_project_maintainers`
- `open_source_project_security_contacts`
- `open_source_project_forums`
- `open_source_project_social_links`
- `open_source_project_sources`
- `open_source_project_related_organizations`

## CPE Vendor Token Associations

CPE vendor tokens are associations, not aliases.

For example, `nginx` should not become a company alias for F5. If evidence shows the CPE vendor token belongs under F5 for catalog purposes, store it as a CPE token associated with F5.

Each resolved type gets its own CPE token table. These tables intentionally do not share a generic parent because Corpscout does not use a generic `entries` root table.

- `company_cpe_vendor_tokens` belongs only to `companies`.
- `organization_cpe_vendor_tokens` belongs only to `organizations`.
- `open_source_project_cpe_vendor_tokens` belongs only to `open_source_projects`.

The three tables should mirror their non-FK columns so consumers can use the same semantics regardless of resolved entity type.

`association_type` explains why this CPE vendor token belongs to the resolved entity. It should be explicit and should not have a default. If the system cannot choose an association type confidently, the token should remain in `identity_observations` for review.

Allowed association types:

- `official_identifier`: the CPE vendor token directly names this resolved entity.
- `owned_product_or_brand`: the token names a product, product family, or brand owned by this entity.
- `maintained_or_governed_project`: the token names a project maintained or governed by this entity.
- `legacy_or_acquired`: the token comes from a previous owner, acquired entity, or legacy brand that now maps to this entity.
- `manual_review`: a reviewer intentionally associated the token, but the precise reason does not fit the other categories.

Company CPE token table:

```sql
CREATE TABLE company_cpe_vendor_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id UUID NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
    cpe_vendor_token TEXT NOT NULL,
    association_type TEXT NOT NULL,
    source TEXT NOT NULL DEFAULT 'manual',
    confidence REAL,
    evidence JSONB NOT NULL DEFAULT '{}'::jsonb,
    status TEXT NOT NULL DEFAULT 'active',
    first_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT chk_company_cpe_vendor_tokens_token CHECK (btrim(cpe_vendor_token) <> ''),
    CONSTRAINT chk_company_cpe_vendor_tokens_association_type CHECK (
        association_type IN (
            'official_identifier',
            'owned_product_or_brand',
            'maintained_or_governed_project',
            'legacy_or_acquired',
            'manual_review'
        )
    ),
    CONSTRAINT chk_company_cpe_vendor_tokens_status CHECK (status IN ('active', 'needs_review', 'rejected', 'superseded')),
    CONSTRAINT chk_company_cpe_vendor_tokens_confidence CHECK (confidence IS NULL OR confidence BETWEEN 0 AND 1),
    CONSTRAINT chk_company_cpe_vendor_tokens_evidence_object CHECK (jsonb_typeof(evidence) = 'object'),
    CONSTRAINT uq_company_cpe_vendor_tokens UNIQUE (company_id, cpe_vendor_token)
);
```

Organization CPE token table:

```sql
CREATE TABLE organization_cpe_vendor_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    cpe_vendor_token TEXT NOT NULL,
    association_type TEXT NOT NULL,
    source TEXT NOT NULL DEFAULT 'manual',
    confidence REAL,
    evidence JSONB NOT NULL DEFAULT '{}'::jsonb,
    status TEXT NOT NULL DEFAULT 'active',
    first_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT chk_organization_cpe_vendor_tokens_token CHECK (btrim(cpe_vendor_token) <> ''),
    CONSTRAINT chk_organization_cpe_vendor_tokens_association_type CHECK (
        association_type IN (
            'official_identifier',
            'owned_product_or_brand',
            'maintained_or_governed_project',
            'legacy_or_acquired',
            'manual_review'
        )
    ),
    CONSTRAINT chk_organization_cpe_vendor_tokens_status CHECK (status IN ('active', 'needs_review', 'rejected', 'superseded')),
    CONSTRAINT chk_organization_cpe_vendor_tokens_confidence CHECK (confidence IS NULL OR confidence BETWEEN 0 AND 1),
    CONSTRAINT chk_organization_cpe_vendor_tokens_evidence_object CHECK (jsonb_typeof(evidence) = 'object'),
    CONSTRAINT uq_organization_cpe_vendor_tokens UNIQUE (organization_id, cpe_vendor_token)
);
```

Open-source project CPE token table:

```sql
CREATE TABLE open_source_project_cpe_vendor_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    open_source_project_id UUID NOT NULL REFERENCES open_source_projects(id) ON DELETE CASCADE,
    cpe_vendor_token TEXT NOT NULL,
    association_type TEXT NOT NULL,
    source TEXT NOT NULL DEFAULT 'manual',
    confidence REAL,
    evidence JSONB NOT NULL DEFAULT '{}'::jsonb,
    status TEXT NOT NULL DEFAULT 'active',
    first_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT chk_open_source_project_cpe_vendor_tokens_token CHECK (btrim(cpe_vendor_token) <> ''),
    CONSTRAINT chk_open_source_project_cpe_vendor_tokens_association_type CHECK (
        association_type IN (
            'official_identifier',
            'owned_product_or_brand',
            'maintained_or_governed_project',
            'legacy_or_acquired',
            'manual_review'
        )
    ),
    CONSTRAINT chk_open_source_project_cpe_vendor_tokens_status CHECK (status IN ('active', 'needs_review', 'rejected', 'superseded')),
    CONSTRAINT chk_open_source_project_cpe_vendor_tokens_confidence CHECK (confidence IS NULL OR confidence BETWEEN 0 AND 1),
    CONSTRAINT chk_open_source_project_cpe_vendor_tokens_evidence_object CHECK (jsonb_typeof(evidence) = 'object'),
    CONSTRAINT uq_open_source_project_cpe_vendor_tokens UNIQUE (open_source_project_id, cpe_vendor_token)
);
```

Do not use these tables for unresolved CPE observations. They should contain resolved associations only.

Examples:

- `F5` company has CPE vendor token `f5` with `association_type = 'official_identifier'`.
- `F5` company has CPE vendor token `nginx` with `association_type = 'owned_product_or_brand'` when evidence supports F5 ownership or maintenance context.
- `Apache Software Foundation` organization has CPE vendor tokens `apache` and `apache_software_foundation` with `association_type = 'official_identifier'`.
- `CNCF` organization can have CPE vendor token `kubernetes` with `association_type = 'maintained_or_governed_project'` if the resolver intentionally maps that token to CNCF rather than the project row.
- `Nmap` open-source project has CPE vendor token `nmap` with `association_type = 'official_identifier'` if it resolves to the project rather than a company.

## Observation And Candidate Queues

Inputs from CPE, backoffice-v2, GitHub, domains, registry crawls, and manual imports should first land in observation/candidate tables when not fully trusted.

Recommended generic observation table:

```sql
CREATE TABLE identity_observations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    source_system TEXT NOT NULL,
    source_ref TEXT,
    raw_name TEXT,
    normalized_name TEXT,
    website TEXT,
    domain TEXT,
    cpe_vendor_token TEXT,
    github_owner TEXT,
    evidence JSONB NOT NULL DEFAULT '{}'::jsonb,
    context JSONB NOT NULL DEFAULT '{}'::jsonb,
    candidate_entity_type TEXT,
    candidate_company_id UUID REFERENCES companies(id) ON DELETE SET NULL,
    candidate_organization_id UUID REFERENCES organizations(id) ON DELETE SET NULL,
    candidate_open_source_project_id UUID REFERENCES open_source_projects(id) ON DELETE SET NULL,
    confidence REAL,
    status TEXT NOT NULL DEFAULT 'unresolved',
    first_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT chk_identity_observations_candidate_type CHECK (
        candidate_entity_type IS NULL OR candidate_entity_type IN ('company', 'organization', 'open_source_project')
    ),
    CONSTRAINT chk_identity_observations_status CHECK (
        status IN ('unresolved', 'candidate', 'resolved', 'rejected', 'ambiguous')
    ),
    CONSTRAINT chk_identity_observations_confidence CHECK (
        confidence IS NULL OR confidence BETWEEN 0 AND 1
    ),
    CONSTRAINT chk_identity_observations_evidence_object CHECK (jsonb_typeof(evidence) = 'object'),
    CONSTRAINT chk_identity_observations_context_object CHECK (jsonb_typeof(context) = 'object')
);
```

This is intentionally a queue/candidate table, not a resolved identity table. It may be generic because it stores unresolved observations, not trusted profile data.

Promotion rules:

- Promote to `companies`, `organizations`, or `open_source_projects` only when source evidence is strong enough or a human approves the resolution.
- Promote a CPE token to a resolved CPE token association only after the target entity is known.
- Keep ambiguous tokens in `identity_observations`.
- Rejected observations remain as durable negative evidence.

## Source And Observation Responsibilities

Keep `company_sources`; it is not deprecated by `identity_observations`.

`company_sources` remains the company-specific audit/source table for accepted profile facts on resolved company rows. It should be used for sources that support company fields such as legal name, website, headquarters, market, services, ownership notes, and registry data.

`identity_observations` is for raw identity inputs, candidate matches, ambiguous inputs, and negative evidence before or during resolution. When an observation is promoted into a resolved company fact, keep the original observation for traceability and copy or link the accepted evidence into the resolved entity's evidence fields or `company_sources`.

Organizations and open-source projects should get equivalent explicit source tables, such as `organization_sources` and `open_source_project_sources`, instead of sharing `company_sources`.

## Deferred Field Update Review Queue

This phase intentionally does not add a separate queue for proposed field updates to already-resolved entities.

Trusted imports and manual review can update resolved profile fields directly while recording source/evidence metadata in the appropriate resolved source table. Lower-confidence field changes, such as a newly discovered website or address for an existing company, should not be auto-applied unless the importer can meet the normal confidence rules. A future phase can introduce a dedicated field-update review workflow, but this design should not block the resolved entity table migration on that queue.

## Resolver API

Corpscout should expose a unified resolver API over the three precise root tables.

Example request:

```json
{
  "name": "Apache Software Foundation",
  "website": "https://www.apache.org",
  "cpe_vendor_tokens": ["apache", "apache_software_foundation"],
  "domains": ["apache.org"],
  "source_system": "backoffice-v2"
}
```

Example response:

```json
{
  "matched": true,
  "entity_type": "organization",
  "entity_id": "00000000-0000-0000-0000-000000000000",
  "display_name": "Apache Software Foundation",
  "confidence": 0.97,
  "resolution_reason": "matched official domain and CPE vendor tokens"
}
```

If no high-confidence match exists:

```json
{
  "matched": false,
  "status": "observation_created",
  "observation_id": "00000000-0000-0000-0000-000000000000"
}
```

The resolver should never create resolved rows from weak input automatically. It can create observations and candidates.

## Unified Read View

Provide a read-only SQL view for browsing/searching resolved entities:

```sql
CREATE VIEW v_resolved_entities AS
SELECT
    'company'::text AS entity_type,
    id AS entity_id,
    COALESCE(display_name, name) AS display_name,
    canonical_slug,
    website,
    status,
    updated_at
FROM companies
UNION ALL
SELECT
    'organization',
    id,
    display_name,
    canonical_slug,
    website,
    status,
    updated_at
FROM organizations
UNION ALL
SELECT
    'open_source_project',
    id,
    display_name,
    canonical_slug,
    website,
    status,
    updated_at
FROM open_source_projects;
```

This gives the UI and backoffice-v2 one search surface without requiring a root table.

## Backoffice-v2 Integration

Backoffice-v2 should store a typed mapping, not a generic company-only FK.

Recommended local mapping:

```sql
CREATE TABLE catalog_entity_corpscout_mappings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    local_kind TEXT NOT NULL,
    local_id UUID NOT NULL,
    corpscout_entity_type TEXT NOT NULL,
    corpscout_entity_id UUID NOT NULL,
    confidence REAL,
    status TEXT NOT NULL DEFAULT 'active',
    evidence JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT chk_catalog_entity_corpscout_mappings_local_kind CHECK (
        local_kind IN ('vendor', 'service_provider')
    ),
    CONSTRAINT chk_catalog_entity_corpscout_mappings_entity_type CHECK (
        corpscout_entity_type IN ('company', 'organization', 'open_source_project')
    ),
    CONSTRAINT chk_catalog_entity_corpscout_mappings_status CHECK (
        status IN ('active', 'needs_review', 'rejected', 'superseded')
    )
);
```

Backoffice-v2 flow:

1. Send vendor/provider identity hints to Corpscout.
2. Corpscout resolves to a company, organization, or open-source project, or creates an unresolved observation.
3. Backoffice-v2 stores the typed Corpscout mapping when matched.
4. Backoffice-v2 uses resolved CPE vendor token associations to map CPE/CVE data to local vendors.
5. Backoffice-v2 creates product candidates/products from CPE product tokens under the mapped local vendor.
6. Product and service enrichment remain in backoffice-v2.

## CPE Import Flow

1. Import CPE vendor tokens into `identity_observations`.
2. Group observations by normalized CPE vendor token and supporting context.
3. Resolve against known companies, organizations, and open-source projects.
4. If confidence is high, create or update the appropriate `*_cpe_vendor_tokens` row.
5. If confidence is medium or ambiguous, keep it as a candidate for review.
6. If rejected, mark the observation as `rejected` and keep the negative evidence.

Example:

```text
CPE token: nginx
Resolved entity: company F5
Association: company_cpe_vendor_tokens(company_id=F5, cpe_vendor_token='nginx')
Backoffice-v2: vendor F5, product NGINX
```

```text
CPE token: apache
Resolved entity: organization Apache Software Foundation
Association: organization_cpe_vendor_tokens(organization_id=ASF, cpe_vendor_token='apache')
Backoffice-v2: vendor Apache Software Foundation, products Apache HTTP Server, Tomcat, Struts, ...
```

```text
CPE token: nmap
Resolved entity: open-source project Nmap
Association: open_source_project_cpe_vendor_tokens(project_id=Nmap, cpe_vendor_token='nmap')
Backoffice-v2: vendor Nmap or mapped catalog vendor for Nmap, products created from CPE product tokens
```

## Review Rules

Automatic promotion requires strong evidence. Examples:

- official website/domain match
- registry or trusted source confirms identity
- CPE token appears consistently with known product ownership
- GitHub/repository/project site confirms the OSS project identity
- manual review confirms ambiguous cases

Do not promote when:

- only a raw CPE vendor token is available
- the token is a product or brand name with unclear owner
- the same token appears in unrelated contexts
- the source confidence is low
- the proposed entity type is uncertain

## Migration Strategy

1. Keep existing `companies` tables, including `company_sources`.
2. Add `canonical_slug`, `display_name`, `resolution_status`, `confidence`, and `evidence` to `companies`.
3. Backfill company slugs with the canonical slug algorithm, resolve collisions, then enforce `canonical_slug` uniqueness and `NOT NULL`.
4. Add `company_relationships`.
5. Add `organizations` and `open_source_projects`.
6. Add per-type CPE token association tables.
7. Add `identity_observations`.
8. Add `v_resolved_entities`.
9. Add resolver queries and API endpoints.
10. Teach backoffice-v2 to store typed Corpscout mappings.
11. Gradually migrate company-only export/import logic to the resolver API.

The first implementation should not remove existing company endpoints. It should add the new tables and resolver surface alongside them.

## Testing Plan

Database tests:

- migrations create all three resolved root tables
- no generic `entries` table is introduced
- `company_relationships` enforces resolved company parents on both sides
- company relationships reject self-references
- company relationships enforce current active/review uniqueness by subject, related company, and type
- company relationship consumers prefer `direct_parent` over `subsidiary_of` for parent-chain lookups
- `ultimate_parent` rows can be rebuilt or superseded when the direct-parent chain changes
- canonical slug generation is deterministic and handles collisions with the entity ID suffix
- existing `company_sources` data remains available after the migration
- CPE token uniqueness is enforced per entity type
- observation rows can exist without resolved entity IDs
- resolved CPE token tables require a concrete parent row
- `v_resolved_entities` returns companies, organizations, and open-source projects

Service tests:

- high-confidence inputs resolve to existing entities
- low-confidence inputs create observations only
- ambiguous CPE tokens stay unresolved
- rejected observations are not promoted later without explicit review

Backoffice-v2 integration tests:

- vendor can map to `company`
- vendor can map to `organization`
- vendor can map to `open_source_project`
- CPE vendor token association can select the correct local vendor
- CPE product token still creates product candidates in backoffice-v2, not Corpscout

## Implementation Defaults

- `companies.name` remains the registry/legal name for existing imports. `companies.display_name` is the consumer-facing label.
- GLEIF parent fields remain on `companies` as source fields, but resolved company-to-company ownership and parentage should be written to `company_relationships`.
- Parent-chain consumers should prefer `direct_parent` over `subsidiary_of`; `ultimate_parent` is a denormalized cache of the direct-parent chain.
- `company_sources` remains the source table for resolved company facts. `identity_observations` does not replace it.
- A field-update review queue is deferred. Do not add it to the first migration plan.
- The first implementation adds CPE token association tables and minimal source/evidence storage. Rich organization and project contact/forum tables can be phased in after resolver and CPE mapping are stable.
- Automatic CPE token promotion requires confidence `>= 0.90` plus evidence from at least one trusted source. Anything below that stays in `identity_observations`.
- The resolver writes observations synchronously in the request path. Queue-based enrichment can process unresolved observations later.
