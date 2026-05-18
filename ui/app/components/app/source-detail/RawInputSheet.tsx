import { useEffect, useState } from "react";
import { Ban, RotateCcw } from "lucide-react";
import { toast } from "sonner";
import { api, errorMessage } from "~/lib/api";
import { pgrest } from "~/lib/pgrest";
import { formatDate } from "~/lib/utils";
import type {
  DataSource,
  RawPayloadRow,
  SourceRawInput,
  SuggestionSourceLink,
} from "~/types/api";
import { Alert, AlertDescription } from "~/components/ui/alert";
import { Badge } from "~/components/ui/badge";
import { Button } from "~/components/ui/button";
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from "~/components/ui/sheet";
import { canIgnore, canRetry, statusClass } from "~/components/app/source-detail/sourceDetailUtils";

interface RawInputSheetProps {
  source: DataSource;
  row: SourceRawInput | null;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onChanged: () => void;
}

export function RawInputSheet({
  source,
  row,
  open,
  onOpenChange,
  onChanged,
}: RawInputSheetProps) {
  const [payload, setPayload] = useState<Record<string, unknown> | null>(null);
  const [links, setLinks] = useState<SuggestionSourceLink[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string>();
  const [action, setAction] = useState<"retry" | "ignore" | null>(null);
  const retryOnlyResetsStatus = source.name === "ai_company_profile" || source.name === "domain_discovery";

  useEffect(() => {
    if (!open || !row) {
      setPayload(null);
      setLinks([]);
      setError(undefined);
      return;
    }

    let cancelled = false;
    setLoading(true);
    setError(undefined);

    Promise.all([
      pgrest<RawPayloadRow>(row.source_input_table, {
        id: "eq." + row.id,
        select: "raw_payload",
        limit: 1,
      }),
      pgrest<SuggestionSourceLink>("suggestion_source_links", {
        source_input_table: "eq." + row.source_input_table,
        source_input_key: "eq." + row.id,
        order: "created_at.desc",
      }),
    ])
      .then(([payloadResult, linksResult]) => {
        if (cancelled) return;
        setPayload(payloadResult.data[0]?.raw_payload ?? null);
        setLinks(linksResult.data);
      })
      .catch(() => {
        if (!cancelled) setError("Failed to load raw input details.");
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });

    return () => {
      cancelled = true;
    };
  }, [open, row]);

  async function handleAction(nextAction: "retry" | "ignore") {
    if (!row) return;

    setAction(nextAction);
    try {
      if (nextAction === "retry") {
        await api.retryRawInput(source.name, row.id);
        toast.success("Raw input queued for retry.");
      } else {
        await api.ignoreRawInput(source.name, row.id);
        toast.success("Raw input ignored.");
      }
      onChanged();
      onOpenChange(false);
    } catch (err) {
      toast.error(errorMessage(
        err,
        nextAction === "retry" ? "Failed to retry raw input." : "Failed to ignore raw input.",
      ));
    } finally {
      setAction(null);
    }
  }

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent className="w-full overflow-y-auto sm:max-w-2xl">
        {row ? (
          <div className="space-y-5">
            <SheetHeader className="px-0">
              <div className="flex flex-wrap items-start justify-between gap-3 pr-8">
                <div className="min-w-0 space-y-1">
                  <SheetTitle className="break-all">{row.source_native_id || row.id}</SheetTitle>
                  <SheetDescription className="break-all">{row.source_input_table}</SheetDescription>
                </div>
                <div className="flex flex-wrap items-center gap-2">
                  <Badge className={statusClass(row.processing_status)} variant="outline">
                    {row.processing_status}
                  </Badge>
                  <Badge variant="outline">{row.processing_attempts} attempts</Badge>
                </div>
              </div>
            </SheetHeader>

            <div className="flex flex-wrap gap-2">
              <Button
                size="sm"
                variant="outline"
                disabled={!canRetry(row) || action !== null}
                onClick={() => handleAction("retry")}
              >
                <RotateCcw className="size-4" />
                Retry
              </Button>
              <Button
                size="sm"
                variant="outline"
                disabled={!canIgnore(row) || action !== null}
                onClick={() => handleAction("ignore")}
              >
                <Ban className="size-4" />
                Ignore
              </Button>
            </div>
            {retryOnlyResetsStatus && (
              <Alert>
                <AlertDescription>
                  Retry resets this raw input to pending. No processor job is queued for this source yet.
                </AlertDescription>
              </Alert>
            )}

            {error && (
              <Alert variant="destructive">
                <AlertDescription>{error}</AlertDescription>
              </Alert>
            )}

            <div className="grid gap-3 rounded-md border p-3 text-sm sm:grid-cols-3">
              <MetadataItem label="First seen" value={formatDate(row.first_seen_at)} />
              <MetadataItem label="Last seen" value={formatDate(row.last_seen_at)} />
              <MetadataItem label="Payload hash" value={row.payload_hash} breakValue />
            </div>

            <section className="space-y-2">
              <h3 className="text-sm font-medium">Suggestion links</h3>
              {links.length === 0 ? (
                <p className="text-sm text-muted-foreground">None produced</p>
              ) : (
                <ul className="divide-y rounded-md border">
                  {links.map((link) => (
                    <li key={link.id} className="flex flex-wrap items-center gap-2 p-3 text-sm">
                      <span className="font-medium">{link.suggestion_table}</span>
                      <span className="break-all text-muted-foreground">{link.suggestion_id}</span>
                    </li>
                  ))}
                </ul>
              )}
            </section>

            {row.processing_error && (
              <section className="space-y-2">
                <h3 className="text-sm font-medium">Last error</h3>
                <pre className="overflow-x-auto whitespace-pre-wrap rounded-md border bg-muted p-3 text-xs">
                  {row.processing_error}
                </pre>
              </section>
            )}

            <section className="space-y-2">
              <h3 className="text-sm font-medium">Raw payload</h3>
              <pre className="max-h-[32rem] overflow-auto rounded-md border bg-muted p-3 text-xs">
                {loading ? "Loading..." : JSON.stringify(payload, null, 2)}
              </pre>
            </section>
          </div>
        ) : (
          <SheetHeader className="px-0">
            <SheetTitle>Raw input</SheetTitle>
            <SheetDescription>No raw input selected.</SheetDescription>
          </SheetHeader>
        )}
      </SheetContent>
    </Sheet>
  );
}

function MetadataItem({
  label,
  value,
  breakValue,
}: {
  label: string;
  value: string;
  breakValue?: boolean;
}) {
  return (
    <div className="min-w-0 space-y-1">
      <div className="text-xs font-medium uppercase text-muted-foreground">{label}</div>
      <div className={breakValue ? "break-all" : undefined}>{value}</div>
    </div>
  );
}
