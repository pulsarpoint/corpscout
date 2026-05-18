import { useEffect, useState } from "react";
import { Link, useParams } from "react-router";
import { ArrowLeft, ExternalLink } from "lucide-react";
import { Badge } from "~/components/ui/badge";
import { Button } from "~/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "~/components/ui/card";
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
} from "~/components/ui/sheet";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "~/components/ui/table";
import { CrawlDomainDialog } from "~/components/app/CrawlDomainDialog";
import {
  getDomain,
  getDomainCrawlJobs,
  getDomainCrawlPages,
  getDomainCrawlPageContent,
  getDomainFaviconUrl,
} from "~/lib/api";
import { pgrest } from "~/lib/pgrest";
import type { DomainDetail, DomainCrawlJob, DomainCrawlPage } from "~/types/api";

interface CompanyDomainRow {
  id: string;
  company_id: string;
  signal: string;
  confidence: number;
  status: string;
}

function crawlJobStatus(job: DomainCrawlJob): string {
  if (!job.river_state) return "pending";
  switch (job.river_state) {
    case "completed": return "completed";
    case "available":
    case "running":
    case "retryable": return "running";
    case "discarded":
    case "cancelled": return "failed";
    default: return job.river_state;
  }
}

function StatusBadge({ status }: { status: string }) {
  const variant =
    status === "completed" ? "default" :
    status === "running" ? "secondary" :
    status === "failed" ? "destructive" : "outline";
  return <Badge variant={variant}>{status}</Badge>;
}

export default function DomainDetailPage() {
  const { id } = useParams<{ id: string }>();
  const [domain, setDomain] = useState<DomainDetail | null>(null);
  const [companies, setCompanies] = useState<CompanyDomainRow[]>([]);
  const [crawlJobs, setCrawlJobs] = useState<DomainCrawlJob[]>([]);
  const [loading, setLoading] = useState(true);

  const [pagesJob, setPagesJob] = useState<DomainCrawlJob | null>(null);
  const [pages, setPages] = useState<DomainCrawlPage[]>([]);
  const [pagesLoading, setPagesLoading] = useState(false);

  const [contentPage, setContentPage] = useState<{ page: DomainCrawlPage; type: "markdown" | "html" | "headers" } | null>(null);
  const [content, setContent] = useState<string>("");
  const [contentLoading, setContentLoading] = useState(false);

  const [crawlDialogOpen, setCrawlDialogOpen] = useState(false);

  useEffect(() => {
    if (!id) return;
    Promise.all([
      getDomain(id),
      pgrest<CompanyDomainRow>("v_company_domains", { domain_id: `eq.${id}` }),
      getDomainCrawlJobs(id),
    ])
      .then(([d, c, j]) => {
        setDomain(d);
        setCompanies(c.data);
        setCrawlJobs(j);
      })
      .finally(() => setLoading(false));
  }, [id]);

  async function openPages(job: DomainCrawlJob) {
    setPagesJob(job);
    setPagesLoading(true);
    try {
      const p = await getDomainCrawlPages(id!, job.id);
      setPages(p);
    } finally {
      setPagesLoading(false);
    }
  }

  async function openContent(page: DomainCrawlPage, type: "markdown" | "html" | "headers") {
    setContentPage({ page, type });
    setContentLoading(true);
    try {
      const text = await getDomainCrawlPageContent(id!, pagesJob!.id, page.page_num, type);
      setContent(typeof text === "string" ? text : JSON.stringify(text, null, 2));
    } catch {
      setContent("Content unavailable");
    } finally {
      setContentLoading(false);
    }
  }

  function refreshJobs() {
    if (!id) return;
    getDomainCrawlJobs(id).then(setCrawlJobs);
  }

  const faviconJob = crawlJobs.find((j) => j.favicon_s3_key);

  if (loading) {
    return <div className="p-8 text-muted-foreground">Loading…</div>;
  }

  if (!domain) {
    return <div className="p-8 text-destructive">Domain not found.</div>;
  }

  return (
    <div className="p-6 space-y-6 max-w-5xl mx-auto">
      <Link to="/domains" className="inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground">
        <ArrowLeft className="h-4 w-4" /> Domains
      </Link>

      <Card>
        <CardContent className="pt-6">
          <div className="flex items-start justify-between gap-4">
            <div className="flex items-center gap-4">
              {faviconJob && (
                <img
                  src={getDomainFaviconUrl(id!, faviconJob.id)}
                  alt="favicon"
                  className="h-8 w-8 rounded object-contain"
                  onError={(e) => { (e.target as HTMLImageElement).style.display = "none"; }}
                />
              )}
              <div>
                <h1 className="text-2xl font-bold flex items-center gap-2">
                  {domain.domain}
                  <a
                    href={`https://${domain.domain}`}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="text-muted-foreground hover:text-foreground"
                  >
                    <ExternalLink className="h-4 w-4" />
                  </a>
                </h1>
                <div className="text-sm text-muted-foreground mt-1">
                  First seen: {new Date(domain.first_seen_at).toLocaleDateString()}
                  {" · "}
                  Companies: {companies.length}
                </div>
              </div>
            </div>
            <Button onClick={() => setCrawlDialogOpen(true)}>Trigger Crawl</Button>
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader><CardTitle>Linked Companies</CardTitle></CardHeader>
        <CardContent>
          {companies.length === 0 ? (
            <div className="text-sm text-muted-foreground">No linked companies.</div>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Company</TableHead>
                  <TableHead>Signal</TableHead>
                  <TableHead>Confidence</TableHead>
                  <TableHead>Status</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {companies.map((c) => (
                  <TableRow key={c.id}>
                    <TableCell>
                      <Link to={`/companies/${c.company_id}`} className="hover:underline text-sm font-mono">
                        {c.company_id.slice(0, 8)}…
                      </Link>
                    </TableCell>
                    <TableCell>{c.signal}</TableCell>
                    <TableCell>{c.confidence}</TableCell>
                    <TableCell>{c.status}</TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>

      <Card>
        <CardHeader><CardTitle>Crawl History</CardTitle></CardHeader>
        <CardContent>
          {crawlJobs.length === 0 ? (
            <div className="text-sm text-muted-foreground">No crawl jobs yet.</div>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Date</TableHead>
                  <TableHead>Mode</TableHead>
                  <TableHead>Status</TableHead>
                  <TableHead>Actions</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {crawlJobs.map((job) => (
                  <TableRow key={job.id}>
                    <TableCell className="text-sm">{new Date(job.created_at).toLocaleString()}</TableCell>
                    <TableCell><Badge variant="outline">{job.mode}</Badge></TableCell>
                    <TableCell><StatusBadge status={crawlJobStatus(job)} /></TableCell>
                    <TableCell>
                      {crawlJobStatus(job) === "completed" && (
                        <Button variant="outline" size="sm" onClick={() => openPages(job)}>
                          View pages
                        </Button>
                      )}
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>

      <Sheet open={!!pagesJob} onOpenChange={(open) => { if (!open) { setPagesJob(null); setPages([]); } }}>
        <SheetContent side="right" className="w-[600px] sm:max-w-[600px] overflow-y-auto">
          <SheetHeader>
            <SheetTitle>Pages — {pagesJob?.mode} crawl</SheetTitle>
          </SheetHeader>
          <div className="mt-4">
            {pagesLoading ? (
              <div className="text-muted-foreground text-sm">Loading pages…</div>
            ) : pages.length === 0 ? (
              <div className="text-muted-foreground text-sm">No pages recorded.</div>
            ) : (
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>#</TableHead>
                    <TableHead>URL</TableHead>
                    <TableHead>Status</TableHead>
                    <TableHead>Content</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {pages.map((p) => (
                    <TableRow key={p.id}>
                      <TableCell className="text-muted-foreground">{p.page_num}</TableCell>
                      <TableCell className="max-w-[200px] truncate text-sm" title={p.url}>
                        {p.title || p.url}
                      </TableCell>
                      <TableCell>{p.status_code}</TableCell>
                      <TableCell>
                        <div className="flex gap-1">
                          <Button variant="outline" size="sm" onClick={() => openContent(p, "markdown")}>MD</Button>
                          <Button variant="outline" size="sm" onClick={() => openContent(p, "html")}>HTML</Button>
                          <Button variant="outline" size="sm" onClick={() => openContent(p, "headers")}>Headers</Button>
                        </div>
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            )}
          </div>
        </SheetContent>
      </Sheet>

      <Sheet open={!!contentPage} onOpenChange={(open) => { if (!open) setContentPage(null); }}>
        <SheetContent side="right" className="w-[700px] sm:max-w-[700px] overflow-y-auto">
          <SheetHeader>
            <SheetTitle>
              {contentPage?.type.toUpperCase()} — Page {contentPage?.page.page_num}
            </SheetTitle>
          </SheetHeader>
          <div className="mt-4">
            {contentLoading ? (
              <div className="text-muted-foreground text-sm">Loading…</div>
            ) : (
              <div className="relative">
                <Button
                  variant="outline"
                  size="sm"
                  className="absolute top-2 right-2 z-10"
                  onClick={() => navigator.clipboard.writeText(content)}
                >
                  Copy
                </Button>
                <pre className="text-xs bg-muted rounded p-4 overflow-x-auto max-h-[70vh] whitespace-pre-wrap break-words">
                  {content}
                </pre>
              </div>
            )}
          </div>
        </SheetContent>
      </Sheet>

      <CrawlDomainDialog
        domainId={id!}
        domainName={domain.domain}
        open={crawlDialogOpen}
        onOpenChange={setCrawlDialogOpen}
        onSuccess={refreshJobs}
      />
    </div>
  );
}
