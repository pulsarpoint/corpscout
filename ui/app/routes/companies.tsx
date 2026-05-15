import { useCallback, useEffect, useState } from "react";
import { useSearchParams } from "react-router";
import { api } from "~/lib/api";
import type { Company } from "~/types/api";
import { CompaniesTable } from "~/components/app/CompaniesTable";
import { Button } from "~/components/ui/button";
import { Skeleton } from "~/components/ui/skeleton";
import { Alert, AlertDescription } from "~/components/ui/alert";

export default function CompaniesPage() {
  const [searchParams, setSearchParams] = useSearchParams();
  const [companies, setCompanies] = useState<Company[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string>();

  const page = Number(searchParams.get("page") ?? 1);
  const q = searchParams.get("q") ?? undefined;
  const status = searchParams.get("status") ?? undefined;

  const fetchData = useCallback(async () => {
    setLoading(true);
    setError(undefined);
    try {
      const res = await api.getCompanies({ page, limit: 50, q, status });
      setCompanies(res.items);
      setTotal(res.total);
    } catch {
      setError("Failed to load companies.");
    } finally {
      setLoading(false);
    }
  }, [page, q, status]);

  useEffect(() => { fetchData(); }, [fetchData]);

  return (
    <div>
      <div className="mb-4 flex items-center justify-between">
        <h1 className="text-xl font-semibold">Companies</h1>
        <span className="text-sm text-muted-foreground">{total.toLocaleString()} total</span>
      </div>

      <div className="mb-4 flex gap-2">
        <input
          className="h-9 rounded-md border border-input bg-background px-3 text-sm w-64 focus:outline-none focus:ring-1 focus:ring-ring"
          placeholder="Search name…"
          defaultValue={q}
          onKeyDown={(e) => {
            if (e.key === "Enter") {
              setSearchParams({ q: e.currentTarget.value, page: "1" });
            }
          }}
        />
        <select
          className="h-9 rounded-md border border-input bg-background px-3 text-sm focus:outline-none focus:ring-1 focus:ring-ring"
          value={status ?? ""}
          onChange={(e) => setSearchParams({ q: q ?? "", status: e.target.value, page: "1" })}
        >
          <option value="">All statuses</option>
          <option value="active">Active</option>
          <option value="inactive">Inactive</option>
          <option value="dissolved">Dissolved</option>
        </select>
      </div>

      {loading ? (
        <div className="space-y-2">
          {Array.from({ length: 10 }).map((_, i) => <Skeleton key={i} className="h-10 w-full" />)}
        </div>
      ) : error ? (
        <Alert variant="destructive">
          <AlertDescription>{error}</AlertDescription>
        </Alert>
      ) : (
        <>
          <CompaniesTable companies={companies} />
          <div className="mt-4 flex justify-between">
            <Button
              variant="outline"
              disabled={page <= 1}
              onClick={() => setSearchParams({ q: q ?? "", status: status ?? "", page: String(page - 1) })}
            >
              Previous
            </Button>
            <span className="self-center text-sm text-muted-foreground">Page {page}</span>
            <Button
              variant="outline"
              disabled={companies.length < 50}
              onClick={() => setSearchParams({ q: q ?? "", status: status ?? "", page: String(page + 1) })}
            >
              Next
            </Button>
          </div>
        </>
      )}
    </div>
  );
}
