import { useOutletContext } from "react-router";
import type { SourceDetailContext } from "~/routes/sources_.$name";
import { PipelineTab } from "~/components/app/source-detail/PipelineTab";

export default function SourcePipelinePage() {
  const { source } = useOutletContext<SourceDetailContext>();
  return <PipelineTab source={source} />;
}
