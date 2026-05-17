package httpapi

import "net/http"

func (h *Handlers) handleListCompanySuggestions(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"items": []any{}, "page": 1, "limit": 20})
}

func (h *Handlers) handleGetCompanySuggestion(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotImplemented, "not implemented")
}

func (h *Handlers) handleApproveCompanySuggestion(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotImplemented, "not implemented")
}

func (h *Handlers) handleRejectCompanySuggestion(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotImplemented, "not implemented")
}

func (h *Handlers) handleApproveCompanyWithSections(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotImplemented, "not implemented")
}

func (h *Handlers) handleApproveCompanyStatusSuggestion(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotImplemented, "not implemented")
}

func (h *Handlers) handleRejectCompanyStatusSuggestion(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotImplemented, "not implemented")
}

func (h *Handlers) handleApproveCompanyContactSuggestion(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotImplemented, "not implemented")
}

func (h *Handlers) handleRejectCompanyContactSuggestion(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotImplemented, "not implemented")
}
