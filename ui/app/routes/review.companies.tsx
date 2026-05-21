import { useCallback, useEffect, useMemo, useState } from "react";
import {
  useReactTable,
  getCoreRowModel,
  flexRender,
  type ColumnDef,
} from "@tanstack/react-table";
import { ArrowUpDown, ArrowUp, ArrowDown, Search, X } from "lucide-react";
import { api } from "~/lib/api";
import type { RawInput, RawInputDetail } from "~/types/api";
import { Input } from "~/components/ui/input";
import { Button } from "~/components/ui/button";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "~/components/ui/table";
import { Badge } from "~/components/ui/badge";
import { Skeleton } from "~/components/ui/skeleton";
import { Separator } from "~/components/ui/separator";
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
} from "~/components/ui/sheet";

const PAGE_SIZE = 50;

const SOURCE_LABELS: Record<string, string> = {
  companies_house: "Companies House",
  brreg: "Brreg",
};

const STATUS_OPTIONS = ["pending", "processing", "processed", "failed", "ignored", "superseded"];
const SOURCE_OPTIONS = ["companies_house", "brreg"];

function timeAgo(dateStr: string): string {
  const diff = Date.now() - new Date(dateStr).getTime();
  const minutes = Math.floor(diff / 60_000);
  if (minutes < 1) return "just now";
  if (minutes < 60) return `${minutes}m ago`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours}h ago`;
  const days = Math.floor(hours / 24);
  if (days < 30) return `${days}d ago`;
  const months = Math.floor(days / 30);
  if (months < 12) return `${months}mo ago`;
  return `${Math.floor(months / 12)}y ago`;
}

function statusBadgeVariant(status: string): "default" | "secondary" | "destructive" | "outline" {
  switch (status) {
    case "pending": return "default";
    case "processing": return "secondary";
    case "processed": return "outline";
    case "failed": return "destructive";
    default: return "outline";
  }
}

function SortIcon({ column, currentSort, currentDir }: { column: string; currentSort: string; currentDir: "asc" | "desc" }) {
  if (currentSort !== column) return <ArrowUpDown className="ml-1 inline size-3 text-muted-foreground" />;
  return currentDir === "asc"
    ? <ArrowUp className="ml-1 inline size-3" />
    : <ArrowDown className="ml-1 inline size-3" />;
}

function DetailRow({ label, value }: { label: string; value: React.ReactNode }) {
  if (!value && value !== 0) return null;
  return (
    <div className="grid grid-cols-[140px_1fr] gap-2 py-1.5">
      <span className="text-sm text-muted-foreground">{label}</span>
      <span className="text-sm break-all">{value}</span>
    </div>
  );
}

function RawInputDetailSheet({
  open,
  onClose,
  source,
  id,
}: {
  open: boolean;
  onClose: () => void;
  source: string;
  id: string;
}) {
  const [detail, setDetail] = useState<RawInputDetail | null>(null);
  const [loading, setLoading] = useState(false);
  const [jsonExpanded, setJsonExpanded] = useState(false);

  useEffect(() => {
    if (!open || !id) return;
    setDetail(null);
    setJsonExpanded(false);
    setLoading(true);
    api.getRawInput(source, id).then(setDetail).finally(() => setLoading(false));
  }, [open, source, id]);

  const typeLabel = detail?.source === "companies_house"
    ? detail.company_type
    : detail?.registration_status;

  return (
    <Sheet open={open} onOpenChange={(v) => !v && onClose()}>
      <SheetContent className="w-[480px] sm:max-w-[480px] overflow-y-auto">
        {loading && (
          <div className="space-y-3 pt-6">
            <Skeleton className="h-6 w-3/4" />
            <Skeleton className="h-4 w-1/2" />
            <Skeleton className="h-4 w-full" />
            <Skeleton className="h-4 w-full" />
          </div>
        )}

        {detail && (
          <>
            <SheetHeader className="pb-4">
              <SheetTitle className="text-lg leading-snug">{detail.name}</SheetTitle>
              <div className="flex items-center gap-2 mt-1">
                <Badge variant="outline" className="text-xs">
                  {SOURCE_LABELS[detail.source] ?? detail.source}
                </Badge>
                <Badge variant={statusBadgeVariant(detail.status)} className="text-xs">
                  {detail.status}
                </Badge>
                {detail.country_iso2 && (
                  <span className="text-xs text-muted-foreground">{detail.country_iso2}</span>
                )}
              </div>
            </SheetHeader>

            <Separator className="mb-4" />

            <section className="space-y-0.5 mb-4">
              <DetailRow label="Native ID" value={
                <span className="font-mono text-xs">{detail.native_id}</span>
              } />
              {typeLabel && <DetailRow label="Type" value={typeLabel} />}
              {detail.website && <DetailRow label="Website" value={
                <a href={detail.website} target="_blank" rel="noreferrer"
                   className="text-primary underline-offset-4 hover:underline">
                  {detail.website}
                </a>
              } />}
              {detail.run_id && <DetailRow label="Run ID" value={
                <span className="font-mono text-xs">{detail.run_id}</span>
              } />}
            </section>

            <Separator className="mb-4" />

            <section className="space-y-0.5 mb-4">
              <p className="text-xs font-medium uppercase tracking-wide text-muted-foreground mb-2">Timestamps</p>
              <DetailRow label="Created" value={`${new Date(detail.created_at).toLocaleString()} (${timeAgo(detail.created_at)})`} />
              <DetailRow label="First seen" value={`${new Date(detail.first_seen_at).toLocaleString()} (${timeAgo(detail.first_seen_at)})`} />
              <DetailRow label="Last seen" value={`${new Date(detail.last_seen_at).toLocaleString()} (${timeAgo(detail.last_seen_at)})`} />
              {detail.processed_at && (
                <DetailRow label="Processed" value={`${new Date(detail.processed_at).toLocaleString()} (${timeAgo(detail.processed_at)})`} />
              )}
            </section>

            <Separator className="mb-4" />

            <section className="space-y-0.5 mb-4">
              <p className="text-xs font-medium uppercase tracking-wide text-muted-foreground mb-2">Processing</p>
              <DetailRow label="Attempts" value={detail.processing_attempts} />
              <DetailRow label="Hash" value={
                <span className="font-mono text-xs">{detail.payload_hash.slice(0, 16)}…</span>
              } />
              {detail.processing_error && (
                <div className="mt-2 rounded-md bg-destructive/10 px-3 py-2">
                  <p className="text-xs font-medium text-destructive mb-1">Error</p>
                  <p className="text-xs text-destructive break-all">{detail.processing_error}</p>
                </div>
              )}
            </section>

            <Separator className="mb-4" />

            <section>
              <button
                className="flex w-full items-center justify-between text-xs font-medium uppercase tracking-wide text-muted-foreground mb-2"
                onClick={() => setJsonExpanded((v) => !v)}
              >
                Raw payload
                <span className="normal-case font-normal">{jsonExpanded ? "hide" : "show"}</span>
              </button>
              {jsonExpanded && (
                <pre className="rounded-md bg-muted p-3 text-xs overflow-auto max-h-96 whitespace-pre-wrap break-all">
                  {JSON.stringify(detail.raw_payload, null, 2)}
                </pre>
              )}
            </section>
          </>
        )}
      </SheetContent>
    </Sheet>
  );
}

export default function ReviewCompaniesPage() {
  const [items, setItems] = useState<RawInput[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(true);
  const [page, setPage] = useState(1);

  const [sourceFilter, setSourceFilter] = useState("");
  const [statusFilter, setStatusFilter] = useState("");
  const [searchQ, setSearchQ] = useState("");
  const [searchInput, setSearchInput] = useState("");
  const [sortCol, setSortCol] = useState("created_at");
  const [sortDir, setSortDir] = useState<"asc" | "desc">("desc");

  const [selectedRow, setSelectedRow] = useState<RawInput | null>(null);

  const load = useCallback(
    async (p: number, src: string, st: string, q: string, sort: string, dir: "asc" | "desc") => {
      setLoading(true);
      try {
        const res = await api.getRawInputs({
          page: p,
          limit: PAGE_SIZE,
          source: src || undefined,
          status: st || undefined,
          q: q || undefined,
          sort,
          dir,
        });
        setItems(res.items);
        setTotal(res.total);
        setPage(p);
      } finally {
        setLoading(false);
      }
    },
    [],
  );

  useEffect(() => {
    load(1, sourceFilter, statusFilter, searchQ, sortCol, sortDir);
  }, [load, sourceFilter, statusFilter, searchQ, sortCol, sortDir]);

  const handleSort = (col: string) => {
    if (col === sortCol) {
      setSortDir((d) => (d === "asc" ? "desc" : "asc"));
    } else {
      setSortCol(col);
      setSortDir("desc");
    }
  };

  const applySearch = () => setSearchQ(searchInput);
  const clearSearch = () => { setSearchInput(""); setSearchQ(""); };

  const totalPages = Math.ceil(total / PAGE_SIZE);

  const columns = useMemo<ColumnDef<RawInput>[]>(
    () => [
      {
        accessorKey: "source",
        header: () => (
          <button className="flex items-center font-medium" onClick={() => handleSort("source")}>
            Source <SortIcon column="source" currentSort={sortCol} currentDir={sortDir} />
          </button>
        ),
        cell: ({ getValue }) => (
          <span className="text-xs font-medium">
            {SOURCE_LABELS[getValue() as string] ?? (getValue() as string)}
          </span>
        ),
      },
      {
        accessorKey: "name",
        header: () => (
          <button className="flex items-center font-medium" onClick={() => handleSort("name")}>
            Name <SortIcon column="name" currentSort={sortCol} currentDir={sortDir} />
          </button>
        ),
        cell: ({ getValue }) => <span className="font-medium">{getValue() as string}</span>,
      },
      {
        accessorKey: "native_id",
        header: "ID",
        cell: ({ getValue }) => (
          <span className="font-mono text-xs text-muted-foreground">{getValue() as string}</span>
        ),
      },
      {
        accessorKey: "status",
        header: () => (
          <button className="flex items-center font-medium" onClick={() => handleSort("status")}>
            Status <SortIcon column="status" currentSort={sortCol} currentDir={sortDir} />
          </button>
        ),
        cell: ({ getValue }) => {
          const v = getValue() as string;
          return <Badge variant={statusBadgeVariant(v)} className="text-xs">{v}</Badge>;
        },
      },
      {
        accessorKey: "created_at",
        header: () => (
          <button className="flex items-center font-medium" onClick={() => handleSort("created_at")}>
            Created <SortIcon column="created_at" currentSort={sortCol} currentDir={sortDir} />
          </button>
        ),
        cell: ({ getValue }) => {
          const v = getValue() as string;
          return (
            <div className="text-sm">
              <div>{new Date(v).toLocaleDateString()}</div>
              <div className="text-xs text-muted-foreground">{timeAgo(v)}</div>
            </div>
          );
        },
      },
    ],
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [sortCol, sortDir],
  );

  const table = useReactTable({
    data: items,
    columns,
    getCoreRowModel: getCoreRowModel(),
    manualSorting: true,
    manualPagination: true,
    pageCount: totalPages,
  });

  return (
    <div className="space-y-4">
      {/* Filters */}
      <div className="flex flex-wrap items-center gap-3">
        <div className="relative flex items-center">
          <Search className="absolute left-2.5 size-4 text-muted-foreground" />
          <Input
            className="h-8 w-56 pl-8 pr-8 text-sm"
            placeholder="Search by name…"
            value={searchInput}
            onChange={(e) => setSearchInput(e.target.value)}
            onKeyDown={(e) => e.key === "Enter" && applySearch()}
          />
          {searchInput && (
            <button className="absolute right-2" onClick={clearSearch}>
              <X className="size-3.5 text-muted-foreground" />
            </button>
          )}
        </div>
        {searchInput !== searchQ && (
          <Button size="sm" variant="secondary" className="h-8" onClick={applySearch}>
            Search
          </Button>
        )}

        <select
          className="h-8 rounded-md border bg-background px-2 text-sm"
          value={sourceFilter}
          onChange={(e) => setSourceFilter(e.target.value)}
        >
          <option value="">All sources</option>
          {SOURCE_OPTIONS.map((s) => (
            <option key={s} value={s}>{SOURCE_LABELS[s] ?? s}</option>
          ))}
        </select>

        <select
          className="h-8 rounded-md border bg-background px-2 text-sm"
          value={statusFilter}
          onChange={(e) => setStatusFilter(e.target.value)}
        >
          <option value="">All statuses</option>
          {STATUS_OPTIONS.map((s) => (
            <option key={s} value={s}>{s}</option>
          ))}
        </select>

        <span className="ml-auto text-sm text-muted-foreground">
          {loading ? "Loading…" : `${total.toLocaleString()} entries`}
        </span>
      </div>

      {/* Table */}
      <div className="rounded-md border">
        <Table>
          <TableHeader>
            {table.getHeaderGroups().map((hg) => (
              <TableRow key={hg.id}>
                {hg.headers.map((header) => (
                  <TableHead key={header.id}>
                    {header.isPlaceholder
                      ? null
                      : flexRender(header.column.columnDef.header, header.getContext())}
                  </TableHead>
                ))}
              </TableRow>
            ))}
          </TableHeader>
          <TableBody>
            {loading ? (
              Array.from({ length: 8 }).map((_, i) => (
                <TableRow key={i}>
                  {columns.map((_, j) => (
                    <TableCell key={j}><Skeleton className="h-4 w-full" /></TableCell>
                  ))}
                </TableRow>
              ))
            ) : table.getRowModel().rows.length === 0 ? (
              <TableRow>
                <TableCell colSpan={columns.length} className="py-12 text-center text-muted-foreground">
                  No raw inputs found.
                </TableCell>
              </TableRow>
            ) : (
              table.getRowModel().rows.map((row) => (
                <TableRow
                  key={row.id}
                  className="cursor-pointer hover:bg-muted/50"
                  onClick={() => setSelectedRow(row.original)}
                >
                  {row.getVisibleCells().map((cell) => (
                    <TableCell key={cell.id}>
                      {flexRender(cell.column.columnDef.cell, cell.getContext())}
                    </TableCell>
                  ))}
                </TableRow>
              ))
            )}
          </TableBody>
        </Table>
      </div>

      {/* Pagination */}
      {totalPages > 1 && (
        <div className="flex items-center justify-between">
          <span className="text-sm text-muted-foreground">
            Page {page} of {totalPages}
          </span>
          <div className="flex gap-2">
            <Button
              size="sm"
              variant="outline"
              disabled={page <= 1 || loading}
              onClick={() => load(page - 1, sourceFilter, statusFilter, searchQ, sortCol, sortDir)}
            >
              Previous
            </Button>
            <Button
              size="sm"
              variant="outline"
              disabled={page >= totalPages || loading}
              onClick={() => load(page + 1, sourceFilter, statusFilter, searchQ, sortCol, sortDir)}
            >
              Next
            </Button>
          </div>
        </div>
      )}

      {/* Detail sheet */}
      {selectedRow && (
        <RawInputDetailSheet
          open={!!selectedRow}
          onClose={() => setSelectedRow(null)}
          source={selectedRow.source}
          id={selectedRow.id}
        />
      )}
    </div>
  );
}
