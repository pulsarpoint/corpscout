package httpapi

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	db "github.com/pulsarpoint/corpscout/scheduler/internal/db/gen"
)

// reviewCandidateItem is a JSON-safe version of ListCandidatesForReviewRow.
// sqlc emits Evidence as []byte (base64 in JSON) for JOIN queries; this type
// casts it to json.RawMessage so the frontend receives a proper JSON object.
type reviewCandidateItem struct {
	ID               uuid.UUID       `json:"id"`
	CompanyID        uuid.UUID       `json:"company_id"`
	DomainID         uuid.UUID       `json:"domain_id"`
	RelationshipType string          `json:"relationship_type"`
	Status           string          `json:"status"`
	Signal           string          `json:"signal"`
	Confidence       int16           `json:"confidence"`
	Evidence         json.RawMessage `json:"evidence"`
	FirstSeenAt      time.Time       `json:"first_seen_at"`
	LastSeenAt       time.Time       `json:"last_seen_at"`
	CompanyName      string          `json:"company_name"`
	Domain           string          `json:"domain"`
}

func toReviewItem(r db.ListCandidatesForReviewRow) reviewCandidateItem {
	ev := json.RawMessage(r.Evidence)
	if len(ev) == 0 {
		ev = json.RawMessage("null")
	}
	return reviewCandidateItem{
		ID:               r.ID,
		CompanyID:        r.CompanyID,
		DomainID:         r.DomainID,
		RelationshipType: r.RelationshipType,
		Status:           r.Status,
		Signal:           r.Signal,
		Confidence:       r.Confidence,
		Evidence:         ev,
		FirstSeenAt:      r.FirstSeenAt,
		LastSeenAt:       r.LastSeenAt,
		CompanyName:      r.CompanyName,
		Domain:           r.Domain,
	}
}

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
	total, err := h.db.CountCandidatesForReview(r.Context())
	if err != nil {
		slog.Error("count candidates for review", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	out := make([]reviewCandidateItem, len(items))
	for i, row := range items {
		out[i] = toReviewItem(row)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items": out, "page": page, "limit": limit, "total": total,
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
