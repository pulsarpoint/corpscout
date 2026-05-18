package httpapi

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	pgx "github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"

	"github.com/pulsarpoint/corpscout/scheduler/internal/crawlerclient"
	db "github.com/pulsarpoint/corpscout/scheduler/internal/db/gen"
)

// Handlers holds shared dependencies for all REST API handlers.
type Handlers struct {
	db           db.Querier
	rv           *river.Client[pgx.Tx]
	pool         *pgxpool.Pool
	crawler      *crawlerclient.Client
	postgrestURL string
}

// NewHandlers constructs Handlers. pool, rv and crawler may be nil in tests.
func NewHandlers(q db.Querier, rv *river.Client[pgx.Tx], pool *pgxpool.Pool, crawler *crawlerclient.Client, postgrestURL string) *Handlers {
	return &Handlers{db: q, rv: rv, pool: pool, crawler: crawler, postgrestURL: postgrestURL}
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
		r.Get("/domains", h.handleListDomains)
		r.Get("/countries", h.handleListCountries)
		r.Get("/sources", h.handleListSources)
		r.Get("/sources/{name}", h.handleGetSource)
		r.Patch("/sources/{name}", h.handlePatchSource)
		r.Post("/sources/{name}/trigger", h.handleTriggerSource)
		r.Post("/sources/{name}/probe", h.handleProbeSource)
		r.Post("/sources/{name}/raw-inputs/{id}/retry", h.handleRetryRawInput)
		r.Post("/sources/{name}/raw-inputs/{id}/ignore", h.handleIgnoreRawInput)
		r.Get("/jobs", h.handleListJobs)
		r.Get("/jobs/stats", h.handleJobStats)
		r.Post("/jobs/cancel-bulk", h.handleCancelBulk)
		r.Get("/jobs/{id}", h.handleGetJob)
		r.Post("/jobs/{id}/cancel", h.handleCancelJob)
		r.Get("/pull-runs", h.handleListPullRuns)
		r.Post("/resolve", h.handleResolve)
		r.Get("/organizations", h.handleListOrganizations)
		r.Post("/organizations", h.handleCreateOrganization)
		r.Get("/organizations/{id}", h.handleGetOrganization)
		r.Get("/open-source-projects", h.handleListOpenSourceProjects)
		r.Post("/open-source-projects", h.handleCreateOpenSourceProject)
		r.Get("/open-source-projects/{id}", h.handleGetOpenSourceProject)
		r.Get("/review", h.handleListReview)
		r.Post("/review/bulk", h.handleBulkReview)
		r.Post("/review/{id}/reviews", h.handleCreateReview)
		r.Get("/suggestions/companies", h.handleListCompanySuggestions)
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
