package httpapi_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	db "github.com/pulsarpoint/corpscout/scheduler/internal/db/gen"
)

func TestListDomains_returns_paginated(t *testing.T) {
	q := &stubQuerier{}

	row := db.ListDomainsRow{Domain: "example.com", CompanyName: "Acme Corp"}
	q.On("ListDomains", mock.Anything, mock.AnythingOfType("db.ListDomainsParams")).
		Return([]db.ListDomainsRow{row}, nil)
	q.On("CountDomains", mock.Anything, mock.AnythingOfType("db.CountDomainsParams")).
		Return(int64(1), nil)

	r := routerForHandlers(q)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/domains", nil)

	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var body map[string]any
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))

	items, ok := body["items"].([]any)
	require.True(t, ok, "items should be a list")
	assert.Len(t, items, 1)

	item := items[0].(map[string]any)
	assert.Equal(t, "example.com", item["domain"])
	assert.Equal(t, "Acme Corp", item["company_name"])

	total, ok := body["total"].(float64)
	require.True(t, ok, "total should be a number")
	assert.Equal(t, float64(1), total)

	q.AssertExpectations(t)
}
