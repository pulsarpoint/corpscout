package httpapi_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/pashagolub/pgxmock/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulsarpoint/corpscout/scheduler/internal/httpapi"
)

func TestListRawInputs_includesCVRAndAriregisterRows(t *testing.T) {
	pool := newSQLContainsMock(t)
	defer pool.Close()

	createdAt := time.Date(2026, 5, 22, 10, 0, 0, 0, time.UTC)
	pool.ExpectQuery("COUNT(*) FROM;;companies_house_company_raw_inputs;;brreg_company_raw_inputs;;cvr_company_raw_inputs;;ariregister_company_raw_inputs;;company_name ILIKE;;organization_name ILIKE;;legal_name ILIKE").
		WithArgs("%Registry%").
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(int64(2)))
	pool.ExpectQuery("SELECT id, source, name, native_id, status, translation_status, created_at;;companies_house_company_raw_inputs;;brreg_company_raw_inputs;;cvr_company_raw_inputs;;ariregister_company_raw_inputs;;company_name ILIKE;;organization_name ILIKE;;legal_name ILIKE").
		WithArgs("%Registry%", 50, 0).
		WillReturnRows(pgxmock.NewRows([]string{"id", "source", "name", "native_id", "status", "translation_status", "created_at"}).
			AddRow("cvr-id", "cvr", "Danish Registry ApS", "12345678", "pending", "translated", createdAt).
			AddRow("ari-id", "ariregister", "Estonian Registry OU", "87654321", "pending", "failed", createdAt))

	r := routerFor(httpapi.NewHandlers(&stubQuerier{}, nil, pool, nil, nil, "", nil, ""))
	req := httptest.NewRequest(http.MethodGet, "/api/v1/raw-inputs?q=Registry", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var body struct {
		Items []struct {
			Source            string  `json:"source"`
			Name              string  `json:"name"`
			NativeID          string  `json:"native_id"`
			TranslationStatus *string `json:"translation_status"`
		} `json:"items"`
		Total int64 `json:"total"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.Equal(t, int64(2), body.Total)
	require.Len(t, body.Items, 2)
	assert.Equal(t, "cvr", body.Items[0].Source)
	assert.Equal(t, "Danish Registry ApS", body.Items[0].Name)
	assert.Equal(t, "12345678", body.Items[0].NativeID)
	require.NotNil(t, body.Items[0].TranslationStatus)
	assert.Equal(t, "translated", *body.Items[0].TranslationStatus)
	assert.Equal(t, "ariregister", body.Items[1].Source)
	assert.Equal(t, "Estonian Registry OU", body.Items[1].Name)
	assert.Equal(t, "87654321", body.Items[1].NativeID)
	require.NotNil(t, body.Items[1].TranslationStatus)
	assert.Equal(t, "failed", *body.Items[1].TranslationStatus)
	require.NoError(t, pool.ExpectationsWereMet())
}

func TestListRawInputs_translationStatusFiltersTranslatedSourcesOnly(t *testing.T) {
	pool := newSQLContainsMock(t)
	defer pool.Close()

	createdAt := time.Date(2026, 5, 22, 10, 0, 0, 0, time.UTC)
	pool.ExpectQuery("COUNT(*) FROM;;brreg_company_raw_inputs;;cvr_company_raw_inputs;;ariregister_company_raw_inputs;;translation_status =;;!companies_house_company_raw_inputs").
		WithArgs("translated").
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(int64(3)))
	pool.ExpectQuery("SELECT id, source, name, native_id, status, translation_status, created_at;;brreg_company_raw_inputs;;cvr_company_raw_inputs;;ariregister_company_raw_inputs;;translation_status =;;!companies_house_company_raw_inputs").
		WithArgs("translated", 50, 0).
		WillReturnRows(pgxmock.NewRows([]string{"id", "source", "name", "native_id", "status", "translation_status", "created_at"}).
			AddRow("brreg-id", "brreg", "Norway AS", "991234567", "pending", "translated", createdAt).
			AddRow("cvr-id", "cvr", "Denmark ApS", "12345678", "pending", "translated", createdAt).
			AddRow("ari-id", "ariregister", "Estonia OU", "87654321", "pending", "translated", createdAt))

	r := routerFor(httpapi.NewHandlers(&stubQuerier{}, nil, pool, nil, nil, "", nil, ""))
	req := httptest.NewRequest(http.MethodGet, "/api/v1/raw-inputs?translation_status=translated", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var body struct {
		Items []struct {
			Source string `json:"source"`
		} `json:"items"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.Len(t, body.Items, 3)
	assert.Equal(t, []string{"brreg", "cvr", "ariregister"}, []string{body.Items[0].Source, body.Items[1].Source, body.Items[2].Source})
	require.NoError(t, pool.ExpectationsWereMet())
}

func TestGetRawInput_includesTranslatedPayloadAndMetadata(t *testing.T) {
	tests := []struct {
		name       string
		source     string
		tableName  string
		rowName    string
		nativeID   string
		translated string
	}{
		{
			name:       "cvr",
			source:     "cvr",
			tableName:  "cvr_company_raw_inputs",
			rowName:    "Dansk Selskab ApS",
			nativeID:   "12345678",
			translated: `{"company_name":"Danish Company ApS"}`,
		},
		{
			name:       "ariregister",
			source:     "ariregister",
			tableName:  "ariregister_company_raw_inputs",
			rowName:    "Eesti OU",
			nativeID:   "87654321",
			translated: `{"legal_name":"Estonian OU"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool := newSQLContainsMock(t)
			defer pool.Close()

			now := time.Date(2026, 5, 22, 10, 0, 0, 0, time.UTC)
			pool.ExpectQuery(tt.tableName + ";;raw_payload_en;;translation_status;;translation_model;;translation_prompt_version;;translation_fx_rate_date").
				WithArgs("raw-id").
				WillReturnRows(pgxmock.NewRows([]string{
					"id", "source", "name", "native_id", "processing_status", "company_type", "registration_status", "website", "country_iso2",
					"run_id", "processing_attempts", "processing_error", "payload_hash", "raw_payload", "raw_payload_en",
					"translation_status", "translation_attempts", "translation_error", "translation_model", "translation_prompt_version",
					"translation_fx_source", "translation_fx_rate_date", "translated_at", "first_seen_at", "last_seen_at", "processed_at", "created_at", "updated_at",
				}).AddRow(
					"raw-id", tt.source, tt.rowName, tt.nativeID, "pending", "", "active", "https://example.com", "DK",
					"run-1", 1, "", "hash", []byte(`{"source":"raw"}`), []byte(tt.translated),
					"translated", 2, "", "qwen3:6b", "v1", "exchangerate.host", "2026-05-21", now,
					now, now, nil, now, now,
				))

			r := routerFor(httpapi.NewHandlers(&stubQuerier{}, nil, pool, nil, nil, "", nil, ""))
			req := httptest.NewRequest(http.MethodGet, "/api/v1/raw-inputs/"+tt.source+"/raw-id", nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			require.Equal(t, http.StatusOK, w.Code)
			var body map[string]any
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
			assert.Equal(t, tt.source, body["source"])
			assert.Equal(t, tt.rowName, body["name"])
			assert.Equal(t, tt.nativeID, body["native_id"])
			assert.Equal(t, "translated", body["translation_status"])
			assert.Equal(t, "qwen3:6b", body["translation_model"])
			assert.Equal(t, "v1", body["translation_prompt_version"])
			assert.Equal(t, "2026-05-21", body["translation_fx_rate_date"])
			assert.NotNil(t, body["raw_payload_en"])
			require.NoError(t, pool.ExpectationsWereMet())
		})
	}
}

func TestGetRawInput_unsupportedSourceReturnsSafeClientError(t *testing.T) {
	pool := newSQLContainsMock(t)
	defer pool.Close()

	r := routerFor(httpapi.NewHandlers(&stubQuerier{}, nil, pool, nil, nil, "", nil, ""))
	req := httptest.NewRequest(http.MethodGet, "/api/v1/raw-inputs/unsupported/raw-id", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
	require.NotContains(t, strings.ToLower(w.Body.String()), "stack")
	require.NotContains(t, strings.ToLower(w.Body.String()), "select")
}

func newSQLContainsMock(t *testing.T) pgxmock.PgxPoolIface {
	t.Helper()
	pool, err := pgxmock.NewPool(pgxmock.QueryMatcherOption(pgxmock.QueryMatcherFunc(matchSQLContains)))
	require.NoError(t, err)
	return pool
}

func matchSQLContains(expectedSQL, actualSQL string) error {
	normalized := strings.Join(strings.Fields(actualSQL), " ")
	for _, token := range strings.Split(expectedSQL, ";;") {
		token = strings.TrimSpace(token)
		if token == "" {
			continue
		}
		if strings.HasPrefix(token, "!") {
			forbidden := strings.TrimPrefix(token, "!")
			if strings.Contains(normalized, forbidden) {
				return fmt.Errorf("actual sql contains forbidden token %q: %s", forbidden, normalized)
			}
			continue
		}
		if !strings.Contains(normalized, token) {
			return fmt.Errorf("actual sql missing token %q: %s", token, normalized)
		}
	}
	return nil
}
