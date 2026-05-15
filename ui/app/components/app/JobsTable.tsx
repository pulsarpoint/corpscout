import type { Job } from "~/types/api";
import { timeAgo } from "~/lib/utils";
import { Badge } from "~/components/ui/badge";
import {
  CheckCircle2, XCircle, Clock, Loader2, AlertTriangle, Ban,
} from "lucide-react";
import {
  Table, TableBody, TableCell, TableHead, TableHeader, TableRow,
} from "~/components/ui/table";

const STATE_LABELS: Record<string, string> = {
  available:  "Queued",
  running:    "Running",
  completed:  "Done",
  retryable:  "Will Retry",
  scheduled:  "Scheduled",
  failed:     "Failed",
  discarded:  "Abandoned",
  cancelled:  "Cancelled",
};

function StateBadge({ state }: { state: string }) {
  const label = STATE_LABELS[state] ?? state;
  if (state === "completed")
    return <Badge className="bg-green-100 text-green-800 border-green-200 gap-1" variant="outline"><CheckCircle2 className="size-3" />{label}</Badge>;
  if (state === "running")
    return <Badge className="bg-blue-100 text-blue-800 border-blue-200 gap-1" variant="outline"><Loader2 className="size-3 animate-spin" />{label}</Badge>;
  if (state === "retryable")
    return <Badge className="bg-amber-100 text-amber-800 border-amber-200 gap-1" variant="outline"><AlertTriangle className="size-3" />{label}</Badge>;
  if (state === "failed" || state === "discarded")
    return <Badge className="bg-red-100 text-red-800 border-red-200 gap-1" variant="outline"><XCircle className="size-3" />{label}</Badge>;
  if (state === "scheduled")
    return <Badge className="bg-purple-100 text-purple-800 border-purple-200 gap-1" variant="outline"><Clock className="size-3" />{label}</Badge>;
  if (state === "cancelled")
    return <Badge className="bg-gray-100 text-gray-500 border-gray-200 gap-1" variant="outline"><Ban className="size-3" />{label}</Badge>;
  return <Badge variant="outline">{label}</Badge>;
}

interface JobsTableProps {
  jobs: Job[];
}

export function JobsTable({ jobs }: JobsTableProps) {
  return (
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead>State</TableHead>
          <TableHead>Attempts</TableHead>
          <TableHead>Scheduled</TableHead>
          <TableHead>Created</TableHead>
          <TableHead>Finalized</TableHead>
          <TableHead>Error</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {jobs.map((j) => (
          <TableRow key={j.id}>
            <TableCell><StateBadge state={j.state} /></TableCell>
            <TableCell className="text-sm">
              {j.attempt > 1
                ? <span className="text-amber-600 font-medium">{j.attempt}/{j.max_attempts}</span>
                : <span className="text-muted-foreground">{j.attempt}/{j.max_attempts}</span>}
            </TableCell>
            <TableCell className="text-sm text-muted-foreground">{j.scheduled_at ? timeAgo(j.scheduled_at) : "—"}</TableCell>
            <TableCell className="text-sm text-muted-foreground">{timeAgo(j.created_at)}</TableCell>
            <TableCell className="text-sm text-muted-foreground">{j.finalized_at ? timeAgo(j.finalized_at) : "—"}</TableCell>
            <TableCell className="text-xs text-red-600 max-w-xs truncate font-mono">{j.last_error ?? ""}</TableCell>
          </TableRow>
        ))}
      </TableBody>
    </Table>
  );
}
