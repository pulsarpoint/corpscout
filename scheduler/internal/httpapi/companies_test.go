package httpapi_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	db "github.com/pulsarpoint/corpscout/scheduler/internal/db/gen"
)

// routerForHandlers returns a chi router with all /api/v1 routes registered.
func routerForHandlers(q *stubQuerier) *chi.Mux {
	h := newTestHandlers(q)
	r := chi.NewRouter()
	h.RegisterRoutes(r)
	return r
}

func TestListCompanies_returns_paginated_json(t *testing.T) {
	q := &stubQuerier{}

	companyID := uuid.New()
	companies := []db.Company{
		{ID: companyID, Name: "Acme Corp", Status: "active"},
	}

	q.On("ListCompanies", mock.Anything, mock.Anything).Return(companies, nil)
	q.On("CountCompanies", mock.Anything, mock.Anything).Return(int64(1), nil)

	r := routerForHandlers(q)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/companies", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))

	items, ok := body["items"].([]any)
	require.True(t, ok, "items should be a list")
	assert.Len(t, items, 1)
	assert.Equal(t, float64(1), body["total"])

	q.AssertExpectations(t)
}

func TestGetCompany_returns_company_with_domains(t *testing.T) {
	q := &stubQuerier{}

	companyID := uuid.New()
	company := db.Company{ID: companyID, Name: "Acme Corp", Status: "active"}
	domains := []db.ListDomainsForCompanyRow{
		{Domain: "acme.com", CompanyID: companyID},
	}

	q.On("GetCompany", mock.Anything, companyID).Return(company, nil)
	q.On("ListDomainsForCompany", mock.Anything, companyID).Return(domains, nil)

	r := routerForHandlers(q)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/companies/"+companyID.String(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))

	assert.Equal(t, companyID.String(), body["id"])
	domainsOut, ok := body["domains"].([]any)
	require.True(t, ok, "domains should be a list")
	assert.Len(t, domainsOut, 1)

	q.AssertExpectations(t)
}

func TestGetCompany_returns_404_for_unknown_id(t *testing.T) {
	q := &stubQuerier{}

	unknownID := uuid.New()
	q.On("GetCompany", mock.Anything, unknownID).Return(db.Company{}, errors.New("not found"))

	r := routerForHandlers(q)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/companies/"+unknownID.String(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)

	q.AssertExpectations(t)
}
