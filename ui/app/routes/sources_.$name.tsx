import { useEffect, useState } from "react";
import { Link, useParams } from "react-router";
import { ChevronLeft, CheckCircle2, XCircle, Clock, Loader2 } from "lucide-react";
import { api } from "~/lib/api";
import type { DataSource, PullRun, Job } from "~/types/api";
import { timeAgo, formatDate } from "~/lib/utils";
import { Badge } from "~/components/ui/badge";
import { Card, CardContent, CardHeader, CardTitle } from "~/components/ui/card";
import { Skeleton } from "~/components/ui/skeleton";
import { Alert, AlertDescription } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
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

function jobStateBadge(state: string) {
  const colors: Record<string, string> = {
    completed: "bg-green-100 text-green-800 border-green-200",
    failed: "bg-red-100 text-red-800 border-red-200",
    running: "bg-blue-100 text-blue-800 border-blue-200",
    available: "bg-yellow-100 text-yellow-800 border-yellow-200",
    scheduled: "bg-gray-100 text-gray-700 border-gray-200",
    retryable: "bg-orange-100 text-orange-800 border-orange-200",
    cancelled: "bg-gray-100 text-gray-500 border-gray-200",
    discarded: "bg-gray-100 text-gray-500 border-gray-200",
  };
  return (
    <Badge className={colors[state] ?? "bg-gray-100 text-gray-700"} variant="outline">
      {state}
    </Badge>
  );
}

const PULL_RUNS_PAGE_SIZE = 15;
const JOBS_PAGE_SIZE = 10;

export default function SourceDetailPage() {
  const { name } = useParams<{ name: string }>();

  const [source, setSource] = useState<DataSource>();
  const [sourceLoading, setSourceLoading] = useState(true);
  const [sourceError, setSourceError] = useState<string>();

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
      .then(setSource)
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

  if (sourceLoading) return <Skeleton className="h-64 w-full" />;
  if (sourceError || !source) return <Alert variant="destructive"><AlertDescription>{sourceError}</AlertDescription></Alert>;

  return (
    <div className="space-y-6">
      <div className="flex items-center gap-2">
        <Link to="/sources" className="text-sm text-muted-foreground hover:underline flex items-center gap-1">
          <ChevronLeft className="size-4" />
          Sources
        </Link>
      </div>

      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-3">
            {source.display_name || source.name}
            {source.enabled
              ? <Badge className="bg-green-100 text-green-800 border-green-200" variant="outline">Enabled</Badge>
              : <Badge variant="outline" className="text-muted-foreground">Disabled</Badge>}
          </CardTitle>
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
            <p className="mt-1 text-sm font-medium">{source.crawl_interval_hours}h</p>
          </div>
          <div>
            <p className="text-xs text-muted-foreground uppercase tracking-wide">Last crawled</p>
            <p className="mt-1 text-sm">
              {source.last_crawled_at ? timeAgo(source.last_crawled_at) : "Never"}
            </p>
          </div>
        </CardContent>
      </Card>

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
                    <TableCell className="text-sm">
                      {run.completed_at ? formatDate(run.completed_at) : "—"}
                    </TableCell>
                    <TableCell className="text-sm text-red-600 max-w-xs truncate">
                      {run.error_message ?? ""}
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
            <div className="flex items-center justify-between mt-2">
              <Button
                size="sm"
                variant="outline"
                disabled={pullRunsPage === 1}
                onClick={() => setPullRunsPage((p) => p - 1)}
              >
                Previous
              </Button>
              <span className="text-sm text-muted-foreground">Page {pullRunsPage}</span>
              <Button
                size="sm"
                variant="outline"
                disabled={!pullRunsHasMore}
                onClick={() => setPullRunsPage((p) => p + 1)}
              >
                Next
              </Button>
            </div>
          </>
        )}
      </div>

      <div>
        <h2 className="mb-3 text-base font-semibold">Recent Jobs</h2>
        {jobsLoading ? (
          <Skeleton className="h-40 w-full" />
        ) : jobs.length === 0 ? (
          <p className="text-sm text-muted-foreground">No jobs found.</p>
        ) : (
          <>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>ID</TableHead>
                  <TableHead>Kind</TableHead>
                  <TableHead>State</TableHead>
                  <TableHead className="text-right">Attempt</TableHead>
                  <TableHead>Created</TableHead>
                  <TableHead>Finalized</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {jobs.map((job) => (
                  <TableRow key={job.id}>
                    <TableCell className="font-mono text-xs">{job.id}</TableCell>
                    <TableCell className="text-sm">{job.kind}</TableCell>
                    <TableCell>{jobStateBadge(job.state)}</TableCell>
                    <TableCell className="text-right text-sm">{job.attempt}/{job.max_attempts}</TableCell>
                    <TableCell className="text-sm">{timeAgo(job.created_at)}</TableCell>
                    <TableCell className="text-sm">
                      {job.finalized_at ? timeAgo(job.finalized_at) : "—"}
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
            <div className="flex items-center justify-between mt-2">
              <Button
                size="sm"
                variant="outline"
                disabled={jobsPage === 1}
                onClick={() => setJobsPage((p) => p - 1)}
              >
                Previous
              </Button>
              <span className="text-sm text-muted-foreground">Page {jobsPage}</span>
              <Button
                size="sm"
                variant="outline"
                disabled={!jobsHasMore}
                onClick={() => setJobsPage((p) => p + 1)}
              >
                Next
              </Button>
            </div>
          </>
        )}
      </div>
    </div>
  );
}
