package httpapi_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	db "github.com/pulsarpoint/corpscout/scheduler/internal/db/gen"
)

func TestListSources_returns_all(t *testing.T) {
	q := &stubQuerier{}

	sources := []db.DataSource{
		{ID: uuid.New(), Name: "gleif", AdapterType: "api", Enabled: true, CrawlIntervalHours: 24},
	}

	q.On("ListSources", mock.Anything).Return(sources, nil)

	r := routerForHandlers(q)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/sources", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var body []any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Len(t, body, 1)

	q.AssertExpectations(t)
}

func TestPatchSource_updates_enabled(t *testing.T) {
	q := &stubQuerier{}

	q.On("UpdateSourceEnabled", mock.Anything, db.UpdateSourceEnabledParams{
		Name: "gleif", Enabled: false,
	}).Return(nil)

	r := routerForHandlers(q)

	body := strings.NewReader(`{"enabled": false}`)
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/sources/gleif", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	q.AssertExpectations(t)
}

func TestPatchSource_updates_interval(t *testing.T) {
	q := &stubQuerier{}

	q.On("UpdateSourceInterval", mock.Anything, db.UpdateSourceIntervalParams{
		Name: "gleif", CrawlIntervalHours: 72,
	}).Return(nil)

	r := routerForHandlers(q)

	body := strings.NewReader(`{"crawl_interval_hours": 72}`)
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/sources/gleif", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	q.AssertExpectations(t)
}

func TestTriggerSource_returns_404_for_unknown(t *testing.T) {
	q := &stubQuerier{}

	q.On("GetSourceByName", mock.Anything, "unknown").Return(db.DataSource{}, errors.New("not found"))

	r := routerForHandlers(q)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/sources/unknown/trigger", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusNotFound, w.Code)
	q.AssertExpectations(t)
}
