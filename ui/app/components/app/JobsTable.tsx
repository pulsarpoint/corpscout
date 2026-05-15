import type { Job } from "~/types/api";
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

function stateBadge(state: string): string {
  if (state === "completed") return "bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200";
  if (state === "running") return "bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-200";
  if (state === "failed" || state === "discarded") return "bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200";
  if (state === "available" || state === "retryable") return "bg-yellow-100 text-yellow-800 dark:bg-yellow-900 dark:text-yellow-200";
  return "bg-gray-100 text-gray-800 dark:bg-gray-800 dark:text-gray-200";
}

interface JobsTableProps {
  jobs: Job[];
}

export function JobsTable({ jobs }: JobsTableProps) {
  return (
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead>ID</TableHead>
          <TableHead>Kind</TableHead>
          <TableHead>State</TableHead>
          <TableHead>Queue</TableHead>
          <TableHead>Attempt</TableHead>
          <TableHead>Scheduled</TableHead>
          <TableHead>Created</TableHead>
          <TableHead>Finalized</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {jobs.map((j) => (
          <TableRow key={j.id}>
            <TableCell className="font-mono text-xs">{j.id}</TableCell>
            <TableCell className="text-sm">{j.kind}</TableCell>
            <TableCell>
              <Badge className={stateBadge(j.state)} variant="outline">
                {j.state}
              </Badge>
            </TableCell>
            <TableCell className="text-sm">{j.queue}</TableCell>
            <TableCell className="text-sm">{j.attempt}/{j.max_attempts}</TableCell>
            <TableCell className="text-sm">{j.scheduled_at ? formatDate(j.scheduled_at) : "—"}</TableCell>
            <TableCell className="text-sm">{formatDate(j.created_at)}</TableCell>
            <TableCell className="text-sm">{j.finalized_at ? formatDate(j.finalized_at) : "—"}</TableCell>
          </TableRow>
        ))}
      </TableBody>
    </Table>
  );
}
