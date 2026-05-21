import { useOutletContext } from "react-router";
import type { SourceDetailContext } from "~/routes/sources_.$name";
import { ScheduleTab } from "~/components/app/source-detail/ScheduleTab";

export default function SourceSchedulePage() {
  const { source, saving, triggering, processing, onPatch, onTrigger, onProcess } =
    useOutletContext<SourceDetailContext>();
  return (
    <ScheduleTab
      source={source}
      saving={saving}
      triggering={triggering}
      processing={processing}
      onPatch={onPatch}
      onTrigger={onTrigger}
      onProcess={onProcess}
    />
  );
}
