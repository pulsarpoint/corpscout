package httpapi

import (
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/riverqueue/river"

	db "github.com/pulsarpoint/corpscout/scheduler/internal/db/gen"
	"github.com/pulsarpoint/corpscout/scheduler/internal/workers"
)

type triggerCrawlRequest struct {
	Mode     string `json:"mode"`
	MaxPages int    `json:"max_pages"`
}

func (h *Handlers) handleTriggerDomainCrawl(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	domainID, err := uuid.Parse(idStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid domain id")
		return
	}

	var req triggerCrawlRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Mode == "" {
		req.Mode = "deep"
	}
	if req.MaxPages <= 0 {
		req.MaxPages = 10
	}
	if req.Mode != "homepage" && req.Mode != "deep" {
		writeError(w, http.StatusBadRequest, "mode must be homepage or deep")
		return
	}

	domain, err := h.db.GetDomainByID(r.Context(), domainID)
	if err != nil {
		writeError(w, http.StatusNotFound, "domain not found")
		return
	}

	job, err := h.db.InsertDomainCrawlJob(r.Context(), db.InsertDomainCrawlJobParams{
		DomainID: domainID,
		Mode:     req.Mode,
		MaxPages: int32(req.MaxPages),
	})
	if err != nil {
		slog.Error("insert domain crawl job", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to create crawl job")
		return
	}

	riverJob, err := h.rv.Insert(r.Context(), workers.DomainCrawlArgs{
		DomainCrawlJobID: job.ID.String(),
		DomainID:         domainID.String(),
		Domain:           domain.Domain,
		Mode:             req.Mode,
		MaxPages:         req.MaxPages,
	}, &river.InsertOpts{Queue: "domain_crawl"})
	if err != nil {
		slog.Error("enqueue domain crawl job", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to enqueue crawl job")
		return
	}

	writeJSON(w, http.StatusAccepted, map[string]any{
		"job_id":       job.ID,
		"river_job_id": riverJob.Job.ID,
	})
}

func (h *Handlers) handleListDomainCrawlJobs(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	domainID, err := uuid.Parse(idStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid domain id")
		return
	}
	jobs, err := h.db.ListDomainCrawlJobs(r.Context(), domainID)
	if err != nil {
		slog.Error("list domain crawl jobs", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list crawl jobs")
		return
	}
	if jobs == nil {
		jobs = []db.ListDomainCrawlJobsRow{}
	}
	writeJSON(w, http.StatusOK, jobs)
}

func (h *Handlers) handleGetDomainCrawlJob(w http.ResponseWriter, r *http.Request) {
	domainID, jobID, ok := parseDomainAndJobID(w, r)
	if !ok {
		return
	}
	job, err := h.db.GetDomainCrawlJob(r.Context(), db.GetDomainCrawlJobParams{
		ID:       jobID,
		DomainID: domainID,
	})
	if err != nil {
		writeError(w, http.StatusNotFound, "crawl job not found")
		return
	}
	writeJSON(w, http.StatusOK, job)
}

func (h *Handlers) handleListDomainCrawlJobPages(w http.ResponseWriter, r *http.Request) {
	_, jobID, ok := parseDomainAndJobID(w, r)
	if !ok {
		return
	}
	pages, err := h.db.ListDomainCrawlJobPages(r.Context(), jobID)
	if err != nil {
		slog.Error("list domain crawl job pages", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list pages")
		return
	}
	if pages == nil {
		pages = []db.DomainCrawlJobPage{}
	}
	writeJSON(w, http.StatusOK, pages)
}

func (h *Handlers) handleGetPageMarkdown(w http.ResponseWriter, r *http.Request) {
	page, ok := h.fetchPage(w, r)
	if !ok {
		return
	}
	data, _, err := h.s3.Download(r.Context(), page.MdS3Key)
	if err != nil {
		writeError(w, http.StatusNotFound, "content unavailable")
		return
	}
	w.Header().Set("Content-Type", "text/markdown")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func (h *Handlers) handleGetPageHTML(w http.ResponseWriter, r *http.Request) {
	page, ok := h.fetchPage(w, r)
	if !ok {
		return
	}
	data, _, err := h.s3.Download(r.Context(), page.HtmlS3Key)
	if err != nil {
		writeError(w, http.StatusNotFound, "content unavailable")
		return
	}
	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func (h *Handlers) handleGetPageHeaders(w http.ResponseWriter, r *http.Request) {
	page, ok := h.fetchPage(w, r)
	if !ok {
		return
	}
	data, _, err := h.s3.Download(r.Context(), page.HeadersS3Key)
	if err != nil {
		writeError(w, http.StatusNotFound, "content unavailable")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func (h *Handlers) handleGetJobFavicon(w http.ResponseWriter, r *http.Request) {
	domainID, jobID, ok := parseDomainAndJobID(w, r)
	if !ok {
		return
	}
	job, err := h.db.GetDomainCrawlJob(r.Context(), db.GetDomainCrawlJobParams{
		ID:       jobID,
		DomainID: domainID,
	})
	if err != nil || job.FaviconS3Key == nil {
		writeError(w, http.StatusNotFound, "favicon not available")
		return
	}
	data, ct, err := h.s3.Download(r.Context(), *job.FaviconS3Key)
	if err != nil {
		writeError(w, http.StatusNotFound, "favicon not available")
		return
	}
	if ct == "" {
		ct = "image/x-icon"
	}
	w.Header().Set("Content-Type", ct)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

// parseDomainAndJobID extracts and parses domain id and job_id from URL params.
func parseDomainAndJobID(w http.ResponseWriter, r *http.Request) (uuid.UUID, uuid.UUID, bool) {
	domainID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid domain id")
		return uuid.UUID{}, uuid.UUID{}, false
	}
	jobID, err := uuid.Parse(chi.URLParam(r, "job_id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid job id")
		return uuid.UUID{}, uuid.UUID{}, false
	}
	return domainID, jobID, true
}

// fetchPage looks up a DomainCrawlJobPage by job_id and page_num URL params.
func (h *Handlers) fetchPage(w http.ResponseWriter, r *http.Request) (db.DomainCrawlJobPage, bool) {
	_, jobID, ok := parseDomainAndJobID(w, r)
	if !ok {
		return db.DomainCrawlJobPage{}, false
	}
	pageNum, err := strconv.Atoi(chi.URLParam(r, "page_num"))
	if err != nil || pageNum < 1 {
		writeError(w, http.StatusBadRequest, "invalid page_num")
		return db.DomainCrawlJobPage{}, false
	}
	page, err := h.db.GetDomainCrawlJobPage(r.Context(), db.GetDomainCrawlJobPageParams{
		JobID:   jobID,
		PageNum: int32(pageNum),
	})
	if err != nil {
		writeError(w, http.StatusNotFound, "page not found")
		return db.DomainCrawlJobPage{}, false
	}
	return page, true
}
