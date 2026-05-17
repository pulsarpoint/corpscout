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

func TestListPullRuns_returns_items(t *testing.T) {
	q := &stubQuerier{}

	q.On("ListPullRuns", mock.Anything, db.ListPullRunsParams{Column1: "", Limit: 20, Offset: 0}).Return(
		[]db.ListPullRunsRow{
			{SourceName: "brreg", Status: "completed"},
		},
		nil,
	)

	r := routerForHandlers(q)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/pull-runs", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var body map[string]any
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
	items := body["items"].([]any)
	assert.Len(t, items, 1)
	q.AssertExpectations(t)
}

func TestListPullRuns_empty_returns_empty_slice(t *testing.T) {
	q := &stubQuerier{}

	q.On("ListPullRuns", mock.Anything, db.ListPullRunsParams{Column1: "", Limit: 20, Offset: 0}).Return(
		[]db.ListPullRunsRow{},
		nil,
	)

	r := routerForHandlers(q)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/pull-runs", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var body map[string]any
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
	items := body["items"].([]any)
	assert.Len(t, items, 0)
	q.AssertExpectations(t)
}
