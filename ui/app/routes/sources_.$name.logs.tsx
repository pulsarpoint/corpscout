import { useOutletContext } from "react-router";
import type { SourceDetailContext } from "~/routes/sources_.$name";
import { LogsTab } from "~/components/app/source-detail/LogsTab";

export default function SourceLogsPage() {
  const { source } = useOutletContext<SourceDetailContext>();
  return <LogsTab sourceName={source.name} />;
}
