import type {
  StatsResponse,
  ReviewListResponse,
  CompanyListResponse,
  CompanyDetail,
  DomainListResponse,
  DataSource,
  SourceProbeResult,
  PullRunsResponse,
  JobsResponse,
  Country,
} from "~/types/api";

const BASE = "/api/v1";

async function get<T>(path: string): Promise<T> {
  const res = await fetch(BASE + path);
  if (!res.ok) throw new Error(`${res.status} ${res.statusText}`);
  return res.json() as Promise<T>;
}

async function post<T>(path: string, body: unknown): Promise<T> {
  const res = await fetch(BASE + path, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
  if (!res.ok) throw new Error(`${res.status} ${res.statusText}`);
  return res.json() as Promise<T>;
}

async function patch<T>(path: string, body: unknown): Promise<T> {
  const res = await fetch(BASE + path, {
    method: "PATCH",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
  if (!res.ok) throw new Error(`${res.status} ${res.statusText}`);
  return res.json() as Promise<T>;
}

export const api = {
  getStats: () => get<StatsResponse>("/stats"),

  getReview: (page = 1, limit = 50) =>
    get<ReviewListResponse>(`/review?page=${page}&limit=${limit}`),

  createReview: (id: string, action: "approved" | "rejected" | "superseded") =>
    post<unknown>(`/review/${id}/reviews`, { action, reviewed_by: "" }),

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

  patchSource: (name: string, body: { enabled?: boolean; crawl_interval_hours?: number }) =>
    patch<{ status: string }>(`/sources/${name}`, body),

  triggerSource: (name: string) =>
    post<{ status: string }>(`/sources/${name}/trigger`, {}),

  probeSource: (name: string) =>
    post<SourceProbeResult>(`/sources/${name}/probe`, {}),

  getPullRuns: (page = 1, limit = 20, source?: string) => {
    const qs = new URLSearchParams({ page: String(page), limit: String(limit) });
    if (source) qs.set("source", source);
    return get<PullRunsResponse>(`/pull-runs?${qs.toString()}`);
  },

  getJobs: (params: { page?: number; limit?: number; status?: string; source?: string }) => {
    const qs = new URLSearchParams();
    if (params.page) qs.set("page", String(params.page));
    if (params.limit) qs.set("limit", String(params.limit));
    if (params.status) qs.set("status", params.status);
    if (params.source) qs.set("source", params.source);
    const q = qs.toString();
    return get<JobsResponse>(`/jobs${q ? `?${q}` : ""}`);
  },

  getCountries: () => get<Country[]>("/countries"),
};
