package httpapi_test

import (
	"encoding/json"
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

func TestHandleListOrganizations_Empty(t *testing.T) {
	q := &stubQuerier{}
	q.On("ListOrganizations", mock.Anything, mock.Anything).Return([]db.Organization{}, nil)
	q.On("CountOrganizations", mock.Anything, (*string)(nil)).Return(int64(0), nil)

	h := newTestHandlers(q)
	r := routerFor(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/organizations", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]any
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Equal(t, float64(0), resp["total"])
}

func TestHandleGetOrganization_NotFound(t *testing.T) {
	q := &stubQuerier{}
	q.On("GetOrganizationByID", mock.Anything, mock.AnythingOfType("uuid.UUID")).
		Return(db.Organization{}, errNotFound)

	h := newTestHandlers(q)
	r := routerFor(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/organizations/"+uuid.New().String(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestHandleCreateOrganization_BadBody(t *testing.T) {
	h := newTestHandlers(&stubQuerier{})
	r := routerFor(h)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/organizations", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandleCreateOrganization_MissingRequiredFields(t *testing.T) {
	h := newTestHandlers(&stubQuerier{})
	r := routerFor(h)

	body := `{"organization_type":"foundation"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/organizations", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}
