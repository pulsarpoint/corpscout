package httpapi

import (
	"log/slog"
	"net/http"
	"time"

	db "github.com/pulsarpoint/corpscout/scheduler/internal/db/gen"
)

type temporalExecutionRow struct {
	ID             string  `json:"id"`
	WorkflowID     *string `json:"workflow_id,omitempty"`
	WorkflowRunID  *string `json:"workflow_run_id,omitempty"`
	WorkflowType   string  `json:"workflow_type"`
	SourceName     string  `json:"source_name"`
	Country        *string `json:"country,omitempty"`
	Status         string  `json:"status"`
	RecordsWritten *int32  `json:"records_written,omitempty"`
	PagesFetched   *int32  `json:"pages_fetched,omitempty"`
	ErrorMessage   *string `json:"error_message,omitempty"`
	RiverJobID     *int64  `json:"river_job_id,omitempty"`
	StartedAt      string  `json:"started_at"`
	CompletedAt    *string `json:"completed_at,omitempty"`
	TemporalUIURL  string  `json:"temporal_ui_url,omitempty"`
}

func (h *Handlers) handleListTemporalExecutions(w http.ResponseWriter, r *http.Request) {
	if h.pool == nil {
		writeError(w, http.StatusServiceUnavailable, "database not available")
		return
	}

	page := queryInt(r, "page", 1)
	limit := min(queryInt(r, "limit", 50), 200)
	offset := (page - 1) * limit

	rows, err := h.db.ListTemporalExecutions(r.Context(), db.ListTemporalExecutionsParams{
		Limit:  int32(limit),
		Offset: int32(offset),
	})
	if err != nil {
		slog.Error("list temporal executions", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	items := make([]temporalExecutionRow, 0, len(rows))
	for _, row := range rows {
		item := temporalExecutionRow{
			ID:             row.ID.String(),
			WorkflowID:     row.WorkflowID,
			WorkflowRunID:  row.WorkflowRunID,
			WorkflowType:   row.WorkflowType,
			SourceName:     row.SourceName,
			Country:        row.Country,
			Status:         row.Status,
			RecordsWritten: row.RecordsWritten,
			PagesFetched:   row.PagesFetched,
			ErrorMessage:   row.ErrorMessage,
			RiverJobID:     row.RiverJobID,
			StartedAt:      row.StartedAt.Format(time.RFC3339),
		}
		if row.CompletedAt.Valid {
			t := row.CompletedAt.Time.Format(time.RFC3339)
			item.CompletedAt = &t
		}
		if row.WorkflowID != nil && h.temporalUIURL != "" {
			item.TemporalUIURL = h.temporalUIURL + "/namespaces/corpscout/workflows/" + *row.WorkflowID
		}
		items = append(items, item)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"items": items, "page": page, "limit": limit,
	})
}
