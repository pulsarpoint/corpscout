package httpapi

import (
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	db "github.com/pulsarpoint/corpscout/scheduler/internal/db/gen"
)

func (h *Handlers) handleListDomains(w http.ResponseWriter, r *http.Request) {
	page := queryInt(r, "page", 1)
	limit := min(queryInt(r, "limit", 50), 200)
	offset := int32((page - 1) * limit)

	var minConf *int16
	if s := r.URL.Query().Get("min_confidence"); s != "" {
		if n, err := strconv.Atoi(s); err == nil {
			v := int16(n)
			minConf = &v
		}
	}

	params := db.ListDomainsParams{
		Status:        queryString(r, "status"),
		Signal:        queryString(r, "signal"),
		Q:             queryString(r, "q"),
		MinConfidence: minConf,
		Offset:        offset,
		Limit:         int32(limit),
	}

	domains, err := h.db.ListDomains(r.Context(), params)
	if err != nil {
		slog.Error("list domains", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	total, err := h.db.CountDomains(r.Context(), db.CountDomainsParams{
		Status: params.Status, Signal: params.Signal, Q: params.Q, MinConfidence: params.MinConfidence,
	})
	if err != nil {
		slog.Error("count domains", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items": domains, "total": total, "page": page, "limit": limit,
	})
}

func (h *Handlers) handleGetDomain(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid domain id")
		return
	}
	domain, err := h.db.GetDomainByID(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "domain not found")
		return
	}
	writeJSON(w, http.StatusOK, domain)
}
