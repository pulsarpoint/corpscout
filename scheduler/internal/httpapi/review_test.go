package httpapi_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	db "github.com/pulsarpoint/corpscout/scheduler/internal/db/gen"
)

func TestHandleListReview(t *testing.T) {
	domainID := uuid.New()
	companyID := uuid.New()
	rows := []db.ListCandidatesForReviewRow{
		{
			ID:               uuid.New(),
			CompanyID:        companyID,
			DomainID:         domainID,
			RelationshipType: "candidate",
			Status:           "needs_review",
			Signal:           "search",
			Confidence:       60,
			CompanyName:      "Acme Ltd",
			Domain:           "acme.co.uk",
			FirstSeenAt:      time.Now(),
			LastSeenAt:       time.Now(),
		},
	}

	q := &stubQuerier{}
	q.On("ListCandidatesForReview", mock.Anything, db.ListCandidatesForReviewParams{
		Limit: 50, Offset: 0,
	}).Return(rows, nil)

	r := chi.NewRouter()
	newTestHandlers(q).RegisterRoutes(r)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/review", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	items := resp["items"].([]any)
	assert.Len(t, items, 1)
	assert.Equal(t, float64(1), resp["page"])
	assert.Equal(t, float64(50), resp["limit"])
	q.AssertExpectations(t)
}

func TestHandleCreateReview_Approved(t *testing.T) {
	cdID := uuid.New()
	note := "looks good"
	review := db.CompanyDomainReview{
		ID:              uuid.New(),
		CompanyDomainID: cdID,
		Action:          "approved",
		ReviewedBy:      "alice",
		ReviewNote:      &note,
		CreatedAt:       time.Now(),
	}

	q := &stubQuerier{}
	q.On("CreateDomainReview", mock.Anything, db.CreateDomainReviewParams{
		CompanyDomainID: cdID,
		Action:          "approved",
		ReviewedBy:      "alice",
		ReviewNote:      &note,
	}).Return(review, nil)
	q.On("UpdateCompanyDomainStatus", mock.Anything, db.UpdateCompanyDomainStatusParams{
		ID:               cdID,
		Status:           "active",
		RelationshipType: "official_site",
	}).Return(nil)

	r := chi.NewRouter()
	newTestHandlers(q).RegisterRoutes(r)

	body, _ := json.Marshal(map[string]any{
		"action":      "approved",
		"reviewed_by": "alice",
		"review_note": note,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/review/"+cdID.String()+"/reviews",
		bytes.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var resp db.CompanyDomainReview
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "approved", resp.Action)
	q.AssertExpectations(t)
}

func TestHandleCreateReview_Rejected(t *testing.T) {
	cdID := uuid.New()
	review := db.CompanyDomainReview{
		ID:              uuid.New(),
		CompanyDomainID: cdID,
		Action:          "rejected",
		ReviewedBy:      "bob",
		CreatedAt:       time.Now(),
	}

	q := &stubQuerier{}
	q.On("CreateDomainReview", mock.Anything, db.CreateDomainReviewParams{
		CompanyDomainID: cdID,
		Action:          "rejected",
		ReviewedBy:      "bob",
		ReviewNote:      (*string)(nil),
	}).Return(review, nil)
	q.On("UpdateCompanyDomainStatus", mock.Anything, db.UpdateCompanyDomainStatusParams{
		ID:               cdID,
		Status:           "rejected",
		RelationshipType: "candidate",
	}).Return(nil)

	r := chi.NewRouter()
	newTestHandlers(q).RegisterRoutes(r)

	body, _ := json.Marshal(map[string]any{"action": "rejected", "reviewed_by": "bob"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/review/"+cdID.String()+"/reviews",
		bytes.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	q.AssertExpectations(t)
}

func TestHandleCreateReview_InvalidAction(t *testing.T) {
	r := chi.NewRouter()
	newTestHandlers(&stubQuerier{}).RegisterRoutes(r)

	body, _ := json.Marshal(map[string]any{"action": "delete", "reviewed_by": "eve"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/review/"+uuid.New().String()+"/reviews",
		bytes.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}
