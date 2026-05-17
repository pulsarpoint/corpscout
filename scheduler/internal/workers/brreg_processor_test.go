package workers_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	db "github.com/pulsarpoint/corpscout/scheduler/internal/db/gen"
	"github.com/pulsarpoint/corpscout/scheduler/internal/workers"
)

func TestBrregProcessor_NewCompany_CreatesSuggestionWithWebsite(t *testing.T) {
	ctx := context.Background()
	rawID := uuid.New()
	sourceID := uuid.New()
	website := "https://example.no"
	payload := json.RawMessage(`{"organisasjonsnummer":"123456789","navn":"Norsk AS","hjemmeside":"https://example.no"}`)

	rawRow := db.BrregCompanyRawInput{
		ID:               rawID,
		OrganizationNumber: "123456789",
		OrganizationName: ptrStr("Norsk AS"),
		Website:          ptrStr(website),
		RawPayload:       payload,
		PayloadHash:      "br123",
		ProcessingStatus: "processing",
	}

	companySuggestionCreated := false
	contactSuggestionCreated := false
	countryID := uuid.New()

	q := &mockQuerier{
		claimBrreg: func() []db.BrregCompanyRawInput { return []db.BrregCompanyRawInput{rawRow} },
		getCompanyByRegAndCountry: func(reg *string, iso string) (db.Company, error) {
			assert.Equal(t, "NO", iso)
			return db.Company{}, pgxErrNoRows
		},
		getSourceByName: func(name string) (db.DataSource, error) {
			return db.DataSource{ID: sourceID, Name: name}, nil
		},
		getCountryIDByISO2: func(iso string) (uuid.UUID, error) {
			assert.Equal(t, "NO", iso)
			return countryID, nil
		},
		insertCompanySuggestion: func(arg db.InsertCompanySuggestionParams) (db.CompanySuggestion, error) {
			assert.True(t, arg.ProposedCountryID.Valid, "must set proposed_country_id")
			companySuggestionCreated = true
			return db.CompanySuggestion{ID: uuid.New()}, nil
		},
		insertCompanyContactSuggestion: func(arg db.InsertCompanyContactSuggestionParams) (db.CompanyContactSuggestion, error) {
			assert.Equal(t, "website", arg.ContactKind)
			contactSuggestionCreated = true
			return db.CompanyContactSuggestion{ID: uuid.New()}, nil
		},
		insertSuggestionSourceLink: func() (db.SuggestionSourceLink, error) {
			return db.SuggestionSourceLink{}, nil
		},
		markBrregProcessed: func(id uuid.UUID) error { return nil },
	}

	proc := workers.NewBrregProcessor(q)
	require.NoError(t, proc.ProcessBatch(ctx, "brreg"))
	assert.True(t, companySuggestionCreated)
	assert.True(t, contactSuggestionCreated, "website should create a contact suggestion")
}
