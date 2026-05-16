package httpapi_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandleResolve_MissingBody(t *testing.T) {
	h := newTestHandlers(&stubQuerier{})
	r := routerFor(h)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/resolve", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandleResolve_EmptyRequest_NoPool(t *testing.T) {
	// When pool is nil the handler returns 503.
	h := newTestHandlers(&stubQuerier{})
	r := routerFor(h)

	body := `{"name":"Apache"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/resolve", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestHandleResolve_EmptyPayload_ReturnsBadRequest(t *testing.T) {
	h := newTestHandlers(&stubQuerier{})
	r := routerFor(h)

	body := `{}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/resolve", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var resp map[string]string
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Contains(t, resp["error"], "at least one")
}
