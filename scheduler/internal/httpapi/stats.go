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
		"total_companies":           stats.TotalCompanies,
		"total_domains":             stats.TotalDomains,
		"active_domains":            stats.ActiveDomains,
		"pending_review":            stats.PendingReview,
		"pending_raw_inputs":        stats.PendingRawInputs,
		"enabled_sources":           stats.EnabledSources,
		"pull_runs_completed_today": stats.PullRunsCompletedToday,
		"pull_runs_failed_today":    stats.PullRunsFailedToday,
		"records_upserted_24h":      stats.RecordsUpserted24h,
		"records_upserted_7d":       stats.RecordsUpserted7d,
	})
}
