package workers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"encoding/json"

	"github.com/google/uuid"
	"github.com/pulsarpoint/corpscout/scheduler/internal/crawlerclient"
	db "github.com/pulsarpoint/corpscout/scheduler/internal/db/gen"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestDomainResolveWorker_upserts_active_domain_for_high_confidence(t *testing.T) {
	ctx := context.Background()

	companyID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	countryID := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")
	domainID := uuid.MustParse("cccccccc-cccc-cccc-cccc-cccccccccccc")

	lei := "LEI123"
	company := db.Company{
		ID:        companyID,
		Name:      "Acme Corp",
		CountryID: countryID,
		Lei:       &lei,
	}
	country := db.Country{
		ID:        countryID,
		IsoAlpha2: "US",
	}
	domain := db.Domain{
		ID:     domainID,
		Domain: "acme.com",
	}

	// Build a test HTTP server returning one high-confidence candidate.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := crawlerclient.ResolveResponse{
			Candidates: []crawlerclient.DomainCandidate{
				{
					Domain:     "acme.com",
					Signal:     "wikidata",
					Confidence: 85,
					Evidence:   map[string]any{"source": "wikidata"},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	crawler := crawlerclient.New(srv.URL)
	q := &mockQuerier{}

	q.On("GetCompany", ctx, companyID).Return(company, nil)
	q.On("GetCountryByID", ctx, countryID).Return(country, nil)
	q.On("UpsertDomain", ctx, "acme.com").Return(domain, nil)
	q.On("UpsertCompanyDomain", ctx, mock.MatchedBy(func(p db.UpsertCompanyDomainParams) bool {
		return p.CompanyID == companyID &&
			p.DomainID == domainID &&
			p.Status == "active" &&
			p.RelationshipType == "official_site"
	})).Return(db.CompanyDomain{}, nil)

	worker := NewDomainResolveWorker(q, crawler)

	job := &river.Job[DomainResolveArgs]{
		JobRow: &rivertype.JobRow{ID: 1},
		Args:   DomainResolveArgs{CompanyID: companyID.String()},
	}

	err := worker.Work(ctx, job)
	assert.NoError(t, err)

	q.AssertExpectations(t)
	q.AssertCalled(t, "UpsertCompanyDomain", ctx, mock.MatchedBy(func(p db.UpsertCompanyDomainParams) bool {
		return p.Status == "active" && p.RelationshipType == "official_site"
	}))
}

func TestDomainResolveWorker_marks_low_confidence_needs_review(t *testing.T) {
	ctx := context.Background()

	companyID := uuid.MustParse("dddddddd-dddd-dddd-dddd-dddddddddddd")
	countryID := uuid.MustParse("eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee")
	domainID := uuid.MustParse("ffffffff-ffff-ffff-ffff-ffffffffffff")

	company := db.Company{
		ID:        companyID,
		Name:      "Maybe Corp",
		CountryID: countryID,
		Lei:       nil,
	}
	country := db.Country{
		ID:        countryID,
		IsoAlpha2: "GB",
	}
	domain := db.Domain{
		ID:     domainID,
		Domain: "maybe.com",
	}

	// Build a test HTTP server returning one low-confidence candidate.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := crawlerclient.ResolveResponse{
			Candidates: []crawlerclient.DomainCandidate{
				{
					Domain:     "maybe.com",
					Signal:     "certsh",
					Confidence: 60,
					Evidence:   map[string]any{"source": "certsh"},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	crawler := crawlerclient.New(srv.URL)
	q := &mockQuerier{}

	q.On("GetCompany", ctx, companyID).Return(company, nil)
	q.On("GetCountryByID", ctx, countryID).Return(country, nil)
	q.On("UpsertDomain", ctx, "maybe.com").Return(domain, nil)
	q.On("UpsertCompanyDomain", ctx, mock.MatchedBy(func(p db.UpsertCompanyDomainParams) bool {
		return p.CompanyID == companyID &&
			p.DomainID == domainID &&
			p.Status == "needs_review" &&
			p.RelationshipType == "candidate"
	})).Return(db.CompanyDomain{}, nil)

	worker := NewDomainResolveWorker(q, crawler)

	job := &river.Job[DomainResolveArgs]{
		JobRow: &rivertype.JobRow{ID: 2},
		Args:   DomainResolveArgs{CompanyID: companyID.String()},
	}

	err := worker.Work(ctx, job)
	assert.NoError(t, err)

	q.AssertExpectations(t)
	q.AssertCalled(t, "UpsertCompanyDomain", ctx, mock.MatchedBy(func(p db.UpsertCompanyDomainParams) bool {
		return p.Status == "needs_review" && p.RelationshipType == "candidate"
	}))
}
