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

	db "github.com/pulsarpoint/corpscout/scheduler/internal/db/gen"
	"github.com/pulsarpoint/corpscout/scheduler/internal/crawlerclient"
)

// Handlers holds shared dependencies for all REST API handlers.
type Handlers struct {
	db      db.Querier
	rv      *river.Client[pgx.Tx]
	pool    *pgxpool.Pool
	crawler *crawlerclient.Client
}

// NewHandlers constructs Handlers. pool, rv and crawler may be nil in tests.
func NewHandlers(q db.Querier, rv *river.Client[pgx.Tx], pool *pgxpool.Pool, crawler *crawlerclient.Client) *Handlers {
	return &Handlers{db: q, rv: rv, pool: pool, crawler: crawler}
}

// RegisterRoutes mounts all /api/v1 routes on the router.
func (h *Handlers) RegisterRoutes(r chi.Router) {
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
		r.Get("/jobs", h.handleListJobs)
		r.Get("/pull-runs", h.handleListPullRuns)
		r.Get("/review", h.handleListReview)
		r.Post("/review/{id}/reviews", h.handleCreateReview)
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

