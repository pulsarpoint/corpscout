package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type resolveRequest struct {
	Name            string   `json:"name"`
	Website         string   `json:"website"`
	CPEVendorTokens []string `json:"cpe_vendor_tokens"`
}

type resolveResponse struct {
	Matched          bool   `json:"matched"`
	EntityType       string `json:"entity_type,omitempty"`
	EntityID         string `json:"entity_id,omitempty"`
	DisplayName      string `json:"display_name,omitempty"`
	CanonicalSlug    string `json:"canonical_slug,omitempty"`
	Website          string `json:"website,omitempty"`
	ResolutionReason string `json:"resolution_reason,omitempty"`
	Status           string `json:"status,omitempty"`
}

type resolvedRow struct {
	EntityType    string
	EntityID      uuid.UUID
	DisplayName   string
	CanonicalSlug string
	Website       *string
	Status        string
	UpdatedAt     time.Time
}

func (h *Handlers) handleResolve(w http.ResponseWriter, r *http.Request) {
	var req resolveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Name == "" && req.Website == "" && len(req.CPEVendorTokens) == 0 {
		writeError(w, http.StatusBadRequest, "provide at least one of: name, website, cpe_vendor_tokens")
		return
	}

	if h.pool == nil {
		writeError(w, http.StatusServiceUnavailable, "database not available")
		return
	}

	ctx := r.Context()

	// Priority 1: CPE vendor token approved link.
	for _, token := range req.CPEVendorTokens {
		row, err := h.resolverCPELookup(ctx, token)
		if err != nil {
			slog.Error("resolver cpe lookup", "token", token, "error", err)
			continue
		}
		if row != nil {
			writeJSON(w, http.StatusOK, h.toResolveResponse(row, "matched approved CPE vendor token"))
			return
		}
	}

	// Priority 2: website exact match.
	if req.Website != "" {
		row, err := h.resolverWebsiteLookup(ctx, req.Website)
		if err != nil {
			slog.Error("resolver website lookup", "website", req.Website, "error", err)
		} else if row != nil {
			writeJSON(w, http.StatusOK, h.toResolveResponse(row, "matched website"))
			return
		}
	}

	// Priority 3: display name ILIKE.
	if req.Name != "" {
		row, err := h.resolverNameLookup(ctx, req.Name)
		if err != nil {
			slog.Error("resolver name lookup", "name", req.Name, "error", err)
		} else if row != nil {
			writeJSON(w, http.StatusOK, h.toResolveResponse(row, "matched display name"))
			return
		}
	}

	writeJSON(w, http.StatusOK, resolveResponse{Matched: false, Status: "no_match"})
}

func (h *Handlers) toResolveResponse(row *resolvedRow, reason string) resolveResponse {
	resp := resolveResponse{
		Matched:          true,
		EntityType:       row.EntityType,
		EntityID:         row.EntityID.String(),
		DisplayName:      row.DisplayName,
		CanonicalSlug:    row.CanonicalSlug,
		ResolutionReason: reason,
	}
	if row.Website != nil {
		resp.Website = *row.Website
	}
	return resp
}

// resolverCPELookup joins cpe_entity_links against v_resolved_entities.
func (h *Handlers) resolverCPELookup(ctx context.Context, token string) (*resolvedRow, error) {
	row := h.pool.QueryRow(ctx, `
		SELECT v.entity_type, v.entity_id, v.display_name, v.canonical_slug, v.website, v.status, v.updated_at
		FROM cpe_entity_links l
		JOIN v_resolved_entities v
		  ON v.entity_type = l.entity_type
		 AND v.entity_id = COALESCE(l.company_id, l.organization_id, l.open_source_project_id)
		WHERE l.cpe_vendor_token = $1
		  AND l.removed_at IS NULL
		LIMIT 1
	`, token)
	return scanResolvedRow(row)
}

// resolverWebsiteLookup searches v_resolved_entities by exact website.
func (h *Handlers) resolverWebsiteLookup(ctx context.Context, website string) (*resolvedRow, error) {
	row := h.pool.QueryRow(ctx, `
		SELECT entity_type, entity_id, display_name, canonical_slug, website, status, updated_at
		FROM v_resolved_entities
		WHERE lower(website) = lower($1)
		ORDER BY entity_type, display_name
		LIMIT 1
	`, website)
	return scanResolvedRow(row)
}

// resolverNameLookup searches v_resolved_entities by ILIKE on display_name.
func (h *Handlers) resolverNameLookup(ctx context.Context, name string) (*resolvedRow, error) {
	row := h.pool.QueryRow(ctx, `
		SELECT entity_type, entity_id, display_name, canonical_slug, website, status, updated_at
		FROM v_resolved_entities
		WHERE lower(display_name) = lower($1)
		ORDER BY entity_type, display_name
		LIMIT 1
	`, name)
	return scanResolvedRow(row)
}

func scanResolvedRow(row pgx.Row) (*resolvedRow, error) {
	var r resolvedRow
	err := row.Scan(&r.EntityType, &r.EntityID, &r.DisplayName, &r.CanonicalSlug, &r.Website, &r.Status, &r.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &r, nil
}
