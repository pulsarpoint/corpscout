import { Link } from "react-router";
import type { DataSource } from "~/types/api";
import { timeAgo } from "~/lib/utils";
import { Button } from "~/components/ui/button";
import { Switch } from "~/components/ui/switch";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "~/components/ui/table";

interface SourcesTableProps {
  sources: DataSource[];
  onToggle: (name: string, enabled: boolean) => void;
  onTrigger: (name: string) => void;
  triggeringName?: string;
}

export function SourcesTable({ sources, onToggle, onTrigger, triggeringName }: SourcesTableProps) {
  return (
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead>Name</TableHead>
          <TableHead>Group</TableHead>
          <TableHead>Enabled</TableHead>
          <TableHead>Schedule</TableHead>
          <TableHead>Last Started</TableHead>
          <TableHead></TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {sources.map((s) => (
          <TableRow key={s.name}>
            <TableCell className="font-medium">
              <Link
                to={`/sources/${s.name}`}
                className="hover:underline text-foreground"
              >
                {s.display_name || s.name}
              </Link>
              {s.description && (
                <p className="text-xs text-muted-foreground mt-0.5 max-w-sm line-clamp-2">{s.description}</p>
              )}
            </TableCell>
            <TableCell className="text-sm text-muted-foreground">{s.source_group}</TableCell>
            <TableCell>
              <Switch
                checked={s.enabled}
                onCheckedChange={(checked) => onToggle(s.name, checked)}
              />
            </TableCell>
            <TableCell>{s.schedule_expression ?? "-"}</TableCell>
            <TableCell className="text-sm">
              {s.last_started_at ? timeAgo(s.last_started_at) : "Never"}
            </TableCell>
            <TableCell>
              <Button
                size="sm"
                variant="outline"
                disabled={triggeringName === s.name}
                onClick={() => onTrigger(s.name)}
              >
                {triggeringName === s.name ? "Queuing…" : "Trigger"}
              </Button>
            </TableCell>
          </TableRow>
        ))}
      </TableBody>
    </Table>
  );
}
