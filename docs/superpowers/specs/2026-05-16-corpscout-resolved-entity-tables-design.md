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

Each resolved type gets its own CPE token table:

```sql
CREATE TABLE company_cpe_vendor_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id UUID NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
    cpe_vendor_token TEXT NOT NULL,
    association_type TEXT NOT NULL DEFAULT 'owned_or_maintained',
    source TEXT NOT NULL DEFAULT 'manual',
    confidence REAL,
    evidence JSONB NOT NULL DEFAULT '{}'::jsonb,
    status TEXT NOT NULL DEFAULT 'active',
    first_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT chk_company_cpe_vendor_tokens_token CHECK (btrim(cpe_vendor_token) <> ''),
    CONSTRAINT chk_company_cpe_vendor_tokens_status CHECK (status IN ('active', 'needs_review', 'rejected', 'superseded')),
    CONSTRAINT chk_company_cpe_vendor_tokens_confidence CHECK (confidence IS NULL OR confidence BETWEEN 0 AND 1),
    CONSTRAINT chk_company_cpe_vendor_tokens_evidence_object CHECK (jsonb_typeof(evidence) = 'object'),
    CONSTRAINT uq_company_cpe_vendor_tokens UNIQUE (company_id, cpe_vendor_token)
);
```

Mirror that shape for:

- `organization_cpe_vendor_tokens`
- `open_source_project_cpe_vendor_tokens`

Do not use these tables for unresolved CPE observations. They should contain resolved associations only.

Examples:

- `F5` company has CPE vendor tokens `f5` and `nginx` when evidence supports F5 ownership/maintenance context.
- `Apache Software Foundation` organization has CPE vendor tokens `apache` and `apache_software_foundation`.
- `Nmap` open-source project has CPE vendor token `nmap` if it resolves to the project rather than a company.

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

1. Keep existing `companies` tables.
2. Add `canonical_slug`, `display_name`, `resolution_status`, `confidence`, and `evidence` to `companies`.
3. Add `organizations` and `open_source_projects`.
4. Add per-type CPE token association tables.
5. Add `identity_observations`.
6. Add `v_resolved_entities`.
7. Add resolver queries and API endpoints.
8. Teach backoffice-v2 to store typed Corpscout mappings.
9. Gradually migrate company-only export/import logic to the resolver API.

The first implementation should not remove existing company endpoints. It should add the new tables and resolver surface alongside them.

## Testing Plan

Database tests:

- migrations create all three resolved root tables
- no generic `entries` table is introduced
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
- The first implementation adds CPE token association tables and minimal source/evidence storage. Rich organization and project contact/forum tables can be phased in after resolver and CPE mapping are stable.
- Automatic CPE token promotion requires confidence `>= 0.90` plus evidence from at least one trusted source. Anything below that stays in `identity_observations`.
- The resolver writes observations synchronously in the request path. Queue-based enrichment can process unresolved observations later.
