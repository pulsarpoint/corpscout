export interface StatsResponse {
  total_companies: number;
  total_domains: number;
  active_domains: number;
  pending_review: number;
  enabled_sources: number;
  pull_runs_completed_today: number;
  pull_runs_failed_today: number;
  records_upserted_24h: number;
  records_upserted_7d: number;
}

export type Signal = "registry_website" | "wikidata" | "certsh" | "whois" | "search";

export interface ReviewCandidate {
  id: string;
  company_id: string;
  domain_id: string;
  relationship_type: string;
  status: string;
  signal: Signal;
  confidence: number;
  evidence: Record<string, unknown> | null;
  first_seen_at: string;
  last_seen_at: string;
  company_name: string;
  domain: string;
}

export interface ReviewListResponse {
  items: ReviewCandidate[];
  page: number;
  limit: number;
}

export interface Company {
  id: string;
  lei: string | null;
  name: string;
  country_id: string;
  registration_number: string | null;
  status: string;
  created_at: string;
  updated_at: string;
}

export interface CompanyListResponse {
  items: Company[];
  total: number;
  page: number;
  limit: number;
}

export interface CompanyDomainEntry {
  domain: string;
  id: string;
  company_id: string;
  domain_id: string;
  relationship_type: string;
  status: string;
  signal: string;
  confidence: number;
  evidence: Record<string, unknown> | null;
  first_seen_at: string;
  last_seen_at: string;
}

export interface CompanyDetail extends Company {
  domains: CompanyDomainEntry[];
}

export interface DomainWithCompany {
  domain: string;
  company_name: string;
  id: string;
  company_id: string;
  domain_id: string;
  relationship_type: string;
  status: string;
  signal: string;
  confidence: number;
  evidence: Record<string, unknown> | null;
  first_seen_at: string;
  last_seen_at: string;
}

export interface DomainListResponse {
  items: DomainWithCompany[];
  total: number;
  page: number;
  limit: number;
}

export interface SourceConfig {
  api_url: string;
  docs_url: string;
  protocol: string;
  page_size: number;
  fields: string[];
  auth_env: string | null;
  notes: string;
}

export interface DataSource {
  id: string;
  name: string;
  display_name: string;
  description: string;
  source_type: string;
  adapter_type: string;
  enabled: boolean;
  crawl_interval_hours: number;
  last_crawled_at: string | null;
  config: SourceConfig | null;
}

export interface SourceProbeResult {
  records_count: number;
  total: number;
  has_more: boolean;
  sample: Record<string, unknown> | null;
  error: string | null;
  duration_ms: number;
}

export interface PullRun {
  id: string;
  source_name: string;
  river_job_id: number | null;
  started_at: string;
  completed_at: string | null;
  status: string;
  records_fetched: number;
  records_upserted: number;
  error_message: string | null;
}

export interface PullRunsResponse {
  items: PullRun[];
  page: number;
  limit: number;
}

export interface JobError {
  at: string;
  attempt: number;
  error: string;
  trace: string;
}

export interface Job {
  id: number;
  kind: string;
  state: string;
  args: Record<string, unknown>;
  attempt: number;
  max_attempts: number;
  queue: string;
  priority: number;
  created_at: string;
  scheduled_at: string;
  finalized_at: string | null;
  last_error: string | null;
  subject: string | null;
  errors: JobError[] | null;
}

export interface JobStat {
  kind: string;
  state: string;
  count: number;
}

export interface JobsResponse {
  items: Job[];
  page: number;
  limit: number;
}

export interface Country {
  id: string;
  name: string;
  iso_alpha2: string;
}

// PostgREST view types

export interface VCompany {
  id: string;
  name: string;
  short_name: string | null;
  registration_number: string | null;
  lei: string | null;
  status: string;
  website: string | null;
  short_description: string | null;
  founded_year: number | null;
  employee_estimate: Record<string, unknown>;
  revenue_estimate: Record<string, unknown>;
  ownership: Record<string, unknown>;
  created_at: string;
  updated_at: string;
  country_id: string;
  country_name: string;
  country_iso2: string;
  primary_source: string | null;
  primary_source_display_name: string | null;
  domain_count: number;
  headquarters_location: string | null;
}

export interface VCompanyLocation {
  id: string;
  company_id: string;
  location_type: "headquarters" | "registered_address" | "office";
  label: string | null;
  address_line1: string | null;
  address_line2: string | null;
  city: string | null;
  region: string | null;
  postal_code: string | null;
  country: string | null;
  country_code: string | null;
  latitude: number | null;
  longitude: number | null;
  source: string;
  confidence: number | null;
  evidence: Record<string, unknown>;
  created_at: string;
  updated_at: string;
}

export interface VCompanyPhone {
  id: string;
  company_id: string;
  phone: string;
  description: string | null;
  purpose: string;
  source: string;
  confidence: number | null;
  evidence: Record<string, unknown>;
  created_at: string;
  updated_at: string;
}

export interface VCompanyEmail {
  id: string;
  company_id: string;
  email: string;
  description: string | null;
  purpose: string;
  name: string | null;
  source: string;
  confidence: number | null;
  evidence: Record<string, unknown>;
  created_at: string;
  updated_at: string;
}

export interface VCompanyIndustry {
  id: string;
  company_id: string;
  industry: string;
  source: string;
  confidence: number | null;
  created_at: string;
}

export interface VCompanyMarket {
  id: string;
  company_id: string;
  market: string;
  source: string;
  confidence: number | null;
  created_at: string;
}

export interface VCompanyService {
  id: string;
  company_id: string;
  service: string;
  description: string | null;
  source: string;
  confidence: number | null;
  created_at: string;
}

export interface VCompanySource {
  company_id: string;
  external_id: string | null;
  fetched_at: string | null;
  source_id: string;
  source_name: string;
  source_display_name: string;
  source_type: string;
}

export interface VDomain {
  id: string;
  domain: string;
  first_seen_at: string | null;
  last_verified_at: string | null;
  company_count: number;
  max_confidence: number | null;
  primary_company_name: string | null;
  primary_company_id: string | null;
  primary_signal: string | null;
}
