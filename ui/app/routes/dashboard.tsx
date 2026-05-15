import { useEffect, useState } from "react";
import { BarChart3, Building2, CheckSquare, Globe, Server, XCircle } from "lucide-react";
import { api } from "~/lib/api";
import type { StatsResponse, PullRun } from "~/types/api";
import { StatsCard } from "~/components/app/StatsCard";
import { PullRunsTable } from "~/components/app/PullRunsTable";
import { Skeleton } from "~/components/ui/skeleton";
import { Alert, AlertDescription } from "~/components/ui/alert";

export default function DashboardPage() {
  const [stats, setStats] = useState<StatsResponse>();
  const [runs, setRuns] = useState<PullRun[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string>();

  useEffect(() => {
    Promise.all([api.getStats(), api.getPullRuns(1, 10)])
      .then(([s, r]) => {
        setStats(s);
        setRuns(r.items);
      })
      .catch(() => setError("Failed to load dashboard data."))
      .finally(() => setLoading(false));
  }, []);

  if (loading) {
    return (
      <div className="space-y-4">
        <div className="grid grid-cols-2 gap-4 sm:grid-cols-4">
          {Array.from({ length: 8 }).map((_, i) => (
            <Skeleton key={i} className="h-24 w-full" />
          ))}
        </div>
        <Skeleton className="h-48 w-full" />
      </div>
    );
  }

  if (error || !stats) {
    return (
      <Alert variant="destructive">
        <AlertDescription>{error ?? "Unknown error"}</AlertDescription>
      </Alert>
    );
  }

  return (
    <div className="space-y-6">
      <h1 className="text-xl font-semibold">Dashboard</h1>

      <div className="grid grid-cols-2 gap-4 sm:grid-cols-4">
        <StatsCard title="Total Companies" value={stats.total_companies} icon={Building2} />
        <StatsCard title="Total Domains" value={stats.total_domains} icon={Globe} />
        <StatsCard title="Active Domains" value={stats.active_domains} icon={BarChart3} variant="success" />
        <StatsCard title="Pending Review" value={stats.pending_review} icon={CheckSquare} href="/review" />
      </div>

      <div className="grid grid-cols-2 gap-4 sm:grid-cols-4">
        <StatsCard
          title="Runs Completed Today"
          value={stats.pull_runs_completed_today}
          icon={Server}
          variant="success"
        />
        <StatsCard
          title="Runs Failed Today"
          value={stats.pull_runs_failed_today}
          icon={XCircle}
          variant={stats.pull_runs_failed_today > 0 ? "danger" : "default"}
        />
        <StatsCard title="Records Upserted 24h" value={stats.records_upserted_24h} />
        <StatsCard title="Records Upserted 7d" value={stats.records_upserted_7d} />
      </div>

      <div>
        <h2 className="mb-3 text-base font-semibold">Recent Pull Runs</h2>
        {runs.length === 0 ? (
          <p className="text-sm text-muted-foreground">No pull runs yet.</p>
        ) : (
          <PullRunsTable runs={runs} />
        )}
      </div>
    </div>
  );
}
