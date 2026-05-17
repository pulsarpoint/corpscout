package httpapi

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

	db "github.com/pulsarpoint/corpscout/scheduler/internal/db/gen"
	"github.com/pulsarpoint/corpscout/scheduler/internal/workers"
)

type sourceView struct {
	db.DataSource
	Config json.RawMessage `json:"config"`
}

func toSourceView(s db.DataSource) sourceView {
	cfg := json.RawMessage(s.Config)
	if len(cfg) == 0 {
		cfg = json.RawMessage("null")
	}
	return sourceView{DataSource: s, Config: cfg}
}

func toSourceViews(sources []db.DataSource) []sourceView {
	out := make([]sourceView, len(sources))
	for i, s := range sources {
		out[i] = toSourceView(s)
	}
	return out
}

func (h *Handlers) handleListSources(w http.ResponseWriter, r *http.Request) {
	sources, err := h.db.ListSources(r.Context())
	if err != nil {
		slog.Error("list sources", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, toSourceViews(sources))
}

func (h *Handlers) handleGetSource(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	source, err := h.db.GetSourceByName(r.Context(), name)
	if err != nil {
		writeError(w, http.StatusNotFound, "source not found")
		return
	}
	writeJSON(w, http.StatusOK, toSourceView(source))
}

type patchSourceRequest struct {
	Enabled            *bool   `json:"enabled"`
	ScheduleKind       *string `json:"schedule_kind"`
	ScheduleExpression *string `json:"schedule_expression"`
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
	if req.ScheduleKind != nil || req.ScheduleExpression != nil {
		src, err := h.db.GetSourceByName(r.Context(), name)
		if err != nil {
			writeError(w, http.StatusNotFound, "source not found")
			return
		}
		kind := src.ScheduleKind
		expr := src.ScheduleExpression
		if req.ScheduleKind != nil {
			kind = *req.ScheduleKind
		}
		if req.ScheduleExpression != nil {
			expr = req.ScheduleExpression
		}
		if err := h.db.UpdateSourceSchedule(r.Context(), db.UpdateSourceScheduleParams{
			Name:               name,
			ScheduleKind:       kind,
			ScheduleExpression: expr,
		}); err != nil {
			slog.Error("update source schedule", "name", name, "error", err)
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
	if source.PullTaskType != "source_pull" {
		writeError(w, http.StatusUnprocessableEntity, "pull task type not supported for manual trigger")
		return
	}
	if h.rv == nil {
		writeError(w, http.StatusServiceUnavailable, "scheduler not available")
		return
	}
	if _, err := h.rv.Insert(r.Context(), workers.SourcePullArgs{
		SourceName:  name,
		TriggerType: "manual",
	}, &river.InsertOpts{
		Queue: "source_pull",
		UniqueOpts: river.UniqueOpts{
			ByArgs:  true,
			ByState: []rivertype.JobState{rivertype.JobStateAvailable, rivertype.JobStateRunning, rivertype.JobStateScheduled},
		},
	}); err != nil {
		slog.Error("trigger source", "name", name, "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "queued"})
}

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
			"records_count": 0, "total": 0, "has_more": false,
			"sample": nil, "error": err.Error(), "duration_ms": durationMs,
		})
		return
	}
	var sample any
	if len(resp.Records) > 0 {
		sample = resp.Records[0]
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"records_count": len(resp.Records), "total": resp.Total,
		"has_more": resp.HasMore, "sample": sample,
		"error": nil, "duration_ms": durationMs,
	})
}
