import { useEffect, useState } from "react";
import { api } from "~/lib/api";
import type { RawInputDetail } from "~/types/api";
import { Badge } from "~/components/ui/badge";
import { Separator } from "~/components/ui/separator";
import { Skeleton } from "~/components/ui/skeleton";
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
} from "~/components/ui/sheet";

const SOURCE_LABELS: Record<string, string> = {
  companies_house: "Companies House",
  brreg: "Brreg",
};

function timeAgo(dateStr: string): string {
  const diff = Date.now() - new Date(dateStr).getTime();
  const minutes = Math.floor(diff / 60_000);
  if (minutes < 1) return "just now";
  if (minutes < 60) return `${minutes}m ago`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours}h ago`;
  const days = Math.floor(hours / 24);
  if (days < 30) return `${days}d ago`;
  const months = Math.floor(days / 30);
  if (months < 12) return `${months}mo ago`;
  return `${Math.floor(months / 12)}y ago`;
}

function statusBadgeVariant(status: string): "default" | "secondary" | "destructive" | "outline" {
  switch (status) {
    case "pending": return "default";
    case "processing": return "secondary";
    case "processed": return "outline";
    case "failed": return "destructive";
    default: return "outline";
  }
}

function translationBadgeClass(status?: string) {
  switch (status) {
    case "translated": return "border-green-200 bg-green-100 text-green-800";
    case "failed": return "border-red-200 bg-red-100 text-red-800";
    case "translating": return "border-blue-200 bg-blue-100 text-blue-800";
    case "pending": return "border-amber-200 bg-amber-100 text-amber-800";
    default: return "";
  }
}

function DetailRow({ label, value }: { label: string; value: React.ReactNode }) {
  if (!value && value !== 0) return null;
  return (
    <div className="grid grid-cols-[140px_1fr] gap-2 py-1.5">
      <span className="text-sm text-muted-foreground">{label}</span>
      <span className="text-sm break-all">{value}</span>
    </div>
  );
}

export function RawInputDetailSheet({
  open,
  onClose,
  source,
  id,
}: {
  open: boolean;
  onClose: () => void;
  source: string;
  id: string;
}) {
  const [detail, setDetail] = useState<RawInputDetail | null>(null);
  const [loading, setLoading] = useState(false);
  const [jsonExpanded, setJsonExpanded] = useState(false);

  useEffect(() => {
    if (!open || !id) return;
    setDetail(null);
    setJsonExpanded(false);
    setLoading(true);
    api.getRawInput(source, id).then(setDetail).finally(() => setLoading(false));
  }, [open, source, id]);

  const typeLabel = detail?.source === "companies_house"
    ? detail.company_type
    : detail?.registration_status;

  return (
    <Sheet open={open} onOpenChange={(v) => !v && onClose()}>
      <SheetContent className="w-[480px] sm:max-w-[480px] overflow-y-auto">
        {loading && (
          <div className="space-y-3 pt-6">
            <Skeleton className="h-6 w-3/4" />
            <Skeleton className="h-4 w-1/2" />
            <Skeleton className="h-4 w-full" />
            <Skeleton className="h-4 w-full" />
          </div>
        )}
        {detail && (
          <>
            <SheetHeader className="pb-4">
              <SheetTitle className="text-lg leading-snug">{detail.name}</SheetTitle>
              <div className="flex items-center gap-2 mt-1">
                <Badge variant="outline" className="text-xs">
                  {SOURCE_LABELS[detail.source] ?? detail.source}
                </Badge>
                <Badge variant={statusBadgeVariant(detail.status)} className="text-xs">
                  {detail.status}
                </Badge>
                {detail.source === "brreg" && (
                  <Badge variant="outline" className={`text-xs ${translationBadgeClass(detail.translation_status)}`}>
                    {detail.translation_status ?? "pending"}
                  </Badge>
                )}
                {detail.country_iso2 && (
                  <span className="text-xs text-muted-foreground">{detail.country_iso2}</span>
                )}
              </div>
            </SheetHeader>

            <Separator className="mb-4" />

            <section className="space-y-0.5 mb-4">
              <DetailRow label="Native ID" value={<span className="font-mono text-xs">{detail.native_id}</span>} />
              {typeLabel && <DetailRow label="Type" value={typeLabel} />}
              {detail.website && (
                <DetailRow label="Website" value={
                  <a href={detail.website} target="_blank" rel="noreferrer"
                     className="text-primary underline-offset-4 hover:underline">
                    {detail.website}
                  </a>
                } />
              )}
              {detail.run_id && <DetailRow label="Run ID" value={<span className="font-mono text-xs">{detail.run_id}</span>} />}
            </section>

            <Separator className="mb-4" />

            <section className="space-y-0.5 mb-4">
              <p className="text-xs font-medium uppercase tracking-wide text-muted-foreground mb-2">Timestamps</p>
              <DetailRow label="Created" value={`${new Date(detail.created_at).toLocaleString()} (${timeAgo(detail.created_at)})`} />
              <DetailRow label="First seen" value={`${new Date(detail.first_seen_at).toLocaleString()} (${timeAgo(detail.first_seen_at)})`} />
              <DetailRow label="Last seen" value={`${new Date(detail.last_seen_at).toLocaleString()} (${timeAgo(detail.last_seen_at)})`} />
              {detail.processed_at && (
                <DetailRow label="Processed" value={`${new Date(detail.processed_at).toLocaleString()} (${timeAgo(detail.processed_at)})`} />
              )}
            </section>

            <Separator className="mb-4" />

            <section className="space-y-0.5 mb-4">
              <p className="text-xs font-medium uppercase tracking-wide text-muted-foreground mb-2">Processing</p>
              <DetailRow label="Attempts" value={detail.processing_attempts} />
              <DetailRow label="Hash" value={<span className="font-mono text-xs">{detail.payload_hash.slice(0, 16)}…</span>} />
              {detail.processing_error && (
                <div className="mt-2 rounded-md bg-destructive/10 px-3 py-2">
                  <p className="text-xs font-medium text-destructive mb-1">Error</p>
                  <p className="text-xs text-destructive break-all">{detail.processing_error}</p>
                </div>
              )}
            </section>

            {detail.source === "brreg" && (
              <>
                <Separator className="mb-4" />
                <section className="space-y-0.5 mb-4">
                  <p className="text-xs font-medium uppercase tracking-wide text-muted-foreground mb-2">Translation</p>
                  <DetailRow label="Status" value={detail.translation_status ?? "pending"} />
                  <DetailRow label="Attempts" value={detail.translation_attempts ?? 0} />
                  <DetailRow label="Model" value={detail.translation_model} />
                  <DetailRow label="FX" value={detail.translation_fx_source ? `${detail.translation_fx_source} ${detail.translation_fx_rate_date ?? ""}` : undefined} />
                  {detail.translated_at && (
                    <DetailRow label="Translated" value={`${new Date(detail.translated_at).toLocaleString()} (${timeAgo(detail.translated_at)})`} />
                  )}
                  {detail.translation_error && (
                    <div className="mt-2 rounded-md bg-destructive/10 px-3 py-2">
                      <p className="text-xs font-medium text-destructive mb-1">Translation error</p>
                      <p className="text-xs text-destructive break-all">{detail.translation_error}</p>
                    </div>
                  )}
                </section>
              </>
            )}

            <Separator className="mb-4" />

            <section>
              <button
                className="flex w-full items-center justify-between text-xs font-medium uppercase tracking-wide text-muted-foreground mb-2"
                onClick={() => setJsonExpanded((v) => !v)}
              >
                Raw payload
                <span className="normal-case font-normal">{jsonExpanded ? "hide" : "show"}</span>
              </button>
              {jsonExpanded && (
                <pre className="rounded-md bg-muted p-3 text-xs overflow-auto max-h-96 whitespace-pre-wrap break-all">
                  {JSON.stringify(detail.raw_payload, null, 2)}
                </pre>
              )}
            </section>
          </>
        )}
      </SheetContent>
    </Sheet>
  );
}
