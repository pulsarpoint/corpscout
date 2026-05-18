package httpapi

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	db "github.com/pulsarpoint/corpscout/scheduler/internal/db/gen"
)

func (h *Handlers) handleListReview(w http.ResponseWriter, r *http.Request) {
	page := queryInt(r, "page", 1)
	limit := min(queryInt(r, "limit", 50), 200)
	offset := int32((page - 1) * limit)
	status := "needs_review"

	params := db.ListDomainsParams{
		Status: &status,
		Offset: offset,
		Limit:  int32(limit),
	}

	items, err := h.db.ListDomains(r.Context(), params)
	if err != nil {
		slog.Error("list review queue", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	total, err := h.db.CountDomains(r.Context(), db.CountDomainsParams{Status: &status})
	if err != nil {
		slog.Error("count review queue", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items": items, "total": total, "page": page, "limit": limit,
	})
}

func (h *Handlers) handleCreateReview(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}

	var body struct {
		Action string `json:"action"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	var status string
	switch body.Action {
	case "approved":
		status = "active"
	case "rejected":
		status = "rejected"
	case "superseded":
		status = "superseded"
	default:
		writeError(w, http.StatusBadRequest, "action must be approved, rejected, or superseded")
		return
	}

	if err := h.db.ReviewCompanyDomain(r.Context(), db.ReviewCompanyDomainParams{
		ID:     id,
		Status: status,
	}); err != nil {
		slog.Error("review company domain", "id", id, "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
