import { useEffect, useState } from "react";
import { api } from "~/lib/api";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "~/components/ui/tabs";
import { DomainCandidatesTab } from "~/components/app/review/DomainCandidatesTab";
import { CompanySuggestionsTab } from "~/components/app/review/CompanySuggestionsTab";
import { FinancialSuggestionsTab } from "~/components/app/review/FinancialSuggestionsTab";

export default function ReviewPage() {
  const [pendingDomains, setPendingDomains] = useState<number | null>(null);

  useEffect(() => {
    api.getStats().then((s) => setPendingDomains(s.pending_review)).catch(() => {});
  }, []);

  return (
    <div>
      <h1 className="mb-4 text-xl font-semibold">Review Queue</h1>
      <Tabs defaultValue="domain_candidates">
        <TabsList className="mb-4">
          <TabsTrigger value="domain_candidates" className="gap-2">
            Domain Candidates
            {pendingDomains != null && (
              <span className="rounded-full bg-primary px-2 py-0.5 text-xs font-medium text-primary-foreground">
                {pendingDomains.toLocaleString()}
              </span>
            )}
          </TabsTrigger>
          <TabsTrigger value="company_suggestions" className="gap-2">
            New Companies
            <span className="rounded-full bg-muted px-2 py-0.5 text-xs text-muted-foreground">0</span>
          </TabsTrigger>
          <TabsTrigger value="financial_suggestions" className="gap-2">
            Financial Suggestions
          </TabsTrigger>
        </TabsList>
        <TabsContent value="domain_candidates">
          <DomainCandidatesTab />
        </TabsContent>
        <TabsContent value="company_suggestions">
          <CompanySuggestionsTab />
        </TabsContent>
        <TabsContent value="financial_suggestions">
          <FinancialSuggestionsTab />
        </TabsContent>
      </Tabs>
    </div>
  );
}
