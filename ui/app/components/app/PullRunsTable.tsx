import type { PullRun } from "~/types/api";
import { formatDate } from "~/lib/utils";
import { Badge } from "~/components/ui/badge";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "~/components/ui/table";
import { differenceInSeconds } from "date-fns";

function statusVariant(status: string): string {
  if (status === "completed") return "bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200";
  if (status === "failed") return "bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200";
  if (status === "running") return "bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-200";
  return "bg-gray-100 text-gray-800 dark:bg-gray-800 dark:text-gray-200";
}

function duration(run: PullRun): string {
  if (!run.completed_at) return "—";
  const secs = differenceInSeconds(new Date(run.completed_at), new Date(run.started_at));
  if (secs < 60) return `${secs}s`;
  return `${Math.floor(secs / 60)}m ${secs % 60}s`;
}

interface PullRunsTableProps {
  runs: PullRun[];
}

export function PullRunsTable({ runs }: PullRunsTableProps) {
  return (
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead>Source</TableHead>
          <TableHead>Status</TableHead>
          <TableHead className="text-right">Fetched</TableHead>
          <TableHead className="text-right">Upserted</TableHead>
          <TableHead>Started</TableHead>
          <TableHead>Duration</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {runs.map((run) => (
          <TableRow key={run.id}>
            <TableCell className="font-medium">{run.source_name}</TableCell>
            <TableCell>
              <Badge className={statusVariant(run.status)} variant="outline">
                {run.status}
              </Badge>
            </TableCell>
            <TableCell className="text-right">{run.records_fetched.toLocaleString()}</TableCell>
            <TableCell className="text-right">{run.records_upserted.toLocaleString()}</TableCell>
            <TableCell className="text-sm">{formatDate(run.started_at)}</TableCell>
            <TableCell className="text-sm">{duration(run)}</TableCell>
          </TableRow>
        ))}
      </TableBody>
    </Table>
  );
}
