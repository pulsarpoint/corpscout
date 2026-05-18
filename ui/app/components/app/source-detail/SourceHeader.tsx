import type { DataSource } from "~/types/api";
import { sourceDisplayName } from "~/components/app/source-detail/sourceDetailUtils";
import { Badge } from "~/components/ui/badge";

interface SourceHeaderProps {
  source: DataSource;
}

export function SourceHeader({ source }: SourceHeaderProps) {
  return (
    <div className="space-y-3">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
        <div className="min-w-0 space-y-2">
          <h1 className="text-2xl font-semibold tracking-tight">
            {sourceDisplayName(source)}
          </h1>
          <div className="flex flex-wrap items-center gap-2">
            <Badge variant="outline">{source.source_group}</Badge>
            <Badge variant="outline">{source.pull_task_type}</Badge>
            {source.enabled ? (
              <Badge className="border-green-200 bg-green-100 text-green-800" variant="outline">
                Enabled
              </Badge>
            ) : (
              <Badge className="text-muted-foreground" variant="outline">
                Disabled
              </Badge>
            )}
          </div>
        </div>
        <div className="text-sm text-muted-foreground sm:text-right">
          <div className="font-mono">{source.name}</div>
          <div>{source.input_table_name}</div>
        </div>
      </div>
      {source.description && (
        <p className="max-w-3xl text-sm leading-6 text-muted-foreground">
          {source.description}
        </p>
      )}
    </div>
  );
}
