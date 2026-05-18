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
	pgx "github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	db "github.com/pulsarpoint/corpscout/scheduler/internal/db/gen"
	"github.com/pulsarpoint/corpscout/scheduler/internal/httpapi"
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

type rawInputRetryRecorder struct {
	stubQuerier
	retryGLEIFCalled bool
}

func (r *rawInputRetryRecorder) RetryGLEIFRawInput(ctx context.Context, id uuid.UUID) (uuid.UUID, error) {
	r.retryGLEIFCalled = true
	return r.stubQuerier.RetryGLEIFRawInput(ctx, id)
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

	q.On("GetSourceByName", mock.Anything, "gleif").Return(db.DataSource{
		Name: "gleif",
	}, nil)
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

func TestPatchSource_unknown_field_returns_400(t *testing.T) {
	q := &stubQuerier{}

	r := routerForHandlers(q)

	body := strings.NewReader(`{"crawl_interval_hours": 24}`)
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/sources/gleif", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
}

func TestPatchSource_empty_object_returns_400(t *testing.T) {
	q := &stubQuerier{}

	r := routerForHandlers(q)

	body := strings.NewReader(`{}`)
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/sources/gleif", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
}

func TestPatchSource_enabled_only_missing_source_returns_404_without_update(t *testing.T) {
	q := &sourcePatchWriteRecorder{}

	q.On("GetSourceByName", mock.Anything, "missing").Return(db.DataSource{}, errors.New("not found"))

	r := routerFor(newTestHandlers(q))

	body := strings.NewReader(`{"enabled": false}`)
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/sources/missing", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusNotFound, w.Code)
	require.False(t, q.updateSourceEnabledCalled)
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

	q.On("GetSourceByName", mock.Anything, "gleif").Return(db.DataSource{
		Name: "gleif",
	}, nil)
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

func TestPatchSource_invalid_schedule_kind_returns_422(t *testing.T) {
	q := &stubQuerier{}

	q.On("GetSourceByName", mock.Anything, "gleif").Return(db.DataSource{
		Name:         "gleif",
		ScheduleKind: "interval",
	}, nil)

	r := routerForHandlers(q)

	body := strings.NewReader(`{"schedule_kind": "daily"}`)
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

func TestRetryRawInput_unsupported_source_returns_422(t *testing.T) {
	q := &stubQuerier{}
	id := uuid.New()

	q.On("GetSourceByName", mock.Anything, "nvd_cpe").Return(db.DataSource{
		Name:           "nvd_cpe",
		InputTableName: "cpe_dictionary",
	}, nil)

	r := routerForHandlers(q)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/sources/nvd_cpe/raw-inputs/"+id.String()+"/retry", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusUnprocessableEntity, w.Code)
	q.AssertExpectations(t)
}

func TestRetryRawInput_aiCompanyProfile_returnsOKWithoutRiver(t *testing.T) {
	q := &stubQuerier{}
	id := uuid.New()

	q.On("GetSourceByName", mock.Anything, "ai_company_profile").Return(db.DataSource{
		Name:           "ai_company_profile",
		InputTableName: "ai_company_profile_raw_inputs",
	}, nil)
	q.On("RetryAIRawInput", mock.Anything, id).Return(id, nil)

	r := routerForHandlers(q)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/sources/ai_company_profile/raw-inputs/"+id.String()+"/retry", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.JSONEq(t, `{"status":"retried"}`, w.Body.String())
	q.AssertExpectations(t)
}

func TestIgnoreRawInput_domainDiscovery_returnsOK(t *testing.T) {
	q := &stubQuerier{}
	id := uuid.New()

	q.On("GetSourceByName", mock.Anything, "domain_discovery").Return(db.DataSource{
		Name:           "domain_discovery",
		InputTableName: "domain_discovery_raw_inputs",
	}, nil)
	q.On("IgnoreDomainDiscoveryRawInput", mock.Anything, id).Return(id, nil)

	r := routerForHandlers(q)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/sources/domain_discovery/raw-inputs/"+id.String()+"/ignore", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.JSONEq(t, `{"status":"ignored"}`, w.Body.String())
	q.AssertExpectations(t)
}

func TestRetryRawInput_processorSourceWithNilRiver_returns503BeforeReset(t *testing.T) {
	q := &rawInputRetryRecorder{}
	id := uuid.New()

	q.On("GetSourceByName", mock.Anything, "gleif").Return(db.DataSource{
		Name:           "gleif",
		InputTableName: "gleif_company_raw_inputs",
	}, nil)

	r := routerFor(newTestHandlers(q))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/sources/gleif/raw-inputs/"+id.String()+"/retry", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusServiceUnavailable, w.Code)
	q.AssertExpectations(t)
	require.False(t, q.retryGLEIFCalled)
}

func TestRetryRawInput_processorSourceWithNilPool_returns503BeforeReset(t *testing.T) {
	q := &rawInputRetryRecorder{}
	id := uuid.New()

	q.On("GetSourceByName", mock.Anything, "gleif").Return(db.DataSource{
		Name:           "gleif",
		InputTableName: "gleif_company_raw_inputs",
	}, nil)

	rv, err := river.NewClient[pgx.Tx](riverpgxv5.New(nil), &river.Config{})
	require.NoError(t, err)
	r := routerFor(httpapi.NewHandlers(q, rv, nil, nil, ""))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/sources/gleif/raw-inputs/"+id.String()+"/retry", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusServiceUnavailable, w.Code)
	q.AssertExpectations(t)
	require.False(t, q.retryGLEIFCalled)
}

func TestRetryRawInput_sourceLookupError_returns500(t *testing.T) {
	q := &stubQuerier{}
	id := uuid.New()

	q.On("GetSourceByName", mock.Anything, "gleif").Return(db.DataSource{}, errors.New("database down"))

	r := routerForHandlers(q)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/sources/gleif/raw-inputs/"+id.String()+"/retry", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusInternalServerError, w.Code)
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
