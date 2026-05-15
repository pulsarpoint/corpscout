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
