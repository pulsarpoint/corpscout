package httpapi

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/riverqueue/river"

	db "github.com/pulsarpoint/corpscout/scheduler/internal/db/gen"
	"github.com/pulsarpoint/corpscout/scheduler/internal/workers"
)

func (h *Handlers) handleProbeSource(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if _, err := h.db.GetSourceByName(r.Context(), name); err != nil {
		writeError(w, http.StatusNotFound, "source not found")
		return
	}
	if h.crawler == nil {
		writeError(w, http.StatusServiceUnavailable, "crawler not available")
		return
	}
	start := time.Now()
	resp, err := h.crawler.Crawl(r.Context(), name, time.Time{}, nil, 1)
	durationMs := time.Since(start).Milliseconds()
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"records_count": 0,
			"total":         0,
			"has_more":      false,
			"sample":        nil,
			"error":         err.Error(),
			"duration_ms":   durationMs,
		})
		return
	}
	var sample any
	if len(resp.Records) > 0 {
		sample = resp.Records[0]
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"records_count": len(resp.Records),
		"total":         resp.Total,
		"has_more":      resp.HasMore,
		"sample":        sample,
		"error":         nil,
		"duration_ms":   durationMs,
	})
}

func (h *Handlers) handleGetSource(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	source, err := h.db.GetSourceByName(r.Context(), name)
	if err != nil {
		writeError(w, http.StatusNotFound, "source not found")
		return
	}
	writeJSON(w, http.StatusOK, source)
}

func (h *Handlers) handleListSources(w http.ResponseWriter, r *http.Request) {
	sources, err := h.db.ListSources(r.Context())
	if err != nil {
		slog.Error("list sources", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, sources)
}

type patchSourceRequest struct {
	Enabled            *bool  `json:"enabled"`
	CrawlIntervalHours *int32 `json:"crawl_interval_hours"`
}

func (h *Handlers) handlePatchSource(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	var req patchSourceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Enabled != nil {
		if err := h.db.UpdateSourceEnabled(r.Context(), db.UpdateSourceEnabledParams{
			Name: name, Enabled: *req.Enabled,
		}); err != nil {
			slog.Error("update source enabled", "name", name, "error", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
	}
	if req.CrawlIntervalHours != nil {
		if err := h.db.UpdateSourceInterval(r.Context(), db.UpdateSourceIntervalParams{
			Name: name, CrawlIntervalHours: *req.CrawlIntervalHours,
		}); err != nil {
			slog.Error("update source interval", "name", name, "error", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handlers) handleTriggerSource(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	source, err := h.db.GetSourceByName(r.Context(), name)
	if err != nil {
		writeError(w, http.StatusNotFound, "source not found")
		return
	}
	if h.rv == nil {
		writeError(w, http.StatusServiceUnavailable, "scheduler not available")
		return
	}
	var since time.Time
	if source.LastCrawledAt.Valid {
		since = source.LastCrawledAt.Time
	}
	if _, err := h.rv.Insert(r.Context(), workers.SourceCrawlArgs{
		SourceName: name, Since: since,
	}, &river.InsertOpts{Queue: "source_crawl"}); err != nil {
		slog.Error("trigger source", "name", name, "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "queued"})
}
