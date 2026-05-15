package httpapi

import (
	"log/slog"
	"net/http"
)

func (h *Handlers) handleStats(w http.ResponseWriter, r *http.Request) {
	stats, err := h.db.GetStats(r.Context())
	if err != nil {
		slog.Error("get stats", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"total_companies": stats.TotalCompanies,
		"total_domains":   stats.TotalDomains,
		"active_domains":  stats.ActiveDomains,
		"pending_review":  stats.PendingReview,
		"enabled_sources": stats.EnabledSources,
	})
}
