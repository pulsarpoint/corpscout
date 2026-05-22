package service

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	db "github.com/pulsarpoint/corpscout/scheduler/internal/db/gen"
)

func TestBuildCVRRawCompanyCandidateFallsBackToNormalizedPayload(t *testing.T) {
	rowID := uuid.New()
	row := db.CvrCompanyRawInput{
		ID:                rowID,
		SourceNativeID:    "12345678",
		CvrNumber:         "12345678",
		CompanyName:       ptrStringValue("Payload Fallback ApS"),
		ProcessingStatus:  "pending",
		TranslationStatus: "translated",
		RawPayloadEn: []byte(`{
			"identity": {"registration_number": "12345678", "name": "Payload Fallback ApS"},
			"status": "Registered",
			"contacts": {
				"website": "https://payload.example.dk",
				"email": "payload@example.dk",
				"phone": "+4511122233"
			}
		}`),
	}

	candidate, err := buildCVRRawCompanyCandidate(row, db.DataSource{Name: "cvr"})
	require.NoError(t, err)

	require.Equal(t, "Payload Fallback ApS", candidate.displayName)
	require.Equal(t, "DK", candidate.countryISO2)
	require.Equal(t, "12345678", *candidate.registrationNumber)
	require.Equal(t, "https://payload.example.dk", *candidate.website)
	require.Equal(t, "Registered", *candidate.registrationStatus)
	require.Len(t, candidate.emails, 1)
	require.Equal(t, "payload@example.dk", candidate.emails[0].Value)
	require.Len(t, candidate.phones, 1)
	require.Equal(t, "+4511122233", candidate.phones[0].Value)
}

func TestBuildAriregisterRawCompanyCandidateFallsBackToNormalizedPayload(t *testing.T) {
	rowID := uuid.New()
	row := db.AriregisterCompanyRawInput{
		ID:                rowID,
		SourceNativeID:    "10000001",
		RegistryCode:      "10000001",
		LegalName:         ptrStringValue("Payload Fallback OU"),
		ProcessingStatus:  "pending",
		TranslationStatus: "translated",
		RawPayloadEn: []byte(`{
			"identity": {"registration_number": "10000001", "name": "Payload Fallback OU"},
			"status": "Registered",
			"contacts": {
				"website": "https://payload.example.ee",
				"email": "payload@example.ee",
				"phone": "+3725550100"
			}
		}`),
	}

	candidate, err := buildAriregisterRawCompanyCandidate(row, db.DataSource{Name: "ariregister"})
	require.NoError(t, err)

	require.Equal(t, "Payload Fallback OU", candidate.displayName)
	require.Equal(t, "EE", candidate.countryISO2)
	require.Equal(t, "10000001", *candidate.registrationNumber)
	require.Equal(t, "https://payload.example.ee", *candidate.website)
	require.Equal(t, "Registered", *candidate.registrationStatus)
	require.Len(t, candidate.emails, 1)
	require.Equal(t, "payload@example.ee", candidate.emails[0].Value)
	require.Len(t, candidate.phones, 1)
	require.Equal(t, "+3725550100", candidate.phones[0].Value)
}

func TestBuildCVRRawCompanyCandidateKeepsTableScalarsBeforePayload(t *testing.T) {
	row := db.CvrCompanyRawInput{
		ID:                 uuid.New(),
		SourceNativeID:     "12345678",
		CvrNumber:          "12345678",
		CompanyName:        ptrStringValue("Table Wins ApS"),
		Website:            ptrStringValue("https://table.example.dk"),
		Email:              ptrStringValue("table@example.dk"),
		Phone:              ptrStringValue("+4512345678"),
		RegistrationStatus: ptrStringValue("registered"),
		ProcessingStatus:   "pending",
		TranslationStatus:  "translated",
		RawPayloadEn: []byte(`{
			"status": "Payload status",
			"contacts": {
				"website": "https://payload.example.dk",
				"email": "payload@example.dk",
				"phone": "+4599999999"
			}
		}`),
	}

	candidate, err := buildCVRRawCompanyCandidate(row, db.DataSource{Name: "cvr"})
	require.NoError(t, err)

	require.Equal(t, "https://table.example.dk", *candidate.website)
	require.Equal(t, "registered", *candidate.registrationStatus)
	require.Len(t, candidate.emails, 1)
	require.Equal(t, "table@example.dk", candidate.emails[0].Value)
	require.Len(t, candidate.phones, 1)
	require.Equal(t, "+4512345678", candidate.phones[0].Value)
}

func TestBuildCVRRawCompanyCandidatePreservesOwnersAndBeneficialOwners(t *testing.T) {
	row := db.CvrCompanyRawInput{
		ID:                uuid.New(),
		SourceNativeID:    "12345678",
		CvrNumber:         "12345678",
		CompanyName:       ptrStringValue("Ownership ApS"),
		ProcessingStatus:  "pending",
		TranslationStatus: "translated",
		RawPayloadEn: []byte(`{
			"owners": [{"name": "Direct Owner ApS"}],
			"beneficial_owners": [{"name": "Beneficial Owner"}]
		}`),
	}

	candidate, err := buildCVRRawCompanyCandidate(row, db.DataSource{Name: "cvr"})
	require.NoError(t, err)

	require.Len(t, candidate.ownership, 2)
	require.Equal(t, "Direct Owner ApS", candidate.ownership[0].Data["name"])
	require.Equal(t, "Beneficial Owner", candidate.ownership[1].Data["name"])
}

func TestBuildAriregisterRawCompanyCandidatePreservesShareholdersAndBeneficialOwners(t *testing.T) {
	row := db.AriregisterCompanyRawInput{
		ID:                uuid.New(),
		SourceNativeID:    "10000001",
		RegistryCode:      "10000001",
		LegalName:         ptrStringValue("Ownership OU"),
		ProcessingStatus:  "pending",
		TranslationStatus: "translated",
		RawPayloadEn: []byte(`{
			"shareholders": [{"name": "Direct Shareholder OU"}],
			"beneficial_owners": [{"name": "Beneficial Owner"}]
		}`),
	}

	candidate, err := buildAriregisterRawCompanyCandidate(row, db.DataSource{Name: "ariregister"})
	require.NoError(t, err)

	require.Len(t, candidate.ownership, 2)
	require.Equal(t, "Direct Shareholder OU", candidate.ownership[0].Data["name"])
	require.Equal(t, "Beneficial Owner", candidate.ownership[1].Data["name"])
}
