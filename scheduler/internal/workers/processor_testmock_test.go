package workers_test

import (
	"context"
	"time"

	"github.com/google/uuid"
	pgx "github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	db "github.com/pulsarpoint/corpscout/scheduler/internal/db/gen"
)

var pgxErrNoRows = pgx.ErrNoRows

func ptrStr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func pgTypeTZ(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Valid: true, Time: t}
}

// mockQuerier is a configurable db.Querier for processor unit tests.
type mockQuerier struct {
	db.Querier
	claimGLEIF                     func() []db.GleifCompanyRawInput
	claimCH                        func() []db.CompaniesHouseCompanyRawInput
	claimBrreg                     func() []db.BrregCompanyRawInput
	getCompanyByLEI                func(lei *string) (db.Company, error)
	getCompanyByRegAndCountry      func(reg *string, iso string) (db.Company, error)
	getSourceByName                func(name string) (db.DataSource, error)
	getCountryIDByISO2             func(iso string) (uuid.UUID, error)
	insertCompanySuggestion        func(arg db.InsertCompanySuggestionParams) (db.CompanySuggestion, error)
	insertCompanyStatusSuggestion  func(arg db.InsertCompanyStatusSuggestionParams) (db.CompanyStatusSuggestion, error)
	insertCompanyContactSuggestion func(arg db.InsertCompanyContactSuggestionParams) (db.CompanyContactSuggestion, error)
	insertSuggestionSourceLink     func() (db.SuggestionSourceLink, error)
	markGLEIFProcessed             func(id uuid.UUID) error
	markGLEIFFailed                func(arg db.MarkGLEIFRawInputFailedParams) error
	markCHProcessed                func(id uuid.UUID) error
	markCHFailed                   func(arg db.MarkCompaniesHouseRawInputFailedParams) error
	markBrregProcessed             func(id uuid.UUID) error
	markBrregFailed                func(arg db.MarkBrregRawInputFailedParams) error
}

func (q *mockQuerier) ClaimPendingGLEIFRawInputs(ctx context.Context, arg db.ClaimPendingGLEIFRawInputsParams) ([]db.GleifCompanyRawInput, error) {
	if q.claimGLEIF != nil {
		return q.claimGLEIF(), nil
	}
	return nil, nil
}
func (q *mockQuerier) ClaimPendingCompaniesHouseRawInputs(ctx context.Context, arg db.ClaimPendingCompaniesHouseRawInputsParams) ([]db.CompaniesHouseCompanyRawInput, error) {
	if q.claimCH != nil {
		return q.claimCH(), nil
	}
	return nil, nil
}
func (q *mockQuerier) ClaimPendingBrregRawInputs(ctx context.Context, arg db.ClaimPendingBrregRawInputsParams) ([]db.BrregCompanyRawInput, error) {
	if q.claimBrreg != nil {
		return q.claimBrreg(), nil
	}
	return nil, nil
}
func (q *mockQuerier) GetCompanyByLEI(ctx context.Context, lei *string) (db.Company, error) {
	if q.getCompanyByLEI != nil {
		return q.getCompanyByLEI(lei)
	}
	return db.Company{}, nil
}
func (q *mockQuerier) GetCompanyByRegistrationAndCountry(ctx context.Context, arg db.GetCompanyByRegistrationAndCountryParams) (db.Company, error) {
	if q.getCompanyByRegAndCountry != nil {
		return q.getCompanyByRegAndCountry(arg.RegistrationNumber, arg.IsoAlpha2)
	}
	return db.Company{}, nil
}
func (q *mockQuerier) GetSourceByName(ctx context.Context, name string) (db.DataSource, error) {
	if q.getSourceByName != nil {
		return q.getSourceByName(name)
	}
	return db.DataSource{}, nil
}
func (q *mockQuerier) GetCountryIDByISO2(ctx context.Context, iso string) (uuid.UUID, error) {
	if q.getCountryIDByISO2 != nil {
		return q.getCountryIDByISO2(iso)
	}
	return uuid.UUID{}, nil
}
func (q *mockQuerier) InsertCompanySuggestion(ctx context.Context, arg db.InsertCompanySuggestionParams) (db.CompanySuggestion, error) {
	if q.insertCompanySuggestion != nil {
		return q.insertCompanySuggestion(arg)
	}
	return db.CompanySuggestion{ID: uuid.New()}, nil
}
func (q *mockQuerier) InsertCompanyStatusSuggestion(ctx context.Context, arg db.InsertCompanyStatusSuggestionParams) (db.CompanyStatusSuggestion, error) {
	if q.insertCompanyStatusSuggestion != nil {
		return q.insertCompanyStatusSuggestion(arg)
	}
	return db.CompanyStatusSuggestion{ID: uuid.New()}, nil
}
func (q *mockQuerier) InsertCompanyContactSuggestion(ctx context.Context, arg db.InsertCompanyContactSuggestionParams) (db.CompanyContactSuggestion, error) {
	if q.insertCompanyContactSuggestion != nil {
		return q.insertCompanyContactSuggestion(arg)
	}
	return db.CompanyContactSuggestion{ID: uuid.New()}, nil
}
func (q *mockQuerier) InsertSuggestionSourceLink(ctx context.Context, arg db.InsertSuggestionSourceLinkParams) (db.SuggestionSourceLink, error) {
	if q.insertSuggestionSourceLink != nil {
		return q.insertSuggestionSourceLink()
	}
	return db.SuggestionSourceLink{}, nil
}
func (q *mockQuerier) MarkGLEIFRawInputProcessed(ctx context.Context, id uuid.UUID) error {
	if q.markGLEIFProcessed != nil {
		return q.markGLEIFProcessed(id)
	}
	return nil
}
func (q *mockQuerier) MarkGLEIFRawInputFailed(ctx context.Context, arg db.MarkGLEIFRawInputFailedParams) error {
	if q.markGLEIFFailed != nil {
		return q.markGLEIFFailed(arg)
	}
	return nil
}
func (q *mockQuerier) MarkCompaniesHouseRawInputProcessed(ctx context.Context, id uuid.UUID) error {
	if q.markCHProcessed != nil {
		return q.markCHProcessed(id)
	}
	return nil
}
func (q *mockQuerier) MarkCompaniesHouseRawInputFailed(ctx context.Context, arg db.MarkCompaniesHouseRawInputFailedParams) error {
	if q.markCHFailed != nil {
		return q.markCHFailed(arg)
	}
	return nil
}
func (q *mockQuerier) MarkBrregRawInputProcessed(ctx context.Context, id uuid.UUID) error {
	if q.markBrregProcessed != nil {
		return q.markBrregProcessed(id)
	}
	return nil
}
func (q *mockQuerier) MarkBrregRawInputFailed(ctx context.Context, arg db.MarkBrregRawInputFailedParams) error {
	if q.markBrregFailed != nil {
		return q.markBrregFailed(arg)
	}
	return nil
}
