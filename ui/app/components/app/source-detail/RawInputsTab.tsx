import { useCallback, useEffect, useMemo, useState } from "react";
import {
  flexRender,
  getCoreRowModel,
  useReactTable,
  type ColumnDef,
} from "@tanstack/react-table";
import { Check, Minus } from "lucide-react";
import { pgrest } from "~/lib/pgrest";
import { formatDate } from "~/lib/utils";
import type { DataSource, SourceRawInput } from "~/types/api";
import { Alert, AlertDescription } from "~/components/ui/alert";
import { Badge } from "~/components/ui/badge";
import { Button } from "~/components/ui/button";
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

  const columns = useMemo<ColumnDef<SourceRawInput>[]>(
    () => [
      {
        header: "Status",
        accessorKey: "processing_status",
        cell: ({ row }) => (
          <Badge className={statusClass(row.original.processing_status)} variant="outline">
            {row.original.processing_status}
          </Badge>
        ),
      },
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
    ],
    [],
  );

  const table = useReactTable({
    data: rows,
    columns,
    getCoreRowModel: getCoreRowModel(),
    manualPagination: true,
    pageCount: Math.ceil(total / PAGE_SIZE),
  });

  const fetchRows = useCallback(() => {
    let cancelled = false;

    setLoading(true);
    setError(undefined);

    pgrest<SourceRawInput>("v_source_raw_inputs", {
      source_name: `eq.${source.name}`,
      processing_status: liveStatusFilter(group),
      order: "last_seen_at.desc",
      limit: PAGE_SIZE,
      offset: (page - 1) * PAGE_SIZE,
    })
      .then((result) => {
        if (cancelled) return;
        setRows(result.data);
        setTotal(result.total);
      })
      .catch(() => {
        if (!cancelled) setError("Failed to load raw inputs.");
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });

    return () => {
      cancelled = true;
    };
  }, [group, page, source.name]);

  useEffect(() => fetchRows(), [fetchRows, refreshToken]);

  function selectGroup(nextGroup: "live" | "archive") {
    setGroup(nextGroup);
    setPage(1);
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
