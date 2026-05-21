package workers_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	db "github.com/pulsarpoint/corpscout/scheduler/internal/db/gen"
	"github.com/pulsarpoint/corpscout/scheduler/internal/workers"
)

func TestGLEIFProcessor_NewCompany_CreatesSuggestion(t *testing.T) {
	ctx := context.Background()

	runID := uuid.New()
	rawID := uuid.New()
	sourceID := uuid.New()
	countryID := uuid.New()
	payload := json.RawMessage(`{"lei":"TEST123","legalName":"Test Corp","registration_status":"ISSUED"}`)

	rawRow := db.GleifCompanyRawInput{
		ID:                      rawID,
		SourcePullRunID:         pgtype.UUID{Bytes: runID, Valid: true},
		Lei:                     "TEST123",
		LegalName:               ptrStr("Test Corp"),
		HeadquartersCountryCode: ptrStr("GB"),
		RawPayload:              payload,
		PayloadHash:             "abc123",
		ProcessingStatus:        "processing",
		ProcessingLeaseUntil:    pgTypeTZ(time.Now().Add(30 * time.Second)),
	}

	suggestionCreated := false
	linkCreated := false
	markedProcessed := false

	claimCalls := 0
	q := &mockQuerier{
		claimGLEIF: func() []db.GleifCompanyRawInput {
			claimCalls++
			if claimCalls == 1 {
				return []db.GleifCompanyRawInput{rawRow}
			}
			return nil
		},
		getCompanyByLEI: func(lei *string) (db.Company, error) {
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
			assert.Equal(t, "Test Corp", arg.ProposedDisplayName)
			assert.True(t, arg.ProposedCountryID.Valid, "must set proposed_country_id")
			suggestionCreated = true
			return db.CompanySuggestion{ID: uuid.New()}, nil
		},
		insertSuggestionSourceLink: func() (db.SuggestionSourceLink, error) {
			linkCreated = true
			return db.SuggestionSourceLink{}, nil
		},
		markGLEIFProcessed: func(id uuid.UUID) error {
			assert.Equal(t, rawID, id)
			markedProcessed = true
			return nil
		},
	}

	proc := workers.NewGLEIFProcessor(q)
	err := proc.ProcessBatch(ctx, "gleif")
	require.NoError(t, err)

	assert.True(t, suggestionCreated, "must create company suggestion for unknown LEI")
	assert.True(t, linkCreated, "must create suggestion source link")
	assert.True(t, markedProcessed, "must mark raw input processed")
}

func TestGLEIFProcessor_ExistingCompany_CreatesStatusSuggestion(t *testing.T) {
	ctx := context.Background()

	companyID := uuid.New()
	rawID := uuid.New()
	sourceID := uuid.New()
	payload := json.RawMessage(`{"lei":"EXIST456","legalName":"New Legal Name","registration_status":"LAPSED"}`)

	rawRow := db.GleifCompanyRawInput{
		ID:                 rawID,
		Lei:                "EXIST456",
		LegalName:          ptrStr("New Legal Name"),
		RegistrationStatus: ptrStr("LAPSED"),
		RawPayload:         payload,
		PayloadHash:        "def456",
		ProcessingStatus:   "processing",
	}

	statusSuggestionCreated := false

	claimCalls2 := 0
	q := &mockQuerier{
		claimGLEIF: func() []db.GleifCompanyRawInput {
			claimCalls2++
			if claimCalls2 == 1 {
				return []db.GleifCompanyRawInput{rawRow}
			}
			return nil
		},
		getCompanyByLEI: func(lei *string) (db.Company, error) {
			lei456 := "EXIST456"
			return db.Company{ID: companyID, Lei: &lei456}, nil
		},
		getSourceByName: func(name string) (db.DataSource, error) {
			return db.DataSource{ID: sourceID, Name: name}, nil
		},
		insertCompanyStatusSuggestion: func(arg db.InsertCompanyStatusSuggestionParams) (db.CompanyStatusSuggestion, error) {
			statusSuggestionCreated = true
			return db.CompanyStatusSuggestion{ID: uuid.New()}, nil
		},
		insertSuggestionSourceLink: func() (db.SuggestionSourceLink, error) {
			return db.SuggestionSourceLink{}, nil
		},
		markGLEIFProcessed: func(id uuid.UUID) error { return nil },
	}

	proc := workers.NewGLEIFProcessor(q)
	err := proc.ProcessBatch(ctx, "gleif")
	require.NoError(t, err)
	assert.True(t, statusSuggestionCreated, "must create status suggestion for existing company")
}
