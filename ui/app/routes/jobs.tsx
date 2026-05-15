import { useCallback, useEffect, useRef, useState } from "react";
import { useSearchParams } from "react-router";
import { api } from "~/lib/api";
import type { Job } from "~/types/api";
import { JobsTable } from "~/components/app/JobsTable";
import { Button } from "~/components/ui/button";
import { Skeleton } from "~/components/ui/skeleton";
import { Alert, AlertDescription } from "~/components/ui/alert";

const STATES = ["available", "running", "completed", "failed", "retryable", "scheduled", "discarded", "cancelled"] as const;

export default function JobsPage() {
  const [searchParams, setSearchParams] = useSearchParams();
  const [jobs, setJobs] = useState<Job[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string>();
  const timerRef = useRef<ReturnType<typeof setInterval> | undefined>(undefined);

  const page = Number(searchParams.get("page") ?? 1);
  const status = searchParams.get("status") ?? undefined;
  const source = searchParams.get("source") ?? undefined;

  const fetchData = useCallback(async () => {
    setError(undefined);
    try {
      const res = await api.getJobs({ page, limit: 50, status, source });
      setJobs(res.items);
    } catch {
      setError("Failed to load jobs.");
    } finally {
      setLoading(false);
    }
  }, [page, status, source]);

  useEffect(() => {
    fetchData();
    timerRef.current = setInterval(fetchData, 30_000);
    return () => clearInterval(timerRef.current);
  }, [fetchData]);

  function setParam(key: string, value: string) {
    const next = new URLSearchParams(searchParams);
    if (value) next.set(key, value); else next.delete(key);
    next.set("page", "1");
    setSearchParams(next);
  }

  return (
    <div>
      <div className="mb-4 flex items-center justify-between">
        <h1 className="text-xl font-semibold">Jobs</h1>
        <span className="text-xs text-muted-foreground">Auto-refreshes every 30s</span>
      </div>

      <div className="mb-4 flex gap-2">
        <select
          className="h-9 rounded-md border border-input bg-background px-3 text-sm focus:outline-none focus:ring-1 focus:ring-ring"
          value={status ?? ""}
          onChange={(e) => setParam("status", e.target.value)}
        >
          <option value="">All states</option>
          {STATES.map((s) => <option key={s} value={s}>{s}</option>)}
        </select>
      </div>

      {loading ? (
        <div className="space-y-2">{Array.from({ length: 10 }).map((_, i) => <Skeleton key={i} className="h-10 w-full" />)}</div>
      ) : error ? (
        <Alert variant="destructive"><AlertDescription>{error}</AlertDescription></Alert>
      ) : (
        <>
          <JobsTable jobs={jobs} />
          <div className="mt-4 flex justify-between">
            <Button variant="outline" disabled={page <= 1} onClick={() => setParam("page", String(page - 1))}>Previous</Button>
            <span className="self-center text-sm text-muted-foreground">Page {page}</span>
            <Button variant="outline" disabled={jobs.length < 50} onClick={() => setParam("page", String(page + 1))}>Next</Button>
          </div>
        </>
      )}
    </div>
  );
}
