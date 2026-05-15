import { useEffect, useState } from "react";
import { Link, useParams } from "react-router";
import { ChevronLeft, ExternalLink } from "lucide-react";
import { api } from "~/lib/api";
import { pgrest } from "~/lib/pgrest";
import type { CompanyDetail, VCompanySource } from "~/types/api";
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

const SOURCE_TYPE_LABELS: Record<string, string> = {
  company_registry: "Company Registry",
  financial: "Financial Database",
  open_data: "Open Data",
  web: "Web Crawl",
};

export default function CompanyDetailPage() {
  const { id } = useParams<{ id: string }>();
  const [company, setCompany] = useState<CompanyDetail>();
  const [sources, setSources] = useState<VCompanySource[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string>();

  useEffect(() => {
    if (!id) return;
    Promise.all([
      api.getCompany(id),
      pgrest<VCompanySource>("v_company_sources", { "company_id": `eq.${id}` }),
    ])
      .then(([company, sourcesRes]) => {
        setCompany({ ...company, domains: company.domains ?? [] });
        setSources(sourcesRes.data);
      })
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
            <p className="text-xs text-muted-foreground uppercase tracking-wide">Added</p>
            <p className="mt-1 text-sm">{formatDate(company.created_at)}</p>
          </div>
        </CardContent>
      </Card>

      {sources.length > 0 && (
        <div>
          <h2 className="mb-3 text-base font-semibold">How we discovered this company</h2>
          <div className="rounded-md border">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Source</TableHead>
                  <TableHead>Type</TableHead>
                  <TableHead>External ID</TableHead>
                  <TableHead>Fetched</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {sources.map((s) => (
                  <TableRow key={s.source_id}>
                    <TableCell>
                      <span className="font-medium">{s.source_display_name}</span>
                      <span className="ml-2 text-xs text-muted-foreground font-mono">{s.source_name}</span>
                    </TableCell>
                    <TableCell>
                      <Badge variant="outline">
                        {SOURCE_TYPE_LABELS[s.source_type] ?? s.source_type}
                      </Badge>
                    </TableCell>
                    <TableCell>
                      {s.external_id ? (
                        <span className="font-mono text-xs">{s.external_id}</span>
                      ) : (
                        <span className="text-muted-foreground">—</span>
                      )}
                    </TableCell>
                    <TableCell className="text-sm text-muted-foreground">
                      {s.fetched_at ? formatDate(s.fetched_at) : "—"}
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </div>
        </div>
      )}

      <div>
        <h2 className="mb-3 text-base font-semibold">Associated Domains ({company.domains.length})</h2>
        {company.domains.length === 0 ? (
          <p className="text-sm text-muted-foreground">No domains found for this company.</p>
        ) : (
          <div className="rounded-md border">
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
                    <TableCell>
                      <a
                        href={`https://${d.domain}`}
                        target="_blank"
                        rel="noopener noreferrer"
                        className="font-mono text-primary hover:underline inline-flex items-center gap-1"
                      >
                        {d.domain}
                        <ExternalLink className="size-3 opacity-60" />
                      </a>
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
          </div>
        )}
      </div>
    </div>
  );
}
