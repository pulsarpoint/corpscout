package httpapi

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	db "github.com/pulsarpoint/corpscout/scheduler/internal/db/gen"
	"github.com/pulsarpoint/corpscout/scheduler/internal/service"
)

func (h *Handlers) handleListCompanySuggestions(w http.ResponseWriter, r *http.Request) {
	page := queryInt(r, "page", 1)
	limit := min(queryInt(r, "limit", 20), 100)
	offset := int32((page - 1) * limit)

	items, err := h.db.ListPendingCompanySuggestions(r.Context(), db.ListPendingCompanySuggestionsParams{
		Offset: offset,
		Limit:  int32(limit),
	})
	if err != nil {
		slog.Error("list company suggestions", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	total, _ := h.db.CountPendingCompanySuggestions(r.Context())
	if items == nil {
		items = []db.CompanySuggestion{}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items": items, "page": page, "limit": limit, "total": total,
	})
}

func (h *Handlers) handleGetCompanySuggestion(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	sug, err := h.db.GetCompanySuggestionByID(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "suggestion not found")
		return
	}
	writeJSON(w, http.StatusOK, sug)
}

type reviewRequest struct {
	ReviewedBy string `json:"reviewed_by"`
	ReviewNote string `json:"review_note"`
}

func (h *Handlers) handleApproveCompanySuggestion(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	var req reviewRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if h.pool == nil {
		writeError(w, http.StatusServiceUnavailable, "database pool not available")
		return
	}
	company, err := service.ApproveCompanySuggestion(r.Context(), h.pool, id, req.ReviewedBy, req.ReviewNote)
	if err != nil {
		slog.Error("approve company suggestion", "id", id, "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, company)
}

func (h *Handlers) handleRejectCompanySuggestion(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	var req reviewRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if h.pool == nil {
		writeError(w, http.StatusServiceUnavailable, "database pool not available")
		return
	}
	if err := service.RejectCompanySuggestion(r.Context(), h.pool, id, req.ReviewedBy, req.ReviewNote); err != nil {
		slog.Error("reject company suggestion", "id", id, "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "rejected"})
}

func (h *Handlers) handleApproveCompanyStatusSuggestion(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	var req reviewRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if h.pool == nil {
		writeError(w, http.StatusServiceUnavailable, "database pool not available")
		return
	}
	if err := service.ApproveCompanyStatusSuggestion(r.Context(), h.pool, id, req.ReviewedBy, req.ReviewNote); err != nil {
		slog.Error("approve company status suggestion", "id", id, "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "approved"})
}

func (h *Handlers) handleRejectCompanyStatusSuggestion(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	var req reviewRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if h.pool == nil {
		writeError(w, http.StatusServiceUnavailable, "database pool not available")
		return
	}
	if err := service.RejectCompanyStatusSuggestion(r.Context(), h.pool, id, req.ReviewedBy, req.ReviewNote); err != nil {
		slog.Error("reject company status suggestion", "id", id, "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "rejected"})
}

func (h *Handlers) handleApproveCompanyContactSuggestion(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	var req reviewRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if h.pool == nil {
		writeError(w, http.StatusServiceUnavailable, "database pool not available")
		return
	}
	if err := service.ApproveCompanyContactSuggestion(r.Context(), h.pool, id, req.ReviewedBy, req.ReviewNote); err != nil {
		slog.Error("approve company contact suggestion", "id", id, "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "approved"})
}

func (h *Handlers) handleRejectCompanyContactSuggestion(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	var req reviewRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if h.pool == nil {
		writeError(w, http.StatusServiceUnavailable, "database pool not available")
		return
	}
	if err := service.RejectCompanyContactSuggestion(r.Context(), h.pool, id, req.ReviewedBy, req.ReviewNote); err != nil {
		slog.Error("reject company contact suggestion", "id", id, "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "rejected"})
}

type approveWithSectionsRequest struct {
	ReviewedBy       string `json:"reviewed_by"`
	ReviewNote       string `json:"review_note"`
	ChildSuggestions []struct {
		Table string    `json:"table"`
		ID    uuid.UUID `json:"id"`
	} `json:"child_suggestions"`
}

func (h *Handlers) handleApproveCompanyWithSections(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	var req approveWithSectionsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if h.pool == nil {
		writeError(w, http.StatusServiceUnavailable, "database pool not available")
		return
	}
	children := make([]service.ChildSuggestionRef, 0, len(req.ChildSuggestions))
	for _, c := range req.ChildSuggestions {
		children = append(children, service.ChildSuggestionRef{Table: c.Table, ID: c.ID})
	}
	company, err := service.ApproveCompanyWithSections(r.Context(), h.pool, id, children, req.ReviewedBy, req.ReviewNote)
	if err != nil {
		slog.Error("approve company with sections", "id", id, "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, company)
}
