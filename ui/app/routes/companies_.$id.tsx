import { useEffect, useState } from "react";
import { Link, useParams } from "react-router";
import { ChevronLeft, ExternalLink, MapPin, Phone, Mail, Briefcase, Globe, Building2 } from "lucide-react";
import { pgrest } from "~/lib/pgrest";
import type {
  VCompany,
  VCompanySource,
  VCompanyLocation,
  VCompanyPhone,
  VCompanyEmail,
  VCompanyIndustry,
  VCompanyMarket,
  VCompanyService,
} from "~/types/api";
import { signalColor, confidenceColor, formatDate } from "~/lib/utils";
import { Badge } from "~/components/ui/badge";
import { Card, CardContent, CardHeader, CardTitle } from "~/components/ui/card";
import { Skeleton } from "~/components/ui/skeleton";
import { Alert, AlertDescription } from "~/components/ui/alert";
import {
  Table, TableBody, TableCell, TableHead, TableHeader, TableRow,
} from "~/components/ui/table";

const LOCATION_TYPE_LABELS: Record<string, string> = {
  headquarters: "Headquarters",
  registered_address: "Registered Address",
  office: "Office",
};

const SOURCE_TYPE_LABELS: Record<string, string> = {
  company_registry: "Company Registry",
  financial: "Financial Database",
  open_data: "Open Data",
  web: "Web Crawl",
  global_aggregator: "Global Aggregator",
};

function Section({ icon, title, children }: { icon: React.ReactNode; title: string; children: React.ReactNode }) {
  return (
    <div>
      <h2 className="mb-3 text-base font-semibold flex items-center gap-2">
        {icon}
        {title}
      </h2>
      {children}
    </div>
  );
}

function EmptyState({ message }: { message: string }) {
  return <p className="text-sm text-muted-foreground">{message}</p>;
}

type VCompanyDomain = {
  id: string;
  company_id: string;
  domain_id: string;
  domain: string;
  relationship_type: string;
  status: string;
  signal: string;
  confidence: number;
  evidence: Record<string, unknown> | null;
  first_seen_at: string;
  last_seen_at: string;
};

export default function CompanyDetailPage() {
  const { id } = useParams<{ id: string }>();
  const [company, setCompany] = useState<VCompany>();
  const [sources, setSources] = useState<VCompanySource[]>([]);
  const [domains, setDomains] = useState<VCompanyDomain[]>([]);
  const [locations, setLocations] = useState<VCompanyLocation[]>([]);
  const [phones, setPhones] = useState<VCompanyPhone[]>([]);
  const [emails, setEmails] = useState<VCompanyEmail[]>([]);
  const [industries, setIndustries] = useState<VCompanyIndustry[]>([]);
  const [markets, setMarkets] = useState<VCompanyMarket[]>([]);
  const [services, setServices] = useState<VCompanyService[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string>();

  useEffect(() => {
    if (!id) return;
    const q = `eq.${id}`;
    Promise.all([
      pgrest<VCompany>("v_companies", { id: q, limit: 1 }),
      pgrest<VCompanySource>("v_company_sources", { company_id: q }),
      pgrest<VCompanyDomain>("v_company_domains", { company_id: q }),
      pgrest<VCompanyLocation>("v_company_locations", { company_id: q }),
      pgrest<VCompanyPhone>("v_company_phones", { company_id: q }),
      pgrest<VCompanyEmail>("v_company_emails", { company_id: q }),
      pgrest<VCompanyIndustry>("v_company_industries", { company_id: q }),
      pgrest<VCompanyMarket>("v_company_markets", { company_id: q }),
      pgrest<VCompanyService>("v_company_services", { company_id: q }),
    ])
      .then(([c, src, dom, loc, ph, em, ind, mkt, svc]) => {
        if (c.data.length === 0) { setError("Company not found."); return; }
        setCompany(c.data[0]);
        setSources(src.data);
        setDomains(dom.data);
        setLocations(loc.data);
        setPhones(ph.data);
        setEmails(em.data);
        setIndustries(ind.data);
        setMarkets(mkt.data);
        setServices(svc.data);
      })
      .catch(() => setError("Failed to load company."))
      .finally(() => setLoading(false));
  }, [id]);

  if (loading) return <div className="space-y-4"><Skeleton className="h-40 w-full" /><Skeleton className="h-32 w-full" /></div>;
  if (error || !company) return <Alert variant="destructive"><AlertDescription>{error}</AlertDescription></Alert>;

  const employeeRange = company.employee_estimate as { min?: number; max?: number; label?: string };
  const revenueRange = company.revenue_estimate as { min?: number; max?: number; label?: string; currency?: string };

  return (
    <div className="space-y-6">
      <div className="flex items-center gap-2">
        <Link to="/companies" className="text-sm text-muted-foreground hover:underline flex items-center gap-1">
          <ChevronLeft className="size-4" />Companies
        </Link>
      </div>

      {/* ── Core identity card ─────────────────────────────────────────────── */}
      <Card>
        <CardHeader className="pb-3">
          <div className="flex items-start justify-between gap-4">
            <div>
              <CardTitle className="text-xl">{company.name}</CardTitle>
              {company.short_name && company.short_name !== company.name && (
                <p className="text-sm text-muted-foreground mt-0.5">{company.short_name}</p>
              )}
            </div>
            <Badge variant="outline" className={
              company.status === "active" ? "text-green-700 border-green-300 bg-green-50" :
              company.status === "dissolved" ? "text-red-700 border-red-300 bg-red-50" :
              "text-yellow-700 border-yellow-300 bg-yellow-50"
            }>
              {company.status}
            </Badge>
          </div>
          {company.short_description && (
            <p className="text-sm text-muted-foreground mt-2">{company.short_description}</p>
          )}
        </CardHeader>
        <CardContent className="grid grid-cols-2 gap-4 sm:grid-cols-3 lg:grid-cols-4">
          <div>
            <p className="text-xs text-muted-foreground uppercase tracking-wide">Country</p>
            <p className="mt-1 text-sm font-medium">{company.country_name} <span className="text-muted-foreground font-mono text-xs">{company.country_iso2}</span></p>
          </div>
          <div>
            <p className="text-xs text-muted-foreground uppercase tracking-wide">Registration #</p>
            <p className="mt-1 font-mono text-sm">{company.registration_number ?? "—"}</p>
          </div>
          <div>
            <p className="text-xs text-muted-foreground uppercase tracking-wide">LEI</p>
            <p className="mt-1 font-mono text-sm break-all">{company.lei ?? "—"}</p>
          </div>
          {company.website && (
            <div>
              <p className="text-xs text-muted-foreground uppercase tracking-wide">Website</p>
              <a
                href={company.website.startsWith("http") ? company.website : `https://${company.website}`}
                target="_blank" rel="noopener noreferrer"
                className="mt-1 text-sm text-primary hover:underline flex items-center gap-1"
              >
                {company.website.replace(/^https?:\/\//, "")}
                <ExternalLink className="size-3 opacity-60" />
              </a>
            </div>
          )}
          {company.founded_year && (
            <div>
              <p className="text-xs text-muted-foreground uppercase tracking-wide">Founded</p>
              <p className="mt-1 text-sm">{company.founded_year}</p>
            </div>
          )}
          {employeeRange.label && (
            <div>
              <p className="text-xs text-muted-foreground uppercase tracking-wide">Employees</p>
              <p className="mt-1 text-sm">{employeeRange.label}</p>
            </div>
          )}
          {revenueRange.label && (
            <div>
              <p className="text-xs text-muted-foreground uppercase tracking-wide">Revenue</p>
              <p className="mt-1 text-sm">{revenueRange.label}</p>
            </div>
          )}
          {company.headquarters_location && (
            <div>
              <p className="text-xs text-muted-foreground uppercase tracking-wide">HQ Location</p>
              <p className="mt-1 text-sm">{company.headquarters_location}</p>
            </div>
          )}
          <div>
            <p className="text-xs text-muted-foreground uppercase tracking-wide">Added</p>
            <p className="mt-1 text-sm">{formatDate(company.created_at)}</p>
          </div>
        </CardContent>
      </Card>

      {/* ── Industry / markets / services tags ────────────────────────────── */}
      {(industries.length > 0 || markets.length > 0 || services.length > 0) && (
        <div className="flex flex-wrap gap-4">
          {industries.length > 0 && (
            <div>
              <p className="text-xs text-muted-foreground uppercase tracking-wide mb-1.5">Industries</p>
              <div className="flex flex-wrap gap-1.5">
                {industries.map((i) => <Badge key={i.id} variant="secondary">{i.industry}</Badge>)}
              </div>
            </div>
          )}
          {markets.length > 0 && (
            <div>
              <p className="text-xs text-muted-foreground uppercase tracking-wide mb-1.5">Markets</p>
              <div className="flex flex-wrap gap-1.5">
                {markets.map((m) => <Badge key={m.id} variant="outline">{m.market}</Badge>)}
              </div>
            </div>
          )}
          {services.length > 0 && (
            <div>
              <p className="text-xs text-muted-foreground uppercase tracking-wide mb-1.5">Services / Products</p>
              <div className="flex flex-wrap gap-1.5">
                {services.map((s) => <Badge key={s.id} variant="outline" className="text-blue-700 border-blue-300 bg-blue-50">{s.service}</Badge>)}
              </div>
            </div>
          )}
        </div>
      )}

      {/* ── Locations ─────────────────────────────────────────────────────── */}
      {locations.length > 0 && (
        <Section icon={<MapPin className="size-4" />} title="Locations">
          <div className="rounded-md border">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Type</TableHead>
                  <TableHead>Address</TableHead>
                  <TableHead>City</TableHead>
                  <TableHead>Region</TableHead>
                  <TableHead>Country</TableHead>
                  <TableHead>Source</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {locations.map((loc) => (
                  <TableRow key={loc.id}>
                    <TableCell>
                      <Badge variant="outline">{LOCATION_TYPE_LABELS[loc.location_type] ?? loc.location_type}</Badge>
                    </TableCell>
                    <TableCell className="text-sm">
                      {[loc.address_line1, loc.address_line2].filter(Boolean).join(", ") || "—"}
                    </TableCell>
                    <TableCell className="text-sm">{loc.city ?? "—"}</TableCell>
                    <TableCell className="text-sm">{loc.region ?? "—"}</TableCell>
                    <TableCell className="text-sm">
                      {loc.country ?? "—"}
                      {loc.country_code && <span className="ml-1 text-muted-foreground font-mono text-xs">{loc.country_code}</span>}
                    </TableCell>
                    <TableCell className="text-xs text-muted-foreground">{loc.source}</TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </div>
        </Section>
      )}

      {/* ── Phones / Emails ───────────────────────────────────────────────── */}
      {(phones.length > 0 || emails.length > 0) && (
        <div className="grid grid-cols-1 gap-6 sm:grid-cols-2">
          {phones.length > 0 && (
            <Section icon={<Phone className="size-4" />} title="Phone Numbers">
              <div className="rounded-md border divide-y">
                {phones.map((ph) => (
                  <div key={ph.id} className="px-4 py-2.5 flex items-center justify-between gap-2">
                    <div>
                      <a href={`tel:${ph.phone}`} className="text-sm font-mono hover:underline">{ph.phone}</a>
                      {ph.description && <p className="text-xs text-muted-foreground mt-0.5">{ph.description}</p>}
                    </div>
                    <Badge variant="outline" className="text-xs shrink-0">{ph.purpose}</Badge>
                  </div>
                ))}
              </div>
            </Section>
          )}
          {emails.length > 0 && (
            <Section icon={<Mail className="size-4" />} title="Email Addresses">
              <div className="rounded-md border divide-y">
                {emails.map((em) => (
                  <div key={em.id} className="px-4 py-2.5 flex items-center justify-between gap-2">
                    <div>
                      <a href={`mailto:${em.email}`} className="text-sm hover:underline">{em.email}</a>
                      {em.name && <p className="text-xs text-muted-foreground mt-0.5">{em.name}</p>}
                    </div>
                    <Badge variant="outline" className="text-xs shrink-0">{em.purpose}</Badge>
                  </div>
                ))}
              </div>
            </Section>
          )}
        </div>
      )}

      {/* ── Discovery sources ─────────────────────────────────────────────── */}
      {sources.length > 0 && (
        <Section icon={<Building2 className="size-4" />} title="How we discovered this company">
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
                      <Badge variant="outline">{SOURCE_TYPE_LABELS[s.source_type] ?? s.source_type}</Badge>
                    </TableCell>
                    <TableCell>
                      {s.external_id ? <span className="font-mono text-xs">{s.external_id}</span> : <span className="text-muted-foreground">—</span>}
                    </TableCell>
                    <TableCell className="text-sm text-muted-foreground">
                      {s.fetched_at ? formatDate(s.fetched_at) : "—"}
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </div>
        </Section>
      )}

      {/* ── Associated domains ────────────────────────────────────────────── */}
      <Section icon={<Globe className="size-4" />} title={`Associated Domains (${domains.length})`}>
        {domains.length === 0 ? (
          <EmptyState message="No domains found for this company." />
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
                {domains.map((d) => (
                  <TableRow key={d.id}>
                    <TableCell>
                      <a
                        href={`https://${d.domain}`} target="_blank" rel="noopener noreferrer"
                        className="font-mono text-primary hover:underline inline-flex items-center gap-1"
                      >
                        {d.domain}
                        <ExternalLink className="size-3 opacity-60" />
                      </a>
                    </TableCell>
                    <TableCell>
                      <Badge className={signalColor(d.signal)} variant="outline">{d.signal}</Badge>
                    </TableCell>
                    <TableCell>
                      <span className={`font-bold ${confidenceColor(d.confidence)}`}>{d.confidence}</span>
                    </TableCell>
                    <TableCell>{d.status}</TableCell>
                    <TableCell className="text-sm">{formatDate(d.first_seen_at)}</TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </div>
        )}
      </Section>
    </div>
  );
}
