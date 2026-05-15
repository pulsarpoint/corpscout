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

	items, err := h.db.ListCandidatesForReview(r.Context(), db.ListCandidatesForReviewParams{
		Limit:  int32(limit),
		Offset: offset,
	})
	if err != nil {
		slog.Error("list candidates for review", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if items == nil {
		items = []db.ListCandidatesForReviewRow{}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items": items, "page": page, "limit": limit,
	})
}

type createReviewRequest struct {
	Action     string  `json:"action"`
	ReviewedBy string  `json:"reviewed_by"`
	ReviewNote *string `json:"review_note"`
}

func (h *Handlers) handleCreateReview(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}

	var req createReviewRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.ReviewedBy == "" {
		writeError(w, http.StatusBadRequest, "reviewed_by is required")
		return
	}

	var status, relType string
	switch req.Action {
	case "approved":
		status, relType = "active", "official_site"
	case "rejected":
		status, relType = "rejected", "candidate"
	case "superseded":
		status, relType = "superseded", "candidate"
	default:
		writeError(w, http.StatusBadRequest, "action must be approved, rejected, or superseded")
		return
	}

	review, err := h.db.CreateDomainReviewAndUpdateStatus(r.Context(), db.CreateDomainReviewAndUpdateStatusParams{
		CompanyDomainID:  id,
		Action:           req.Action,
		ReviewedBy:       req.ReviewedBy,
		ReviewNote:       req.ReviewNote,
		Status:           status,
		RelationshipType: relType,
	})
	if err != nil {
		slog.Error("create domain review", "company_domain_id", id, "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusOK, review)
}
