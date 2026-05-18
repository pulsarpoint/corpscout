import { useEffect, useMemo, useState } from "react";
import { Play, Save } from "lucide-react";
import type { DataSource } from "~/types/api";
import { formatDate, timeAgo } from "~/lib/utils";
import { validateDuration } from "~/components/app/source-detail/sourceDetailUtils";
import { Alert, AlertDescription, AlertTitle } from "~/components/ui/alert";
import { Badge } from "~/components/ui/badge";
import { Button } from "~/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "~/components/ui/card";
import { Input } from "~/components/ui/input";
import { Switch } from "~/components/ui/switch";

type SourcePatch = Parameters<typeof import("~/lib/api").api.patchSource>[1];

interface ScheduleTabProps {
  source: DataSource;
  saving: boolean;
  triggering: boolean;
  onPatch: (patch: SourcePatch) => Promise<void>;
  onTrigger: () => Promise<void>;
}

function parseDurationMs(value: string): number | undefined {
  const match = /^(\d+)([hms])$/.exec(value.trim());
  if (!match) return undefined;

  const amount = Number(match[1]);
  const unit = match[2];
  if (unit === "h") return amount * 60 * 60 * 1000;
  if (unit === "m") return amount * 60 * 1000;
  return amount * 1000;
}

function nextRunText(source: DataSource): string {
  if (!source.last_started_at || !source.schedule_expression) return "Not available";

  const durationMs = parseDurationMs(source.schedule_expression);
  if (!durationMs) return "Invalid duration";

  return formatDate(new Date(new Date(source.last_started_at).getTime() + durationMs).toISOString());
}

export function ScheduleTab({
  source,
  saving,
  triggering,
  onPatch,
  onTrigger,
}: ScheduleTabProps) {
  const [duration, setDuration] = useState(source.schedule_expression ?? "");
  const [durationError, setDurationError] = useState<string>();
  const isIntervalSchedule = source.schedule_kind === "interval";

  useEffect(() => {
    setDuration(source.schedule_expression ?? "");
    setDurationError(undefined);
  }, [source.schedule_expression]);

  const nextRun = useMemo(() => nextRunText(source), [source]);

  async function saveDuration() {
    if (!isIntervalSchedule) return;

    const error = validateDuration(duration);
    setDurationError(error);
    if (error) return;

    await onPatch({
      schedule_expression: duration.trim(),
    });
  }

  return (
    <div className="space-y-4">
      <Alert>
        <AlertTitle>Source execution</AlertTitle>
        <AlertDescription className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
          <span>
            {source.enabled ? "Automatic scheduling is allowed." : "Automatic scheduling is paused."} Disabling does not stop manual or in-progress runs.
          </span>
          <Switch
            checked={source.enabled}
            disabled={saving}
            onCheckedChange={() => onPatch({ enabled: !source.enabled })}
          />
        </AlertDescription>
      </Alert>

      <Alert>
        <AlertTitle>Interval schedule</AlertTitle>
        <AlertDescription className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
          <span>
            {source.schedule_enabled ? "The interval scheduler can queue this source." : "The interval scheduler is paused for this source."}
          </span>
          <Switch
            checked={source.schedule_enabled}
            disabled={saving}
            onCheckedChange={() => onPatch({ schedule_enabled: !source.schedule_enabled })}
          />
        </AlertDescription>
      </Alert>

      <div className="grid gap-4 lg:grid-cols-[minmax(0,1fr)_minmax(280px,360px)]">
        <Card>
          <CardHeader>
            <CardTitle>Schedule</CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            {isIntervalSchedule ? (
              <div className="grid gap-2">
                <label className="text-sm font-medium" htmlFor="schedule-duration">
                  Duration
                </label>
                <div className="flex flex-col gap-2 sm:flex-row">
                  <Input
                    id="schedule-duration"
                    value={duration}
                    onChange={(event) => {
                      setDuration(event.target.value);
                      setDurationError(undefined);
                    }}
                    placeholder="24h"
                    aria-invalid={Boolean(durationError)}
                  />
                  <Button disabled={saving} onClick={saveDuration}>
                    <Save className="size-4" />
                    Save
                  </Button>
                </div>
                {durationError && <p className="text-sm text-destructive">{durationError}</p>}
                <p className="text-sm text-muted-foreground">
                  MVP scheduling uses Go duration strings such as 24h, 12h, or 30m.
                </p>
              </div>
            ) : (
              <div className="rounded-md border bg-muted/30 p-3 text-sm text-muted-foreground">
                This source uses {source.schedule_kind} scheduling. Duration editing is not available.
              </div>
            )}

            <div className="grid gap-3 sm:grid-cols-2">
              <div>
                <p className="text-xs uppercase text-muted-foreground">Schedule kind</p>
                <p className="mt-1 text-sm font-medium">{source.schedule_kind}</p>
              </div>
              <div>
                <p className="text-xs uppercase text-muted-foreground">Next run estimate</p>
                <p className="mt-1 text-sm font-medium">{nextRun}</p>
              </div>
            </div>

            <div className="flex flex-col gap-2 border-t pt-4 sm:flex-row sm:items-center sm:justify-between">
              <p className="text-sm text-muted-foreground">
                Manual trigger works even when the source is disabled or the schedule is paused.
              </p>
              <Button disabled={triggering} onClick={onTrigger} variant="outline">
                <Play className="size-4" />
                {triggering ? "Queuing..." : "Trigger now"}
              </Button>
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>Last run</CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            <div>
              <p className="text-xs uppercase text-muted-foreground">Last started</p>
              <p className="mt-1 text-sm">{source.last_started_at ? timeAgo(source.last_started_at) : "Never"}</p>
            </div>
            <div>
              <p className="text-xs uppercase text-muted-foreground">Last success</p>
              <p className="mt-1 text-sm">{source.last_success_at ? formatDate(source.last_success_at) : "-"}</p>
            </div>
            <div>
              <p className="text-xs uppercase text-muted-foreground">Last failure</p>
              <p className="mt-1 text-sm">{source.last_failed_at ? formatDate(source.last_failed_at) : "-"}</p>
            </div>
            <div>
              <p className="text-xs uppercase text-muted-foreground">Consecutive failures</p>
              <Badge variant="outline">{source.consecutive_failures}</Badge>
            </div>
            {source.last_error && (
              <p className="whitespace-pre-wrap rounded-md bg-muted p-3 text-sm text-destructive">
                {source.last_error}
              </p>
            )}
          </CardContent>
        </Card>
      </div>
    </div>
  );
}
