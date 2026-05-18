import { useState, useEffect, useRef, useMemo } from "react";
import {
  flexRender,
  getCoreRowModel,
  useReactTable,
  type ColumnDef,
  type SortingState,
  type OnChangeFn,
  type RowSelectionState,
} from "@tanstack/react-table";
import { ChevronUp, ChevronDown, ChevronsUpDown, X, SlidersHorizontal } from "lucide-react";
import { toast } from "sonner";
import { Button } from "~/components/ui/button";
import { Checkbox } from "~/components/ui/checkbox";
import { Input } from "~/components/ui/input";
import { Badge } from "~/components/ui/badge";
import { Popover, PopoverContent, PopoverTrigger } from "~/components/ui/popover";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "~/components/ui/table";

export type FilterDef =
  | { key: string; label: string; type: "select"; options: { value: string; label: string }[] }
  | { key: string; label: string; type: "min-number" };

export interface ActiveFilter {
  key: string;
  label: string;
  value: string;
  display: string;
}

interface BulkReviewTableProps<TData extends { id: string }> {
  columns: ColumnDef<TData, unknown>[];
  data: TData[];
  total: number;
  loading: boolean;
  page: number;
  pageSize?: number;
  onPageChange: (p: number) => void;
  sorting?: SortingState;
  onSortingChange?: OnChangeFn<SortingState>;
  onApprove: (ids: string[]) => Promise<void>;
  onReject: (ids: string[]) => Promise<void>;
  filterDefs?: FilterDef[];
  onFilterChange?: (filters: ActiveFilter[]) => void;
  onSearch?: (q: string) => void;
  onRowClick?: (row: TData) => void;
  searchPlaceholder?: string;
}

export function BulkReviewTable<TData extends { id: string }>({
  columns,
  data,
  total,
  loading,
  page,
  pageSize = 50,
  onPageChange,
  sorting = [],
  onSortingChange,
  onApprove,
  onReject,
  filterDefs = [],
  onFilterChange,
  onSearch,
  onRowClick,
  searchPlaceholder = "Search…",
}: BulkReviewTableProps<TData>) {
  const [rowSelection, setRowSelection] = useState<RowSelectionState>({});
  const [bulkLoading, setBulkLoading] = useState(false);
  const [activeFilters, setActiveFilters] = useState<ActiveFilter[]>([]);
  const [searchValue, setSearchValue] = useState("");
  const [filterOpen, setFilterOpen] = useState(false);
  const [pendingFilter, setPendingFilter] = useState<Record<string, string>>({});
  const debounceRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const onSearchRef = useRef(onSearch);

  useEffect(() => {
    onSearchRef.current = onSearch;
  });

  useEffect(() => {
    if (debounceRef.current) clearTimeout(debounceRef.current);
    debounceRef.current = setTimeout(() => onSearchRef.current?.(searchValue), 300);
    return () => { if (debounceRef.current) clearTimeout(debounceRef.current); };
  }, [searchValue]);

  useEffect(() => {
    setRowSelection({});
  }, [data]);

  const allColumns = useMemo<ColumnDef<TData, unknown>[]>(
    () => [
      {
        id: "select",
        header: ({ table }) => (
          <Checkbox
            checked={table.getIsAllPageRowsSelected()}
            onCheckedChange={(v) => table.toggleAllPageRowsSelected(!!v)}
            aria-label="Select all"
          />
        ),
        cell: ({ row }) => (
          <Checkbox
            checked={row.getIsSelected()}
            onCheckedChange={(v) => row.toggleSelected(!!v)}
            aria-label="Select row"
            onClick={(e) => e.stopPropagation()}
          />
        ),
        enableSorting: false,
      },
      ...columns,
    ],
    [columns],
  );

  const table = useReactTable({
    data,
    columns: allColumns,
    state: { sorting, rowSelection },
    getRowId: (row) => row.id,
    onSortingChange,
    onRowSelectionChange: setRowSelection,
    getCoreRowModel: getCoreRowModel(),
    manualSorting: true,
    manualPagination: true,
    pageCount: Math.ceil(total / pageSize),
    enableRowSelection: true,
  });

  const selectedIds = Object.keys(rowSelection);
  const pageCount = Math.ceil(total / pageSize);
  const start = total === 0 ? 0 : (page - 1) * pageSize + 1;
  const end = Math.min(page * pageSize, total);

  const handleBulkAction = async (action: "approve" | "reject") => {
    if (selectedIds.length === 0) return;
    setBulkLoading(true);
    try {
      if (action === "approve") {
        await onApprove(selectedIds);
        toast.success(`Approved ${selectedIds.length} items.`);
      } else {
        await onReject(selectedIds);
        toast.success(`Rejected ${selectedIds.length} items.`);
      }
      setRowSelection({});
    } catch {
      toast.error("Bulk action failed. Some items may not have been updated.");
    } finally {
      setBulkLoading(false);
    }
  };

  const applyFilter = () => {
    const newFilters: ActiveFilter[] = Object.entries(pendingFilter)
      .filter(([, v]) => v !== "")
      .map(([key, value]) => {
        const def = filterDefs.find((d) => d.key === key)!;
        const display =
          def.type === "select"
            ? (def.options.find((o) => o.value === value)?.label ?? value)
            : `≥ ${value}`;
        return { key, label: def.label, value, display };
      });
    setActiveFilters(newFilters);
    onFilterChange?.(newFilters);
    setFilterOpen(false);
  };

  const removeFilter = (key: string) => {
    const updated = activeFilters.filter((f) => f.key !== key);
    setActiveFilters(updated);
    setPendingFilter((prev) => { const next = { ...prev }; delete next[key]; return next; });
    onFilterChange?.(updated);
  };

  const clearFilters = () => {
    setActiveFilters([]);
    setPendingFilter({});
    onFilterChange?.([]);
  };

  const openFilter = () => {
    const init: Record<string, string> = {};
    for (const f of activeFilters) init[f.key] = f.value;
    setPendingFilter(init);
    setFilterOpen(true);
  };

  return (
    <div className="space-y-3">
      {/* Filter bar */}
      <div className="flex flex-wrap items-center gap-2">
        {onSearch && (
          <Input
            placeholder={searchPlaceholder}
            value={searchValue}
            onChange={(e) => setSearchValue(e.target.value)}
            className="h-8 w-56"
          />
        )}
        {filterDefs.length > 0 && (
          <Popover open={filterOpen} onOpenChange={setFilterOpen}>
            <PopoverTrigger asChild>
              <Button variant="outline" size="sm" className="h-8" onClick={openFilter}>
                <SlidersHorizontal className="mr-1 size-3" />
                Filter
              </Button>
            </PopoverTrigger>
            <PopoverContent className="w-64 space-y-3 p-4" align="start">
              {filterDefs.map((def) => (
                <div key={def.key} className="space-y-1">
                  <label className="text-xs font-medium text-muted-foreground">{def.label}</label>
                  {def.type === "select" ? (
                    <select
                      className="w-full rounded-md border bg-background px-2 py-1 text-sm"
                      value={pendingFilter[def.key] ?? ""}
                      onChange={(e) => setPendingFilter((p) => ({ ...p, [def.key]: e.target.value }))}
                    >
                      <option value="">Any</option>
                      {def.options.map((o) => (
                        <option key={o.value} value={o.value}>{o.label}</option>
                      ))}
                    </select>
                  ) : (
                    <Input
                      type="number"
                      placeholder="e.g. 70"
                      className="h-8"
                      value={pendingFilter[def.key] ?? ""}
                      onChange={(e) => setPendingFilter((p) => ({ ...p, [def.key]: e.target.value }))}
                    />
                  )}
                </div>
              ))}
              <div className="flex gap-2 pt-1">
                <Button size="sm" className="flex-1" onClick={applyFilter}>Apply</Button>
                <Button size="sm" variant="outline" onClick={() => setFilterOpen(false)}>Cancel</Button>
              </div>
            </PopoverContent>
          </Popover>
        )}
        {activeFilters.map((f) => (
          <Badge key={f.key} variant="secondary" className="gap-1 text-xs">
            {f.label}: {f.display}
            <button onClick={() => removeFilter(f.key)} className="ml-1 opacity-60 hover:opacity-100">
              <X className="size-3" />
            </button>
          </Badge>
        ))}
        {activeFilters.length > 0 && (
          <button onClick={clearFilters} className="text-xs text-muted-foreground hover:text-foreground">
            Clear all
          </button>
        )}
        <span className="ml-auto text-xs text-muted-foreground">
          {total.toLocaleString()} total
        </span>
      </div>

      {/* Bulk action bar */}
      {selectedIds.length > 0 && (
        <div className="flex items-center gap-3 rounded-md border border-blue-300 bg-blue-50 px-3 py-2 dark:border-blue-800 dark:bg-blue-950">
          <span className="text-sm font-medium text-blue-700 dark:text-blue-300">
            {selectedIds.length} selected
          </span>
          <Button
            size="sm"
            variant="default"
            className="h-7 bg-green-600 hover:bg-green-700"
            disabled={bulkLoading}
            onClick={() => handleBulkAction("approve")}
          >
            Approve {selectedIds.length}
          </Button>
          <Button
            size="sm"
            variant="destructive"
            className="h-7"
            disabled={bulkLoading}
            onClick={() => handleBulkAction("reject")}
          >
            Reject {selectedIds.length}
          </Button>
          <Button
            size="sm"
            variant="ghost"
            className="ml-auto h-7"
            onClick={() => setRowSelection({})}
          >
            Deselect all
          </Button>
        </div>
      )}

      {/* Table */}
      <div className="rounded-md border">
        <Table>
          <TableHeader>
            {table.getHeaderGroups().map((hg) => (
              <TableRow key={hg.id}>
                {hg.headers.map((header) => {
                  const canSort = header.column.getCanSort();
                  const sorted = header.column.getIsSorted();
                  return (
                    <TableHead
                      key={header.id}
                      className={canSort ? "cursor-pointer select-none" : ""}
                      onClick={canSort ? header.column.getToggleSortingHandler() : undefined}
                    >
                      <span className="flex items-center gap-1">
                        {flexRender(header.column.columnDef.header, header.getContext())}
                        {canSort && (
                          sorted === "asc" ? <ChevronUp className="size-3" /> :
                          sorted === "desc" ? <ChevronDown className="size-3" /> :
                          <ChevronsUpDown className="size-3 opacity-40" />
                        )}
                      </span>
                    </TableHead>
                  );
                })}
              </TableRow>
            ))}
          </TableHeader>
          <TableBody>
            {loading ? (
              <TableRow>
                <TableCell colSpan={allColumns.length} className="h-32 text-center text-muted-foreground">
                  Loading…
                </TableCell>
              </TableRow>
            ) : table.getRowModel().rows.length === 0 ? (
              <TableRow>
                <TableCell colSpan={allColumns.length} className="h-32 text-center">
                  <div className="text-muted-foreground">No results.</div>
                  {activeFilters.length > 0 && (
                    <button onClick={clearFilters} className="mt-1 text-xs text-primary hover:underline">
                      Clear filters
                    </button>
                  )}
                </TableCell>
              </TableRow>
            ) : (
              table.getRowModel().rows.map((row) => (
                <TableRow
                  key={row.id}
                  data-state={row.getIsSelected() ? "selected" : undefined}
                  className={onRowClick ? "cursor-pointer" : ""}
                  onClick={onRowClick ? () => onRowClick(row.original) : undefined}
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
      <div className="flex items-center justify-between text-sm text-muted-foreground">
        <span>{total === 0 ? "No results" : `${start}–${end} of ${total.toLocaleString()}`}</span>
        <div className="flex items-center gap-2">
          <Button variant="outline" size="sm" disabled={page <= 1} onClick={() => onPageChange(page - 1)}>
            Previous
          </Button>
          <span>Page {page} of {pageCount || 1}</span>
          <Button variant="outline" size="sm" disabled={page >= pageCount} onClick={() => onPageChange(page + 1)}>
            Next
          </Button>
        </div>
      </div>
    </div>
  );
}
