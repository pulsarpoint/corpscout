import type {
  StatsResponse,
  ReviewListResponse,
  CompanySuggestionListResponse,
  CompanyListResponse,
  CompanyDetail,
  DomainListResponse,
  DataSource,
  SourceProbeResult,
  PullRunsResponse,
  JobsResponse,
  JobStat,
  Job,
  Country,
  DomainDetail,
  DomainCrawlJob,
  DomainCrawlPage,
  TriggerCrawlRequest,
  TriggerCrawlResponse,
  DomainImportBatch,
  EnrichmentSourcesResponse,
  CompanyFinancial,
  CompanyFinancialPending,
} from "~/types/api";

const BASE = "/api/v1";

export class ApiError extends Error {
  status: number;

  constructor(status: number, message: string) {
    super(message);
    this.name = "ApiError";
    this.status = status;
  }
}

export function errorMessage(error: unknown, fallback: string): string {
  return error instanceof Error && error.message ? error.message : fallback;
}

async function responseError(res: Response): Promise<ApiError> {
  const fallback = `${res.status} ${res.statusText}`;
  const contentType = res.headers.get("Content-Type") ?? "";

  if (contentType.includes("application/json")) {
    try {
      const body = await res.json() as { error?: unknown; message?: unknown };
      const message = typeof body.error === "string"
        ? body.error
        : typeof body.message === "string"
          ? body.message
          : fallback;
      return new ApiError(res.status, message);
    } catch {
      return new ApiError(res.status, fallback);
    }
  }

  const text = await res.text();
  return new ApiError(res.status, text.trim() || fallback);
}

async function get<T>(path: string): Promise<T> {
  const res = await fetch(BASE + path);
  if (!res.ok) throw await responseError(res);
  return res.json() as Promise<T>;
}

async function post<T>(path: string, body: unknown): Promise<T> {
  const res = await fetch(BASE + path, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
  if (!res.ok) throw await responseError(res);
  return res.json() as Promise<T>;
}

async function patch<T>(path: string, body: unknown): Promise<T> {
  const res = await fetch(BASE + path, {
    method: "PATCH",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
  if (!res.ok) throw await responseError(res);
  return res.json() as Promise<T>;
}

async function getRaw(path: string): Promise<string> {
  const res = await fetch(BASE + path);
  if (!res.ok) throw await responseError(res);
  return res.text();
}

export const api = {
  getStats: () => get<StatsResponse>("/stats"),

  getReview: (
    page = 1,
    limit = 50,
    filters?: { signal?: string; min_confidence?: number; q?: string },
  ) => {
    const qs = new URLSearchParams({ page: String(page), limit: String(limit) });
    if (filters?.signal) qs.set("signal", filters.signal);
    if (filters?.min_confidence != null) qs.set("min_confidence", String(filters.min_confidence));
    if (filters?.q) qs.set("q", filters.q);
    return get<ReviewListResponse>(`/review?${qs.toString()}`);
  },

  createReview: (id: string, action: "approved" | "rejected" | "superseded") =>
    post<unknown>(`/review/${id}/reviews`, { action, reviewed_by: "ops" }),

  bulkReview: (ids: string[], action: "approved" | "rejected" | "superseded") =>
    post<{ updated: number; skipped: number }>("/review/bulk", { ids, action }),

  getReviewIDs: (filters?: { signal?: string; min_confidence?: number; q?: string }) => {
    const qs = new URLSearchParams();
    if (filters?.signal) qs.set("signal", filters.signal);
    if (filters?.min_confidence != null) qs.set("min_confidence", String(filters.min_confidence));
    if (filters?.q) qs.set("q", filters.q);
    return get<{ ids: string[] }>(`/review/ids${qs.toString() ? `?${qs.toString()}` : ""}`);
  },

  getCompanySuggestions: (page = 1, limit = 50) =>
    get<CompanySuggestionListResponse>(`/suggestions/companies?page=${page}&limit=${limit}`),

  getCompanySuggestionIDs: () =>
    get<{ ids: string[] }>("/suggestions/companies/ids"),

  bulkCompanySuggestions: (ids: string[], action: "approve" | "reject") =>
    post<{ updated: number; skipped: number }>("/suggestions/companies/bulk", { ids, action }),

  approveCompanySuggestion: (id: string) =>
    post<unknown>(`/suggestions/companies/${id}/approve`, { reviewed_by: "ops" }),

  rejectCompanySuggestion: (id: string) =>
    post<unknown>(`/suggestions/companies/${id}/reject`, { reviewed_by: "ops" }),

  getCompanies: (params: {
    page?: number;
    limit?: number;
    country?: string;
    source?: string;
    status?: string;
    q?: string;
  }) => {
    const qs = new URLSearchParams();
    if (params.page) qs.set("page", String(params.page));
    if (params.limit) qs.set("limit", String(params.limit));
    if (params.country) qs.set("country", params.country);
    if (params.source) qs.set("source", params.source);
    if (params.status) qs.set("status", params.status);
    if (params.q) qs.set("q", params.q);
    const q = qs.toString();
    return get<CompanyListResponse>(`/companies${q ? `?${q}` : ""}`);
  },

  getCompany: (id: string) => get<CompanyDetail>(`/companies/${id}`),

  getDomains: (params: {
    page?: number;
    limit?: number;
    min_confidence?: number;
    signal?: string;
  }) => {
    const qs = new URLSearchParams();
    if (params.page) qs.set("page", String(params.page));
    if (params.limit) qs.set("limit", String(params.limit));
    if (params.min_confidence) qs.set("min_confidence", String(params.min_confidence));
    if (params.signal) qs.set("signal", params.signal);
    const q = qs.toString();
    return get<DomainListResponse>(`/domains${q ? `?${q}` : ""}`);
  },

  getSources: () => get<DataSource[]>("/sources"),

  getSource: (name: string) => get<DataSource>(`/sources/${name}`),

  patchSource: (
    name: string,
    body: {
      enabled?: boolean;
      schedule_enabled?: boolean;
      schedule_kind?: DataSource["schedule_kind"];
      schedule_expression?: string | null;
      config?: Record<string, unknown>;
    },
  ) =>
    patch<{ status: string }>(`/sources/${name}`, body),

  triggerSource: (name: string) =>
    post<{ status: string }>(`/sources/${name}/trigger`, {}),

  probeSource: (name: string) =>
    post<SourceProbeResult>(`/sources/${name}/probe`, {}),

  retryRawInput: (name: string, id: string) =>
    post<{ status: string }>(`/sources/${name}/raw-inputs/${id}/retry`, {}),

  ignoreRawInput: (name: string, id: string) =>
    post<{ status: string }>(`/sources/${name}/raw-inputs/${id}/ignore`, {}),

  getPullRuns: (page = 1, limit = 20, source?: string) => {
    const qs = new URLSearchParams({ page: String(page), limit: String(limit) });
    if (source) qs.set("source", source);
    return get<PullRunsResponse>(`/pull-runs?${qs.toString()}`);
  },

  getJobs: (params: { page?: number; limit?: number; status?: string; source?: string; kind?: string }) => {
    const qs = new URLSearchParams();
    if (params.page) qs.set("page", String(params.page));
    if (params.limit) qs.set("limit", String(params.limit));
    if (params.status) qs.set("status", params.status);
    if (params.source) qs.set("source", params.source);
    if (params.kind) qs.set("kind", params.kind);
    const q = qs.toString();
    return get<JobsResponse>(`/jobs${q ? `?${q}` : ""}`);
  },

  getJobStats: () => get<JobStat[]>("/jobs/stats"),

  getJob: (id: number) => get<Job>(`/jobs/${id}`),

  cancelJob: (id: number) => post<{ status: string; id: number }>(`/jobs/${id}/cancel`, {}),

  cancelBulkByIds: (ids: number[]) =>
    post<{ cancelled: number }>("/jobs/cancel-bulk", { ids }),

  cancelBulkByFilter: (filter: { status?: string; kind?: string }) =>
    post<{ cancelled: number }>("/jobs/cancel-bulk", { filter }),

  getCountries: () => get<Country[]>("/countries"),

  getCompanyEnrichmentSources: (id: string) =>
    get<EnrichmentSourcesResponse>(`/companies/${id}/enrichment-sources`),

  enrichCompanyFromSource: (id: string, source: string) =>
    post<{ job_id: number }>(`/companies/${id}/enrich-from-source`, { source }),

  getFinancialSuggestions: (page = 1, limit = 50) =>
    get<{ items: CompanyFinancialPending[]; total: number; page: number; limit: number }>(
      `/financials/review?page=${page}&limit=${limit}`,
    ),

  getFinancialSuggestionIDs: () =>
    get<{ ids: string[] }>("/financials/review/ids"),

  bulkFinancialSuggestions: (ids: string[], action: "approve" | "reject") =>
    post<void>("/financials/review/bulk", { ids, action }),

  getCompanyFinancials: (companyId: string) =>
    get<{ items: CompanyFinancial[] }>(`/companies/${companyId}/financials`),

  reviewFinancial: (id: string, action: "approve" | "reject") =>
    post<void>(`/financials/${id}/review`, { action }),
};

export function triggerDomainCrawl(domainId: string, req: TriggerCrawlRequest): Promise<TriggerCrawlResponse> {
  return post<TriggerCrawlResponse>(`/domains/${domainId}/crawl`, req);
}

export function getDomain(domainId: string): Promise<DomainDetail> {
  return get<DomainDetail>(`/domains/${domainId}`);
}

export function getDomainCrawlJobs(domainId: string): Promise<DomainCrawlJob[]> {
  return get<DomainCrawlJob[]>(`/domains/${domainId}/crawl-jobs`);
}

export function getDomainCrawlPages(domainId: string, jobId: string): Promise<DomainCrawlPage[]> {
  return get<DomainCrawlPage[]>(`/domains/${domainId}/crawl-jobs/${jobId}/pages`);
}

export function getDomainCrawlPageContent(
  domainId: string,
  jobId: string,
  pageNum: number,
  type: "markdown" | "html" | "headers"
): Promise<string> {
  return getRaw(`/domains/${domainId}/crawl-jobs/${jobId}/pages/${pageNum}/${type}`);
}

export function getDomainFaviconUrl(domainId: string, jobId: string): string {
  return `/api/v1/domains/${domainId}/crawl-jobs/${jobId}/favicon`;
}

export async function uploadDomainsCSV(file: File): Promise<DomainImportBatch> {
  const formData = new FormData();
  formData.append("file", file);
  const res = await fetch(`${BASE}/domains/import`, {
    method: "POST",
    body: formData,
  });
  if (!res.ok) throw await responseError(res);
  return res.json() as Promise<DomainImportBatch>;
}

export function getImportBatches(limit = 10): Promise<DomainImportBatch[]> {
  return get<DomainImportBatch[]>(`/domains/import-batches?limit=${limit}`);
}
