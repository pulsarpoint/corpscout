import { useEffect, useState } from "react";
import { toast } from "sonner";
import { api } from "~/lib/api";
import type { DataSource } from "~/types/api";
import { SourcesTable } from "~/components/app/SourcesTable";
import { Skeleton } from "~/components/ui/skeleton";
import { Alert, AlertDescription } from "~/components/ui/alert";

export default function SourcesPage() {
  const [sources, setSources] = useState<DataSource[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string>();
  const [triggeringName, setTriggeringName] = useState<string>();

  useEffect(() => {
    api.getSources()
      .then(setSources)
      .catch(() => setError("Failed to load sources."))
      .finally(() => setLoading(false));
  }, []);

  async function handleToggle(name: string, enabled: boolean) {
    setSources((prev) => prev.map((s) => s.name === name ? { ...s, enabled } : s));
    try {
      await api.patchSource(name, { enabled });
      toast.success(`${name} ${enabled ? "enabled" : "disabled"}.`);
    } catch {
      setSources((prev) => prev.map((s) => s.name === name ? { ...s, enabled: !enabled } : s));
      toast.error("Failed to update source.");
    }
  }

  async function handleTrigger(name: string) {
    setTriggeringName(name);
    try {
      await api.triggerSource(name);
      toast.success(`${name} crawl queued.`);
    } catch {
      toast.error(`Failed to trigger ${name}.`);
    } finally {
      setTriggeringName(undefined);
    }
  }

  if (loading) return <div className="space-y-2">{Array.from({ length: 5 }).map((_, i) => <Skeleton key={i} className="h-10 w-full" />)}</div>;
  if (error) return <Alert variant="destructive"><AlertDescription>{error}</AlertDescription></Alert>;

  return (
    <div>
      <h1 className="mb-4 text-xl font-semibold">Sources</h1>
      <SourcesTable
        sources={sources}
        onToggle={handleToggle}
        onTrigger={handleTrigger}
        triggeringName={triggeringName}
      />
    </div>
  );
}
