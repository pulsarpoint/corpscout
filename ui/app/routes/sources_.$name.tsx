import { useEffect, useRef, useState } from "react";
import { Link, NavLink, Outlet, useParams } from "react-router";
import { ChevronLeft } from "lucide-react";
import { toast } from "sonner";
import { api, errorMessage } from "~/lib/api";
import type { DataSource } from "~/types/api";
import { Alert, AlertDescription } from "~/components/ui/alert";
import { Skeleton } from "~/components/ui/skeleton";
import { SourceHeader } from "~/components/app/source-detail/SourceHeader";
import { hasRawInputs, hasPipeline } from "~/components/app/source-detail/sourceDetailUtils";
import { cn } from "~/lib/utils";

type SourcePatch = Parameters<typeof api.patchSource>[1];

export interface SourceDetailContext {
  source: DataSource;
  saving: boolean;
  triggering: boolean;
  processing: boolean;
  onPatch: (patch: SourcePatch) => Promise<void>;
  onTrigger: () => Promise<void>;
  onProcess: () => Promise<void>;
}

export default function SourceDetailLayout() {
  const { name } = useParams<{ name: string }>();
  const latestNameRef = useRef<string | undefined>(undefined);
  const [source, setSource] = useState<DataSource>();
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string>();
  const [saving, setSaving] = useState(false);
  const [triggering, setTriggering] = useState(false);
  const [processing, setProcessing] = useState(false);

  useEffect(() => {
    if (!name) return;
    latestNameRef.current = name;
    let ignore = false;
    setSource(undefined);
    setLoading(true);
    setError(undefined);
    setSaving(false);
    setTriggering(false);
    setProcessing(false);
    api.getSource(name)
      .then((loadedSource) => { if (!ignore) setSource(loadedSource); })
      .catch(() => { if (!ignore) setError("Source not found."); })
      .finally(() => { if (!ignore) setLoading(false); });
    return () => { ignore = true; };
  }, [name]);

  async function refreshSource(sourceName: string) {
    const refreshed = await api.getSource(sourceName);
    if (latestNameRef.current === sourceName) setSource(refreshed);
  }

  async function handlePatch(patch: SourcePatch) {
    const sourceName = source?.name;
    if (!sourceName) return;
    setSaving(true);
    try {
      await api.patchSource(sourceName, patch);
      await refreshSource(sourceName);
      toast.success("Source updated.");
    } catch (err) {
      toast.error(errorMessage(err, "Failed to update source."));
    } finally {
      if (latestNameRef.current === sourceName) setSaving(false);
    }
  }

  async function handleTrigger() {
    const sourceName = source?.name;
    if (!sourceName) return;
    setTriggering(true);
    try {
      await api.triggerSource(sourceName);
      toast.success(`${sourceName} pull queued.`);
    } catch (err) {
      toast.error(errorMessage(err, `Failed to trigger ${sourceName}.`));
    } finally {
      if (latestNameRef.current === sourceName) setTriggering(false);
    }
  }

  async function handleProcess() {
    const sourceName = source?.name;
    if (!sourceName) return;
    setProcessing(true);
    try {
      await api.processSource(sourceName);
      toast.success(`${sourceName} processing queued.`);
    } catch (err) {
      toast.error(errorMessage(err, `Failed to process ${sourceName}.`));
    } finally {
      if (latestNameRef.current === sourceName) setProcessing(false);
    }
  }

  if (loading) return <Skeleton className="h-64 w-full" />;
  if (error || !source) {
    return (
      <Alert variant="destructive">
        <AlertDescription>{error ?? "Source not found."}</AlertDescription>
      </Alert>
    );
  }

  const tabs = [
    { label: "Schedule", to: `/sources/${source.name}/schedule` },
    { label: "Config", to: `/sources/${source.name}/config` },
    { label: "Logs", to: `/sources/${source.name}/logs` },
    ...(hasRawInputs(source) ? [{ label: "Raw Inputs", to: `/sources/${source.name}/raw_input` }] : []),
    ...(hasPipeline(source) ? [{ label: "Pipeline", to: `/sources/${source.name}/pipeline` }] : []),
  ];

  const context: SourceDetailContext = { source, saving, triggering, processing, onPatch: handlePatch, onTrigger: handleTrigger, onProcess: handleProcess };

  return (
    <div className="space-y-6">
      <Link
        to="/sources"
        className="inline-flex items-center gap-1 text-sm text-muted-foreground hover:underline"
      >
        <ChevronLeft className="size-4" />
        Sources
      </Link>

      <SourceHeader source={source} />

      <nav className="flex gap-1 border-b">
        {tabs.map((tab) => (
          <NavLink
            key={tab.to}
            to={tab.to}
            className={({ isActive }) =>
              cn(
                "relative px-4 py-2 text-sm font-medium transition-colors hover:text-foreground",
                isActive
                  ? "border-b-2 border-primary text-foreground"
                  : "text-muted-foreground",
              )
            }
          >
            {tab.label}
          </NavLink>
        ))}
      </nav>

      <Outlet context={context} />
    </div>
  );
}
