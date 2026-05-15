import { useCallback, useEffect, useState } from "react";
import { useSearchParams } from "react-router";
import { api } from "~/lib/api";
import type { DomainWithCompany } from "~/types/api";
import { DomainsTable } from "~/components/app/DomainsTable";
import { Button } from "~/components/ui/button";
import { Skeleton } from "~/components/ui/skeleton";
import { Alert, AlertDescription } from "~/components/ui/alert";

const SIGNALS = ["registry_website", "wikidata", "certsh", "whois", "search"] as const;

export default function DomainsPage() {
  const [searchParams, setSearchParams] = useSearchParams();
  const [domains, setDomains] = useState<DomainWithCompany[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string>();

  const page = Number(searchParams.get("page") ?? 1);
  const signal = searchParams.get("signal") ?? undefined;
  const minConf = searchParams.get("min_confidence") ? Number(searchParams.get("min_confidence")) : undefined;

  const fetchData = useCallback(async () => {
    setLoading(true);
    setError(undefined);
    try {
      const res = await api.getDomains({ page, limit: 50, signal, min_confidence: minConf });
      setDomains(res.items);
      setTotal(res.total);
    } catch {
      setError("Failed to load domains.");
    } finally {
      setLoading(false);
    }
  }, [page, signal, minConf]);

  useEffect(() => { fetchData(); }, [fetchData]);

  function setParam(key: string, value: string) {
    const next = new URLSearchParams(searchParams);
    if (value) next.set(key, value); else next.delete(key);
    next.set("page", "1");
    setSearchParams(next);
  }

  return (
    <div>
      <div className="mb-4 flex items-center justify-between">
        <h1 className="text-xl font-semibold">Domains</h1>
        <span className="text-sm text-muted-foreground">{total.toLocaleString()} total</span>
      </div>

      <div className="mb-4 flex gap-2">
        <select
          className="h-9 rounded-md border border-input bg-background px-3 text-sm focus:outline-none focus:ring-1 focus:ring-ring"
          value={signal ?? ""}
          onChange={(e) => setParam("signal", e.target.value)}
        >
          <option value="">All signals</option>
          {SIGNALS.map((s) => <option key={s} value={s}>{s}</option>)}
        </select>
        <select
          className="h-9 rounded-md border border-input bg-background px-3 text-sm focus:outline-none focus:ring-1 focus:ring-ring"
          value={minConf?.toString() ?? ""}
          onChange={(e) => setParam("min_confidence", e.target.value)}
        >
          <option value="">Any confidence</option>
          <option value="50">≥ 50</option>
          <option value="65">≥ 65</option>
          <option value="75">≥ 75</option>
          <option value="90">≥ 90</option>
        </select>
      </div>

      {loading ? (
        <div className="space-y-2">
          {Array.from({ length: 10 }).map((_, i) => <Skeleton key={i} className="h-10 w-full" />)}
        </div>
      ) : error ? (
        <Alert variant="destructive"><AlertDescription>{error}</AlertDescription></Alert>
      ) : (
        <>
          <DomainsTable domains={domains} />
          <div className="mt-4 flex justify-between">
            <Button variant="outline" disabled={page <= 1} onClick={() => setParam("page", String(page - 1))}>Previous</Button>
            <span className="self-center text-sm text-muted-foreground">Page {page}</span>
            <Button variant="outline" disabled={domains.length < 50} onClick={() => setParam("page", String(page + 1))}>Next</Button>
          </div>
        </>
      )}
    </div>
  );
}
