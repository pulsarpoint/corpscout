import { Link } from "react-router";
import type { Company } from "~/types/api";
import { Badge } from "~/components/ui/badge";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "~/components/ui/table";

function statusBadge(status: string) {
  if (status === "active") return "bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200";
  if (status === "dissolved") return "bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200";
  return "bg-gray-100 text-gray-800 dark:bg-gray-800 dark:text-gray-200";
}

interface CompaniesTableProps {
  companies: Company[];
}

export function CompaniesTable({ companies }: CompaniesTableProps) {
  return (
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead>Name</TableHead>
          <TableHead>Registration #</TableHead>
          <TableHead>LEI</TableHead>
          <TableHead>Status</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {companies.map((c) => (
          <TableRow key={c.id} className="cursor-pointer hover:bg-muted/50">
            <TableCell>
              <Link to={`/companies/${c.id}`} className="font-medium hover:underline">
                {c.name}
              </Link>
            </TableCell>
            <TableCell className="font-mono text-sm">{c.registration_number ?? "—"}</TableCell>
            <TableCell className="font-mono text-sm">{c.lei ?? "—"}</TableCell>
            <TableCell>
              <Badge className={statusBadge(c.status)} variant="outline">
                {c.status}
              </Badge>
            </TableCell>
          </TableRow>
        ))}
      </TableBody>
    </Table>
  );
}
