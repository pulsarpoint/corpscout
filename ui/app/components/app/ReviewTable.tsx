import type { ReviewCandidate } from "~/types/api";
import { signalColor, confidenceColor } from "~/lib/utils";
import { Button } from "~/components/ui/button";
import { Badge } from "~/components/ui/badge";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "~/components/ui/table";

interface ReviewTableProps {
  items: ReviewCandidate[];
  onApprove: (id: string) => Promise<void>;
  onReject: (id: string) => Promise<void>;
  onView: (candidate: ReviewCandidate) => void;
  actionLoading?: string;
}

export function ReviewTable({
  items,
  onApprove,
  onReject,
  onView,
  actionLoading,
}: ReviewTableProps) {
  return (
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead>Company</TableHead>
          <TableHead>Domain</TableHead>
          <TableHead>Signal</TableHead>
          <TableHead>Confidence</TableHead>
          <TableHead className="text-right">Actions</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {items.map((item) => (
          <TableRow key={item.id}>
            <TableCell className="font-medium">{item.company_name}</TableCell>
            <TableCell className="font-mono text-primary">{item.domain}</TableCell>
            <TableCell>
              <Badge className={signalColor(item.signal)} variant="outline">
                {item.signal}
              </Badge>
            </TableCell>
            <TableCell>
              <span className={`font-bold ${confidenceColor(item.confidence)}`}>
                {item.confidence}
              </span>
            </TableCell>
            <TableCell className="text-right">
              <div className="flex justify-end gap-1">
                <Button
                  size="sm"
                  variant="default"
                  disabled={actionLoading === item.id}
                  onClick={() => onApprove(item.id)}
                >
                  Approve
                </Button>
                <Button
                  size="sm"
                  variant="destructive"
                  disabled={actionLoading === item.id}
                  onClick={() => onReject(item.id)}
                >
                  Reject
                </Button>
                <Button
                  size="sm"
                  variant="outline"
                  onClick={() => onView(item)}
                >
                  View
                </Button>
              </div>
            </TableCell>
          </TableRow>
        ))}
      </TableBody>
    </Table>
  );
}
