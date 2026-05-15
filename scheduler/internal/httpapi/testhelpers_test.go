package httpapi_test

import (
	"context"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"

	db "github.com/pulsarpoint/corpscout/scheduler/internal/db/gen"
	"github.com/pulsarpoint/corpscout/scheduler/internal/httpapi"
)

// stubQuerier implements db.Querier for use in handler tests.
// Methods called by tests use mock.Called; all others return zero values.
type stubQuerier struct {
	mock.Mock
}

// --- methods used by test cases (use mock.Called) ---

func (s *stubQuerier) ListCompanies(ctx context.Context, arg db.ListCompaniesParams) ([]db.Company, error) {
	ret := s.Called(ctx, arg)
	if v, ok := ret.Get(0).([]db.Company); ok {
		return v, ret.Error(1)
	}
	return nil, ret.Error(1)
}

func (s *stubQuerier) CountCompanies(ctx context.Context, arg db.CountCompaniesParams) (int64, error) {
	ret := s.Called(ctx, arg)
	return ret.Get(0).(int64), ret.Error(1)
}

func (s *stubQuerier) GetCompany(ctx context.Context, id uuid.UUID) (db.Company, error) {
	ret := s.Called(ctx, id)
	return ret.Get(0).(db.Company), ret.Error(1)
}

func (s *stubQuerier) ListDomainsForCompany(ctx context.Context, companyID uuid.UUID) ([]db.ListDomainsForCompanyRow, error) {
	ret := s.Called(ctx, companyID)
	if v, ok := ret.Get(0).([]db.ListDomainsForCompanyRow); ok {
		return v, ret.Error(1)
	}
	return nil, ret.Error(1)
}

func (s *stubQuerier) ListCountries(ctx context.Context) ([]db.Country, error) {
	ret := s.Called(ctx)
	if v, ok := ret.Get(0).([]db.Country); ok {
		return v, ret.Error(1)
	}
	return nil, ret.Error(1)
}

// --- stub-only methods (zero return values, no mock.On needed) ---

func (s *stubQuerier) CompletePullRun(ctx context.Context, arg db.CompletePullRunParams) error {
	return nil
}

func (s *stubQuerier) CountDomains(ctx context.Context, arg db.CountDomainsParams) (int64, error) {
	ret := s.Called(ctx, arg)
	return ret.Get(0).(int64), ret.Error(1)
}

func (s *stubQuerier) CreateDomainReview(ctx context.Context, arg db.CreateDomainReviewParams) (db.CompanyDomainReview, error) {
	ret := s.Called(ctx, arg)
	return ret.Get(0).(db.CompanyDomainReview), ret.Error(1)
}

func (s *stubQuerier) CreateDomainReviewAndUpdateStatus(ctx context.Context, arg db.CreateDomainReviewAndUpdateStatusParams) (db.CompanyDomainReview, error) {
	ret := s.Called(ctx, arg)
	return ret.Get(0).(db.CompanyDomainReview), ret.Error(1)
}

func (s *stubQuerier) CreatePullRun(ctx context.Context, arg db.CreatePullRunParams) (db.SourcePullRun, error) {
	return db.SourcePullRun{}, nil
}

func (s *stubQuerier) FailPullRun(ctx context.Context, arg db.FailPullRunParams) error {
	return nil
}

func (s *stubQuerier) GetCountryByID(ctx context.Context, id uuid.UUID) (db.Country, error) {
	return db.Country{}, nil
}

func (s *stubQuerier) GetCountryByISO2(ctx context.Context, isoAlpha2 string) (db.Country, error) {
	return db.Country{}, nil
}

func (s *stubQuerier) GetSourceByName(ctx context.Context, name string) (db.DataSource, error) {
	ret := s.Called(ctx, name)
	return ret.Get(0).(db.DataSource), ret.Error(1)
}

func (s *stubQuerier) GetStats(ctx context.Context) (db.GetStatsRow, error) {
	ret := s.Called(ctx)
	return ret.Get(0).(db.GetStatsRow), ret.Error(1)
}

func (s *stubQuerier) InsertSourceSnapshot(ctx context.Context, arg db.InsertSourceSnapshotParams) error {
	return nil
}

func (s *stubQuerier) ListCandidatesForReview(ctx context.Context, arg db.ListCandidatesForReviewParams) ([]db.ListCandidatesForReviewRow, error) {
	ret := s.Called(ctx, arg)
	if v, ok := ret.Get(0).([]db.ListCandidatesForReviewRow); ok {
		return v, ret.Error(1)
	}
	return nil, ret.Error(1)
}

func (s *stubQuerier) ListDomains(ctx context.Context, arg db.ListDomainsParams) ([]db.ListDomainsRow, error) {
	ret := s.Called(ctx, arg)
	if v, ok := ret.Get(0).([]db.ListDomainsRow); ok {
		return v, ret.Error(1)
	}
	return nil, ret.Error(1)
}

func (s *stubQuerier) ListReviewsForClaim(ctx context.Context, companyDomainID uuid.UUID) ([]db.CompanyDomainReview, error) {
	return nil, nil
}

func (s *stubQuerier) ListSources(ctx context.Context) ([]db.DataSource, error) {
	ret := s.Called(ctx)
	if v, ok := ret.Get(0).([]db.DataSource); ok {
		return v, ret.Error(1)
	}
	return nil, ret.Error(1)
}

func (s *stubQuerier) UpdateCompanyDomainStatus(ctx context.Context, arg db.UpdateCompanyDomainStatusParams) error {
	return nil
}

func (s *stubQuerier) UpdateSourceCursor(ctx context.Context, arg db.UpdateSourceCursorParams) error {
	return nil
}

func (s *stubQuerier) UpdateSourceEnabled(ctx context.Context, arg db.UpdateSourceEnabledParams) error {
	ret := s.Called(ctx, arg)
	return ret.Error(0)
}

func (s *stubQuerier) UpdateSourceInterval(ctx context.Context, arg db.UpdateSourceIntervalParams) error {
	ret := s.Called(ctx, arg)
	return ret.Error(0)
}

func (s *stubQuerier) UpsertCompanyAlias(ctx context.Context, arg db.UpsertCompanyAliasParams) error {
	return nil
}

func (s *stubQuerier) UpsertCompanyByLEI(ctx context.Context, arg db.UpsertCompanyByLEIParams) (db.Company, error) {
	return db.Company{}, nil
}

func (s *stubQuerier) UpsertCompanyByRegNumber(ctx context.Context, arg db.UpsertCompanyByRegNumberParams) (db.Company, error) {
	return db.Company{}, nil
}

func (s *stubQuerier) UpsertCompanyDomain(ctx context.Context, arg db.UpsertCompanyDomainParams) (db.CompanyDomain, error) {
	return db.CompanyDomain{}, nil
}

func (s *stubQuerier) UpsertCompanySource(ctx context.Context, arg db.UpsertCompanySourceParams) error {
	return nil
}

func (s *stubQuerier) UpsertDataSource(ctx context.Context, arg db.UpsertDataSourceParams) (db.DataSource, error) {
	return db.DataSource{}, nil
}

func (s *stubQuerier) UpsertDomain(ctx context.Context, domain string) (db.Domain, error) {
	return db.Domain{}, nil
}

// --- helpers ---

// newTestHandlers creates a Handlers instance with the given stub, nil river client and nil pool.
func newTestHandlers(q db.Querier) *httpapi.Handlers {
	return httpapi.NewHandlers(q, nil, nil)
}

// ensure stubQuerier satisfies the interface at compile time
var _ db.Querier = (*stubQuerier)(nil)

