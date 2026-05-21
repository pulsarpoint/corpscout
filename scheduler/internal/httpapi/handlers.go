package httpapi

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	pgx "github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
	"go.temporal.io/sdk/client"

	"github.com/pulsarpoint/corpscout/scheduler/internal/crawlerclient"
	db "github.com/pulsarpoint/corpscout/scheduler/internal/db/gen"
	"github.com/pulsarpoint/corpscout/scheduler/internal/s3client"
)

type riverInserter interface {
	Insert(context.Context, river.JobArgs, *river.InsertOpts) (*rivertype.JobInsertResult, error)
	InsertTx(context.Context, pgx.Tx, river.JobArgs, *river.InsertOpts) (*rivertype.JobInsertResult, error)
	JobCancel(context.Context, int64) (*rivertype.JobRow, error)
}

// Handlers holds shared dependencies for all REST API handlers.
type Handlers struct {
	db            db.Querier
	rv            riverInserter
	pool          *pgxpool.Pool
	crawler       *crawlerclient.Client
	s3            *s3client.Client
	postgrestURL  string
	temporal      client.Client
	temporalUIURL string
}

// NewHandlers constructs Handlers. pool, rv, crawler, s3 and temporal may be nil in tests.
func NewHandlers(q db.Querier, rv riverInserter, pool *pgxpool.Pool, crawler *crawlerclient.Client, s3 *s3client.Client, postgrestURL string, tc client.Client, temporalUIURL string) *Handlers {
	return &Handlers{db: q, rv: rv, pool: pool, crawler: crawler, s3: s3, postgrestURL: postgrestURL, temporal: tc, temporalUIURL: temporalUIURL}
}

// RegisterRoutes mounts all /api/v1 routes on the router.
func (h *Handlers) RegisterRoutes(r chi.Router) {
	if h.postgrestURL != "" {
		proxy := newPostgRESTProxy(h.postgrestURL)
		r.HandleFunc("/api/v1/db/*", proxy)
		r.HandleFunc("/api/v1/db", proxy)
	}
	r.Route("/api/v1", func(r chi.Router) {
		r.Get("/stats", h.handleStats)
		r.Get("/companies", h.handleListCompanies)
		r.Get("/companies/{id}", h.handleGetCompany)
		r.Get("/companies/{id}/enrichment-sources", h.handleGetCompanyEnrichmentSources)
		r.Post("/companies/{id}/enrich-from-source", h.handleEnrichCompanyFromSource)
		r.Patch("/companies/{id}", h.handlePatchCompany)
		r.Patch("/companies/{id}/financials", h.handlePatchCompanyFinancials)
		r.Get("/domains", h.handleListDomains)
		r.Post("/domains/import", h.handleImportDomains)
		r.Get("/domains/import-batches", h.handleListImportBatches)
		r.Get("/domains/{id}", h.handleGetDomain)
		r.Post("/domains/{id}/crawl", h.handleTriggerDomainCrawl)
		r.Get("/domains/{id}/crawl-jobs", h.handleListDomainCrawlJobs)
		r.Get("/domains/{id}/crawl-jobs/{job_id}", h.handleGetDomainCrawlJob)
		r.Get("/domains/{id}/crawl-jobs/{job_id}/pages", h.handleListDomainCrawlJobPages)
		r.Get("/domains/{id}/crawl-jobs/{job_id}/pages/{page_num}/markdown", h.handleGetPageMarkdown)
		r.Get("/domains/{id}/crawl-jobs/{job_id}/pages/{page_num}/html", h.handleGetPageHTML)
		r.Get("/domains/{id}/crawl-jobs/{job_id}/pages/{page_num}/headers", h.handleGetPageHeaders)
		r.Get("/domains/{id}/crawl-jobs/{job_id}/favicon", h.handleGetJobFavicon)
		r.Get("/countries", h.handleListCountries)
		r.Get("/sources", h.handleListSources)
		r.Get("/sources/{name}", h.handleGetSource)
		r.Patch("/sources/{name}", h.handlePatchSource)
		r.Post("/sources/{name}/trigger", h.handleTriggerSource)
		r.Post("/sources/{name}/process", h.handleProcessSource)
		r.Post("/sources/brreg/translate", h.handleTranslateBrreg)
		r.Get("/sources/brreg/translation-stats", h.handleBrregTranslationStats)
		r.Post("/sources/{name}/probe", h.handleProbeSource)
		r.Post("/sources/{name}/raw-inputs/{id}/retry", h.handleRetryRawInput)
		r.Post("/sources/{name}/raw-inputs/{id}/ignore", h.handleIgnoreRawInput)
		r.Get("/jobs", h.handleListJobs)
		r.Get("/jobs/stats", h.handleJobStats)
		r.Post("/jobs/cancel-bulk", h.handleCancelBulk)
		r.Get("/jobs/{id}", h.handleGetJob)
		r.Post("/jobs/{id}/cancel", h.handleCancelJob)
		r.Get("/temporal-executions", h.handleListTemporalExecutions)
		r.Get("/pull-runs", h.handleListPullRuns)
		r.Post("/resolve", h.handleResolve)
		r.Get("/organizations", h.handleListOrganizations)
		r.Post("/organizations", h.handleCreateOrganization)
		r.Get("/organizations/{id}", h.handleGetOrganization)
		r.Get("/open-source-projects", h.handleListOpenSourceProjects)
		r.Post("/open-source-projects", h.handleCreateOpenSourceProject)
		r.Get("/open-source-projects/{id}", h.handleGetOpenSourceProject)
		r.Get("/review", h.handleListReview)
		r.Get("/review/ids", h.handleListReviewIDs)
		r.Post("/review/bulk", h.handleBulkReview)
		r.Post("/review/{id}/reviews", h.handleCreateReview)
		r.Get("/financials/review", h.handleListPendingFinancials)
		r.Get("/financials/review/ids", h.handleListPendingFinancialIDs)
		r.Post("/financials/review/bulk", h.handleBulkReviewFinancials)
		r.Get("/companies/{id}/financials", h.handleListCompanyFinancials)
		r.Post("/financials/{id}/review", h.handleReviewFinancial)
		r.Get("/raw-inputs", h.handleListRawInputs)
		r.Get("/raw-inputs/{source}/{id}", h.handleGetRawInput)
		r.Get("/suggestions/companies", h.handleListCompanySuggestions)
		r.Get("/suggestions/companies/ids", h.handleListCompanySuggestionIDs)
		r.Post("/suggestions/companies/bulk", h.handleBulkCompanySuggestions)
		r.Get("/suggestions/companies/{id}", h.handleGetCompanySuggestion)
		r.Post("/suggestions/companies/{id}/approve", h.handleApproveCompanySuggestion)
		r.Post("/suggestions/companies/{id}/reject", h.handleRejectCompanySuggestion)
		r.Post("/suggestions/companies/{id}/approve-with-sections", h.handleApproveCompanyWithSections)
		r.Post("/suggestions/company-status/{id}/approve", h.handleApproveCompanyStatusSuggestion)
		r.Post("/suggestions/company-status/{id}/reject", h.handleRejectCompanyStatusSuggestion)
		r.Post("/suggestions/company-contact/{id}/approve", h.handleApproveCompanyContactSuggestion)
		r.Post("/suggestions/company-contact/{id}/reject", h.handleRejectCompanyContactSuggestion)
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("write json response", "error", err)
	}
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func queryInt(r *http.Request, key string, fallback int) int {
	s := r.URL.Query().Get(key)
	if s == "" {
		return fallback
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 1 {
		return fallback
	}
	return n
}

func queryString(r *http.Request, key string) *string {
	s := r.URL.Query().Get(key)
	if s == "" {
		return nil
	}
	return &s
}

func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func decodeJSON(r *http.Request, v any) error {
	return json.NewDecoder(r.Body).Decode(v)
}
