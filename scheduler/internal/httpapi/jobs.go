package httpapi

import (
	"log/slog"
	"net/http"
	"time"
)

type jobRow struct {
	ID          int64      `json:"id"`
	Kind        string     `json:"kind"`
	State       string     `json:"state"`
	Args        []byte     `json:"args"`
	Attempt     int        `json:"attempt"`
	MaxAttempts int        `json:"max_attempts"`
	Queue       string     `json:"queue"`
	Priority    int        `json:"priority"`
	CreatedAt   time.Time  `json:"created_at"`
	ScheduledAt time.Time  `json:"scheduled_at"`
	FinalizedAt *time.Time `json:"finalized_at,omitempty"`
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
	kindFilter := r.URL.Query().Get("source")

	rows, err := h.pool.Query(r.Context(), `
        SELECT id, kind, state::text, args, attempt, max_attempts, queue, priority,
               created_at, scheduled_at, finalized_at
        FROM river_job
        WHERE ($1::text IS NULL OR state::text = $1)
          AND ($2::text IS NULL OR args->>'source_name' = $2)
        ORDER BY created_at DESC
        LIMIT $3 OFFSET $4
    `, nilIfEmpty(stateFilter), nilIfEmpty(kindFilter), limit, offset)
	if err != nil {
		slog.Error("list jobs query", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	defer rows.Close()

	jobs := make([]jobRow, 0)
	for rows.Next() {
		var j jobRow
		if err := rows.Scan(
			&j.ID, &j.Kind, &j.State, &j.Args,
			&j.Attempt, &j.MaxAttempts, &j.Queue, &j.Priority,
			&j.CreatedAt, &j.ScheduledAt, &j.FinalizedAt,
		); err != nil {
			slog.Error("scan job row", "error", err)
			continue
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

func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
