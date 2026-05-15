package httpapi_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestListJobs_returns_503_when_pool_nil(t *testing.T) {
	q := &stubQuerier{}
	r := routerForHandlers(q) // newTestHandlers inside uses nil pool

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/jobs", nil)

	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
}
