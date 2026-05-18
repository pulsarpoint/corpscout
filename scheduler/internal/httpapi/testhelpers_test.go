package httpapi_test

import (
	"context"
	"errors"

	"github.com/go-chi/chi/v5"
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

func (s *stubQuerier) hasExpectation(method string) bool {
	for _, call := range s.ExpectedCalls {
		if call.Method == method {
			return true
		}
	}
	return false
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

func (s *stubQuerier) CountDomains(ctx context.Context, arg db.CountDomainsParams) (int64, error) {
	ret := s.Called(ctx, arg)
	return ret.Get(0).(int64), ret.Error(1)
}

func (s *stubQuerier) CreatePullRun(ctx context.Context, arg db.CreatePullRunParams) (db.SourcePullRun, error) {
	return db.SourcePullRun{}, nil
}

func (s *stubQuerier) FailPullRun(ctx context.Context, arg db.FailPullRunParams) error {
	return nil
}

func (s *stubQuerier) SucceedPullRun(ctx context.Context, arg db.SucceedPullRunParams) error {
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

func (s *stubQuerier) ListDomains(ctx context.Context, arg db.ListDomainsParams) ([]db.ListDomainsRow, error) {
	ret := s.Called(ctx, arg)
	if v, ok := ret.Get(0).([]db.ListDomainsRow); ok {
		return v, ret.Error(1)
	}
	return nil, ret.Error(1)
}

func (s *stubQuerier) ListPullRuns(ctx context.Context, arg db.ListPullRunsParams) ([]db.ListPullRunsRow, error) {
	ret := s.Called(ctx, arg)
	if v, ok := ret.Get(0).([]db.ListPullRunsRow); ok {
		return v, ret.Error(1)
	}
	return nil, ret.Error(1)
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

func (s *stubQuerier) UpdateSourceEnabled(ctx context.Context, arg db.UpdateSourceEnabledParams) error {
	ret := s.Called(ctx, arg)
	return ret.Error(0)
}

func (s *stubQuerier) UpdateSourceSchedule(ctx context.Context, arg db.UpdateSourceScheduleParams) error {
	ret := s.Called(ctx, arg)
	return ret.Error(0)
}

func (s *stubQuerier) UpdateSourceScheduleEnabled(ctx context.Context, arg db.UpdateSourceScheduleEnabledParams) error {
	if !s.hasExpectation("UpdateSourceScheduleEnabled") {
		return nil
	}
	ret := s.Called(ctx, arg)
	return ret.Error(0)
}

func (s *stubQuerier) UpdateSourceConfig(ctx context.Context, arg db.UpdateSourceConfigParams) error {
	return nil
}

func (s *stubQuerier) UpdateSourcePullStarted(ctx context.Context, name string) error {
	return nil
}

func (s *stubQuerier) UpdateSourcePullSucceeded(ctx context.Context, arg db.UpdateSourcePullSucceededParams) error {
	return nil
}

func (s *stubQuerier) UpdateSourcePullFailed(ctx context.Context, arg db.UpdateSourcePullFailedParams) error {
	return nil
}

func (s *stubQuerier) UpsertCompanyDomain(ctx context.Context, arg db.UpsertCompanyDomainParams) (db.CompanyDomain, error) {
	return db.CompanyDomain{}, nil
}

func (s *stubQuerier) InterruptStalePullRuns(ctx context.Context) error {
	return nil
}

func (s *stubQuerier) GetCompanyEmails(ctx context.Context, companyID uuid.UUID) ([]db.CompanyEmail, error) {
	return nil, nil
}
func (s *stubQuerier) GetCompanyIndustries(ctx context.Context, companyID uuid.UUID) ([]db.CompanyIndustry, error) {
	return nil, nil
}
func (s *stubQuerier) GetCompanyLocations(ctx context.Context, companyID uuid.UUID) ([]db.CompanyLocation, error) {
	return nil, nil
}
func (s *stubQuerier) GetCompanyMarkets(ctx context.Context, companyID uuid.UUID) ([]db.CompanyMarket, error) {
	return nil, nil
}
func (s *stubQuerier) GetCompanyPhones(ctx context.Context, companyID uuid.UUID) ([]db.CompanyPhone, error) {
	return nil, nil
}
func (s *stubQuerier) GetCompanyServices(ctx context.Context, companyID uuid.UUID) ([]db.CompanyService, error) {
	return nil, nil
}
func (s *stubQuerier) UpdateCompanyEnrichment(ctx context.Context, arg db.UpdateCompanyEnrichmentParams) (db.Company, error) {
	return db.Company{}, nil
}
func (s *stubQuerier) UpsertCompanyEmail(ctx context.Context, arg db.UpsertCompanyEmailParams) (db.CompanyEmail, error) {
	return db.CompanyEmail{}, nil
}
func (s *stubQuerier) UpsertCompanyIndustry(ctx context.Context, arg db.UpsertCompanyIndustryParams) (db.CompanyIndustry, error) {
	return db.CompanyIndustry{}, nil
}
func (s *stubQuerier) UpsertCompanyLocation(ctx context.Context, arg db.UpsertCompanyLocationParams) (db.CompanyLocation, error) {
	return db.CompanyLocation{}, nil
}
func (s *stubQuerier) UpsertCompanyMarket(ctx context.Context, arg db.UpsertCompanyMarketParams) (db.CompanyMarket, error) {
	return db.CompanyMarket{}, nil
}
func (s *stubQuerier) UpsertCompanyPhone(ctx context.Context, arg db.UpsertCompanyPhoneParams) (db.CompanyPhone, error) {
	return db.CompanyPhone{}, nil
}
func (s *stubQuerier) UpsertCompanyService(ctx context.Context, arg db.UpsertCompanyServiceParams) (db.CompanyService, error) {
	return db.CompanyService{}, nil
}

func (s *stubQuerier) UpsertDomain(ctx context.Context, domain string) (db.Domain, error) {
	return db.Domain{}, nil
}

func (s *stubQuerier) UpsertCompanyRelationship(ctx context.Context, arg db.UpsertCompanyRelationshipParams) (db.CompanyRelationship, error) {
	return db.CompanyRelationship{}, nil
}
func (s *stubQuerier) ListCompanyRelationships(ctx context.Context, subjectCompanyID uuid.UUID) ([]db.CompanyRelationship, error) {
	return nil, nil
}
func (s *stubQuerier) UpdateCompanyRelationshipStatus(ctx context.Context, arg db.UpdateCompanyRelationshipStatusParams) error {
	return nil
}
func (s *stubQuerier) GetCompanyBySlug(ctx context.Context, canonicalSlug string) (db.Company, error) {
	ret := s.Called(ctx, canonicalSlug)
	return ret.Get(0).(db.Company), ret.Error(1)
}
func (s *stubQuerier) UpdateCompanySlug(ctx context.Context, arg db.UpdateCompanySlugParams) error {
	return nil
}

func (s *stubQuerier) InsertOrganization(ctx context.Context, arg db.InsertOrganizationParams) (db.Organization, error) {
	ret := s.Called(ctx, arg)
	return ret.Get(0).(db.Organization), ret.Error(1)
}
func (s *stubQuerier) GetOrganizationByID(ctx context.Context, id uuid.UUID) (db.Organization, error) {
	ret := s.Called(ctx, id)
	return ret.Get(0).(db.Organization), ret.Error(1)
}
func (s *stubQuerier) GetOrganizationBySlug(ctx context.Context, canonicalSlug string) (db.Organization, error) {
	ret := s.Called(ctx, canonicalSlug)
	return ret.Get(0).(db.Organization), ret.Error(1)
}
func (s *stubQuerier) ListOrganizations(ctx context.Context, arg db.ListOrganizationsParams) ([]db.Organization, error) {
	ret := s.Called(ctx, arg)
	if v, ok := ret.Get(0).([]db.Organization); ok {
		return v, ret.Error(1)
	}
	return nil, ret.Error(1)
}
func (s *stubQuerier) CountOrganizations(ctx context.Context, q_ *string) (int64, error) {
	ret := s.Called(ctx, q_)
	return ret.Get(0).(int64), ret.Error(1)
}
func (s *stubQuerier) UpdateOrganizationStatus(ctx context.Context, arg db.UpdateOrganizationStatusParams) error {
	return nil
}
func (s *stubQuerier) InsertOpenSourceProject(ctx context.Context, arg db.InsertOpenSourceProjectParams) (db.OpenSourceProject, error) {
	ret := s.Called(ctx, arg)
	return ret.Get(0).(db.OpenSourceProject), ret.Error(1)
}
func (s *stubQuerier) GetOpenSourceProjectByID(ctx context.Context, id uuid.UUID) (db.OpenSourceProject, error) {
	ret := s.Called(ctx, id)
	return ret.Get(0).(db.OpenSourceProject), ret.Error(1)
}
func (s *stubQuerier) GetOpenSourceProjectBySlug(ctx context.Context, canonicalSlug string) (db.OpenSourceProject, error) {
	ret := s.Called(ctx, canonicalSlug)
	return ret.Get(0).(db.OpenSourceProject), ret.Error(1)
}
func (s *stubQuerier) ListOpenSourceProjects(ctx context.Context, arg db.ListOpenSourceProjectsParams) ([]db.OpenSourceProject, error) {
	ret := s.Called(ctx, arg)
	if v, ok := ret.Get(0).([]db.OpenSourceProject); ok {
		return v, ret.Error(1)
	}
	return nil, ret.Error(1)
}
func (s *stubQuerier) CountOpenSourceProjects(ctx context.Context, q_ *string) (int64, error) {
	ret := s.Called(ctx, q_)
	return ret.Get(0).(int64), ret.Error(1)
}
func (s *stubQuerier) UpdateOpenSourceProjectStatus(ctx context.Context, arg db.UpdateOpenSourceProjectStatusParams) error {
	return nil
}

func (s *stubQuerier) CountPendingCPELinkSuggestions(ctx context.Context) (int64, error) {
	return 0, nil
}
func (s *stubQuerier) GetCPEEntityLinkByToken(ctx context.Context, cpeVendorToken string) (db.CpeEntityLink, error) {
	return db.CpeEntityLink{}, nil
}
func (s *stubQuerier) InsertCPEEntityLink(ctx context.Context, arg db.InsertCPEEntityLinkParams) (db.CpeEntityLink, error) {
	return db.CpeEntityLink{}, nil
}
func (s *stubQuerier) InsertCPELinkSuggestion(ctx context.Context, arg db.InsertCPELinkSuggestionParams) (db.CpeEntityLinkSuggestion, error) {
	return db.CpeEntityLinkSuggestion{}, nil
}
func (s *stubQuerier) InsertCVEEntityLink(ctx context.Context, arg db.InsertCVEEntityLinkParams) (db.CveEntityLink, error) {
	return db.CveEntityLink{}, nil
}
func (s *stubQuerier) InsertCVELinkSuggestion(ctx context.Context, arg db.InsertCVELinkSuggestionParams) (db.CveEntityLinkSuggestion, error) {
	return db.CveEntityLinkSuggestion{}, nil
}
func (s *stubQuerier) ListPendingCPELinkSuggestions(ctx context.Context, arg db.ListPendingCPELinkSuggestionsParams) ([]db.CpeEntityLinkSuggestion, error) {
	return nil, nil
}
func (s *stubQuerier) ListPendingCVELinkSuggestions(ctx context.Context, arg db.ListPendingCVELinkSuggestionsParams) ([]db.CveEntityLinkSuggestion, error) {
	return nil, nil
}
func (s *stubQuerier) UpdateCPELinkSuggestionStatus(ctx context.Context, arg db.UpdateCPELinkSuggestionStatusParams) error {
	return nil
}
func (s *stubQuerier) UpdateCVELinkSuggestionStatus(ctx context.Context, arg db.UpdateCVELinkSuggestionStatusParams) error {
	return nil
}
func (s *stubQuerier) CountPendingCVELinkSuggestions(ctx context.Context) (int64, error) {
	return 0, nil
}
func (s *stubQuerier) ListCVEEntityLinksByCVEID(ctx context.Context, cveID string) ([]db.CveEntityLink, error) {
	return nil, nil
}

// Suggestion methods
func (s *stubQuerier) CountPendingCompanySuggestions(ctx context.Context) (int64, error) {
	ret := s.Called(ctx)
	return ret.Get(0).(int64), ret.Error(1)
}
func (s *stubQuerier) GetCompanySuggestionByID(ctx context.Context, id uuid.UUID) (db.CompanySuggestion, error) {
	return db.CompanySuggestion{}, nil
}
func (s *stubQuerier) ListPendingCompanySuggestions(ctx context.Context, arg db.ListPendingCompanySuggestionsParams) ([]db.CompanySuggestion, error) {
	ret := s.Called(ctx, arg)
	if v, ok := ret.Get(0).([]db.CompanySuggestion); ok {
		return v, ret.Error(1)
	}
	return nil, ret.Error(1)
}
func (s *stubQuerier) UpdateCompanySuggestionApproved(ctx context.Context, arg db.UpdateCompanySuggestionApprovedParams) error {
	return nil
}
func (s *stubQuerier) UpdateCompanySuggestionRejected(ctx context.Context, arg db.UpdateCompanySuggestionRejectedParams) error {
	return nil
}
func (s *stubQuerier) InsertCompanyDomainSuggestion(ctx context.Context, arg db.InsertCompanyDomainSuggestionParams) (db.CompanyDomainSuggestion, error) {
	return db.CompanyDomainSuggestion{}, nil
}
func (s *stubQuerier) InsertCompanyContactSuggestion(ctx context.Context, arg db.InsertCompanyContactSuggestionParams) (db.CompanyContactSuggestion, error) {
	return db.CompanyContactSuggestion{}, nil
}
func (s *stubQuerier) InsertCompanyLocationSuggestion(ctx context.Context, arg db.InsertCompanyLocationSuggestionParams) (db.CompanyLocationSuggestion, error) {
	return db.CompanyLocationSuggestion{}, nil
}
func (s *stubQuerier) InsertCompanyStatusSuggestion(ctx context.Context, arg db.InsertCompanyStatusSuggestionParams) (db.CompanyStatusSuggestion, error) {
	return db.CompanyStatusSuggestion{}, nil
}
func (s *stubQuerier) InsertCompanyRelationshipSuggestion(ctx context.Context, arg db.InsertCompanyRelationshipSuggestionParams) (db.CompanyRelationshipSuggestion, error) {
	return db.CompanyRelationshipSuggestion{}, nil
}
func (s *stubQuerier) InsertOrganizationSuggestion(ctx context.Context, arg db.InsertOrganizationSuggestionParams) (db.OrganizationSuggestion, error) {
	return db.OrganizationSuggestion{}, nil
}
func (s *stubQuerier) InsertOpenSourceProjectSuggestion(ctx context.Context, arg db.InsertOpenSourceProjectSuggestionParams) (db.OpenSourceProjectSuggestion, error) {
	return db.OpenSourceProjectSuggestion{}, nil
}
func (s *stubQuerier) InsertSuggestionSourceLink(ctx context.Context, arg db.InsertSuggestionSourceLinkParams) (db.SuggestionSourceLink, error) {
	return db.SuggestionSourceLink{}, nil
}
func (s *stubQuerier) InsertCompanySuggestion(ctx context.Context, arg db.InsertCompanySuggestionParams) (db.CompanySuggestion, error) {
	return db.CompanySuggestion{}, nil
}

// Company methods
func (s *stubQuerier) GetCompanyByLEI(ctx context.Context, lei *string) (db.Company, error) {
	return db.Company{}, nil
}
func (s *stubQuerier) GetCompanyByRegistrationAndCountry(ctx context.Context, arg db.GetCompanyByRegistrationAndCountryParams) (db.Company, error) {
	return db.Company{}, nil
}
func (s *stubQuerier) GetCountryIDByISO2(ctx context.Context, isoAlpha2 string) (uuid.UUID, error) {
	return uuid.UUID{}, nil
}
func (s *stubQuerier) InsertCompany(ctx context.Context, arg db.InsertCompanyParams) (db.Company, error) {
	return db.Company{}, nil
}

// Raw input stubs
func (s *stubQuerier) UpsertGLEIFCompanyRawInput(ctx context.Context, arg db.UpsertGLEIFCompanyRawInputParams) (db.GleifCompanyRawInput, error) {
	return db.GleifCompanyRawInput{}, nil
}
func (s *stubQuerier) ClaimPendingGLEIFRawInputs(ctx context.Context, arg db.ClaimPendingGLEIFRawInputsParams) ([]db.GleifCompanyRawInput, error) {
	return nil, nil
}
func (s *stubQuerier) MarkGLEIFRawInputProcessed(ctx context.Context, id uuid.UUID) error {
	return nil
}
func (s *stubQuerier) MarkGLEIFRawInputFailed(ctx context.Context, arg db.MarkGLEIFRawInputFailedParams) error {
	return nil
}
func (s *stubQuerier) UpsertCompaniesHouseRawInput(ctx context.Context, arg db.UpsertCompaniesHouseRawInputParams) (db.CompaniesHouseCompanyRawInput, error) {
	return db.CompaniesHouseCompanyRawInput{}, nil
}
func (s *stubQuerier) ClaimPendingCompaniesHouseRawInputs(ctx context.Context, arg db.ClaimPendingCompaniesHouseRawInputsParams) ([]db.CompaniesHouseCompanyRawInput, error) {
	return nil, nil
}
func (s *stubQuerier) MarkCompaniesHouseRawInputProcessed(ctx context.Context, id uuid.UUID) error {
	return nil
}
func (s *stubQuerier) MarkCompaniesHouseRawInputFailed(ctx context.Context, arg db.MarkCompaniesHouseRawInputFailedParams) error {
	return nil
}
func (s *stubQuerier) UpsertBrregRawInput(ctx context.Context, arg db.UpsertBrregRawInputParams) (db.BrregCompanyRawInput, error) {
	return db.BrregCompanyRawInput{}, nil
}
func (s *stubQuerier) ClaimPendingBrregRawInputs(ctx context.Context, arg db.ClaimPendingBrregRawInputsParams) ([]db.BrregCompanyRawInput, error) {
	return nil, nil
}
func (s *stubQuerier) MarkBrregRawInputProcessed(ctx context.Context, id uuid.UUID) error {
	return nil
}
func (s *stubQuerier) MarkBrregRawInputFailed(ctx context.Context, arg db.MarkBrregRawInputFailedParams) error {
	return nil
}
func (s *stubQuerier) RetryGLEIFRawInput(ctx context.Context, id uuid.UUID) (uuid.UUID, error) {
	if !s.hasExpectation("RetryGLEIFRawInput") {
		return uuid.UUID{}, nil
	}
	ret := s.Called(ctx, id)
	return ret.Get(0).(uuid.UUID), ret.Error(1)
}
func (s *stubQuerier) IgnoreGLEIFRawInput(ctx context.Context, id uuid.UUID) (uuid.UUID, error) {
	if !s.hasExpectation("IgnoreGLEIFRawInput") {
		return uuid.UUID{}, nil
	}
	ret := s.Called(ctx, id)
	return ret.Get(0).(uuid.UUID), ret.Error(1)
}
func (s *stubQuerier) RetryCompaniesHouseRawInput(ctx context.Context, id uuid.UUID) (uuid.UUID, error) {
	if !s.hasExpectation("RetryCompaniesHouseRawInput") {
		return uuid.UUID{}, nil
	}
	ret := s.Called(ctx, id)
	return ret.Get(0).(uuid.UUID), ret.Error(1)
}
func (s *stubQuerier) IgnoreCompaniesHouseRawInput(ctx context.Context, id uuid.UUID) (uuid.UUID, error) {
	if !s.hasExpectation("IgnoreCompaniesHouseRawInput") {
		return uuid.UUID{}, nil
	}
	ret := s.Called(ctx, id)
	return ret.Get(0).(uuid.UUID), ret.Error(1)
}
func (s *stubQuerier) RetryBrregRawInput(ctx context.Context, id uuid.UUID) (uuid.UUID, error) {
	if !s.hasExpectation("RetryBrregRawInput") {
		return uuid.UUID{}, nil
	}
	ret := s.Called(ctx, id)
	return ret.Get(0).(uuid.UUID), ret.Error(1)
}
func (s *stubQuerier) IgnoreBrregRawInput(ctx context.Context, id uuid.UUID) (uuid.UUID, error) {
	if !s.hasExpectation("IgnoreBrregRawInput") {
		return uuid.UUID{}, nil
	}
	ret := s.Called(ctx, id)
	return ret.Get(0).(uuid.UUID), ret.Error(1)
}
func (s *stubQuerier) RetryAIRawInput(ctx context.Context, id uuid.UUID) (uuid.UUID, error) {
	if !s.hasExpectation("RetryAIRawInput") {
		return uuid.UUID{}, nil
	}
	ret := s.Called(ctx, id)
	return ret.Get(0).(uuid.UUID), ret.Error(1)
}
func (s *stubQuerier) IgnoreAIRawInput(ctx context.Context, id uuid.UUID) (uuid.UUID, error) {
	if !s.hasExpectation("IgnoreAIRawInput") {
		return uuid.UUID{}, nil
	}
	ret := s.Called(ctx, id)
	return ret.Get(0).(uuid.UUID), ret.Error(1)
}
func (s *stubQuerier) RetryDomainDiscoveryRawInput(ctx context.Context, id uuid.UUID) (uuid.UUID, error) {
	if !s.hasExpectation("RetryDomainDiscoveryRawInput") {
		return uuid.UUID{}, nil
	}
	ret := s.Called(ctx, id)
	return ret.Get(0).(uuid.UUID), ret.Error(1)
}
func (s *stubQuerier) IgnoreDomainDiscoveryRawInput(ctx context.Context, id uuid.UUID) (uuid.UUID, error) {
	if !s.hasExpectation("IgnoreDomainDiscoveryRawInput") {
		return uuid.UUID{}, nil
	}
	ret := s.Called(ctx, id)
	return ret.Get(0).(uuid.UUID), ret.Error(1)
}

// Section suggestion stubs (new in Task 10)
func (s *stubQuerier) GetCompanyStatusSuggestionByID(ctx context.Context, id uuid.UUID) (db.CompanyStatusSuggestion, error) {
	return db.CompanyStatusSuggestion{}, nil
}
func (s *stubQuerier) UpdateCompanyStatusSuggestionApproved(ctx context.Context, arg db.UpdateCompanyStatusSuggestionApprovedParams) error {
	return nil
}
func (s *stubQuerier) UpdateCompanyStatusSuggestionRejected(ctx context.Context, arg db.UpdateCompanyStatusSuggestionRejectedParams) error {
	return nil
}
func (s *stubQuerier) GetCompanyContactSuggestionByID(ctx context.Context, id uuid.UUID) (db.CompanyContactSuggestion, error) {
	return db.CompanyContactSuggestion{}, nil
}
func (s *stubQuerier) UpdateCompanyContactSuggestionApproved(ctx context.Context, arg db.UpdateCompanyContactSuggestionApprovedParams) error {
	return nil
}
func (s *stubQuerier) UpdateCompanyContactSuggestionRejected(ctx context.Context, arg db.UpdateCompanyContactSuggestionRejectedParams) error {
	return nil
}
func (s *stubQuerier) UpdateCompanyStatus(ctx context.Context, arg db.UpdateCompanyStatusParams) error {
	return nil
}
func (s *stubQuerier) UpdateCompanyWebsite(ctx context.Context, arg db.UpdateCompanyWebsiteParams) error {
	return nil
}

// --- helpers ---

// newTestHandlers creates a Handlers instance with the given stub, nil river client and nil pool.
func newTestHandlers(q db.Querier) *httpapi.Handlers {
	return httpapi.NewHandlers(q, nil, nil, nil, "")
}

var errNotFound = errors.New("not found")

func routerFor(h *httpapi.Handlers) chi.Router {
	r := chi.NewRouter()
	h.RegisterRoutes(r)
	return r
}

// ensure stubQuerier satisfies the interface at compile time
var _ db.Querier = (*stubQuerier)(nil)
