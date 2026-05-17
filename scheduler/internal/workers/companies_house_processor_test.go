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

func TestCompaniesHouseProcessor_NewCompany_CreatesSuggestion(t *testing.T) {
	ctx := context.Background()
	rawID := uuid.New()
	sourceID := uuid.New()
	payload := json.RawMessage(`{"company_number":"12345678","company_name":"UK Test Ltd","company_status":"active"}`)

	rawRow := db.CompaniesHouseCompanyRawInput{
		ID:               rawID,
		CompanyNumber:    "12345678",
		CompanyName:      ptrStr("UK Test Ltd"),
		CompanyStatus:    ptrStr("active"),
		RawPayload:       payload,
		PayloadHash:      "ch123",
		ProcessingStatus: "processing",
	}

	created := false
	countryID := uuid.New()
	q := &mockQuerier{
		claimCH: func() []db.CompaniesHouseCompanyRawInput { return []db.CompaniesHouseCompanyRawInput{rawRow} },
		getCompanyByRegAndCountry: func(reg *string, iso string) (db.Company, error) {
			assert.Equal(t, "GB", iso)
			return db.Company{}, pgxErrNoRows
		},
		getSourceByName: func(name string) (db.DataSource, error) {
			return db.DataSource{ID: sourceID, Name: name}, nil
		},
		getCountryIDByISO2: func(iso string) (uuid.UUID, error) {
			assert.Equal(t, "GB", iso)
			return countryID, nil
		},
		insertCompanySuggestion: func(arg db.InsertCompanySuggestionParams) (db.CompanySuggestion, error) {
			assert.Equal(t, "UK Test Ltd", arg.ProposedDisplayName)
			assert.True(t, arg.ProposedCountryID.Valid, "must set proposed_country_id")
			created = true
			return db.CompanySuggestion{ID: uuid.New()}, nil
		},
		insertSuggestionSourceLink: func() (db.SuggestionSourceLink, error) {
			return db.SuggestionSourceLink{}, nil
		},
		markCHProcessed: func(id uuid.UUID) error { return nil },
	}

	proc := workers.NewCompaniesHouseProcessor(q)
	require.NoError(t, proc.ProcessBatch(ctx, "companies_house"))
	assert.True(t, created)
}
