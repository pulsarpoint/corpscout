import { useCallback, useEffect, useRef, useState } from "react";
import { Link, useSearchParams } from "react-router";
import {
  ChevronDown, ChevronUp, RefreshCw,
  CheckCircle2, XCircle, Clock, Loader2, Ban, AlertTriangle,
} from "lucide-react";
import { api } from "~/lib/api";
import type { Job, JobError, JobStat } from "~/types/api";
import { timeAgo, formatDate } from "~/lib/utils";
import { Badge } from "~/components/ui/badge";
import { Button } from "~/components/ui/button";
import { Skeleton } from "~/components/ui/skeleton";
import { Alert, AlertDescription } from "~/components/ui/alert";
import {
  Table, TableBody, TableCell, TableHead, TableHeader, TableRow,
} from "~/components/ui/table";

// ── State helpers ────────────────────────────────────────────────────────────

const STATE_LABELS: Record<string, string> = {
  available:  "Queued",
  running:    "Running",
  completed:  "Done",
  retryable:  "Will Retry",
  scheduled:  "Scheduled",
  failed:     "Failed",
  discarded:  "Abandoned",
  cancelled:  "Cancelled",
};

const KIND_LABELS: Record<string, string> = {
  source_crawl:   "Source Crawl",
  domain_resolve: "Domain Resolve",
  gleif_enrich:   "GLEIF Enrich",
};

const CANCELLABLE_STATES = new Set(["available", "running", "scheduled", "retryable"]);

function StateBadge({ state }: { state: string }) {
  const label = STATE_LABELS[state] ?? state;
  if (state === "completed")
    return <Badge className="bg-green-100 text-green-800 border-green-200 gap-1" variant="outline"><CheckCircle2 className="size-3" />{label}</Badge>;
  if (state === "running")
    return <Badge className="bg-blue-100 text-blue-800 border-blue-200 gap-1" variant="outline"><Loader2 className="size-3 animate-spin" />{label}</Badge>;
  if (state === "retryable")
    return <Badge className="bg-amber-100 text-amber-800 border-amber-200 gap-1" variant="outline"><AlertTriangle className="size-3" />{label}</Badge>;
  if (state === "failed" || state === "discarded")
    return <Badge className="bg-red-100 text-red-800 border-red-200 gap-1" variant="outline"><XCircle className="size-3" />{label}</Badge>;
  if (state === "scheduled")
    return <Badge className="bg-purple-100 text-purple-800 border-purple-200 gap-1" variant="outline"><Clock className="size-3" />{label}</Badge>;
  if (state === "cancelled")
    return <Badge className="bg-gray-100 text-gray-500 border-gray-200 gap-1" variant="outline"><Ban className="size-3" />{label}</Badge>;
  return <Badge variant="outline">{label}</Badge>;
}

function KindBadge({ kind }: { kind: string }) {
  if (kind === "source_crawl")
    return <Badge className="bg-indigo-100 text-indigo-800 border-indigo-200 text-xs" variant="outline">Source Crawl</Badge>;
  if (kind === "domain_resolve")
    return <Badge className="bg-cyan-100 text-cyan-800 border-cyan-200 text-xs" variant="outline">Domain Resolve</Badge>;
  if (kind === "gleif_enrich")
    return <Badge className="bg-emerald-100 text-emerald-800 border-emerald-200 text-xs" variant="outline">GLEIF Enrich</Badge>;
  return <Badge variant="outline" className="text-xs">{kind}</Badge>;
}

// ── Stats strip ──────────────────────────────────────────────────────────────

function summarise(stats: JobStat[]) {
  let running = 0, queued = 0, failed = 0, total = 0;
  for (const s of stats) {
    total += s.count;
    if (s.state === "running") running += s.count;
    if (s.state === "available" || s.state === "scheduled") queued += s.count;
    if (s.state === "failed" || s.state === "discarded" || s.state === "retryable") failed += s.count;
  }
  return { running, queued, failed, total };
}

function StatCard({ label, value, color }: { label: string; value: number; color: string }) {
  return (
    <div className="rounded-lg border bg-card px-4 py-3">
      <p className="text-xs text-muted-foreground uppercase tracking-wide">{label}</p>
      <p className={`mt-0.5 text-2xl font-semibold ${color}`}>{value.toLocaleString()}</p>
    </div>
  );
}

// ── Expanded row ─────────────────────────────────────────────────────────────

function ErrorList({ errors }: { errors: JobError[] }) {
  return (
    <div className="space-y-2">
      {errors.map((e, i) => (
        <div key={i} className="rounded border border-red-200 bg-red-50 dark:bg-red-950/20 px-3 py-2 text-xs">
          <div className="flex items-center justify-between mb-1">
            <span className="font-semibold text-red-700 dark:text-red-400">Attempt {e.attempt}</span>
            <span className="text-muted-foreground">{formatDate(e.at)}</span>
          </div>
          <p className="font-mono text-red-800 dark:text-red-300 whitespace-pre-wrap break-all">{e.error}</p>
          {e.trace && (
            <pre className="mt-1 text-muted-foreground whitespace-pre-wrap break-all opacity-70">{e.trace}</pre>
          )}
        </div>
      ))}
    </div>
  );
}

function ExpandedJob({ job, onCancel }: { job: Job; onCancel: (id: number) => Promise<void> }) {
  const [cancelling, setCancelling] = useState(false);
  const [cancelError, setCancelError] = useState<string>();

  async function handleCancel() {
    setCancelling(true);
    setCancelError(undefined);
    try {
      await onCancel(job.id);
    } catch {
      setCancelError("Failed to cancel job.");
    } finally {
      setCancelling(false);
    }
  }

  return (
    <div className="px-4 py-3 bg-muted/30 border-t space-y-3 text-sm">
      <div className="grid grid-cols-2 gap-x-8 gap-y-1 sm:grid-cols-4">
        <div><span className="text-muted-foreground">ID</span><br /><span className="font-mono">{job.id}</span></div>
        <div><span className="text-muted-foreground">Queue</span><br />{job.queue}</div>
        <div><span className="text-muted-foreground">Attempt</span><br />{job.attempt} / {job.max_attempts}</div>
        <div><span className="text-muted-foreground">Scheduled</span><br />{formatDate(job.scheduled_at)}</div>
        {job.finalized_at && (
          <div><span className="text-muted-foreground">Finalized</span><br />{formatDate(job.finalized_at)}</div>
        )}
      </div>
      <div>
        <p className="text-xs text-muted-foreground uppercase tracking-wide mb-1">Args</p>
        <pre className="rounded bg-muted px-3 py-2 text-xs font-mono overflow-auto max-h-32">
          {JSON.stringify(job.args, null, 2)}
        </pre>
      </div>
      {job.errors && job.errors.length > 0 && (
        <div>
          <p className="text-xs text-muted-foreground uppercase tracking-wide mb-1">Error history</p>
          <ErrorList errors={job.errors} />
        </div>
      )}
      {CANCELLABLE_STATES.has(job.state) && (
        <div className="flex items-center gap-3">
          <Button
            variant="destructive"
            size="sm"
            disabled={cancelling}
            onClick={(e) => { e.stopPropagation(); handleCancel(); }}
          >
            {cancelling ? <Loader2 className="size-3 animate-spin mr-1" /> : <Ban className="size-3 mr-1" />}
            Cancel job
          </Button>
          {cancelError && <span className="text-xs text-red-600">{cancelError}</span>}
        </div>
      )}
    </div>
  );
}

// ── Main page ────────────────────────────────────────────────────────────────

const RIVER_STATES = ["available", "running", "completed", "retryable", "scheduled", "discarded", "cancelled"] as const;

export default function JobsPage() {
  const [searchParams, setSearchParams] = useSearchParams();
  const [jobs, setJobs] = useState<Job[]>([]);
  const [stats, setStats] = useState<JobStat[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string>();
  const [expandedId, setExpandedId] = useState<number | null>(null);
  const [selected, setSelected] = useState<Set<number>>(new Set());
  const [bulkWorking, setBulkWorking] = useState(false);
  const [bulkResult, setBulkResult] = useState<string>();
  const timerRef = useRef<ReturnType<typeof setInterval> | undefined>(undefined);

  const page = Number(searchParams.get("page") ?? 1);
  const status = searchParams.get("status") ?? undefined;
  const kind = searchParams.get("kind") ?? undefined;

  const fetchData = useCallback(async () => {
    setError(undefined);
    try {
      const [jobsRes, statsRes] = await Promise.all([
        api.getJobs({ page, limit: 50, status, kind }),
        api.getJobStats(),
      ]);
      setJobs(jobsRes.items);
      setStats(statsRes);
      // Drop selections for jobs that are no longer on this page.
      setSelected((prev) => {
        const pageIds = new Set(jobsRes.items.map((j) => j.id));
        const next = new Set([...prev].filter((id) => pageIds.has(id)));
        return next.size === prev.size ? prev : next;
      });
    } catch {
      setError("Failed to load jobs.");
    } finally {
      setLoading(false);
    }
  }, [page, status, kind]);

  useEffect(() => {
    setLoading(true);
    setSelected(new Set());
    setBulkResult(undefined);
    fetchData();
    timerRef.current = setInterval(fetchData, 30_000);
    return () => clearInterval(timerRef.current);
  }, [fetchData]);

  async function cancelJob(id: number) {
    await api.cancelJob(id);
    await fetchData();
  }

  function setParam(key: string, value: string) {
    const next = new URLSearchParams(searchParams);
    if (value) next.set(key, value); else next.delete(key);
    next.set("page", "1");
    setSearchParams(next);
  }

  // ── Selection helpers ───────────────────────────────────────────────────────

  const cancellableOnPage = jobs.filter((j) => CANCELLABLE_STATES.has(j.state));
  const allCancellableSelected =
    cancellableOnPage.length > 0 &&
    cancellableOnPage.every((j) => selected.has(j.id));
  const someSelected = selected.size > 0;

  function toggleSelectAll() {
    if (allCancellableSelected) {
      setSelected(new Set());
    } else {
      setSelected(new Set(cancellableOnPage.map((j) => j.id)));
    }
  }

  function toggleJob(id: number) {
    setSelected((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id); else next.add(id);
      return next;
    });
  }

  // ── Bulk actions ────────────────────────────────────────────────────────────

  async function cancelSelected() {
    setBulkWorking(true);
    setBulkResult(undefined);
    try {
      const res = await api.cancelBulkByIds([...selected]);
      setSelected(new Set());
      setBulkResult(`Cancelled ${res.cancelled} job${res.cancelled !== 1 ? "s" : ""}.`);
      await fetchData();
    } catch {
      setBulkResult("Bulk cancel failed.");
    } finally {
      setBulkWorking(false);
    }
  }

  async function cancelAllMatching() {
    setBulkWorking(true);
    setBulkResult(undefined);
    try {
      const res = await api.cancelBulkByFilter({ status, kind });
      setSelected(new Set());
      setBulkResult(`Cancelled ${res.cancelled} job${res.cancelled !== 1 ? "s" : ""}.`);
      await fetchData();
    } catch {
      setBulkResult("Bulk cancel failed.");
    } finally {
      setBulkWorking(false);
    }
  }

  const summary = summarise(stats);

  // Count of cancellable jobs matching current filter (from stats).
  const cancellableFilterTotal = (() => {
    const matchingStates = status
      ? (CANCELLABLE_STATES.has(status) ? [status] : [])
      : [...CANCELLABLE_STATES];
    return stats
      .filter((s) => matchingStates.includes(s.state) && (!kind || s.kind === kind))
      .reduce((n, s) => n + s.count, 0);
  })();

  return (
    <div className="space-y-5">
      <div className="flex items-center justify-between">
        <h1 className="text-xl font-semibold">Jobs</h1>
        <div className="flex items-center gap-2 text-xs text-muted-foreground">
          <RefreshCw className="size-3" />
          Auto-refreshes every 30s
        </div>
      </div>

      {/* Stats */}
      <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
        <StatCard label="Total" value={summary.total} color="" />
        <StatCard label="Running" value={summary.running} color="text-blue-600" />
        <StatCard label="Queued" value={summary.queued} color="text-amber-600" />
        <StatCard label="Failed / Retrying" value={summary.failed} color={summary.failed > 0 ? "text-red-600" : ""} />
      </div>

      {/* Filters */}
      <div className="flex flex-wrap gap-2">
        <select
          className="h-9 rounded-md border border-input bg-background px-3 text-sm focus:outline-none focus:ring-1 focus:ring-ring"
          value={kind ?? ""}
          onChange={(e) => setParam("kind", e.target.value)}
        >
          <option value="">All types</option>
          <option value="source_crawl">Source Crawl</option>
          <option value="domain_resolve">Domain Resolve</option>
          <option value="gleif_enrich">GLEIF Enrich</option>
        </select>
        <select
          className="h-9 rounded-md border border-input bg-background px-3 text-sm focus:outline-none focus:ring-1 focus:ring-ring"
          value={status ?? ""}
          onChange={(e) => setParam("status", e.target.value)}
        >
          <option value="">All states</option>
          {RIVER_STATES.map((s) => (
            <option key={s} value={s}>{STATE_LABELS[s] ?? s}</option>
          ))}
        </select>
      </div>

      {/* Bulk action bar */}
      {someSelected && (
        <div className="flex items-center gap-3 rounded-md border border-destructive/30 bg-destructive/5 px-4 py-2">
          <span className="text-sm font-medium">{selected.size} selected</span>
          <Button
            variant="destructive"
            size="sm"
            disabled={bulkWorking}
            onClick={cancelSelected}
          >
            {bulkWorking ? <Loader2 className="size-3 animate-spin mr-1" /> : <Ban className="size-3 mr-1" />}
            Cancel selected
          </Button>
          {cancellableFilterTotal > selected.size && (
            <Button
              variant="outline"
              size="sm"
              disabled={bulkWorking}
              onClick={cancelAllMatching}
              className="border-destructive/50 text-destructive hover:bg-destructive/10"
            >
              {bulkWorking ? <Loader2 className="size-3 animate-spin mr-1" /> : <Ban className="size-3 mr-1" />}
              Cancel all {cancellableFilterTotal.toLocaleString()} matching
            </Button>
          )}
          <Button variant="ghost" size="sm" onClick={() => setSelected(new Set())}>
            Clear selection
          </Button>
          {bulkResult && <span className="text-sm text-muted-foreground">{bulkResult}</span>}
        </div>
      )}

      {/* "Cancel all matching" shortcut when nothing is selected but there are cancellable jobs */}
      {!someSelected && cancellableFilterTotal > 0 && (
        <div className="flex items-center gap-3">
          <Button
            variant="outline"
            size="sm"
            disabled={bulkWorking}
            onClick={cancelAllMatching}
            className="border-destructive/50 text-destructive hover:bg-destructive/10"
          >
            {bulkWorking ? <Loader2 className="size-3 animate-spin mr-1" /> : <Ban className="size-3 mr-1" />}
            Cancel all {cancellableFilterTotal.toLocaleString()} matching
          </Button>
          {bulkResult && <span className="text-sm text-muted-foreground">{bulkResult}</span>}
        </div>
      )}

      {loading ? (
        <div className="space-y-2">{Array.from({ length: 10 }).map((_, i) => <Skeleton key={i} className="h-10 w-full" />)}</div>
      ) : error ? (
        <Alert variant="destructive"><AlertDescription>{error}</AlertDescription></Alert>
      ) : jobs.length === 0 ? (
        <p className="py-10 text-center text-sm text-muted-foreground">No jobs match the current filters.</p>
      ) : (
        <>
          <div className="rounded-md border overflow-hidden">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead className="w-10">
                    <input
                      type="checkbox"
                      className="rounded border-input size-4 cursor-pointer accent-primary"
                      checked={allCancellableSelected}
                      ref={(el) => {
                        if (el) el.indeterminate = someSelected && !allCancellableSelected;
                      }}
                      onChange={toggleSelectAll}
                      title="Select all cancellable jobs on this page"
                    />
                  </TableHead>
                  <TableHead>Type</TableHead>
                  <TableHead>Subject</TableHead>
                  <TableHead>State</TableHead>
                  <TableHead>Attempts</TableHead>
                  <TableHead>Created</TableHead>
                  <TableHead>Error</TableHead>
                  <TableHead className="w-8"></TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {jobs.map((j) => (
                  <>
                    <TableRow
                      key={j.id}
                      className={`cursor-pointer hover:bg-muted/50 ${selected.has(j.id) ? "bg-muted/40" : ""}`}
                      onClick={() => setExpandedId(expandedId === j.id ? null : j.id)}
                    >
                      <TableCell onClick={(e) => e.stopPropagation()}>
                        {CANCELLABLE_STATES.has(j.state) ? (
                          <input
                            type="checkbox"
                            className="rounded border-input size-4 cursor-pointer accent-primary"
                            checked={selected.has(j.id)}
                            onChange={() => toggleJob(j.id)}
                          />
                        ) : (
                          <span className="block size-4" />
                        )}
                      </TableCell>
                      <TableCell><KindBadge kind={j.kind} /></TableCell>
                      <TableCell className="text-sm max-w-[200px] truncate">
                        {j.kind === "source_crawl" && j.subject ? (
                          <Link
                            to={`/sources/${j.subject}`}
                            className="hover:underline text-foreground font-medium"
                            onClick={(e) => e.stopPropagation()}
                          >
                            {j.subject}
                          </Link>
                        ) : (
                          <span className="text-muted-foreground">{j.subject ?? "—"}</span>
                        )}
                      </TableCell>
                      <TableCell><StateBadge state={j.state} /></TableCell>
                      <TableCell className="text-sm">
                        {j.attempt > 1
                          ? <span className="text-amber-600 font-medium">{j.attempt}/{j.max_attempts}</span>
                          : <span className="text-muted-foreground">{j.attempt}/{j.max_attempts}</span>}
                      </TableCell>
                      <TableCell className="text-sm text-muted-foreground">{timeAgo(j.created_at)}</TableCell>
                      <TableCell className="text-xs text-red-600 max-w-[250px] truncate font-mono">
                        {j.last_error ?? ""}
                      </TableCell>
                      <TableCell>
                        {expandedId === j.id
                          ? <ChevronUp className="size-4 text-muted-foreground" />
                          : <ChevronDown className="size-4 text-muted-foreground" />}
                      </TableCell>
                    </TableRow>
                    {expandedId === j.id && (
                      <TableRow key={`${j.id}-detail`}>
                        <TableCell colSpan={8} className="p-0">
                          <ExpandedJob job={j} onCancel={cancelJob} />
                        </TableCell>
                      </TableRow>
                    )}
                  </>
                ))}
              </TableBody>
            </Table>
          </div>

          <div className="flex items-center justify-between">
            <Button variant="outline" size="sm" disabled={page <= 1} onClick={() => setParam("page", String(page - 1))}>Previous</Button>
            <span className="text-sm text-muted-foreground">Page {page}</span>
            <Button variant="outline" size="sm" disabled={jobs.length < 50} onClick={() => setParam("page", String(page + 1))}>Next</Button>
          </div>
        </>
      )}
    </div>
  );
}
