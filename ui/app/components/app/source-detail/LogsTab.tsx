import { useEffect, useState } from "react";
import { api } from "~/lib/api";
import type { PullRun } from "~/types/api";
import { Button } from "~/components/ui/button";
import { Alert, AlertDescription } from "~/components/ui/alert";
import { PullRunsTable } from "~/components/app/PullRunsTable";
import { Skeleton } from "~/components/ui/skeleton";

interface LogsTabProps {
  sourceName: string;
}

const PAGE_SIZE = 20;

export function LogsTab({ sourceName }: LogsTabProps) {
  const [items, setItems] = useState<PullRun[]>([]);
  const [page, setPage] = useState(1);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string>();

  useEffect(() => {
    let cancelled = false;

    setLoading(true);
    setError(undefined);
    api.getPullRuns(page, PAGE_SIZE, sourceName)
      .then((response) => {
        if (!cancelled) setItems(response.items ?? []);
      })
      .catch(() => {
        if (!cancelled) setError("Failed to load pull runs.");
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });

    return () => {
      cancelled = true;
    };
  }, [page, sourceName]);

  const hasMore = items.length === PAGE_SIZE;

  if (loading) return <Skeleton className="h-64 w-full" />;
  if (error) return <Alert variant="destructive"><AlertDescription>{error}</AlertDescription></Alert>;

  return (
    <div className="space-y-3">
      {items.length === 0 ? (
        <p className="text-sm text-muted-foreground">No pull runs yet.</p>
      ) : (
        <PullRunsTable runs={items} />
      )}
      <div className="flex items-center justify-between">
        <Button
          size="sm"
          variant="outline"
          disabled={page === 1}
          onClick={() => setPage((current) => Math.max(1, current - 1))}
        >
          Previous
        </Button>
        <span className="text-sm text-muted-foreground">Page {page}</span>
        <Button
          size="sm"
          variant="outline"
          disabled={!hasMore}
          onClick={() => setPage((current) => current + 1)}
        >
          Next
        </Button>
      </div>
    </div>
  );
}
