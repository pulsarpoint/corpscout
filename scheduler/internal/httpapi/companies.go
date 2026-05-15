package httpapi

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	db "github.com/pulsarpoint/corpscout/scheduler/internal/db/gen"
)

func (h *Handlers) handleListCompanies(w http.ResponseWriter, r *http.Request) {
	page := queryInt(r, "page", 1)
	limit := min(queryInt(r, "limit", 50), 200)
	offset := int32((page - 1) * limit)

	var countryID pgtype.UUID
	if s := r.URL.Query().Get("country"); s != "" {
		if id, err := uuid.Parse(s); err == nil {
			countryID = pgtype.UUID{Bytes: id, Valid: true}
		}
	}
	var sourceID pgtype.UUID
	if s := r.URL.Query().Get("source"); s != "" {
		if id, err := uuid.Parse(s); err == nil {
			sourceID = pgtype.UUID{Bytes: id, Valid: true}
		}
	}

	params := db.ListCompaniesParams{
		Status:    queryString(r, "status"),
		CountryID: countryID,
		SourceID:  sourceID,
		Q:         queryString(r, "q"),
		Offset:    offset,
		Limit:     int32(limit),
	}

	companies, err := h.db.ListCompanies(r.Context(), params)
	if err != nil {
		slog.Error("list companies", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	total, err := h.db.CountCompanies(r.Context(), db.CountCompaniesParams{
		Status: params.Status, CountryID: params.CountryID,
		SourceID: params.SourceID, Q: params.Q,
	})
	if err != nil {
		slog.Error("count companies", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items": companies, "total": total, "page": page, "limit": limit,
	})
}

func (h *Handlers) handleGetCompany(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid company id")
		return
	}
	company, err := h.db.GetCompany(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "company not found")
		return
	}
	domains, err := h.db.ListDomainsForCompany(r.Context(), id)
	if err != nil {
		slog.Error("list domains for company", "company_id", id, "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"id": company.ID, "lei": company.Lei, "name": company.Name,
		"country_id": company.CountryID, "registration_number": company.RegistrationNumber,
		"status": company.Status, "created_at": company.CreatedAt, "updated_at": company.UpdatedAt,
		"domains": domains,
	})
}
