package httpapi_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	db "github.com/pulsarpoint/corpscout/scheduler/internal/db/gen"
	"github.com/pulsarpoint/corpscout/scheduler/internal/httpapi"
)

func TestHandleListCompanySuggestions_ReturnsPendingSuggestions(t *testing.T) {
	q := &stubQuerier{}
	sugID := uuid.New()
	q.On("ListPendingCompanySuggestions", mock.Anything, mock.Anything).
		Return([]db.CompanySuggestion{
			{ID: sugID, ProposedDisplayName: "Test Corp", Status: "pending"},
		}, nil)
	q.On("CountPendingCompanySuggestions", mock.Anything).
		Return(int64(1), nil)

	r := chi.NewRouter()
	httpapi.NewHandlers(q, nil, nil, nil, nil, "").RegisterRoutes(r)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/suggestions/companies", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	items, ok := resp["items"].([]any)
	require.True(t, ok)
	assert.Len(t, items, 1)
}

func TestHandleTriggerSource_NonPullTaskType_Returns422(t *testing.T) {
	q := &stubQuerier{}
	q.On("GetSourceByName", mock.Anything, "ai_company_profile").
		Return(db.DataSource{
			Name:         "ai_company_profile",
			PullTaskType: "ai_company_profile_pull",
			Enabled:      true,
		}, nil)

	r := chi.NewRouter()
	httpapi.NewHandlers(q, nil, nil, nil, nil, "").RegisterRoutes(r)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/sources/ai_company_profile/trigger", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
}

func TestHandleApproveCompanyStatusSuggestion_NilPool_Returns503(t *testing.T) {
	q := &stubQuerier{}
	r := chi.NewRouter()
	httpapi.NewHandlers(q, nil, nil, nil, nil, "").RegisterRoutes(r)

	body := strings.NewReader(`{"reviewed_by":"admin","review_note":"ok"}`)
	req := httptest.NewRequest(http.MethodPost,
		"/api/v1/suggestions/company-status/"+uuid.New().String()+"/approve", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestHandleApproveCompanyWithSections_NilPool_Returns503(t *testing.T) {
	q := &stubQuerier{}
	r := chi.NewRouter()
	httpapi.NewHandlers(q, nil, nil, nil, nil, "").RegisterRoutes(r)

	body := strings.NewReader(`{
		"reviewed_by":"admin",
		"child_suggestions":[
			{"table":"company_status_suggestions","id":"` + uuid.New().String() + `"}
		]
	}`)
	req := httptest.NewRequest(http.MethodPost,
		"/api/v1/suggestions/companies/"+uuid.New().String()+"/approve-with-sections", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}
