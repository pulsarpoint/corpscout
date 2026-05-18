package httpapi

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	db "github.com/pulsarpoint/corpscout/scheduler/internal/db/gen"
)

func (h *Handlers) handleListReview(w http.ResponseWriter, r *http.Request) {
	page := queryInt(r, "page", 1)
	limit := min(queryInt(r, "limit", 50), 200)
	offset := int32((page - 1) * limit)
	status := "needs_review"

	var minConf *int16
	if s := r.URL.Query().Get("min_confidence"); s != "" {
		if n, err := strconv.Atoi(s); err == nil {
			v := int16(n)
			minConf = &v
		}
	}

	params := db.ListDomainsParams{
		Status:        &status,
		Signal:        queryString(r, "signal"),
		MinConfidence: minConf,
		Q:             queryString(r, "q"),
		Offset:        offset,
		Limit:         int32(limit),
	}

	items, err := h.db.ListDomains(r.Context(), params)
	if err != nil {
		slog.Error("list review queue", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	total, err := h.db.CountDomains(r.Context(), db.CountDomainsParams{
		Status:        &status,
		Signal:        params.Signal,
		MinConfidence: params.MinConfidence,
		Q:             params.Q,
	})
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

	status, ok := reviewActionToStatus(body.Action)
	if !ok {
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

func (h *Handlers) handleBulkReview(w http.ResponseWriter, r *http.Request) {
	var body struct {
		IDs    []string `json:"ids"`
		Action string   `json:"action"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if len(body.IDs) == 0 {
		writeError(w, http.StatusBadRequest, "ids must not be empty")
		return
	}
	status, ok := reviewActionToStatus(body.Action)
	if !ok {
		writeError(w, http.StatusBadRequest, "action must be approved, rejected, or superseded")
		return
	}

	updated := 0
	for _, idStr := range body.IDs {
		id, err := uuid.Parse(idStr)
		if err != nil {
			continue
		}
		if err := h.db.ReviewCompanyDomain(r.Context(), db.ReviewCompanyDomainParams{
			ID:     id,
			Status: status,
		}); err != nil {
			slog.Error("bulk review company domain", "id", id, "error", err)
			continue
		}
		updated++
	}
	writeJSON(w, http.StatusOK, map[string]any{"updated": updated})
}

func reviewActionToStatus(action string) (string, bool) {
	switch action {
	case "approved":
		return "active", true
	case "rejected":
		return "rejected", true
	case "superseded":
		return "superseded", true
	default:
		return "", false
	}
}
