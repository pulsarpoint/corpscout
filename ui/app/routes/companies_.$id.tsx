import { useEffect, useState } from "react";
import { Link, useParams } from "react-router";
import { ChevronLeft } from "lucide-react";
import { api } from "~/lib/api";
import type { CompanyDetail } from "~/types/api";
import { signalColor, confidenceColor, formatDate } from "~/lib/utils";
import { Badge } from "~/components/ui/badge";
import { Card, CardContent, CardHeader, CardTitle } from "~/components/ui/card";
import { Skeleton } from "~/components/ui/skeleton";
import { Alert, AlertDescription } from "~/components/ui/alert";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "~/components/ui/table";

export default function CompanyDetailPage() {
  const { id } = useParams<{ id: string }>();
  const [company, setCompany] = useState<CompanyDetail>();
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string>();

  useEffect(() => {
    if (!id) return;
    api.getCompany(id)
      .then((data) => setCompany({ ...data, domains: data.domains ?? [] }))
      .catch(() => setError("Company not found."))
      .finally(() => setLoading(false));
  }, [id]);

  if (loading) return <Skeleton className="h-64 w-full" />;
  if (error || !company) return <Alert variant="destructive"><AlertDescription>{error}</AlertDescription></Alert>;

  return (
    <div className="space-y-6">
      <div className="flex items-center gap-2">
        <Link to="/companies" className="text-sm text-muted-foreground hover:underline flex items-center gap-1">
          <ChevronLeft className="size-4" />
          Companies
        </Link>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>{company.name}</CardTitle>
        </CardHeader>
        <CardContent className="grid grid-cols-2 gap-4 sm:grid-cols-3">
          <div>
            <p className="text-xs text-muted-foreground uppercase tracking-wide">Status</p>
            <p className="mt-1 text-sm font-medium">{company.status}</p>
          </div>
          <div>
            <p className="text-xs text-muted-foreground uppercase tracking-wide">Registration #</p>
            <p className="mt-1 font-mono text-sm">{company.registration_number ?? "—"}</p>
          </div>
          <div>
            <p className="text-xs text-muted-foreground uppercase tracking-wide">LEI</p>
            <p className="mt-1 font-mono text-sm">{company.lei ?? "—"}</p>
          </div>
          <div>
            <p className="text-xs text-muted-foreground uppercase tracking-wide">Created</p>
            <p className="mt-1 text-sm">{formatDate(company.created_at)}</p>
          </div>
        </CardContent>
      </Card>

      <div>
        <h2 className="mb-3 text-base font-semibold">Associated Domains ({company.domains.length})</h2>
        {company.domains.length === 0 ? (
          <p className="text-sm text-muted-foreground">No domains found for this company.</p>
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Domain</TableHead>
                <TableHead>Signal</TableHead>
                <TableHead>Confidence</TableHead>
                <TableHead>Status</TableHead>
                <TableHead>First Seen</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {company.domains.map((d) => (
                <TableRow key={d.id}>
                  <TableCell className="font-mono text-primary">{d.domain}</TableCell>
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
        )}
      </div>
    </div>
  );
}
