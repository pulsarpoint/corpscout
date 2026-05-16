package httpapi

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/go-chi/chi/v5"
	"github.com/riverqueue/river"
)

type jobRow struct {
	ID          int64           `json:"id"`
	Kind        string          `json:"kind"`
	State       string          `json:"state"`
	Args        json.RawMessage `json:"args"`
	Attempt     int             `json:"attempt"`
	MaxAttempts int             `json:"max_attempts"`
	Queue       string          `json:"queue"`
	Priority    int             `json:"priority"`
	CreatedAt   time.Time       `json:"created_at"`
	ScheduledAt time.Time       `json:"scheduled_at"`
	FinalizedAt *time.Time      `json:"finalized_at,omitempty"`
	LastError   *string         `json:"last_error,omitempty"`
	Subject     *string         `json:"subject,omitempty"`
	Errors      json.RawMessage `json:"errors,omitempty"`
}

type jobStats struct {
	Kind  string `json:"kind"`
	State string `json:"state"`
	Count int64  `json:"count"`
}

func (h *Handlers) handleListJobs(w http.ResponseWriter, r *http.Request) {
	if h.pool == nil {
		writeError(w, http.StatusServiceUnavailable, "database not available")
		return
	}

	page := queryInt(r, "page", 1)
	limit := min(queryInt(r, "limit", 50), 200)
	offset := (page - 1) * limit

	stateFilter := r.URL.Query().Get("status")
	sourceFilter := r.URL.Query().Get("source")
	kindFilter := r.URL.Query().Get("kind")

	rows, err := h.pool.Query(r.Context(), `
		SELECT
			j.id, j.kind, j.state::text, j.args,
			j.attempt, j.max_attempts, j.queue, j.priority,
			j.created_at, j.scheduled_at, j.finalized_at,
			CASE WHEN array_length(j.errors, 1) > 0
				THEN j.errors[array_length(j.errors, 1)]->>'error'
			END AS last_error,
			CASE
				WHEN j.kind = 'source_crawl' THEN j.args->>'source_name'
				WHEN j.kind = 'domain_resolve' THEN COALESCE(c.name, j.args->>'company_id')
			END AS subject,
			array_to_json(j.errors)::text AS errors
		FROM river_job j
		LEFT JOIN companies c
			ON j.kind = 'domain_resolve'
			AND c.id = (j.args->>'company_id')::uuid
		WHERE ($1::text IS NULL OR j.state::text = $1)
		  AND ($2::text IS NULL OR j.args->>'source_name' = $2)
		  AND ($3::text IS NULL OR j.kind = $3)
		ORDER BY j.created_at DESC
		LIMIT $4 OFFSET $5
	`, nilIfEmpty(stateFilter), nilIfEmpty(sourceFilter), nilIfEmpty(kindFilter), limit, offset)
	if err != nil {
		slog.Error("list jobs query", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	defer rows.Close()

	jobs := make([]jobRow, 0)
	for rows.Next() {
		var j jobRow
		var rawErrors []byte
		if err := rows.Scan(
			&j.ID, &j.Kind, &j.State, &j.Args,
			&j.Attempt, &j.MaxAttempts, &j.Queue, &j.Priority,
			&j.CreatedAt, &j.ScheduledAt, &j.FinalizedAt,
			&j.LastError, &j.Subject, &rawErrors,
		); err != nil {
			slog.Error("scan job row", "error", err)
			continue
		}
		if len(rawErrors) > 0 {
			j.Errors = json.RawMessage(rawErrors)
		}
		jobs = append(jobs, j)
	}
	if err := rows.Err(); err != nil {
		slog.Error("list jobs rows", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"items": jobs, "page": page, "limit": limit,
	})
}

func (h *Handlers) handleGetJob(w http.ResponseWriter, r *http.Request) {
	if h.pool == nil {
		writeError(w, http.StatusServiceUnavailable, "database not available")
		return
	}

	idStr := chi.URLParam(r, "id")
	var id int64
	if _, err := fmt.Sscan(idStr, &id); err != nil {
		writeError(w, http.StatusBadRequest, "invalid job id")
		return
	}

	rows, err := h.pool.Query(r.Context(), `
		SELECT
			j.id, j.kind, j.state::text, j.args,
			j.attempt, j.max_attempts, j.queue, j.priority,
			j.created_at, j.scheduled_at, j.finalized_at,
			CASE WHEN array_length(j.errors, 1) > 0
				THEN j.errors[array_length(j.errors, 1)]->>'error'
			END AS last_error,
			CASE
				WHEN j.kind = 'source_crawl' THEN j.args->>'source_name'
				WHEN j.kind = 'domain_resolve' THEN COALESCE(c.name, j.args->>'company_id')
			END AS subject,
			array_to_json(j.errors)::text AS errors
		FROM river_job j
		LEFT JOIN companies c
			ON j.kind = 'domain_resolve'
			AND c.id = (j.args->>'company_id')::uuid
		WHERE j.id = $1
	`, id)
	if err != nil {
		slog.Error("get job query", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	defer rows.Close()

	if !rows.Next() {
		writeError(w, http.StatusNotFound, "job not found")
		return
	}

	var j jobRow
	var rawErrors []byte
	if err := rows.Scan(
		&j.ID, &j.Kind, &j.State, &j.Args,
		&j.Attempt, &j.MaxAttempts, &j.Queue, &j.Priority,
		&j.CreatedAt, &j.ScheduledAt, &j.FinalizedAt,
		&j.LastError, &j.Subject, &rawErrors,
	); err != nil {
		slog.Error("scan job row", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if len(rawErrors) > 0 {
		j.Errors = json.RawMessage(rawErrors)
	}

	writeJSON(w, http.StatusOK, j)
}

func (h *Handlers) handleJobStats(w http.ResponseWriter, r *http.Request) {
	if h.pool == nil {
		writeError(w, http.StatusServiceUnavailable, "database not available")
		return
	}

	rows, err := h.pool.Query(r.Context(), `
		SELECT kind, state::text, COUNT(*)
		FROM river_job
		GROUP BY kind, state::text
		ORDER BY kind, state::text
	`)
	if err != nil {
		slog.Error("job stats query", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	defer rows.Close()

	stats := make([]jobStats, 0)
	for rows.Next() {
		var s jobStats
		if err := rows.Scan(&s.Kind, &s.State, &s.Count); err != nil {
			continue
		}
		stats = append(stats, s)
	}

	writeJSON(w, http.StatusOK, stats)
}

func (h *Handlers) handleCancelJob(w http.ResponseWriter, r *http.Request) {
	if h.rv == nil {
		writeError(w, http.StatusServiceUnavailable, "river client not available")
		return
	}

	idStr := chi.URLParam(r, "id")
	var id int64
	if _, err := fmt.Sscan(idStr, &id); err != nil {
		writeError(w, http.StatusBadRequest, "invalid job id")
		return
	}

	_, err := h.rv.JobCancel(r.Context(), id)
	if err != nil {
		if errors.Is(err, river.ErrNotFound) {
			writeError(w, http.StatusNotFound, "job not found")
			return
		}
		slog.Error("cancel job", "job_id", id, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to cancel job")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"status": "cancelled", "id": id})
}

func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
