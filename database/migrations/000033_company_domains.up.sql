CREATE TABLE company_domains (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    native_id     TEXT NOT NULL,
    source        TEXT NOT NULL,
    domain        TEXT NOT NULL,
    signal        TEXT NOT NULL,
    confidence    INT  NOT NULL DEFAULT 0,
    first_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_seen_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (native_id, source, domain)
);

CREATE INDEX idx_company_domains_native_id ON company_domains (native_id);
CREATE INDEX idx_company_domains_domain    ON company_domains (domain);
