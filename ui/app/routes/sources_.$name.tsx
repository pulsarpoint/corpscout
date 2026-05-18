import { useEffect, useState } from "react";
import { Link, useParams } from "react-router";
import { ChevronLeft } from "lucide-react";
import { toast } from "sonner";
import { api } from "~/lib/api";
import type { DataSource } from "~/types/api";
import { Alert, AlertDescription } from "~/components/ui/alert";
import { Skeleton } from "~/components/ui/skeleton";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "~/components/ui/tabs";
import { SourceHeader } from "~/components/app/source-detail/SourceHeader";
import { ScheduleTab } from "~/components/app/source-detail/ScheduleTab";
import { ConfigTab } from "~/components/app/source-detail/ConfigTab";
import { LogsTab } from "~/components/app/source-detail/LogsTab";

type SourcePatch = Parameters<typeof api.patchSource>[1];

export default function SourceDetailPage() {
  const { name } = useParams<{ name: string }>();
  const [source, setSource] = useState<DataSource>();
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string>();
  const [saving, setSaving] = useState(false);
  const [triggering, setTriggering] = useState(false);

  useEffect(() => {
    if (!name) return;

    setLoading(true);
    setError(undefined);
    api.getSource(name)
      .then(setSource)
      .catch(() => setError("Source not found."))
      .finally(() => setLoading(false));
  }, [name]);

  async function refreshSource() {
    if (!name) return;
    const refreshed = await api.getSource(name);
    setSource(refreshed);
  }

  async function handlePatch(patch: SourcePatch) {
    if (!name) return;

    setSaving(true);
    try {
      await api.patchSource(name, patch);
      await refreshSource();
      toast.success("Source updated.");
    } catch {
      toast.error("Failed to update source.");
    } finally {
      setSaving(false);
    }
  }

  async function handleTrigger() {
    if (!name) return;

    setTriggering(true);
    try {
      await api.triggerSource(name);
      toast.success(`${name} pull queued.`);
    } catch {
      toast.error(`Failed to trigger ${name}.`);
    } finally {
      setTriggering(false);
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

      <Tabs defaultValue="schedule" className="space-y-4">
        <TabsList>
          <TabsTrigger value="schedule">Schedule</TabsTrigger>
          <TabsTrigger value="config">Config</TabsTrigger>
          <TabsTrigger value="logs">Logs</TabsTrigger>
        </TabsList>

        <TabsContent value="schedule">
          <ScheduleTab
            source={source}
            saving={saving}
            triggering={triggering}
            onPatch={handlePatch}
            onTrigger={handleTrigger}
          />
        </TabsContent>
        <TabsContent value="config">
          <ConfigTab source={source} saving={saving} onPatch={handlePatch} />
        </TabsContent>
        <TabsContent value="logs">
          <LogsTab sourceName={source.name} />
        </TabsContent>
      </Tabs>
    </div>
  );
}
