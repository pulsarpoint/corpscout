import { useCallback, useEffect, useMemo, useState } from "react";
import {
  flexRender,
  getCoreRowModel,
  useReactTable,
  type ColumnDef,
  type RowSelectionState,
} from "@tanstack/react-table";
import { Check, Languages, Minus } from "lucide-react";
import { toast } from "sonner";
import { api, errorMessage } from "~/lib/api";
import { pgrest } from "~/lib/pgrest";
import { formatDate } from "~/lib/utils";
import type { BrregTranslationStats, DataSource, SourceRawInput } from "~/types/api";
import { Alert, AlertDescription } from "~/components/ui/alert";
import { Badge } from "~/components/ui/badge";
import { Button } from "~/components/ui/button";
import { Checkbox } from "~/components/ui/checkbox";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "~/components/ui/table";
import { RawInputSheet } from "~/components/app/source-detail/RawInputSheet";
import { liveStatusFilter, statusClass } from "~/components/app/source-detail/sourceDetailUtils";

interface RawInputsTabProps {
  source: DataSource;
}

const PAGE_SIZE = 50;

export function RawInputsTab({ source }: RawInputsTabProps) {
  const [group, setGroup] = useState<"live" | "archive">("live");
  const [page, setPage] = useState(1);
  const [rows, setRows] = useState<SourceRawInput[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(false);
  const [selected, setSelected] = useState<SourceRawInput | null>(null);
  const [error, setError] = useState<string>();
  const [refreshToken, setRefreshToken] = useState(0);
  const [translationFilter, setTranslationFilter] = useState<"all" | "pending" | "translated" | "failed">("all");
  const [translationStats, setTranslationStats] = useState<BrregTranslationStats>();
  const [translating, setTranslating] = useState(false);
  const [rowSelection, setRowSelection] = useState<RowSelectionState>({});
  const isBrreg = source.name === "brreg";
  const selectedIds = useMemo(
    () => Object.entries(rowSelection).filter(([, selected]) => selected).map(([id]) => id),
    [rowSelection],
  );

  const columns = useMemo<ColumnDef<SourceRawInput>[]>(() => {
    const cols: ColumnDef<SourceRawInput>[] = [
      {
        header: "Status",
        accessorKey: "processing_status",
        cell: ({ row }) => (
          <Badge className={statusClass(row.original.processing_status)} variant="outline">
            {row.original.processing_status}
          </Badge>
        ),
      },
    ];
    if (isBrreg) {
      cols.unshift({
        id: "select",
        enableSorting: false,
        header: ({ table }) => (
          <Checkbox
            checked={table.getIsAllPageRowsSelected() ? true : table.getIsSomePageRowsSelected() ? "indeterminate" : false}
            onCheckedChange={(checked) => table.toggleAllPageRowsSelected(!!checked)}
            aria-label="Select rows"
          />
        ),
        cell: ({ row }) => (
          <Checkbox
            checked={row.getIsSelected()}
            disabled={!row.getCanSelect()}
            onCheckedChange={(checked) => row.toggleSelected(!!checked)}
            aria-label="Select row"
            onClick={(event) => event.stopPropagation()}
          />
        ),
      });
      cols.splice(1, 0, {
        header: "Translation",
        accessorKey: "translation_status",
        cell: ({ row }) => {
          const status = row.original.translation_status ?? "pending";
          return (
            <Badge className={translationStatusClass(status)} variant="outline">
              {status}
            </Badge>
          );
        },
      });
    }
    cols.push(
      {
        header: "Native ID",
        accessorKey: "source_native_id",
        cell: ({ row }) => (
          <span className="block max-w-[16rem] truncate font-medium">
            {row.original.source_native_id || row.original.id}
          </span>
        ),
      },
      {
        header: "First Seen",
        accessorKey: "first_seen_at",
        cell: ({ row }) => formatDate(row.original.first_seen_at),
      },
      {
        header: "Attempts",
        accessorKey: "processing_attempts",
        cell: ({ row }) => (
          <span className="tabular-nums">{row.original.processing_attempts}</span>
        ),
      },
      {
        header: "Error",
        accessorKey: "processing_error",
        cell: ({ row }) => (
          <span className="block max-w-[22rem] truncate text-muted-foreground">
            {row.original.processing_error ?? "-"}
          </span>
        ),
      },
      {
        header: "Suggestion",
        accessorKey: "has_suggestion",
        cell: ({ row }) => (
          row.original.has_suggestion ? (
            <Check className="size-4 text-green-700" aria-label="Suggestion produced" />
          ) : (
            <Minus className="size-4 text-muted-foreground" aria-label="No suggestion produced" />
          )
        ),
      },
    );
    return cols;
  }, [isBrreg]);

  const table = useReactTable({
    data: rows,
    columns,
    state: { rowSelection },
    getRowId: (row) => row.id,
    onRowSelectionChange: setRowSelection,
    getCoreRowModel: getCoreRowModel(),
    manualPagination: true,
    pageCount: Math.ceil(total / PAGE_SIZE),
    enableRowSelection: (row) => {
      const status = row.original.translation_status ?? "pending";
      return isBrreg && (status === "pending" || status === "failed");
    },
  });

  useEffect(() => {
    setRowSelection({});
  }, [rows]);

  const fetchRows = useCallback(() => {
    let cancelled = false;

    setLoading(true);
    setError(undefined);
    setRows([]);
    setTotal(0);

    const filters: Record<string, string | number> = {
      source_name: `eq.${source.name}`,
      processing_status: liveStatusFilter(group),
      order: "last_seen_at.desc",
      limit: PAGE_SIZE,
      offset: (page - 1) * PAGE_SIZE,
    };
    if (isBrreg && translationFilter !== "all") {
      filters.translation_status = `eq.${translationFilter}`;
    }

    pgrest<SourceRawInput>("v_source_raw_inputs", filters)
      .then((result) => {
        if (cancelled) return;
        const nextPageCount = Math.max(1, Math.ceil(result.total / PAGE_SIZE));
        if (page > nextPageCount) {
          setTotal(result.total);
          setPage(nextPageCount);
          return;
        }
        setRows(result.data);
        setTotal(result.total);
      })
      .catch(() => {
        if (!cancelled) {
          setRows([]);
          setTotal(0);
          setError("Failed to load raw inputs.");
        }
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });

    return () => {
      cancelled = true;
    };
  }, [group, isBrreg, page, source.name, translationFilter]);

  useEffect(() => fetchRows(), [fetchRows, refreshToken]);

  const fetchTranslationStats = useCallback(() => {
    if (!isBrreg) return;
    let cancelled = false;
    api.getBrregTranslationStats()
      .then((stats) => {
        if (!cancelled) setTranslationStats(stats);
      })
      .catch(() => {
        if (!cancelled) setTranslationStats(undefined);
      });
    return () => {
      cancelled = true;
    };
  }, [isBrreg]);

  useEffect(() => fetchTranslationStats(), [fetchTranslationStats, refreshToken]);

  function selectGroup(nextGroup: "live" | "archive") {
    setGroup(nextGroup);
    setPage(1);
  }

  async function translateAll() {
    setTranslating(true);
    try {
      await api.translateBrreg();
      toast.success("Brreg translation workflow started.");
      setRefreshToken((current) => current + 1);
    } catch (err) {
      toast.error(errorMessage(err, "Failed to start Brreg translation."));
    } finally {
      setTranslating(false);
    }
  }

  async function translateSelected() {
    if (selectedIds.length === 0) return;

    setTranslating(true);
    try {
      await api.translateBrreg({ ids: selectedIds });
      toast.success("Brreg translation workflow started.");
      setRowSelection({});
      setRefreshToken((current) => current + 1);
    } catch (err) {
      toast.error(errorMessage(err, "Failed to start Brreg translation."));
    } finally {
      setTranslating(false);
    }
  }

  const pageCount = Math.ceil(total / PAGE_SIZE);
  const start = total === 0 ? 0 : (page - 1) * PAGE_SIZE + 1;
  const end = Math.min(page * PAGE_SIZE, total);

  return (
    <div className="space-y-4">
      <div className="inline-flex rounded-md border bg-background p-1">
        <Button
          size="sm"
          variant={group === "live" ? "secondary" : "ghost"}
          onClick={() => selectGroup("live")}
        >
          Live Queue
        </Button>
        <Button
          size="sm"
          variant={group === "archive" ? "secondary" : "ghost"}
          onClick={() => selectGroup("archive")}
        >
          Archive
        </Button>
      </div>

      {isBrreg && (
        <div className="flex flex-col gap-3 rounded-md border p-3 sm:flex-row sm:items-center sm:justify-between">
          <div className="flex flex-wrap items-center gap-2 text-sm">
            <Badge variant="outline">Pending {translationStats?.pending.toLocaleString() ?? "-"}</Badge>
            <Badge variant="outline">Translated {translationStats?.translated.toLocaleString() ?? "-"}</Badge>
            <Badge variant="outline">Failed {translationStats?.failed.toLocaleString() ?? "-"}</Badge>
            <Badge variant="outline">Ready {translationStats?.ready_to_process.toLocaleString() ?? "-"}</Badge>
          </div>
          <div className="flex flex-wrap items-center gap-2">
            {(["all", "pending", "translated", "failed"] as const).map((filter) => (
              <Button
                key={filter}
                size="sm"
                variant={translationFilter === filter ? "secondary" : "outline"}
                onClick={() => {
                  setTranslationFilter(filter);
                  setPage(1);
                }}
              >
                {filter[0].toUpperCase() + filter.slice(1)}
              </Button>
            ))}
            <Button size="sm" disabled={translating || (translationStats?.pending ?? 0) === 0} onClick={translateAll}>
              <Languages className="size-4" />
              {translating ? "Starting..." : "Translate All"}
            </Button>
            {selectedIds.length > 0 && (
              <Button size="sm" disabled={translating} onClick={translateSelected}>
                <Languages className="size-4" />
                Translate Selected ({selectedIds.length})
              </Button>
            )}
          </div>
        </div>
      )}

      {error && (
        <Alert variant="destructive">
          <AlertDescription>{error}</AlertDescription>
        </Alert>
      )}

      <div className="rounded-md border">
        <Table>
          <TableHeader>
            {table.getHeaderGroups().map((headerGroup) => (
              <TableRow key={headerGroup.id}>
                {headerGroup.headers.map((header) => (
                  <TableHead key={header.id}>
                    {flexRender(header.column.columnDef.header, header.getContext())}
                  </TableHead>
                ))}
              </TableRow>
            ))}
          </TableHeader>
          <TableBody>
            {loading ? (
              <TableRow>
                <TableCell colSpan={columns.length} className="h-32 text-center text-muted-foreground">
                  Loading...
                </TableCell>
              </TableRow>
            ) : table.getRowModel().rows.length === 0 ? (
              <TableRow>
                <TableCell colSpan={columns.length} className="h-32 text-center text-muted-foreground">
                  No raw inputs.
                </TableCell>
              </TableRow>
            ) : (
              table.getRowModel().rows.map((row) => (
                <TableRow
                  key={row.id}
                  className="cursor-pointer"
                  tabIndex={0}
                  onClick={() => setSelected(row.original)}
                  onKeyDown={(event) => {
                    if (event.key === "Enter" || event.key === " ") {
                      event.preventDefault();
                      setSelected(row.original);
                    }
                  }}
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

      <div className="flex items-center justify-between text-sm text-muted-foreground">
        <span>{total === 0 ? "No results" : `${start}-${end} of ${total.toLocaleString()}`}</span>
        <div className="flex items-center gap-2">
          <Button
            size="sm"
            variant="outline"
            disabled={page <= 1}
            onClick={() => setPage((current) => Math.max(1, current - 1))}
          >
            Previous
          </Button>
          <span>Page {page} of {pageCount || 1}</span>
          <Button
            size="sm"
            variant="outline"
            disabled={page >= pageCount}
            onClick={() => setPage((current) => current + 1)}
          >
            Next
          </Button>
        </div>
      </div>

      <RawInputSheet
        source={source}
        row={selected}
        open={selected !== null}
        onOpenChange={(open) => {
          if (!open) setSelected(null);
        }}
        onChanged={() => setRefreshToken((current) => current + 1)}
      />
    </div>
  );
}

function translationStatusClass(status: string) {
  switch (status) {
    case "translated":
      return "border-green-200 bg-green-100 text-green-800";
    case "failed":
      return "border-red-200 bg-red-100 text-red-800";
    case "translating":
      return "border-blue-200 bg-blue-100 text-blue-800";
    default:
      return "border-amber-200 bg-amber-100 text-amber-800";
  }
}
