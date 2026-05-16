package workers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/pulsarpoint/corpscout/scheduler/internal/crawlerclient"
	db "github.com/pulsarpoint/corpscout/scheduler/internal/db/gen"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// mockQuerier implements db.Querier using testify/mock.
type mockQuerier struct {
	mock.Mock
}

func (m *mockQuerier) CompletePullRun(ctx context.Context, arg db.CompletePullRunParams) error {
	args := m.Called(ctx, arg)
	return args.Error(0)
}

func (m *mockQuerier) CountCompanies(ctx context.Context, arg db.CountCompaniesParams) (int64, error) {
	args := m.Called(ctx, arg)
	return args.Get(0).(int64), args.Error(1)
}

func (m *mockQuerier) CountCandidatesForReview(ctx context.Context) (int64, error) {
	return 0, nil
}

func (m *mockQuerier) CountDomains(ctx context.Context, arg db.CountDomainsParams) (int64, error) {
	args := m.Called(ctx, arg)
	return args.Get(0).(int64), args.Error(1)
}

func (m *mockQuerier) CreateDomainReviewAndUpdateStatus(ctx context.Context, arg db.CreateDomainReviewAndUpdateStatusParams) (db.CompanyDomainReview, error) {
	return db.CompanyDomainReview{}, nil
}

func (m *mockQuerier) CreateDomainReview(ctx context.Context, arg db.CreateDomainReviewParams) (db.CompanyDomainReview, error) {
	args := m.Called(ctx, arg)
	return args.Get(0).(db.CompanyDomainReview), args.Error(1)
}

func (m *mockQuerier) CreatePullRun(ctx context.Context, arg db.CreatePullRunParams) (db.SourcePullRun, error) {
	args := m.Called(ctx, arg)
	return args.Get(0).(db.SourcePullRun), args.Error(1)
}

func (m *mockQuerier) FailPullRun(ctx context.Context, arg db.FailPullRunParams) error {
	args := m.Called(ctx, arg)
	return args.Error(0)
}

func (m *mockQuerier) GetCompany(ctx context.Context, id uuid.UUID) (db.Company, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(db.Company), args.Error(1)
}

func (m *mockQuerier) GetCountryByID(ctx context.Context, id uuid.UUID) (db.Country, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(db.Country), args.Error(1)
}

func (m *mockQuerier) GetCountryByISO2(ctx context.Context, isoAlpha2 string) (db.Country, error) {
	args := m.Called(ctx, isoAlpha2)
	return args.Get(0).(db.Country), args.Error(1)
}

func (m *mockQuerier) GetSourceByName(ctx context.Context, name string) (db.DataSource, error) {
	args := m.Called(ctx, name)
	return args.Get(0).(db.DataSource), args.Error(1)
}

func (m *mockQuerier) GetStats(ctx context.Context) (db.GetStatsRow, error) {
	args := m.Called(ctx)
	return args.Get(0).(db.GetStatsRow), args.Error(1)
}

func (m *mockQuerier) InsertSourceSnapshot(ctx context.Context, arg db.InsertSourceSnapshotParams) error {
	args := m.Called(ctx, arg)
	return args.Error(0)
}

func (m *mockQuerier) ListCandidatesForReview(ctx context.Context, arg db.ListCandidatesForReviewParams) ([]db.ListCandidatesForReviewRow, error) {
	args := m.Called(ctx, arg)
	return args.Get(0).([]db.ListCandidatesForReviewRow), args.Error(1)
}

func (m *mockQuerier) ListCompanies(ctx context.Context, arg db.ListCompaniesParams) ([]db.Company, error) {
	args := m.Called(ctx, arg)
	return args.Get(0).([]db.Company), args.Error(1)
}

func (m *mockQuerier) ListCountries(ctx context.Context) ([]db.Country, error) {
	args := m.Called(ctx)
	return args.Get(0).([]db.Country), args.Error(1)
}

func (m *mockQuerier) ListDomains(ctx context.Context, arg db.ListDomainsParams) ([]db.ListDomainsRow, error) {
	args := m.Called(ctx, arg)
	return args.Get(0).([]db.ListDomainsRow), args.Error(1)
}

func (m *mockQuerier) ListDomainsForCompany(ctx context.Context, companyID uuid.UUID) ([]db.ListDomainsForCompanyRow, error) {
	args := m.Called(ctx, companyID)
	return args.Get(0).([]db.ListDomainsForCompanyRow), args.Error(1)
}

func (m *mockQuerier) ListPullRuns(ctx context.Context, arg db.ListPullRunsParams) ([]db.ListPullRunsRow, error) {
	args := m.Called(ctx, arg)
	if v, ok := args.Get(0).([]db.ListPullRunsRow); ok {
		return v, args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockQuerier) ListReviewsForClaim(ctx context.Context, companyDomainID uuid.UUID) ([]db.CompanyDomainReview, error) {
	args := m.Called(ctx, companyDomainID)
	return args.Get(0).([]db.CompanyDomainReview), args.Error(1)
}

func (m *mockQuerier) ListSources(ctx context.Context) ([]db.DataSource, error) {
	args := m.Called(ctx)
	return args.Get(0).([]db.DataSource), args.Error(1)
}

func (m *mockQuerier) UpdateCompanyDomainStatus(ctx context.Context, arg db.UpdateCompanyDomainStatusParams) error {
	args := m.Called(ctx, arg)
	return args.Error(0)
}

func (m *mockQuerier) UpdateSourceCursor(ctx context.Context, arg db.UpdateSourceCursorParams) error {
	args := m.Called(ctx, arg)
	return args.Error(0)
}

func (m *mockQuerier) UpdateSourceEnabled(ctx context.Context, arg db.UpdateSourceEnabledParams) error {
	args := m.Called(ctx, arg)
	return args.Error(0)
}

func (m *mockQuerier) UpdateSourceInterval(ctx context.Context, arg db.UpdateSourceIntervalParams) error {
	args := m.Called(ctx, arg)
	return args.Error(0)
}

func (m *mockQuerier) UpsertCompanyAlias(ctx context.Context, arg db.UpsertCompanyAliasParams) error {
	args := m.Called(ctx, arg)
	return args.Error(0)
}

func (m *mockQuerier) UpsertCompanyByLEI(ctx context.Context, arg db.UpsertCompanyByLEIParams) (db.Company, error) {
	args := m.Called(ctx, arg)
	return args.Get(0).(db.Company), args.Error(1)
}

func (m *mockQuerier) UpsertCompanyByRegNumber(ctx context.Context, arg db.UpsertCompanyByRegNumberParams) (db.Company, error) {
	args := m.Called(ctx, arg)
	return args.Get(0).(db.Company), args.Error(1)
}

func (m *mockQuerier) UpsertCompanyDomain(ctx context.Context, arg db.UpsertCompanyDomainParams) (db.CompanyDomain, error) {
	args := m.Called(ctx, arg)
	return args.Get(0).(db.CompanyDomain), args.Error(1)
}

func (m *mockQuerier) UpsertCompanySource(ctx context.Context, arg db.UpsertCompanySourceParams) error {
	args := m.Called(ctx, arg)
	return args.Error(0)
}

func (m *mockQuerier) UpsertDataSource(ctx context.Context, arg db.UpsertDataSourceParams) (db.DataSource, error) {
	args := m.Called(ctx, arg)
	return args.Get(0).(db.DataSource), args.Error(1)
}

func (m *mockQuerier) UpsertDomain(ctx context.Context, domain string) (db.Domain, error) {
	args := m.Called(ctx, domain)
	return args.Get(0).(db.Domain), args.Error(1)
}

func (m *mockQuerier) InterruptStalePullRuns(ctx context.Context) error {
	return nil
}

func (m *mockQuerier) GetCompanyEmails(ctx context.Context, companyID uuid.UUID) ([]db.CompanyEmail, error) {
	return nil, nil
}
func (m *mockQuerier) GetCompanyIndustries(ctx context.Context, companyID uuid.UUID) ([]db.CompanyIndustry, error) {
	return nil, nil
}
func (m *mockQuerier) GetCompanyLocations(ctx context.Context, companyID uuid.UUID) ([]db.CompanyLocation, error) {
	return nil, nil
}
func (m *mockQuerier) GetCompanyMarkets(ctx context.Context, companyID uuid.UUID) ([]db.CompanyMarket, error) {
	return nil, nil
}
func (m *mockQuerier) GetCompanyPhones(ctx context.Context, companyID uuid.UUID) ([]db.CompanyPhone, error) {
	return nil, nil
}
func (m *mockQuerier) GetCompanyServices(ctx context.Context, companyID uuid.UUID) ([]db.CompanyService, error) {
	return nil, nil
}
func (m *mockQuerier) UpdateCompanyEnrichment(ctx context.Context, arg db.UpdateCompanyEnrichmentParams) (db.Company, error) {
	return db.Company{}, nil
}
func (m *mockQuerier) UpsertCompanyEmail(ctx context.Context, arg db.UpsertCompanyEmailParams) (db.CompanyEmail, error) {
	return db.CompanyEmail{}, nil
}
func (m *mockQuerier) UpsertCompanyIndustry(ctx context.Context, arg db.UpsertCompanyIndustryParams) (db.CompanyIndustry, error) {
	return db.CompanyIndustry{}, nil
}
func (m *mockQuerier) UpsertCompanyLocation(ctx context.Context, arg db.UpsertCompanyLocationParams) (db.CompanyLocation, error) {
	return db.CompanyLocation{}, nil
}
func (m *mockQuerier) UpsertCompanyMarket(ctx context.Context, arg db.UpsertCompanyMarketParams) (db.CompanyMarket, error) {
	return db.CompanyMarket{}, nil
}
func (m *mockQuerier) UpsertCompanyPhone(ctx context.Context, arg db.UpsertCompanyPhoneParams) (db.CompanyPhone, error) {
	return db.CompanyPhone{}, nil
}
func (m *mockQuerier) UpsertCompanyService(ctx context.Context, arg db.UpsertCompanyServiceParams) (db.CompanyService, error) {
	return db.CompanyService{}, nil
}

func (m *mockQuerier) ListCompaniesForGLEIFEnrich(ctx context.Context, arg db.ListCompaniesForGLEIFEnrichParams) ([]db.ListCompaniesForGLEIFEnrichRow, error) {
	return nil, nil
}

func (m *mockQuerier) UpdateCompanyParentLEI(ctx context.Context, arg db.UpdateCompanyParentLEIParams) error {
	return nil
}

func (m *mockQuerier) UpsertCompanyRelationship(ctx context.Context, arg db.UpsertCompanyRelationshipParams) (db.CompanyRelationship, error) {
	return db.CompanyRelationship{}, nil
}
func (m *mockQuerier) ListCompanyRelationships(ctx context.Context, subjectCompanyID uuid.UUID) ([]db.CompanyRelationship, error) {
	return nil, nil
}
func (m *mockQuerier) UpdateCompanyRelationshipStatus(ctx context.Context, arg db.UpdateCompanyRelationshipStatusParams) error {
	return nil
}
func (m *mockQuerier) GetCompanyBySlug(ctx context.Context, canonicalSlug string) (db.Company, error) {
	return db.Company{}, nil
}
func (m *mockQuerier) UpdateCompanySlug(ctx context.Context, arg db.UpdateCompanySlugParams) error {
	return nil
}

// compile-time check
var _ db.Querier = (*mockQuerier)(nil)

// TestSourceCrawlWorker_work_upserts_companies verifies that the worker correctly
// upserts companies, aliases, and marks the pull run complete.
func TestSourceCrawlWorker_work_upserts_companies(t *testing.T) {
	ctx := context.Background()

	// Fixed UUIDs for determinism.
	sourceID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	pullRunID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	countryID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	companyID := uuid.MustParse("44444444-4444-4444-4444-444444444444")

	regNum := "12345678"
	since := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

	// Build a test HTTP server that returns one page of records.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := crawlerclient.CrawlResponse{
			Records: []crawlerclient.CompanyRecord{
				{
					Name:               "Acme Corp",
					CountryISO2:        "GB",
					RegistrationNumber: &regNum,
					Status:             "active",
					Aliases:            []string{"Acme Limited"},
					RawData:            map[string]any{"source_id": "12345678"},
					SnapshotHash:       "abc123",
				},
			},
			HasMore: false,
			Total:   1,
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	crawler := crawlerclient.New(srv.URL)

	q := &mockQuerier{}

	// DataSource returned by GetSourceByName.
	source := db.DataSource{
		ID:                 sourceID,
		Name:               "test_source",
		Enabled:            true,
		CrawlIntervalHours: 24,
		LastCursor:         nil,
	}

	// SourcePullRun returned by CreatePullRun.
	pullRun := db.SourcePullRun{
		ID:       pullRunID,
		SourceID: sourceID,
	}

	// Country returned by GetCountryByISO2.
	country := db.Country{
		ID:        countryID,
		IsoAlpha2: "GB",
	}

	// Company returned by UpsertCompanyByRegNumber.
	company := db.Company{
		ID:        companyID,
		Name:      "Acme Corp",
		CountryID: countryID,
	}

	sourceUUID := pgtype.UUID{Bytes: sourceID, Valid: true}
	pullRunUUID := pgtype.UUID{Bytes: pullRunID, Valid: true}

	// Set up expectations for methods that ARE called.
	q.On("GetSourceByName", ctx, "test_source").Return(source, nil)
	q.On("CreatePullRun", ctx, mock.MatchedBy(func(p db.CreatePullRunParams) bool {
		return p.SourceID == sourceID
	})).Return(pullRun, nil)
	q.On("GetCountryByISO2", ctx, "GB").Return(country, nil)
	q.On("UpsertCompanyByRegNumber", ctx, mock.MatchedBy(func(p db.UpsertCompanyByRegNumberParams) bool {
		return p.Name == "Acme Corp" && p.CountryID == countryID
	})).Return(company, nil)
	q.On("UpsertCompanySource", ctx, mock.MatchedBy(func(p db.UpsertCompanySourceParams) bool {
		return p.CompanyID == companyID && p.SourceID == sourceID && p.PullRunID == pullRunUUID
	})).Return(nil)
	q.On("UpsertCompanyAlias", ctx, mock.MatchedBy(func(p db.UpsertCompanyAliasParams) bool {
		return p.CompanyID == companyID && p.Alias == "Acme Limited" &&
			p.AliasType == "trading_name" && p.SourceID == sourceUUID
	})).Return(nil)
	q.On("InsertSourceSnapshot", ctx, mock.AnythingOfType("db.InsertSourceSnapshotParams")).Return(nil)
	q.On("CompletePullRun", ctx, mock.MatchedBy(func(p db.CompletePullRunParams) bool {
		return p.ID == pullRunID && p.RecordsFetched == 1 && p.RecordsUpserted == 1
	})).Return(nil)
	q.On("UpdateSourceCursor", ctx, mock.MatchedBy(func(p db.UpdateSourceCursorParams) bool {
		return p.ID == sourceID && p.LastCrawledAt.Valid
	})).Return(nil)

	worker := NewSourceCrawlWorker(q, crawler, nil)

	job := &river.Job[SourceCrawlArgs]{
		JobRow: &rivertype.JobRow{
			ID: 999,
		},
		Args: SourceCrawlArgs{
			SourceName: "test_source",
			Since:      since,
		},
	}

	err := worker.Work(ctx, job)
	assert.NoError(t, err)

	q.AssertExpectations(t)
	q.AssertCalled(t, "UpsertCompanyByRegNumber", ctx, mock.Anything)
	q.AssertCalled(t, "UpsertCompanyAlias", ctx, mock.Anything)
	q.AssertCalled(t, "CompletePullRun", ctx, mock.Anything)
}

func TestExtractDomain(t *testing.T) {
	cases := []struct{ in, want string }{
		{"https://www.example.com/path", "www.example.com"},
		{"http://example.com", "example.com"},
		{"www.example.no", "www.example.no"},
		{"example.dk", "example.dk"},
		{"", ""},
		{"localhost", ""},
		{"192.168.1.1", ""},
		{"not-a-domain", ""},
		{"*.example.com", "example.com"},
		{"http://example.com:8080", "example.com"},
	}
	for _, c := range cases {
		if got := extractDomain(c.in); got != c.want {
			t.Errorf("extractDomain(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestSourceCrawlWorker_persists_registry_website(t *testing.T) {
	ctx := context.Background()

	sourceID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	pullRunID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	countryID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	companyID := uuid.MustParse("44444444-4444-4444-4444-444444444444")
	domainID := uuid.MustParse("55555555-5555-5555-5555-555555555555")

	website := "www.acme.no"
	regNum := "87654321"
	since := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := crawlerclient.CrawlResponse{
			Records: []crawlerclient.CompanyRecord{
				{
					Name:               "Acme AS",
					CountryISO2:        "NO",
					RegistrationNumber: &regNum,
					Status:             "active",
					Website:            &website,
					SnapshotHash:       "hash1",
					RawData:            map[string]any{},
				},
			},
			HasMore: false,
			Total:   1,
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	crawler := crawlerclient.New(srv.URL)
	q := &mockQuerier{}

	source := db.DataSource{
		ID:                 sourceID,
		Name:               "brreg",
		Enabled:            true,
		CrawlIntervalHours: 24,
	}
	pullRun := db.SourcePullRun{ID: pullRunID, SourceID: sourceID}
	country := db.Country{ID: countryID, IsoAlpha2: "NO"}
	company := db.Company{ID: companyID, Name: "Acme AS", CountryID: countryID}
	domain := db.Domain{ID: domainID, Domain: "www.acme.no"}

	sourceUUID := pgtype.UUID{Bytes: sourceID, Valid: true}
	pullRunUUID := pgtype.UUID{Bytes: pullRunID, Valid: true}

	q.On("GetSourceByName", ctx, "brreg").Return(source, nil)
	q.On("CreatePullRun", ctx, mock.MatchedBy(func(p db.CreatePullRunParams) bool {
		return p.SourceID == sourceID
	})).Return(pullRun, nil)
	q.On("UpdateSourceCursor", ctx, mock.AnythingOfType("db.UpdateSourceCursorParams")).Return(nil)
	q.On("GetCountryByISO2", ctx, "NO").Return(country, nil)
	q.On("UpsertCompanyByRegNumber", ctx, mock.MatchedBy(func(p db.UpsertCompanyByRegNumberParams) bool {
		return p.Name == "Acme AS"
	})).Return(company, nil)
	q.On("UpsertCompanySource", ctx, mock.MatchedBy(func(p db.UpsertCompanySourceParams) bool {
		return p.CompanyID == companyID && p.SourceID == sourceID && p.PullRunID == pullRunUUID
	})).Return(nil)
	q.On("InsertSourceSnapshot", ctx, mock.AnythingOfType("db.InsertSourceSnapshotParams")).Return(nil)
	// Registry website path: domain upsert + company-domain link.
	q.On("UpsertDomain", ctx, "www.acme.no").Return(domain, nil)
	q.On("UpsertCompanyDomain", ctx, mock.MatchedBy(func(p db.UpsertCompanyDomainParams) bool {
		return p.CompanyID == companyID &&
			p.DomainID == domainID &&
			p.Signal == "registry_website" &&
			p.Status == "active" &&
			p.Confidence == 90
	})).Return(db.CompanyDomain{}, nil)
	q.On("CompletePullRun", ctx, mock.AnythingOfType("db.CompletePullRunParams")).Return(nil)

	_ = sourceUUID

	worker := NewSourceCrawlWorker(q, crawler, nil)
	job := &river.Job[SourceCrawlArgs]{
		JobRow: &rivertype.JobRow{ID: 42},
		Args:   SourceCrawlArgs{SourceName: "brreg", Since: since},
	}

	err := worker.Work(ctx, job)
	assert.NoError(t, err)
	q.AssertExpectations(t)
	// Domain resolve job should NOT have been enqueued (registry website took the fast path).
}
