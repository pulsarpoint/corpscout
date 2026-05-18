package httpapi_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	db "github.com/pulsarpoint/corpscout/scheduler/internal/db/gen"
)

func TestHandleListReview_filters_by_signal(t *testing.T) {
	q := &stubQuerier{}
	signal := "certsh"
	status := "needs_review"
	q.On("ListDomains", mock.Anything, db.ListDomainsParams{
		Status: &status,
		Signal: &signal,
		Offset: 0,
		Limit:  50,
	}).Return([]db.ListDomainsRow{{Domain: "example.com", CompanyName: "Acme"}}, nil)
	q.On("CountDomains", mock.Anything, db.CountDomainsParams{
		Status: &status,
		Signal: &signal,
	}).Return(int64(1), nil)

	r := routerForHandlers(q)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/review?signal=certsh", nil)
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var body map[string]any
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
	assert.Equal(t, float64(1), body["total"])
	q.AssertExpectations(t)
}

func TestHandleBulkReview_approves_multiple(t *testing.T) {
	q := &stubQuerier{}
	q.On("ReviewCompanyDomain", mock.Anything, mock.MatchedBy(func(p db.ReviewCompanyDomainParams) bool {
		return p.Status == "active"
	})).Return(nil).Times(2)

	r := routerForHandlers(q)
	body := map[string]any{
		"ids":    []string{"00000000-0000-0000-0000-000000000001", "00000000-0000-0000-0000-000000000002"},
		"action": "approved",
	}
	b, _ := json.Marshal(body)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/review/bulk", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var resp map[string]any
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
	assert.Equal(t, float64(2), resp["updated"])
	q.AssertExpectations(t)
}

func TestHandleBulkReview_rejects_invalid_action(t *testing.T) {
	q := &stubQuerier{}
	r := routerForHandlers(q)
	body := map[string]any{"ids": []string{"00000000-0000-0000-0000-000000000001"}, "action": "bogus"}
	b, _ := json.Marshal(body)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/review/bulk", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}
