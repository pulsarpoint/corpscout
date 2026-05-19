import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { Link, useSearchParams } from "react-router";
import { type ColumnDef, type SortingState } from "@tanstack/react-table";
import { MoreHorizontal } from "lucide-react";
import { pgrest } from "~/lib/pgrest";
import type { VDomain } from "~/types/api";
import { DataTable } from "~/components/ui/data-table";
import { Badge } from "~/components/ui/badge";
import { Button } from "~/components/ui/button";
import { Checkbox } from "~/components/ui/checkbox";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "~/components/ui/dropdown-menu";
import { CrawlDomainDialog } from "~/components/app/CrawlDomainDialog";
import { signalColor, confidenceColor, formatDate, timeAgo } from "~/lib/utils";

const PAGE_SIZE = 50;

const SIGNALS = ["registry_website", "wikidata", "certsh", "whois", "search"] as const;

export default function DomainsPage() {
  const [searchParams, setSearchParams] = useSearchParams();
  const [domains, setDomains] = useState<VDomain[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(true);
  const [crawlTarget, setCrawlTarget] = useState<{ id: string; domain: string } | null>(null);
  const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set());
  const [bulkCrawlOpen, setBulkCrawlOpen] = useState(false);
  const [selectingAll, setSelectingAll] = useState(false);

  const page = Math.max(1, Number(searchParams.get("page") ?? 1));
  const signal = searchParams.get("signal") ?? "";
  const minConf = searchParams.get("min_confidence") ?? "";
  const orphan = searchParams.get("orphan") === "1";
  const crawledFilter = searchParams.get("crawled") ?? "";
  const sortKey = searchParams.get("sort") ?? "first_seen_at";
  const sortDir = (searchParams.get("dir") ?? "desc") as "asc" | "desc";
  const q = searchParams.get("q") ?? "";

  const hasFilters = !!(signal || minConf || orphan || crawledFilter || q);

  // Local input state so typing doesn't immediately trigger a fetch on each keystroke.
  const [inputValue, setInputValue] = useState(q);
  const debounceRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  // Keep local input in sync when the URL param changes externally (e.g. back/forward).
  useEffect(() => { setInputValue(q); }, [q]);

  const columns = useMemo<ColumnDef<VDomain, unknown>[]>(() => {
    const pageIds = domains.map(d => d.id);
    const allOnPage = pageIds.length > 0 && pageIds.every(id => selectedIds.has(id));
    const someOnPage = pageIds.some(id => selectedIds.has(id));

    return [
      {
        id: "select",
        enableSorting: false,
        header: () => (
          <Checkbox
            checked={allOnPage ? true : someOnPage ? "indeterminate" : false}
            onCheckedChange={(checked) => {
              setSelectedIds(prev => {
                const next = new Set(prev);
                if (checked) pageIds.forEach(id => next.add(id));
                else pageIds.forEach(id => next.delete(id));
                return next;
              });
            }}
          />
        ),
        cell: ({ row }) => (
          <Checkbox
            checked={selectedIds.has(row.original.id)}
            onCheckedChange={(checked) => {
              setSelectedIds(prev => {
                const next = new Set(prev);
                if (checked) next.add(row.original.id);
                else next.delete(row.original.id);
                return next;
              });
            }}
          />
        ),
      },
      {
        accessorKey: "domain",
        header: "Domain",
        enableSorting: true,
        cell: ({ row }) => (
          <Link
            to={`/domains/${row.original.id}`}
            className="font-mono font-medium hover:underline"
          >
            {row.original.domain}
          </Link>
        ),
      },
      {
        accessorKey: "primary_company_name",
        header: "Company",
        enableSorting: true,
        cell: ({ row }) =>
          row.original.primary_company_id ? (
            <Link
              to={`/companies/${row.original.primary_company_id}`}
              className="hover:underline text-sm"
            >
              {row.original.primary_company_name}
            </Link>
          ) : (
            <span className="text-muted-foreground text-sm">No company</span>
          ),
      },
      {
        accessorKey: "primary_signal",
        header: "Signal",
        enableSorting: true,
        cell: ({ row }) =>
          row.original.primary_signal ? (
            <Badge variant="outline" className={signalColor(row.original.primary_signal)}>
              {row.original.primary_signal}
            </Badge>
          ) : (
            <span className="text-muted-foreground">—</span>
          ),
      },
      {
        accessorKey: "max_confidence",
        header: "Confidence",
        enableSorting: true,
        cell: ({ row }) =>
          row.original.max_confidence != null ? (
            <span className={`font-bold ${confidenceColor(row.original.max_confidence)}`}>
              {row.original.max_confidence}
            </span>
          ) : (
            <span className="text-muted-foreground">—</span>
          ),
      },
      {
        accessorKey: "company_count",
        header: "Companies",
        enableSorting: true,
        cell: ({ row }) => (
          <span className={row.original.company_count === 0 ? "text-muted-foreground" : "font-medium"}>
            {row.original.company_count}
          </span>
        ),
      },
      {
        accessorKey: "crawled",
        header: "Crawled",
        enableSorting: true,
        cell: ({ row }) =>
          row.original.crawled ? (
            <Badge variant="outline" className="text-green-600 border-green-600">✓</Badge>
          ) : (
            <span className="text-muted-foreground">—</span>
          ),
      },
      {
        accessorKey: "last_crawled_at",
        header: "Last Crawl",
        enableSorting: true,
        cell: ({ row }) => (
          <span className="text-sm text-muted-foreground">
            {row.original.last_crawled_at ? timeAgo(row.original.last_crawled_at) : "—"}
          </span>
        ),
      },
      {
        accessorKey: "first_seen_at",
        header: "First Seen",
        enableSorting: true,
        cell: ({ row }) => (
          <span className="text-sm text-muted-foreground">
            {row.original.first_seen_at ? formatDate(row.original.first_seen_at) : "—"}
          </span>
        ),
      },
      {
        id: "actions",
        cell: ({ row }) => (
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button variant="ghost" size="icon">
                <MoreHorizontal className="h-4 w-4" />
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end">
              <DropdownMenuItem
                onClick={() =>
                  setCrawlTarget({ id: row.original.id, domain: row.original.domain })
                }
              >
                Crawl domain
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        ),
      },
    ];
  }, [domains, selectedIds, setCrawlTarget]);

  const sorting: SortingState = useMemo(
    () => [{ id: sortKey, desc: sortDir === "desc" }],
    [sortKey, sortDir]
  );

  const fetchData = useCallback(async () => {
    setLoading(true);
    try {
      const params: Record<string, string | number> = {
        limit: PAGE_SIZE,
        offset: (page - 1) * PAGE_SIZE,
        order: `${sortKey}.${sortDir}`,
      };
      if (signal) params["primary_signal"] = `eq.${signal}`;
      if (minConf) params["max_confidence"] = `gte.${minConf}`;
      if (orphan) params["company_count"] = "eq.0";
      if (crawledFilter) params["crawled"] = `eq.${crawledFilter}`;
      if (q) params["domain"] = `ilike.*${q}*`;

      const res = await pgrest<VDomain>("v_domains", params);
      setDomains(res.data);
      setTotal(res.total);
    } finally {
      setLoading(false);
    }
  }, [page, signal, minConf, orphan, crawledFilter, sortKey, sortDir, q]);

  useEffect(() => { fetchData(); }, [fetchData]);

  function setParam(updates: Record<string, string>) {
    const next = new URLSearchParams(searchParams);
    for (const [k, v] of Object.entries(updates)) {
      if (v) next.set(k, v); else next.delete(k);
    }
    next.set("page", updates.page ?? "1");
    setSearchParams(next);
    setSelectedIds(new Set());
  }

  async function selectAllFiltered() {
    setSelectingAll(true);
    try {
      const params: Record<string, string | number> = { select: "id", limit: 100000 };
      if (signal) params["primary_signal"] = `eq.${signal}`;
      if (minConf) params["max_confidence"] = `gte.${minConf}`;
      if (orphan) params["company_count"] = "eq.0";
      if (crawledFilter) params["crawled"] = `eq.${crawledFilter}`;
      if (q) params["domain"] = `ilike.*${q}*`;
      const res = await pgrest<{ id: string }>("v_domains", params);
      setSelectedIds(new Set(res.data.map(d => d.id)));
    } finally {
      setSelectingAll(false);
    }
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h1 className="text-xl font-semibold">Domains</h1>
        <span className="text-sm text-muted-foreground">{total.toLocaleString()} total</span>
      </div>

      <div className="flex flex-wrap gap-2 items-center">
        <input
          type="search"
          placeholder="Search domain…"
          className="h-9 rounded-md border border-input bg-background px-3 text-sm focus:outline-none focus:ring-1 focus:ring-ring w-52"
          value={inputValue}
          onChange={(e) => {
            const val = e.target.value;
            setInputValue(val);
            if (debounceRef.current) clearTimeout(debounceRef.current);
            debounceRef.current = setTimeout(() => setParam({ q: val }), 400);
          }}
        />
        <select
          className="h-9 rounded-md border border-input bg-background px-3 text-sm focus:outline-none focus:ring-1 focus:ring-ring"
          value={signal}
          onChange={(e) => setParam({ signal: e.target.value })}
        >
          <option value="">All signals</option>
          {SIGNALS.map((s) => <option key={s} value={s}>{s}</option>)}
        </select>
        <select
          className="h-9 rounded-md border border-input bg-background px-3 text-sm focus:outline-none focus:ring-1 focus:ring-ring"
          value={minConf}
          onChange={(e) => setParam({ min_confidence: e.target.value })}
        >
          <option value="">Any confidence</option>
          <option value="50">≥ 50</option>
          <option value="65">≥ 65</option>
          <option value="75">≥ 75</option>
          <option value="90">≥ 90</option>
        </select>
        <select
          className="h-9 rounded-md border border-input bg-background px-3 text-sm focus:outline-none focus:ring-1 focus:ring-ring"
          value={crawledFilter}
          onChange={(e) => setParam({ crawled: e.target.value })}
        >
          <option value="">All crawl status</option>
          <option value="true">Crawled</option>
          <option value="false">Not crawled</option>
        </select>
        <label className="flex items-center gap-2 text-sm cursor-pointer select-none">
          <input
            type="checkbox"
            className="rounded border-input"
            checked={orphan}
            onChange={(e) => setParam({ orphan: e.target.checked ? "1" : "" })}
          />
          Unlinked domains only
        </label>
        <Button
          variant="outline"
          size="sm"
          disabled={!hasFilters || selectingAll}
          onClick={selectAllFiltered}
        >
          {selectingAll ? "Selecting…" : `Select all ${total.toLocaleString()} filtered`}
        </Button>
      </div>

      {selectedIds.size > 0 && (
        <div className="flex items-center gap-3 rounded-md border bg-muted/50 px-4 py-2 text-sm">
          <span className="font-medium">{selectedIds.size.toLocaleString()} selected</span>
          <Button variant="outline" size="sm" onClick={() => setSelectedIds(new Set())}>
            Clear
          </Button>
          <Button size="sm" onClick={() => setBulkCrawlOpen(true)}>
            Crawl {selectedIds.size.toLocaleString()} domain{selectedIds.size !== 1 ? "s" : ""}
          </Button>
        </div>
      )}

      <DataTable
        columns={columns}
        data={domains}
        total={total}
        page={page}
        pageSize={PAGE_SIZE}
        sorting={sorting}
        onSortingChange={(updater) => {
          const next = typeof updater === "function" ? updater(sorting) : updater;
          if (next.length > 0) {
            setParam({ sort: next[0].id, dir: next[0].desc ? "desc" : "asc", page: "1" });
          }
        }}
        onPageChange={(p) => setParam({ page: String(p) })}
        loading={loading}
      />

      {crawlTarget && (
        <CrawlDomainDialog
          domainId={crawlTarget.id}
          domainName={crawlTarget.domain}
          open={!!crawlTarget}
          onOpenChange={(open) => { if (!open) setCrawlTarget(null); }}
        />
      )}
      <CrawlDomainDialog
        domainIds={[...selectedIds]}
        open={bulkCrawlOpen}
        onOpenChange={setBulkCrawlOpen}
        onSuccess={() => setSelectedIds(new Set())}
      />
    </div>
  );
}
