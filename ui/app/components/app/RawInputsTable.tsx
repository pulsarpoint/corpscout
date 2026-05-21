import { useCallback, useEffect, useMemo, useState } from "react";
import {
  useReactTable,
  getCoreRowModel,
  flexRender,
  type ColumnDef,
} from "@tanstack/react-table";
import { ArrowDown, ArrowUp, ArrowUpDown, CheckCircle2, Search, X } from "lucide-react";
import { toast } from "sonner";
import { api } from "~/lib/api";
import type { RawInput } from "~/types/api";
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
import { RawInputDetailSheet } from "~/components/app/RawInputDetailSheet";

const PAGE_SIZE = 50;

const SOURCE_LABELS: Record<string, string> = {
  companies_house: "Companies House",
  brreg: "Brreg",
};

const STATUS_OPTIONS = ["pending", "processing", "processed", "failed", "ignored", "superseded"];
const TRANSLATION_OPTIONS = ["pending", "translating", "translated", "failed"];
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

function translationBadgeClass(status?: string) {
  switch (status) {
    case "translated": return "border-green-200 bg-green-100 text-green-800";
    case "failed": return "border-red-200 bg-red-100 text-red-800";
    case "translating": return "border-blue-200 bg-blue-100 text-blue-800";
    case "pending": return "border-amber-200 bg-amber-100 text-amber-800";
    default: return "";
  }
}

function approvalBlockedReason(row: RawInput): string | undefined {
  if (row.source === "brreg" && row.translation_status !== "translated") return "Translate first";
  if (!row.company_suggestion_id) return "Process source first";
  if (row.company_suggestion_status !== "pending") return "Already confirmed";
  if (!row.can_approve_company) return "Not ready";
  return undefined;
}

function SortIcon({ column, currentSort, currentDir }: { column: string; currentSort: string; currentDir: "asc" | "desc" }) {
  if (currentSort !== column) return <ArrowUpDown className="ml-1 inline size-3 text-muted-foreground" />;
  return currentDir === "asc"
    ? <ArrowUp className="ml-1 inline size-3" />
    : <ArrowDown className="ml-1 inline size-3" />;
}

export interface RawInputsTableProps {
  sourceName?: string;
  requiresTranslation: boolean;
  showConfirmAction?: boolean;
}

export function RawInputsTable({ sourceName, requiresTranslation, showConfirmAction = false }: RawInputsTableProps) {
  const [items, setItems] = useState<RawInput[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(true);
  const [page, setPage] = useState(1);

  const [sourceFilter, setSourceFilter] = useState("");
  const [statusFilter, setStatusFilter] = useState("");
  const [translationFilter, setTranslationFilter] = useState("");
  const [searchQ, setSearchQ] = useState("");
  const [searchInput, setSearchInput] = useState("");
  const [sortCol, setSortCol] = useState("created_at");
  const [sortDir, setSortDir] = useState<"asc" | "desc">("desc");
  const [approvingId, setApprovingId] = useState<string | null>(null);
  const [selectedRow, setSelectedRow] = useState<RawInput | null>(null);

  const load = useCallback(
    async (p: number, src: string, st: string, tr: string, q: string, sort: string, dir: "asc" | "desc") => {
      setLoading(true);
      try {
        const res = await api.getRawInputs({
          page: p,
          limit: PAGE_SIZE,
          source: (sourceName ?? src) || undefined,
          status: st || undefined,
          translation_status: tr || undefined,
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
    [sourceName],
  );

  useEffect(() => {
    load(1, sourceFilter, statusFilter, translationFilter, searchQ, sortCol, sortDir);
  }, [load, sourceFilter, statusFilter, translationFilter, searchQ, sortCol, sortDir]);

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

  const confirmCompany = useCallback(async (row: RawInput) => {
    if (!row.company_suggestion_id) return;
    setApprovingId(row.id);
    try {
      await api.approveCompanySuggestion(row.company_suggestion_id);
      toast.success("Company created.");
      setItems((current) => current.map((item) =>
        item.id === row.id
          ? { ...item, company_suggestion_status: "approved", can_approve_company: false }
          : item,
      ));
    } catch {
      toast.error("Failed to confirm company.");
    } finally {
      setApprovingId(null);
    }
  }, []);

  const columns = useMemo<ColumnDef<RawInput>[]>(
    () => {
      const cols: ColumnDef<RawInput>[] = [];

      if (!sourceName) {
        cols.push({
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
        });
      }

      cols.push(
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
      );

      if (requiresTranslation) {
        cols.push({
          accessorKey: "translation_status",
          header: "Translation",
          enableSorting: false,
          cell: ({ row }) => {
            if (!sourceName && row.original.source !== "brreg") {
              return <span className="text-xs text-muted-foreground">not required</span>;
            }
            const status = row.original.translation_status ?? "pending";
            return (
              <Badge variant="outline" className={`text-xs ${translationBadgeClass(status)}`}>
                {status}
              </Badge>
            );
          },
        });
      }

      cols.push({
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
      });

      if (showConfirmAction) {
        cols.push({
          id: "actions",
          header: "",
          enableSorting: false,
          cell: ({ row }) => {
            const reason = approvalBlockedReason(row.original);
            const disabled = Boolean(reason) || approvingId === row.original.id;
            return (
              <div className="flex justify-end" onClick={(event) => event.stopPropagation()}>
                <span title={reason}>
                  <Button size="sm" disabled={disabled} onClick={() => confirmCompany(row.original)}>
                    <CheckCircle2 className="size-4" />
                    {approvingId === row.original.id ? "Confirming..." : "Confirm"}
                  </Button>
                </span>
              </div>
            );
          },
        });
      }

      return cols;
    },
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [sourceName, requiresTranslation, showConfirmAction, sortCol, sortDir, approvingId, confirmCompany],
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

        {!sourceName && (
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
        )}

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

        {requiresTranslation && (
          <select
            className="h-8 rounded-md border bg-background px-2 text-sm"
            value={translationFilter}
            onChange={(e) => setTranslationFilter(e.target.value)}
          >
            <option value="">All translations</option>
            {TRANSLATION_OPTIONS.map((s) => (
              <option key={s} value={s}>{s}</option>
            ))}
          </select>
        )}

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
              onClick={() => load(page - 1, sourceFilter, statusFilter, translationFilter, searchQ, sortCol, sortDir)}
            >
              Previous
            </Button>
            <Button
              size="sm"
              variant="outline"
              disabled={page >= totalPages || loading}
              onClick={() => load(page + 1, sourceFilter, statusFilter, translationFilter, searchQ, sortCol, sortDir)}
            >
              Next
            </Button>
          </div>
        </div>
      )}

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
