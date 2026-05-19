package httpapi

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/riverqueue/river"

	db "github.com/pulsarpoint/corpscout/scheduler/internal/db/gen"
	"github.com/pulsarpoint/corpscout/scheduler/internal/workers"
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

	params := db.ListCompaniesParams{
		Status:    queryString(r, "status"),
		CountryID: countryID,
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
		Status: params.Status, CountryID: params.CountryID, Q: params.Q,
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
	if domains == nil {
		domains = []db.ListDomainsForCompanyRow{}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"id": company.ID, "lei": company.Lei, "name": company.Name,
		"country_id": company.CountryID, "registration_number": company.RegistrationNumber,
		"status": company.Status, "created_at": company.CreatedAt, "updated_at": company.UpdatedAt,
		"domains": domains,
	})
}

func (h *Handlers) handleGetCompanyEnrichmentSources(w http.ResponseWriter, r *http.Request) {
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

	missing := []string{}
	if company.EmployeeCount == nil {
		missing = append(missing, "employee_count")
	}
	if company.RevenueUsd == nil {
		missing = append(missing, "revenue")
	}
	if company.ProfitUsd == nil {
		missing = append(missing, "profit")
	}

	sources, err := h.db.GetSourcesWithCapabilities(r.Context())
	if err != nil {
		slog.Error("get sources with capabilities", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	type sourceResult struct {
		Name        string   `json:"name"`
		DisplayName *string  `json:"display_name"`
		CanProvide  []string `json:"can_provide"`
	}

	var applicable []sourceResult
	for _, src := range sources {
		// country-specific sources: only match if same country
		if src.CountryID.Valid && src.CountryID.Bytes != [16]byte(company.CountryID) {
			continue
		}
		var overlap []string
		for _, cap := range src.Capabilities {
			for _, m := range missing {
				if cap == m {
					overlap = append(overlap, cap)
					break
				}
			}
		}
		if len(overlap) > 0 {
			applicable = append(applicable, sourceResult{
				Name:        src.Name,
				DisplayName: src.DisplayName,
				CanProvide:  overlap,
			})
		}
	}
	if applicable == nil {
		applicable = []sourceResult{}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"missing_fields": missing,
		"sources":        applicable,
	})
}

func (h *Handlers) handleEnrichCompanyFromSource(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid company id")
		return
	}
	var body struct {
		Source string `json:"source"`
	}
	if err := decodeJSON(r, &body); err != nil || body.Source == "" {
		writeError(w, http.StatusBadRequest, "source is required")
		return
	}

	company, err := h.db.GetCompany(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "company not found")
		return
	}
	if company.RegistrationNumber == nil {
		writeError(w, http.StatusUnprocessableEntity, "company has no registration number")
		return
	}

	riverJob, err := h.rv.Insert(r.Context(), workers.EnrichCompanyFinancialsArgs{
		CompanyID:  id.String(),
		OrgNumber:  *company.RegistrationNumber,
		SourceName: body.Source,
	}, &river.InsertOpts{Queue: "enrich_financials"})
	if err != nil {
		slog.Error("insert enrich job", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to enqueue job")
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"job_id": riverJob.Job.ID})
}

func (h *Handlers) handlePatchCompanyFinancials(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid company id")
		return
	}
	var body struct {
		EmployeeCount       *int32  `json:"employee_count"`
		RevenueUsd          *int64  `json:"revenue_usd"`
		RevenueOrigAmount   *int64  `json:"revenue_orig_amount"`
		RevenueOrigCurrency *string `json:"revenue_orig_currency"`
		ProfitUsd           *int64  `json:"profit_usd"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if body.EmployeeCount != nil && *body.EmployeeCount < 0 {
		writeError(w, http.StatusBadRequest, "employee_count must be >= 0")
		return
	}

	company, err := h.db.UpdateCompanyEnrichment(r.Context(), db.UpdateCompanyEnrichmentParams{
		ID:                  id,
		EmployeeCount:       body.EmployeeCount,
		RevenueUsd:          body.RevenueUsd,
		RevenueOrigAmount:   body.RevenueOrigAmount,
		RevenueOrigCurrency: body.RevenueOrigCurrency,
		ProfitUsd:           body.ProfitUsd,
	})
	if err != nil {
		slog.Error("patch company financials", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, company)
}
