package httpapi

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	db "github.com/pulsarpoint/corpscout/scheduler/internal/db/gen"
	"github.com/pulsarpoint/corpscout/scheduler/internal/slug"
)

// nilIfBlank returns nil if s is empty or whitespace-only, otherwise a pointer to s.
func nilIfBlank(s string) *string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return &s
}

// ── Organizations ─────────────────────────────────────────────────────────────

func (h *Handlers) handleListOrganizations(w http.ResponseWriter, r *http.Request) {
	page := queryInt(r, "page", 1)
	limit := min(queryInt(r, "limit", 50), 200)
	offset := int32((page - 1) * limit)

	params := db.ListOrganizationsParams{
		Q:      queryString(r, "q"),
		Limit:  int32(limit),
		Offset: offset,
	}
	orgs, err := h.db.ListOrganizations(r.Context(), params)
	if err != nil {
		slog.Error("list organizations", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	total, err := h.db.CountOrganizations(r.Context(), params.Q)
	if err != nil {
		slog.Error("count organizations", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items": orgs, "total": total, "page": page, "limit": limit,
	})
}

func (h *Handlers) handleGetOrganization(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	org, err := h.db.GetOrganizationByID(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "organization not found")
		return
	}
	writeJSON(w, http.StatusOK, org)
}

type createOrganizationRequest struct {
	DisplayName      string `json:"display_name"`
	OrganizationType string `json:"organization_type"`
	Website          string `json:"website"`
	ShortDescription string `json:"short_description"`
	Description      string `json:"description"`
	CountryCode      string `json:"country_code"`
}

func (h *Handlers) handleCreateOrganization(w http.ResponseWriter, r *http.Request) {
	var req createOrganizationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if strings.TrimSpace(req.DisplayName) == "" || strings.TrimSpace(req.OrganizationType) == "" {
		writeError(w, http.StatusBadRequest, "display_name and organization_type are required")
		return
	}

	newID := uuid.New()
	suffix := strings.ReplaceAll(newID.String(), "-", "")[:8]
	canonicalSlug := slug.GenerateWithFallback(req.DisplayName, "org", suffix)

	params := db.InsertOrganizationParams{
		CanonicalSlug:    canonicalSlug,
		DisplayName:      req.DisplayName,
		OrganizationType: req.OrganizationType,
		Website:          nilIfBlank(req.Website),
		ShortDescription: nilIfBlank(req.ShortDescription),
		Description:      nilIfBlank(req.Description),
		CountryCode:      nilIfBlank(req.CountryCode),
		Governance:       json.RawMessage(`{}`),
		Metadata:         json.RawMessage(`{}`),
		Evidence:         json.RawMessage(`{}`),
	}
	org, err := h.db.InsertOrganization(r.Context(), params)
	if err != nil {
		slog.Error("create organization", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusCreated, org)
}

// ── Open-Source Projects ──────────────────────────────────────────────────────

func (h *Handlers) handleListOpenSourceProjects(w http.ResponseWriter, r *http.Request) {
	page := queryInt(r, "page", 1)
	limit := min(queryInt(r, "limit", 50), 200)
	offset := int32((page - 1) * limit)

	params := db.ListOpenSourceProjectsParams{
		Q:      queryString(r, "q"),
		Limit:  int32(limit),
		Offset: offset,
	}
	projects, err := h.db.ListOpenSourceProjects(r.Context(), params)
	if err != nil {
		slog.Error("list open_source_projects", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	total, err := h.db.CountOpenSourceProjects(r.Context(), params.Q)
	if err != nil {
		slog.Error("count open_source_projects", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items": projects, "total": total, "page": page, "limit": limit,
	})
}

func (h *Handlers) handleGetOpenSourceProject(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	project, err := h.db.GetOpenSourceProjectByID(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}
	writeJSON(w, http.StatusOK, project)
}

type createOpenSourceProjectRequest struct {
	DisplayName      string `json:"display_name"`
	Website          string `json:"website"`
	RepositoryURL    string `json:"repository_url"`
	License          string `json:"license"`
	ShortDescription string `json:"short_description"`
	Description      string `json:"description"`
	LifecycleStatus  string `json:"lifecycle_status"`
}

func (h *Handlers) handleCreateOpenSourceProject(w http.ResponseWriter, r *http.Request) {
	var req createOpenSourceProjectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if strings.TrimSpace(req.DisplayName) == "" {
		writeError(w, http.StatusBadRequest, "display_name is required")
		return
	}

	newID := uuid.New()
	suffix := strings.ReplaceAll(newID.String(), "-", "")[:8]
	canonicalSlug := slug.GenerateWithFallback(req.DisplayName, "osp", suffix)

	lifecycle := req.LifecycleStatus
	if lifecycle == "" {
		lifecycle = "active"
	}

	params := db.InsertOpenSourceProjectParams{
		CanonicalSlug:    canonicalSlug,
		DisplayName:      req.DisplayName,
		Website:          nilIfBlank(req.Website),
		RepositoryUrl:    nilIfBlank(req.RepositoryURL),
		License:          nilIfBlank(req.License),
		ShortDescription: nilIfBlank(req.ShortDescription),
		Description:      nilIfBlank(req.Description),
		LifecycleStatus:  lifecycle,
		Metadata:         json.RawMessage(`{}`),
		Evidence:         json.RawMessage(`{}`),
	}
	project, err := h.db.InsertOpenSourceProject(r.Context(), params)
	if err != nil {
		slog.Error("create open_source_project", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusCreated, project)
}
