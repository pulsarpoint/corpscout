import { useState } from "react";
import { toast } from "sonner";
import { Button } from "~/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "~/components/ui/dialog";
import { Input } from "~/components/ui/input";
import { Label } from "~/components/ui/label";
import { RadioGroup, RadioGroupItem } from "~/components/ui/radio-group";
import { triggerDomainCrawl } from "~/lib/api";

type Props = (
  | { domainId: string; domainName: string; domainIds?: never }
  | { domainIds: string[]; domainId?: never; domainName?: never }
) & {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onSuccess?: () => void;
};

export function CrawlDomainDialog({ domainId, domainName, domainIds, open, onOpenChange, onSuccess }: Props) {
  const isBulk = domainIds !== undefined;
  const [mode, setMode] = useState<"homepage" | "deep">("deep");
  const [maxPages, setMaxPages] = useState(10);
  const [loading, setLoading] = useState(false);

  async function handleSubmit() {
    setLoading(true);
    try {
      if (isBulk) {
        if (domainIds.length === 0) { onOpenChange(false); return; }
        const results = await Promise.allSettled(
          domainIds.map(id => triggerDomainCrawl(id, { mode, max_pages: maxPages }))
        );
        const failed = results.filter(r => r.status === "rejected").length;
        if (failed === 0) {
          toast.success(`${domainIds.length} crawl job${domainIds.length !== 1 ? "s" : ""} started`);
        } else {
          toast.error(`${domainIds.length - failed} started, ${failed} failed`);
        }
      } else {
        await triggerDomainCrawl(domainId, { mode, max_pages: maxPages });
        toast.success("Crawl job started");
      }
      onOpenChange(false);
      onSuccess?.();
    } catch {
      toast.error("Failed to start crawl job.");
    } finally {
      setLoading(false);
    }
  }

  const title = isBulk
    ? `Crawl ${domainIds.length} domain${domainIds.length !== 1 ? "s" : ""}?`
    : `Crawl ${domainName}?`;

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>{title}</DialogTitle>
        </DialogHeader>
        <div className="space-y-4 py-2">
          {isBulk && (
            <p className="text-sm text-muted-foreground">
              This will queue crawl jobs for {domainIds.length.toLocaleString()} domain{domainIds.length !== 1 ? "s" : ""}.
            </p>
          )}
          <div className="space-y-2">
            <Label>Mode</Label>
            <RadioGroup value={mode} onValueChange={(v) => setMode(v as "homepage" | "deep")}>
              <div className="flex items-center space-x-2">
                <RadioGroupItem value="homepage" id="homepage" />
                <Label htmlFor="homepage">Homepage only</Label>
              </div>
              <div className="flex items-center space-x-2">
                <RadioGroupItem value="deep" id="deep" />
                <Label htmlFor="deep">Deep crawl</Label>
              </div>
            </RadioGroup>
          </div>
          {mode === "deep" && (
            <div className="space-y-2">
              <Label htmlFor="max-pages">Max pages</Label>
              <Input
                id="max-pages"
                type="number"
                min={1}
                max={50}
                value={maxPages}
                onChange={(e) => setMaxPages(Number(e.target.value))}
                className="w-24"
              />
            </div>
          )}
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)} disabled={loading}>
            Cancel
          </Button>
          <Button onClick={handleSubmit} disabled={loading}>
            {loading ? "Starting…" : isBulk ? `Start ${domainIds.length.toLocaleString()} Crawls` : "Start Crawl"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
