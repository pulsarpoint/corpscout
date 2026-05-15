package httpapi

import (
	"log/slog"
	"net/http"

	db "github.com/pulsarpoint/corpscout/scheduler/internal/db/gen"
)

func (h *Handlers) handleListPullRuns(w http.ResponseWriter, r *http.Request) {
	page := queryInt(r, "page", 1)
	limit := min(queryInt(r, "limit", 20), 100)
	offset := int32((page - 1) * limit)

	runs, err := h.db.ListPullRuns(r.Context(), db.ListPullRunsParams{
		Limit: int32(limit), Offset: offset,
	})
	if err != nil {
		slog.Error("list pull runs", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if runs == nil {
		runs = []db.ListPullRunsRow{}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items": runs, "page": page, "limit": limit,
	})
}
