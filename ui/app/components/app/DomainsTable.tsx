import { Link } from "react-router";
import type { DomainWithCompany } from "~/types/api";
import { signalColor, confidenceColor, formatDate } from "~/lib/utils";
import { Badge } from "~/components/ui/badge";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "~/components/ui/table";

interface DomainsTableProps {
  domains: DomainWithCompany[];
}

export function DomainsTable({ domains }: DomainsTableProps) {
  return (
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead>Domain</TableHead>
          <TableHead>Company</TableHead>
          <TableHead>Signal</TableHead>
          <TableHead>Confidence</TableHead>
          <TableHead>Status</TableHead>
          <TableHead>First Seen</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {domains.map((d) => (
          <TableRow key={d.id}>
            <TableCell className="font-mono text-primary">{d.domain}</TableCell>
            <TableCell>
              <Link to={`/companies/${d.company_id}`} className="hover:underline text-sm">
                {d.company_name}
              </Link>
            </TableCell>
            <TableCell>
              <Badge className={signalColor(d.signal)} variant="outline">
                {d.signal}
              </Badge>
            </TableCell>
            <TableCell>
              <span className={`font-bold ${confidenceColor(d.confidence)}`}>
                {d.confidence}
              </span>
            </TableCell>
            <TableCell>{d.status}</TableCell>
            <TableCell className="text-sm">{formatDate(d.first_seen_at)}</TableCell>
          </TableRow>
        ))}
      </TableBody>
    </Table>
  );
}
