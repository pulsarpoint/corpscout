import { useOutletContext } from "react-router";
import type { SourceDetailContext } from "~/routes/sources_.$name";
import { RawInputsTable } from "~/components/app/RawInputsTable";

export default function SourceRawInputPage() {
  const { source } = useOutletContext<SourceDetailContext>();
  return (
    <RawInputsTable
      sourceName={source.name}
      requiresTranslation={source.requires_translation}
    />
  );
}
