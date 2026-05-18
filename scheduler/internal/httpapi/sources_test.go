package httpapi_test

import (
	"context"
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

type sourceConfigStubQuerier struct {
	stubQuerier
	updateSourceConfigParams *db.UpdateSourceConfigParams
}

func (s *sourceConfigStubQuerier) UpdateSourceConfig(_ context.Context, arg db.UpdateSourceConfigParams) error {
	s.updateSourceConfigParams = &arg
	return nil
}

type sourcePatchWriteRecorder struct {
	stubQuerier
	updateSourceEnabledCalled bool
}

func (s *sourcePatchWriteRecorder) UpdateSourceEnabled(_ context.Context, _ db.UpdateSourceEnabledParams) error {
	s.updateSourceEnabledCalled = true
	return nil
}

func TestListSources_returns_all(t *testing.T) {
	q := &stubQuerier{}

	sources := []db.DataSource{
		{ID: uuid.New(), Name: "gleif", Enabled: true},
	}

	q.On("ListSources", mock.Anything).Return(sources, nil)

	r := routerFor(newTestHandlers(q))

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

	r := routerFor(newTestHandlers(q))

	body := strings.NewReader(`{"enabled": false}`)
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/sources/gleif", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	q.AssertExpectations(t)
}

func TestPatchSource_updates_schedule(t *testing.T) {
	q := &stubQuerier{}

	expr := "24h"
	q.On("GetSourceByName", mock.Anything, "gleif").Return(db.DataSource{
		Name:         "gleif",
		ScheduleKind: "interval",
	}, nil)
	q.On("UpdateSourceSchedule", mock.Anything, db.UpdateSourceScheduleParams{
		Name:               "gleif",
		ScheduleKind:       "interval",
		ScheduleExpression: &expr,
	}).Return(nil)

	r := routerForHandlers(q)

	body := strings.NewReader(`{"schedule_expression": "24h"}`)
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/sources/gleif", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	q.AssertExpectations(t)
}

func TestPatchSource_updates_schedule_enabled(t *testing.T) {
	q := &stubQuerier{}

	q.On("UpdateSourceScheduleEnabled", mock.Anything, db.UpdateSourceScheduleEnabledParams{
		Name: "gleif", ScheduleEnabled: false,
	}).Return(nil)

	r := routerForHandlers(q)

	body := strings.NewReader(`{"schedule_enabled": false}`)
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/sources/gleif", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	q.AssertExpectations(t)
}

func TestPatchSource_invalid_schedule_expression_returns_422(t *testing.T) {
	q := &stubQuerier{}

	q.On("GetSourceByName", mock.Anything, "gleif").Return(db.DataSource{
		Name:         "gleif",
		ScheduleKind: "interval",
	}, nil)

	r := routerForHandlers(q)

	body := strings.NewReader(`{"schedule_expression": "daily"}`)
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/sources/gleif", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusUnprocessableEntity, w.Code)
	q.AssertExpectations(t)
}

func TestPatchSource_non_positive_schedule_expression_returns_422(t *testing.T) {
	for _, expr := range []string{"0s", "-1h"} {
		t.Run(expr, func(t *testing.T) {
			q := &stubQuerier{}

			q.On("GetSourceByName", mock.Anything, "gleif").Return(db.DataSource{
				Name:         "gleif",
				ScheduleKind: "interval",
			}, nil)
			q.On("UpdateSourceSchedule", mock.Anything, mock.Anything).Return(nil)

			r := routerForHandlers(q)

			body := strings.NewReader(`{"schedule_expression": "` + expr + `"}`)
			req := httptest.NewRequest(http.MethodPatch, "/api/v1/sources/gleif", body)
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			require.Equal(t, http.StatusUnprocessableEntity, w.Code)
		})
	}
}

func TestPatchSource_validates_all_fields_before_writing(t *testing.T) {
	q := &sourcePatchWriteRecorder{}

	q.On("GetSourceByName", mock.Anything, "gleif").Return(db.DataSource{
		Name:   "gleif",
		Config: json.RawMessage(`{}`),
	}, nil)

	r := routerFor(newTestHandlers(q))

	body := strings.NewReader(`{"enabled": false, "config": {"api_token": "secret"}}`)
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/sources/gleif", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusUnprocessableEntity, w.Code)
	require.False(t, q.updateSourceEnabledCalled)
}

func TestPatchSource_config_merge_preserves_json_numeric_type(t *testing.T) {
	q := &sourceConfigStubQuerier{}

	q.On("GetSourceByName", mock.Anything, "gleif").Return(db.DataSource{
		Name:   "gleif",
		Config: json.RawMessage(`{"limit":10,"nested":{"threshold":0.5},"unchanged":true}`),
	}, nil)

	r := routerFor(newTestHandlers(q))

	body := strings.NewReader(`{"config":{"limit":25,"nested":{"threshold":0.75}}}`)
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/sources/gleif", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.NotNil(t, q.updateSourceConfigParams)
	require.Equal(t, "gleif", q.updateSourceConfigParams.Name)
	require.JSONEq(t, `{"limit":25,"nested":{"threshold":0.75},"unchanged":true}`, string(q.updateSourceConfigParams.Config))
	require.Equal(t, `{"limit":25,"nested":{"threshold":0.75},"unchanged":true}`, string(q.updateSourceConfigParams.Config))
	q.AssertExpectations(t)
}

func TestPatchSource_config_secret_key_returns_422(t *testing.T) {
	q := &stubQuerier{}

	q.On("GetSourceByName", mock.Anything, "gleif").Return(db.DataSource{
		Name:   "gleif",
		Config: json.RawMessage(`{}`),
	}, nil)

	r := routerForHandlers(q)

	body := strings.NewReader(`{"config":{"api_token":"secret"}}`)
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/sources/gleif", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusUnprocessableEntity, w.Code)
}

func TestPatchSource_config_secret_key_inside_array_returns_422(t *testing.T) {
	q := &sourceConfigStubQuerier{}

	q.On("GetSourceByName", mock.Anything, "gleif").Return(db.DataSource{
		Name:   "gleif",
		Config: json.RawMessage(`{}`),
	}, nil)

	r := routerFor(newTestHandlers(q))

	body := strings.NewReader(`{"config":{"providers":[{"api_token":"secret"}]}}`)
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/sources/gleif", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusUnprocessableEntity, w.Code)
	require.Nil(t, q.updateSourceConfigParams)
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
