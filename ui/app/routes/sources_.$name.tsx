import { useEffect, useState } from "react";
import { Link, useParams } from "react-router";
import {
  ChevronLeft, CheckCircle2, XCircle, Clock, Loader2,
  Pencil, Check, X, ExternalLink, FlaskConical, ChevronDown, ChevronUp,
} from "lucide-react";
import { toast } from "sonner";
import { api } from "~/lib/api";
import type { DataSource, PullRun, Job, SourceProbeResult } from "~/types/api";
import { timeAgo, formatDate } from "~/lib/utils";
import { Badge } from "~/components/ui/badge";
import { Card, CardContent, CardHeader, CardTitle } from "~/components/ui/card";
import { Skeleton } from "~/components/ui/skeleton";
import { Alert, AlertDescription } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { JobsTable } from "~/components/app/JobsTable";
import {
  Table, TableBody, TableCell, TableHead, TableHeader, TableRow,
} from "~/components/ui/table";

function statusBadge(status: string) {
  if (status === "completed")
    return <Badge className="bg-green-100 text-green-800 border-green-200" variant="outline"><CheckCircle2 className="size-3 mr-1" />completed</Badge>;
  if (status === "failed")
    return <Badge className="bg-red-100 text-red-800 border-red-200" variant="outline"><XCircle className="size-3 mr-1" />failed</Badge>;
  if (status === "running")
    return <Badge className="bg-blue-100 text-blue-800 border-blue-200" variant="outline"><Loader2 className="size-3 mr-1 animate-spin" />running</Badge>;
  return <Badge variant="outline"><Clock className="size-3 mr-1" />{status}</Badge>;
}

const PULL_RUNS_PAGE_SIZE = 15;
const JOBS_PAGE_SIZE = 10;

export default function SourceDetailPage() {
  const { name } = useParams<{ name: string }>();

  const [source, setSource] = useState<DataSource>();
  const [sourceLoading, setSourceLoading] = useState(true);
  const [sourceError, setSourceError] = useState<string>();

  const [editingInterval, setEditingInterval] = useState(false);
  const [intervalValue, setIntervalValue] = useState(0);
  const [savingInterval, setSavingInterval] = useState(false);

  const [probing, setProbing] = useState(false);
  const [probeResult, setProbeResult] = useState<SourceProbeResult>();
  const [probeExpanded, setProbeExpanded] = useState(false);

  const [pullRuns, setPullRuns] = useState<PullRun[]>([]);
  const [pullRunsPage, setPullRunsPage] = useState(1);
  const [pullRunsHasMore, setPullRunsHasMore] = useState(false);
  const [pullRunsLoading, setPullRunsLoading] = useState(true);

  const [jobs, setJobs] = useState<Job[]>([]);
  const [jobsPage, setJobsPage] = useState(1);
  const [jobsHasMore, setJobsHasMore] = useState(false);
  const [jobsLoading, setJobsLoading] = useState(true);

  useEffect(() => {
    if (!name) return;
    api.getSource(name)
      .then((s) => { setSource(s); setIntervalValue(s.crawl_interval_hours); })
      .catch(() => setSourceError("Source not found."))
      .finally(() => setSourceLoading(false));
  }, [name]);

  useEffect(() => {
    if (!name) return;
    setPullRunsLoading(true);
    api.getPullRuns(pullRunsPage, PULL_RUNS_PAGE_SIZE, name)
      .then((data) => {
        setPullRuns(data.items ?? []);
        setPullRunsHasMore((data.items?.length ?? 0) === PULL_RUNS_PAGE_SIZE);
      })
      .finally(() => setPullRunsLoading(false));
  }, [name, pullRunsPage]);

  useEffect(() => {
    if (!name) return;
    setJobsLoading(true);
    api.getJobs({ page: jobsPage, limit: JOBS_PAGE_SIZE, source: name })
      .then((data) => {
        setJobs(data.items ?? []);
        setJobsHasMore((data.items?.length ?? 0) === JOBS_PAGE_SIZE);
      })
      .finally(() => setJobsLoading(false));
  }, [name, jobsPage]);

  async function saveInterval() {
    if (!name || !source) return;
    const hours = Math.max(1, Math.round(intervalValue));
    setSavingInterval(true);
    try {
      await api.patchSource(name, { crawl_interval_hours: hours });
      setSource({ ...source, crawl_interval_hours: hours });
      setIntervalValue(hours);
      setEditingInterval(false);
      toast.success(`Crawl interval updated to ${hours}h.`);
    } catch {
      toast.error("Failed to update crawl interval.");
    } finally {
      setSavingInterval(false);
    }
  }

  function cancelInterval() {
    setIntervalValue(source?.crawl_interval_hours ?? 0);
    setEditingInterval(false);
  }

  async function runProbe() {
    if (!name) return;
    setProbing(true);
    setProbeResult(undefined);
    setProbeExpanded(true);
    try {
      const result = await api.probeSource(name);
      setProbeResult(result);
    } catch {
      toast.error("Probe request failed — check crawler connectivity.");
    } finally {
      setProbing(false);
    }
  }

  if (sourceLoading) return <Skeleton className="h-64 w-full" />;
  if (sourceError || !source) return <Alert variant="destructive"><AlertDescription>{sourceError}</AlertDescription></Alert>;

  const cfg = source.config;

  return (
    <div className="space-y-6">
      <div className="flex items-center gap-2">
        <Link to="/sources" className="text-sm text-muted-foreground hover:underline flex items-center gap-1">
          <ChevronLeft className="size-4" />
          Sources
        </Link>
      </div>

      {/* Info card */}
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-3">
            {source.display_name || source.name}
            {source.enabled
              ? <Badge className="bg-green-100 text-green-800 border-green-200" variant="outline">Enabled</Badge>
              : <Badge variant="outline" className="text-muted-foreground">Disabled</Badge>}
          </CardTitle>
          {source.description && (
            <p className="text-sm text-muted-foreground leading-relaxed">{source.description}</p>
          )}
        </CardHeader>
        <CardContent className="grid grid-cols-2 gap-4 sm:grid-cols-4">
          <div>
            <p className="text-xs text-muted-foreground uppercase tracking-wide">Internal name</p>
            <p className="mt-1 font-mono text-sm">{source.name}</p>
          </div>
          <div>
            <p className="text-xs text-muted-foreground uppercase tracking-wide">Type</p>
            <p className="mt-1 text-sm font-medium">{source.source_type}</p>
          </div>
          <div>
            <p className="text-xs text-muted-foreground uppercase tracking-wide">Adapter</p>
            <p className="mt-1 text-sm font-medium">{source.adapter_type}</p>
          </div>
          <div>
            <p className="text-xs text-muted-foreground uppercase tracking-wide">Crawl interval</p>
            <div className="mt-1 flex items-center gap-2">
              {editingInterval ? (
                <>
                  <input
                    type="number"
                    min={1}
                    className="w-20 rounded-md border border-input bg-background px-2 py-1 text-sm focus:outline-none focus:ring-1 focus:ring-ring"
                    value={intervalValue}
                    onChange={(e) => setIntervalValue(Number(e.target.value))}
                    onKeyDown={(e) => { if (e.key === "Enter") saveInterval(); if (e.key === "Escape") cancelInterval(); }}
                    autoFocus
                  />
                  <span className="text-sm text-muted-foreground">h</span>
                  <button onClick={saveInterval} disabled={savingInterval} className="text-green-600 hover:text-green-700 disabled:opacity-50" aria-label="Save"><Check className="size-4" /></button>
                  <button onClick={cancelInterval} disabled={savingInterval} className="text-muted-foreground hover:text-foreground disabled:opacity-50" aria-label="Cancel"><X className="size-4" /></button>
                </>
              ) : (
                <>
                  <span className="text-sm font-medium">{source.crawl_interval_hours}h</span>
                  <button onClick={() => setEditingInterval(true)} className="text-muted-foreground hover:text-foreground" aria-label="Edit interval"><Pencil className="size-3.5" /></button>
                </>
              )}
            </div>
          </div>
          <div>
            <p className="text-xs text-muted-foreground uppercase tracking-wide">Last crawled</p>
            <p className="mt-1 text-sm">{source.last_crawled_at ? timeAgo(source.last_crawled_at) : "Never"}</p>
          </div>
        </CardContent>
      </Card>

      {/* API details card */}
      {cfg && (
        <Card>
          <CardHeader>
            <CardTitle className="text-base flex items-center justify-between">
              <span>API Details</span>
              <Button size="sm" variant="outline" onClick={runProbe} disabled={probing}>
                {probing
                  ? <><Loader2 className="size-3.5 mr-1.5 animate-spin" />Probing…</>
                  : <><FlaskConical className="size-3.5 mr-1.5" />Probe API</>}
              </Button>
            </CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
              <div>
                <p className="text-xs text-muted-foreground uppercase tracking-wide">Endpoint URL</p>
                <p className="mt-1 font-mono text-sm break-all">{cfg.api_url}</p>
              </div>
              <div>
                <p className="text-xs text-muted-foreground uppercase tracking-wide">Protocol</p>
                <p className="mt-1 text-sm font-medium">{cfg.protocol}</p>
              </div>
              <div>
                <p className="text-xs text-muted-foreground uppercase tracking-wide">Page size</p>
                <p className="mt-1 text-sm font-medium">{cfg.page_size} records / request</p>
              </div>
              <div>
                <p className="text-xs text-muted-foreground uppercase tracking-wide">Auth</p>
                <p className="mt-1 text-sm">
                  {cfg.auth_env
                    ? <span className="font-mono text-amber-700 dark:text-amber-400">{cfg.auth_env}</span>
                    : <span className="text-muted-foreground">None required</span>}
                </p>
              </div>
              <div>
                <p className="text-xs text-muted-foreground uppercase tracking-wide">Fields extracted</p>
                <div className="mt-1 flex flex-wrap gap-1">
                  {cfg.fields.map((f) => (
                    <Badge key={f} variant="outline" className="text-xs font-mono">{f}</Badge>
                  ))}
                </div>
              </div>
              <div>
                <p className="text-xs text-muted-foreground uppercase tracking-wide">Documentation</p>
                <a
                  href={cfg.docs_url}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="mt-1 flex items-center gap-1 text-sm text-blue-600 hover:underline dark:text-blue-400"
                >
                  {cfg.docs_url} <ExternalLink className="size-3" />
                </a>
              </div>
            </div>
            {cfg.notes && (
              <div className="rounded-md bg-muted/50 px-3 py-2 text-sm text-muted-foreground">
                {cfg.notes}
              </div>
            )}

            {/* Probe result */}
            {probeResult && (
              <div className="rounded-md border">
                <button
                  className="flex w-full items-center justify-between px-3 py-2 text-sm font-medium"
                  onClick={() => setProbeExpanded((v) => !v)}
                >
                  <span className="flex items-center gap-2">
                    {probeResult.error
                      ? <XCircle className="size-4 text-red-500" />
                      : <CheckCircle2 className="size-4 text-green-500" />}
                    Probe result — {probeResult.duration_ms}ms
                    {!probeResult.error && (
                      <span className="text-muted-foreground font-normal">
                        · {probeResult.records_count} records · total {probeResult.total === -1 ? "unknown" : probeResult.total}
                        {probeResult.has_more ? " · more available" : ""}
                      </span>
                    )}
                  </span>
                  {probeExpanded ? <ChevronUp className="size-4" /> : <ChevronDown className="size-4" />}
                </button>
                {probeExpanded && (
                  <div className="border-t px-3 py-2">
                    {probeResult.error ? (
                      <p className="text-sm text-red-600 font-mono whitespace-pre-wrap">{probeResult.error}</p>
                    ) : (
                      <pre className="text-xs font-mono overflow-auto max-h-96 whitespace-pre-wrap">
                        {JSON.stringify(probeResult.sample ?? {}, null, 2)}
                      </pre>
                    )}
                  </div>
                )}
              </div>
            )}
          </CardContent>
        </Card>
      )}

      {/* Pull run history */}
      <div>
        <h2 className="mb-3 text-base font-semibold">Pull Run History</h2>
        {pullRunsLoading ? (
          <Skeleton className="h-40 w-full" />
        ) : pullRuns.length === 0 ? (
          <p className="text-sm text-muted-foreground">No pull runs yet.</p>
        ) : (
          <>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Started</TableHead>
                  <TableHead>Status</TableHead>
                  <TableHead className="text-right">Fetched</TableHead>
                  <TableHead className="text-right">Upserted</TableHead>
                  <TableHead>Completed</TableHead>
                  <TableHead>Error</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {pullRuns.map((run) => (
                  <TableRow key={run.id}>
                    <TableCell className="text-sm">{timeAgo(run.started_at)}</TableCell>
                    <TableCell>{statusBadge(run.status)}</TableCell>
                    <TableCell className="text-right font-mono text-sm">{run.records_fetched}</TableCell>
                    <TableCell className="text-right font-mono text-sm">{run.records_upserted}</TableCell>
                    <TableCell className="text-sm">{run.completed_at ? formatDate(run.completed_at) : "—"}</TableCell>
                    <TableCell className="text-sm text-red-600 max-w-xs truncate">{run.error_message ?? ""}</TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
            <div className="flex items-center justify-between mt-2">
              <Button size="sm" variant="outline" disabled={pullRunsPage === 1} onClick={() => setPullRunsPage((p) => p - 1)}>Previous</Button>
              <span className="text-sm text-muted-foreground">Page {pullRunsPage}</span>
              <Button size="sm" variant="outline" disabled={!pullRunsHasMore} onClick={() => setPullRunsPage((p) => p + 1)}>Next</Button>
            </div>
          </>
        )}
      </div>

      {/* Recent jobs */}
      <div>
        <h2 className="mb-3 text-base font-semibold">Recent Jobs</h2>
        {jobsLoading ? (
          <Skeleton className="h-40 w-full" />
        ) : jobs.length === 0 ? (
          <p className="text-sm text-muted-foreground">No jobs found.</p>
        ) : (
          <>
            <JobsTable jobs={jobs} />
            <div className="flex items-center justify-between mt-2">
              <Button size="sm" variant="outline" disabled={jobsPage === 1} onClick={() => setJobsPage((p) => p - 1)}>Previous</Button>
              <span className="text-sm text-muted-foreground">Page {jobsPage}</span>
              <Button size="sm" variant="outline" disabled={!jobsHasMore} onClick={() => setJobsPage((p) => p + 1)}>Next</Button>
            </div>
          </>
        )}
      </div>
    </div>
  );
}
