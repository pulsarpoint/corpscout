import { useCallback, useEffect, useState } from "react";
import { CheckCircle } from "lucide-react";
import { toast } from "sonner";
import { api } from "~/lib/api";
import type { ReviewCandidate } from "~/types/api";
import { ReviewTable } from "~/components/app/ReviewTable";
import { ReviewSheet } from "~/components/app/ReviewSheet";
import { Button } from "~/components/ui/button";
import { Skeleton } from "~/components/ui/skeleton";
import { Alert, AlertDescription } from "~/components/ui/alert";

export default function ReviewPage() {
  const [items, setItems] = useState<ReviewCandidate[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string>();
  const [page, setPage] = useState(1);
  const [hasMore, setHasMore] = useState(false);
  const [selectedItem, setSelectedItem] = useState<ReviewCandidate | null>(null);
  const [actionLoading, setActionLoading] = useState<string>();

  const fetchPage = useCallback(async (p: number) => {
    try {
      setError(undefined);
      const res = await api.getReview(p);
      if (p === 1) {
        setItems(res.items);
      } else {
        setItems((prev) => [...prev, ...res.items]);
      }
      setHasMore(res.items.length === 50);
    } catch {
      setError("Failed to load review queue.");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchPage(1);
  }, [fetchPage]);

  const handleAction = async (id: string, action: "approved" | "rejected" | "superseded") => {
    setActionLoading(id);
    try {
      await api.createReview(id, action);
      setItems((prev) => prev.filter((i) => i.id !== id));
      if (selectedItem?.id === id) setSelectedItem(null);
      toast.success(`Candidate ${action}.`);
    } catch {
      toast.error("Action failed. Please try again.");
    } finally {
      setActionLoading(undefined);
    }
  };

  if (loading) {
    return (
      <div className="space-y-3">
        <Skeleton className="h-8 w-48" />
        {Array.from({ length: 5 }).map((_, i) => (
          <Skeleton key={i} className="h-12 w-full" />
        ))}
      </div>
    );
  }

  if (error) {
    return (
      <Alert variant="destructive">
        <AlertDescription>
          {error}{" "}
          <Button variant="link" className="p-0 h-auto" onClick={() => { setLoading(true); fetchPage(1); }}>
            Retry
          </Button>
        </AlertDescription>
      </Alert>
    );
  }

  return (
    <div>
      <div className="mb-4 flex items-center justify-between">
        <h1 className="text-xl font-semibold">Review Queue</h1>
        <span className="text-sm text-muted-foreground">{items.length} pending</span>
      </div>

      {items.length === 0 ? (
        <div className="flex flex-col items-center justify-center py-24 text-muted-foreground">
          <CheckCircle className="mb-4 size-12 text-green-500" />
          <p className="text-lg font-medium">Queue is empty</p>
          <p className="text-sm">All candidates have been reviewed.</p>
        </div>
      ) : (
        <>
          <ReviewTable
            items={items}
            onApprove={(id) => handleAction(id, "approved")}
            onReject={(id) => handleAction(id, "rejected")}
            onView={setSelectedItem}
            actionLoading={actionLoading}
          />
          {hasMore && (
            <div className="mt-4 flex justify-center">
              <Button
                variant="outline"
                onClick={() => {
                  const next = page + 1;
                  setPage(next);
                  fetchPage(next);
                }}
              >
                Load more
              </Button>
            </div>
          )}
        </>
      )}

      <ReviewSheet
        candidate={selectedItem}
        onClose={() => setSelectedItem(null)}
        onAction={handleAction}
        loading={actionLoading != null}
      />
    </div>
  );
}
