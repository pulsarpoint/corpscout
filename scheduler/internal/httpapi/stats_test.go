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

func TestGetStats_returns_counts(t *testing.T) {
	q := &stubQuerier{}

	q.On("GetStats", mock.Anything).Return(db.GetStatsRow{
		TotalCompanies:         100,
		TotalDomains:           200,
		ActiveDomains:          150,
		PendingReview:          10,
		EnabledSources:         5,
		PullRunsCompletedToday: 3,
		PullRunsFailedToday:    1,
		RecordsUpserted24h:     5000,
		RecordsUpserted7d:      35000,
	}, nil)

	r := routerForHandlers(q)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/stats", nil)

	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var body map[string]any
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))

	assert.Equal(t, float64(100), body["total_companies"])
	assert.Equal(t, float64(200), body["total_domains"])
	assert.Equal(t, float64(150), body["active_domains"])
	assert.Equal(t, float64(10), body["pending_review"])
	assert.Equal(t, float64(5), body["enabled_sources"])
	assert.Equal(t, float64(3), body["pull_runs_completed_today"])
	assert.Equal(t, float64(1), body["pull_runs_failed_today"])
	assert.Equal(t, float64(5000), body["records_upserted_24h"])
	assert.Equal(t, float64(35000), body["records_upserted_7d"])

	q.AssertExpectations(t)
}
