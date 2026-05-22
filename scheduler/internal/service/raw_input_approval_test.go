package service_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
	pgx "github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	pgxmock "github.com/pashagolub/pgxmock/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulsarpoint/corpscout/scheduler/internal/service"
)

func sourceRows(sourceID uuid.UUID, name, inputTable string) *pgxmock.Rows {
	return pgxmock.NewRows([]string{
		"id", "name", "display_name", "description", "source_group", "input_table_name",
		"pull_task_type", "processor_task_type", "enabled", "schedule_kind", "schedule_expression",
		"config", "last_started_at", "last_success_at", "last_failed_at",
		"last_source_marker_type", "last_source_marker", "last_source_modified_at",
		"last_error", "consecutive_failures", "created_at", "updated_at",
		"schedule_enabled", "country_id", "capabilities", "requires_translation",
	}).AddRow(
		sourceID, name, (*string)(nil), (*string)(nil), "registry", inputTable,
		"source_pull", (*string)(nil), true, "interval", (*string)(nil),
		[]byte("{}"), pgtype.Timestamptz{}, pgtype.Timestamptz{}, pgtype.Timestamptz{},
		(*string)(nil), (*string)(nil), pgtype.Timestamptz{},
		(*string)(nil), int32(0), time.Time{}, time.Time{},
		true, pgtype.UUID{}, []string{"company_name"}, false,
	)
}

func chApprovalRows(rowID, runID uuid.UUID, companyNumber string, companyName string) *pgxmock.Rows {
	return pgxmock.NewRows([]string{
		"id", "source_pull_run_id", "source_native_id", "company_number", "company_name",
		"company_status", "company_type", "country_iso2",
		"source_updated_at", "raw_payload", "payload_hash",
		"first_seen_at", "last_seen_at",
		"processing_status", "processing_attempts", "processing_error",
		"processing_lease_by", "processing_lease_until", "processed_at",
		"created_at", "updated_at", "run_id",
	}).AddRow(
		rowID, pgUUID(runID), companyNumber, companyNumber, &companyName,
		ptrString("active"), ptrString("ltd"), ptrString("GB"),
		pgtype.Timestamptz{}, []byte(`{"company_number":"`+companyNumber+`"}`), "hash1",
		time.Time{}, time.Time{},
		"pending", int32(0), (*string)(nil),
		(*string)(nil), pgtype.Timestamptz{}, pgtype.Timestamptz{},
		time.Time{}, time.Time{}, (*string)(nil),
	)
}

func brregApprovalRows(rowID uuid.UUID, orgNumber, orgName, translationStatus string, rawPayloadEn []byte) *pgxmock.Rows {
	return pgxmock.NewRows([]string{
		"id", "source_pull_run_id", "source_native_id", "organization_number",
		"organization_name", "registration_status", "website", "country_iso2",
		"source_updated_at", "raw_payload", "payload_hash",
		"first_seen_at", "last_seen_at",
		"processing_status", "processing_attempts", "processing_error",
		"processing_lease_by", "processing_lease_until", "processed_at",
		"created_at", "updated_at", "run_id",
		"raw_payload_en", "translation_status",
		"translation_attempts", "translation_error", "translation_model",
		"translation_prompt_version", "translated_at",
		"translation_lease_by", "translation_lease_until",
		"translation_fx_source", "translation_fx_rate_date",
	}).AddRow(
		rowID, pgtype.UUID{}, orgNumber, orgNumber,
		&orgName, ptrString("registered"), (*string)(nil), ptrString("NO"),
		pgtype.Timestamptz{}, []byte(`{"organisasjonsnummer":"`+orgNumber+`"}`), "hash1",
		time.Time{}, time.Time{},
		"pending", int32(0), (*string)(nil),
		(*string)(nil), pgtype.Timestamptz{}, pgtype.Timestamptz{},
		time.Time{}, time.Time{}, (*string)(nil),
		rawPayloadEn, translationStatus,
		int32(0), (*string)(nil), (*string)(nil),
		(*string)(nil), pgtype.Timestamptz{},
		(*string)(nil), pgtype.Timestamptz{},
		(*string)(nil), pgtype.Date{},
	)
}

func cvrApprovalRows(rowID uuid.UUID, cvrNumber, companyName, translationStatus string, rawPayloadEn []byte) *pgxmock.Rows {
	return pgxmock.NewRows([]string{
		"id", "source_pull_run_id", "source_native_id", "cvr_number", "company_name",
		"registration_status", "company_type", "website", "email", "phone", "country_iso2",
		"source_updated_at", "raw_payload", "raw_payload_en", "payload_hash",
		"first_seen_at", "last_seen_at", "processing_status", "processing_attempts", "processing_error",
		"processing_lease_by", "processing_lease_until", "processed_at", "run_id",
		"translation_status", "translation_attempts", "translation_error", "translation_model",
		"translation_prompt_version", "translated_at", "translation_lease_by", "translation_lease_until",
		"translation_fx_source", "translation_fx_rate_date", "created_at", "updated_at",
	}).AddRow(
		rowID, pgtype.UUID{}, cvrNumber, cvrNumber, &companyName,
		ptrString("registered"), ptrString("aps"), ptrString("https://example.dk"),
		ptrString("info@example.dk"), ptrString("+45 12 34 56 78"), (*string)(nil),
		pgtype.Timestamptz{}, []byte(`{"cvrNummer":"`+cvrNumber+`"}`), rawPayloadEn, "hash1",
		time.Time{}, time.Time{}, "pending", int32(0), (*string)(nil),
		(*string)(nil), pgtype.Timestamptz{}, pgtype.Timestamptz{}, (*string)(nil),
		translationStatus, int32(0), (*string)(nil), (*string)(nil),
		(*string)(nil), pgtype.Timestamptz{}, (*string)(nil), pgtype.Timestamptz{},
		(*string)(nil), pgtype.Date{}, time.Time{}, time.Time{},
	)
}

func ariregisterApprovalRows(rowID uuid.UUID, registryCode, legalName, translationStatus string, rawPayloadEn []byte) *pgxmock.Rows {
	return pgxmock.NewRows([]string{
		"id", "source_pull_run_id", "source_native_id", "registry_code", "legal_name",
		"registration_status", "legal_form", "vat_number", "website", "email", "phone", "country_iso2",
		"source_updated_at", "raw_payload", "raw_payload_en", "payload_hash",
		"first_seen_at", "last_seen_at", "processing_status", "processing_attempts", "processing_error",
		"processing_lease_by", "processing_lease_until", "processed_at", "run_id",
		"translation_status", "translation_attempts", "translation_error", "translation_model",
		"translation_prompt_version", "translated_at", "translation_lease_by", "translation_lease_until",
		"translation_fx_source", "translation_fx_rate_date", "created_at", "updated_at",
	}).AddRow(
		rowID, pgtype.UUID{}, registryCode, registryCode, &legalName,
		ptrString("registered"), ptrString("OU"), (*string)(nil), ptrString("https://example.ee"),
		ptrString("info@example.ee"), ptrString("+372 5555 0000"), (*string)(nil),
		pgtype.Timestamptz{}, []byte(`{"registry_code":"`+registryCode+`"}`), rawPayloadEn, "hash1",
		time.Time{}, time.Time{}, "pending", int32(0), (*string)(nil),
		(*string)(nil), pgtype.Timestamptz{}, pgtype.Timestamptz{}, (*string)(nil),
		translationStatus, int32(0), (*string)(nil), (*string)(nil),
		(*string)(nil), pgtype.Timestamptz{}, (*string)(nil), pgtype.Timestamptz{},
		(*string)(nil), pgtype.Date{}, time.Time{}, time.Time{},
	)
}

func ptrString(s string) *string { return &s }

func TestApproveCompanyRawInput_CompaniesHouseCreatesCompanyAndMarksProcessed(t *testing.T) {
	ctx := context.Background()
	rowID := uuid.New()
	runID := uuid.New()
	sourceID := uuid.New()
	countryID := uuid.New()
	companyID := uuid.New()

	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT id, name, display_name`).WithArgs("companies_house").
		WillReturnRows(sourceRows(sourceID, "companies_house", "companies_house_company_raw_inputs"))
	mock.ExpectQuery(`SELECT id, source_pull_run_id, source_native_id, company_number`).WithArgs(rowID).
		WillReturnRows(chApprovalRows(rowID, runID, "12345678", "Example Ltd"))
	mock.ExpectQuery(`SELECT id FROM countries`).WithArgs("GB").
		WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow(countryID))
	mock.ExpectQuery(`SELECT c.id`).WithArgs(ptrString("12345678"), "GB").
		WillReturnError(pgx.ErrNoRows)
	mock.ExpectQuery(`SELECT id, lei, name`).WithArgs("example-ltd").
		WillReturnError(pgx.ErrNoRows)
	mock.ExpectQuery(`INSERT INTO companies`).
		WithArgs(
			"example-ltd", "Example Ltd", countryID, ptrString("12345678"), (*string)(nil),
			"active", (*string)(nil), pgUUID(sourceID), (*string)(nil), (*string)(nil), pgxmock.AnyArg(),
		).
		WillReturnRows(companyRows(companyID, "example-ltd", "Example Ltd", countryID))
	mock.ExpectExec(`UPDATE companies_house_company_raw_inputs`).
		WithArgs(rowID).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))
	mock.ExpectCommit()

	company, err := service.ApproveCompanyRawInput(ctx, mock, "companies_house", rowID, "ops", "")
	require.NoError(t, err)
	assert.Equal(t, "example-ltd", company.CanonicalSlug)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestApproveCompanyRawInput_BrregRequiresTranslation(t *testing.T) {
	ctx := context.Background()
	rowID := uuid.New()
	sourceID := uuid.New()

	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT id, name, display_name`).WithArgs("brreg").
		WillReturnRows(sourceRows(sourceID, "brreg", "brreg_company_raw_inputs"))
	mock.ExpectQuery(`SELECT id, source_pull_run_id, source_native_id, organization_number`).WithArgs(rowID).
		WillReturnRows(brregApprovalRows(rowID, "999888777", "Norsk AS", "pending", nil))
	mock.ExpectRollback()

	_, err = service.ApproveCompanyRawInput(ctx, mock, "brreg", rowID, "ops", "")
	require.Error(t, err)
	assert.True(t, errors.Is(err, service.ErrRawInputRequiresTranslation))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestApproveCompanyRawInput_CVRRequiresTranslation(t *testing.T) {
	ctx := context.Background()
	rowID := uuid.New()
	sourceID := uuid.New()

	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT id, name, display_name`).WithArgs("cvr").
		WillReturnRows(sourceRows(sourceID, "cvr", "cvr_company_raw_inputs"))
	mock.ExpectQuery(`SELECT id, source_pull_run_id, source_native_id, cvr_number`).WithArgs(rowID).
		WillReturnRows(cvrApprovalRows(rowID, "12345678", "Dansk ApS", "pending", nil))
	mock.ExpectRollback()

	_, err = service.ApproveCompanyRawInput(ctx, mock, "cvr", rowID, "ops", "")
	require.Error(t, err)
	assert.True(t, errors.Is(err, service.ErrRawInputRequiresTranslation))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestApproveCompanyRawInput_AriregisterRequiresTranslation(t *testing.T) {
	ctx := context.Background()
	rowID := uuid.New()
	sourceID := uuid.New()

	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT id, name, display_name`).WithArgs("ariregister").
		WillReturnRows(sourceRows(sourceID, "ariregister", "ariregister_company_raw_inputs"))
	mock.ExpectQuery(`SELECT id, source_pull_run_id, source_native_id, registry_code`).WithArgs(rowID).
		WillReturnRows(ariregisterApprovalRows(rowID, "12345678", "Eesti OU", "failed", nil))
	mock.ExpectRollback()

	_, err = service.ApproveCompanyRawInput(ctx, mock, "ariregister", rowID, "ops", "")
	require.Error(t, err)
	assert.True(t, errors.Is(err, service.ErrRawInputRequiresTranslation))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestApproveCompanyRawInput_CVRCreatesCompanyAndPersistsEnrichment(t *testing.T) {
	ctx := context.Background()
	rowID := uuid.New()
	sourceID := uuid.New()
	countryID := uuid.New()
	companyID := uuid.New()
	domainID := uuid.New()

	payload := []byte(`{
		"financials": [{
			"year": 2023,
			"employee_count": 42,
			"revenue_amount": 1234000,
			"revenue_currency": "DKK",
			"profit_amount": 234000
		}],
		"ownership": [{"name": "Unresolved Holding", "ownership_percentage": 25}]
	}`)

	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT id, name, display_name`).WithArgs("cvr").
		WillReturnRows(sourceRows(sourceID, "cvr", "cvr_company_raw_inputs"))
	mock.ExpectQuery(`SELECT id, source_pull_run_id, source_native_id, cvr_number`).WithArgs(rowID).
		WillReturnRows(cvrApprovalRows(rowID, "12345678", "Dansk ApS", "translated", payload))
	mock.ExpectQuery(`SELECT id FROM countries`).WithArgs("DK").
		WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow(countryID))
	mock.ExpectQuery(`SELECT c.id`).WithArgs(ptrString("12345678"), "DK").
		WillReturnError(pgx.ErrNoRows)
	mock.ExpectQuery(`SELECT id, lei, name`).WithArgs("dansk-aps").
		WillReturnError(pgx.ErrNoRows)
	mock.ExpectQuery(`INSERT INTO companies`).
		WithArgs(
			"dansk-aps", "Dansk ApS", countryID, ptrString("12345678"), (*string)(nil),
			"active", ptrString("https://example.dk"), pgUUID(sourceID), (*string)(nil), (*string)(nil), pgxmock.AnyArg(),
		).
		WillReturnRows(companyRows(companyID, "dansk-aps", "Dansk ApS", countryID))
	mock.ExpectQuery(`INSERT INTO company_emails`).WithArgs(
		companyID, "info@example.dk", pgxmock.AnyArg(), "official", pgxmock.AnyArg(), "cvr", pgxmock.AnyArg(),
		jsonContainsArg{`"source":"cvr"`, `"source_input_id":"` + rowID.String() + `"`},
	).WillReturnRows(companyEmailRows(companyID, "info@example.dk", "cvr"))
	mock.ExpectQuery(`INSERT INTO company_phones`).WithArgs(
		companyID, "+45 12 34 56 78", pgxmock.AnyArg(), "official", "cvr", pgxmock.AnyArg(),
		jsonContainsArg{`"source":"cvr"`, `"source_input_id":"` + rowID.String() + `"`},
	).WillReturnRows(companyPhoneRows(companyID, "+45 12 34 56 78", "cvr"))
	mock.ExpectQuery(`INSERT INTO company_financials`).WithArgs(
		companyID, int32(2023), "cvr", ptrInt32(42), ptrInt64(1234000), ptrString("DKK"), (*int64)(nil), ptrInt64(234000), (*int64)(nil),
		jsonContainsArg{`"source":"cvr"`, `"source_input_id":"` + rowID.String() + `"`, `"kind":"financial"`, `"raw_fragments"`},
	).WillReturnRows(companyFinancialRows(companyID, 2023, "cvr"))
	mock.ExpectQuery(`INSERT INTO domains`).WithArgs("example.dk", "registry").
		WillReturnRows(domainRows(domainID, "example.dk", "registry"))
	mock.ExpectQuery(`INSERT INTO company_domains`).WithArgs(
		companyID, domainID, "official_site", "needs_review", "registry_website", int16(95),
		jsonContainsArg{`"source":"cvr"`, `"source_input_id":"` + rowID.String() + `"`, `"domain":"example.dk"`},
	).WillReturnRows(companyDomainRows(companyID, domainID, "registry_website", 95))
	mock.ExpectQuery(`INSERT INTO domains`).WithArgs("example.dk", "registry").
		WillReturnRows(domainRows(domainID, "example.dk", "registry"))
	mock.ExpectQuery(`INSERT INTO company_domains`).WithArgs(
		companyID, domainID, "candidate", "needs_review", "registry_email", int16(45),
		jsonContainsArg{`"source":"cvr"`, `"source_input_id":"` + rowID.String() + `"`, `"domain":"example.dk"`},
	).WillReturnRows(companyDomainRows(companyID, domainID, "registry_email", 45))
	mock.ExpectQuery(`UPDATE companies SET`).WithArgs(
		(*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil), (*int32)(nil),
		[]byte(nil), []byte(nil), jsonContainsArg{`"source":"cvr"`, `"source_input_id":"` + rowID.String() + `"`, "Unresolved Holding"}, (*int32)(nil), (*int64)(nil), companyID,
	).WillReturnRows(companyRows(companyID, "dansk-aps", "Dansk ApS", countryID))
	mock.ExpectExec(`UPDATE cvr_company_raw_inputs`).WithArgs(rowID).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))
	mock.ExpectCommit()

	company, err := service.ApproveCompanyRawInput(ctx, mock, "cvr", rowID, "ops", "")
	require.NoError(t, err)
	assert.Equal(t, "dansk-aps", company.CanonicalSlug)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestApproveCompanyRawInput_AriregisterFindsCompanyAndPersistsEnrichment(t *testing.T) {
	ctx := context.Background()
	rowID := uuid.New()
	sourceID := uuid.New()
	countryID := uuid.New()
	companyID := uuid.New()
	domainID := uuid.New()

	payload := []byte(`{
		"annual_reports": [{
			"year": 2022,
			"indicators": {
				"employee_count": 7,
				"revenue_amount": 550000,
				"profit_amount": 44000
			}
		}],
		"beneficial_owners": [{"name": "Unresolved Owner", "control_type": "beneficial_owner"}]
	}`)

	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT id, name, display_name`).WithArgs("ariregister").
		WillReturnRows(sourceRows(sourceID, "ariregister", "ariregister_company_raw_inputs"))
	mock.ExpectQuery(`SELECT id, source_pull_run_id, source_native_id, registry_code`).WithArgs(rowID).
		WillReturnRows(ariregisterApprovalRows(rowID, "12345678", "Eesti OU", "translated", payload))
	mock.ExpectQuery(`SELECT id FROM countries`).WithArgs("EE").
		WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow(countryID))
	mock.ExpectQuery(`SELECT c.id`).WithArgs(ptrString("12345678"), "EE").
		WillReturnRows(companyRows(companyID, "eesti-ou", "Eesti OU", countryID))
	mock.ExpectQuery(`INSERT INTO company_emails`).WithArgs(
		companyID, "info@example.ee", pgxmock.AnyArg(), "official", pgxmock.AnyArg(), "ariregister", pgxmock.AnyArg(),
		jsonContainsArg{`"source":"ariregister"`, `"source_input_id":"` + rowID.String() + `"`},
	).WillReturnRows(companyEmailRows(companyID, "info@example.ee", "ariregister"))
	mock.ExpectQuery(`INSERT INTO company_phones`).WithArgs(
		companyID, "+372 5555 0000", pgxmock.AnyArg(), "official", "ariregister", pgxmock.AnyArg(),
		jsonContainsArg{`"source":"ariregister"`, `"source_input_id":"` + rowID.String() + `"`},
	).WillReturnRows(companyPhoneRows(companyID, "+372 5555 0000", "ariregister"))
	mock.ExpectQuery(`INSERT INTO company_financials`).WithArgs(
		companyID, int32(2022), "ariregister", ptrInt32(7), ptrInt64(550000), ptrString("EUR"), (*int64)(nil), ptrInt64(44000), (*int64)(nil),
		jsonContainsArg{`"source":"ariregister"`, `"source_input_id":"` + rowID.String() + `"`, `"kind":"financial"`, `"raw_fragments"`},
	).WillReturnRows(companyFinancialRows(companyID, 2022, "ariregister"))
	mock.ExpectQuery(`INSERT INTO domains`).WithArgs("example.ee", "registry").
		WillReturnRows(domainRows(domainID, "example.ee", "registry"))
	mock.ExpectQuery(`INSERT INTO company_domains`).WithArgs(
		companyID, domainID, "official_site", "needs_review", "registry_website", int16(95),
		jsonContainsArg{`"source":"ariregister"`, `"source_input_id":"` + rowID.String() + `"`, `"domain":"example.ee"`},
	).WillReturnRows(companyDomainRows(companyID, domainID, "registry_website", 95))
	mock.ExpectQuery(`INSERT INTO domains`).WithArgs("example.ee", "registry").
		WillReturnRows(domainRows(domainID, "example.ee", "registry"))
	mock.ExpectQuery(`INSERT INTO company_domains`).WithArgs(
		companyID, domainID, "candidate", "needs_review", "registry_email", int16(45),
		jsonContainsArg{`"source":"ariregister"`, `"source_input_id":"` + rowID.String() + `"`, `"domain":"example.ee"`},
	).WillReturnRows(companyDomainRows(companyID, domainID, "registry_email", 45))
	mock.ExpectQuery(`UPDATE companies SET`).WithArgs(
		(*string)(nil), (*string)(nil), (*string)(nil), ptrString("https://example.ee"), (*int32)(nil),
		[]byte(nil), []byte(nil), jsonContainsArg{`"source":"ariregister"`, `"source_input_id":"` + rowID.String() + `"`, "Unresolved Owner"}, (*int32)(nil), (*int64)(nil), companyID,
	).WillReturnRows(companyRows(companyID, "eesti-ou", "Eesti OU", countryID))
	mock.ExpectExec(`UPDATE ariregister_company_raw_inputs`).WithArgs(rowID).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))
	mock.ExpectCommit()

	company, err := service.ApproveCompanyRawInput(ctx, mock, "ariregister", rowID, "ops", "")
	require.NoError(t, err)
	assert.Equal(t, companyID, company.ID)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestApproveCompanyRawInput_CVRSkipsReviewedFinancialConflict(t *testing.T) {
	ctx := context.Background()
	rowID := uuid.New()
	sourceID := uuid.New()
	countryID := uuid.New()
	companyID := uuid.New()
	domainID := uuid.New()

	payload := []byte(`{
		"financials": [{
			"year": 2023,
			"revenue_amount": 1234000,
			"revenue_currency": "DKK"
		}]
	}`)

	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT id, name, display_name`).WithArgs("cvr").
		WillReturnRows(sourceRows(sourceID, "cvr", "cvr_company_raw_inputs"))
	mock.ExpectQuery(`SELECT id, source_pull_run_id, source_native_id, cvr_number`).WithArgs(rowID).
		WillReturnRows(cvrApprovalRows(rowID, "12345678", "Dansk ApS", "translated", payload))
	mock.ExpectQuery(`SELECT id FROM countries`).WithArgs("DK").
		WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow(countryID))
	mock.ExpectQuery(`SELECT c.id`).WithArgs(ptrString("12345678"), "DK").
		WillReturnRows(companyRows(companyID, "dansk-aps", "Dansk ApS", countryID))
	mock.ExpectQuery(`INSERT INTO company_emails`).WithArgs(
		companyID, "info@example.dk", pgxmock.AnyArg(), "official", pgxmock.AnyArg(), "cvr", pgxmock.AnyArg(), pgxmock.AnyArg(),
	).WillReturnRows(companyEmailRows(companyID, "info@example.dk", "cvr"))
	mock.ExpectQuery(`INSERT INTO company_phones`).WithArgs(
		companyID, "+45 12 34 56 78", pgxmock.AnyArg(), "official", "cvr", pgxmock.AnyArg(), pgxmock.AnyArg(),
	).WillReturnRows(companyPhoneRows(companyID, "+45 12 34 56 78", "cvr"))
	mock.ExpectQuery(`INSERT INTO company_financials`).WithArgs(
		companyID, int32(2023), "cvr", (*int32)(nil), ptrInt64(1234000), ptrString("DKK"), (*int64)(nil), (*int64)(nil), (*int64)(nil),
		jsonContainsArg{`"source":"cvr"`, `"source_input_id":"` + rowID.String() + `"`, `"kind":"financial"`, `"raw_fragments"`},
	).WillReturnError(pgx.ErrNoRows)
	mock.ExpectQuery(`INSERT INTO domains`).WithArgs("example.dk", "registry").
		WillReturnRows(domainRows(domainID, "example.dk", "registry"))
	mock.ExpectQuery(`INSERT INTO company_domains`).WithArgs(
		companyID, domainID, "official_site", "needs_review", "registry_website", int16(95),
		jsonContainsArg{`"source":"cvr"`, `"source_input_id":"` + rowID.String() + `"`, `"domain":"example.dk"`},
	).WillReturnRows(companyDomainRows(companyID, domainID, "registry_website", 95))
	mock.ExpectQuery(`INSERT INTO domains`).WithArgs("example.dk", "registry").
		WillReturnRows(domainRows(domainID, "example.dk", "registry"))
	mock.ExpectQuery(`INSERT INTO company_domains`).WithArgs(
		companyID, domainID, "candidate", "needs_review", "registry_email", int16(45),
		jsonContainsArg{`"source":"cvr"`, `"source_input_id":"` + rowID.String() + `"`, `"domain":"example.dk"`},
	).WillReturnRows(companyDomainRows(companyID, domainID, "registry_email", 45))
	mock.ExpectQuery(`UPDATE companies SET`).WithArgs(
		(*string)(nil), (*string)(nil), (*string)(nil), ptrString("https://example.dk"), (*int32)(nil),
		[]byte(nil), []byte(nil), []byte(nil), (*int32)(nil), (*int64)(nil), companyID,
	).WillReturnRows(companyRows(companyID, "dansk-aps", "Dansk ApS", countryID))
	mock.ExpectExec(`UPDATE cvr_company_raw_inputs`).WithArgs(rowID).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))
	mock.ExpectCommit()

	company, err := service.ApproveCompanyRawInput(ctx, mock, "cvr", rowID, "ops", "")
	require.NoError(t, err)
	assert.Equal(t, companyID, company.ID)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestApproveCompanyRawInput_AriregisterFinancialIndicatorsIncludeEvidence(t *testing.T) {
	ctx := context.Background()
	rowID := uuid.New()
	sourceID := uuid.New()
	countryID := uuid.New()
	companyID := uuid.New()
	domainID := uuid.New()

	payload := []byte(`{
		"financials": [
			{"year": 2024, "indicator": "Revenue", "value": 1250000},
			{"year": 2024, "indicator": "Profit", "value": 175000},
			{"year": 2024, "indicator": "Employees", "value": 18}
		]
	}`)

	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT id, name, display_name`).WithArgs("ariregister").
		WillReturnRows(sourceRows(sourceID, "ariregister", "ariregister_company_raw_inputs"))
	mock.ExpectQuery(`SELECT id, source_pull_run_id, source_native_id, registry_code`).WithArgs(rowID).
		WillReturnRows(ariregisterApprovalRows(rowID, "12345678", "Eesti OU", "translated", payload))
	mock.ExpectQuery(`SELECT id FROM countries`).WithArgs("EE").
		WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow(countryID))
	mock.ExpectQuery(`SELECT c.id`).WithArgs(ptrString("12345678"), "EE").
		WillReturnRows(companyRows(companyID, "eesti-ou", "Eesti OU", countryID))
	mock.ExpectQuery(`INSERT INTO company_emails`).WithArgs(
		companyID, "info@example.ee", pgxmock.AnyArg(), "official", pgxmock.AnyArg(), "ariregister", pgxmock.AnyArg(), pgxmock.AnyArg(),
	).WillReturnRows(companyEmailRows(companyID, "info@example.ee", "ariregister"))
	mock.ExpectQuery(`INSERT INTO company_phones`).WithArgs(
		companyID, "+372 5555 0000", pgxmock.AnyArg(), "official", "ariregister", pgxmock.AnyArg(), pgxmock.AnyArg(),
	).WillReturnRows(companyPhoneRows(companyID, "+372 5555 0000", "ariregister"))
	mock.ExpectQuery(`INSERT INTO company_financials`).WithArgs(
		companyID, int32(2024), "ariregister", ptrInt32(18), ptrInt64(1250000), ptrString("EUR"), (*int64)(nil), ptrInt64(175000), (*int64)(nil),
		jsonContainsArg{`"source":"ariregister"`, `"source_input_id":"` + rowID.String() + `"`, `"source_native_id":"12345678"`, `"kind":"financial"`, `"source_snapshot"`, `"payload_hash":"hash1"`, `"original_fields"`, `"indicator"`, `"value"`, `"raw_fragments"`},
	).WillReturnRows(companyFinancialRows(companyID, 2024, "ariregister"))
	mock.ExpectQuery(`INSERT INTO domains`).WithArgs("example.ee", "registry").
		WillReturnRows(domainRows(domainID, "example.ee", "registry"))
	mock.ExpectQuery(`INSERT INTO company_domains`).WithArgs(
		companyID, domainID, "official_site", "needs_review", "registry_website", int16(95),
		jsonContainsArg{`"source":"ariregister"`, `"source_input_id":"` + rowID.String() + `"`, `"domain":"example.ee"`},
	).WillReturnRows(companyDomainRows(companyID, domainID, "registry_website", 95))
	mock.ExpectQuery(`INSERT INTO domains`).WithArgs("example.ee", "registry").
		WillReturnRows(domainRows(domainID, "example.ee", "registry"))
	mock.ExpectQuery(`INSERT INTO company_domains`).WithArgs(
		companyID, domainID, "candidate", "needs_review", "registry_email", int16(45),
		jsonContainsArg{`"source":"ariregister"`, `"source_input_id":"` + rowID.String() + `"`, `"domain":"example.ee"`},
	).WillReturnRows(companyDomainRows(companyID, domainID, "registry_email", 45))
	mock.ExpectQuery(`UPDATE companies SET`).WithArgs(
		(*string)(nil), (*string)(nil), (*string)(nil), ptrString("https://example.ee"), (*int32)(nil),
		[]byte(nil), []byte(nil), []byte(nil), (*int32)(nil), (*int64)(nil), companyID,
	).WillReturnRows(companyRows(companyID, "eesti-ou", "Eesti OU", countryID))
	mock.ExpectExec(`UPDATE ariregister_company_raw_inputs`).WithArgs(rowID).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))
	mock.ExpectCommit()

	company, err := service.ApproveCompanyRawInput(ctx, mock, "ariregister", rowID, "ops", "")
	require.NoError(t, err)
	assert.Equal(t, companyID, company.ID)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestApproveCompanyRawInput_CVRIncompleteFinancialFragmentIncludesEvidence(t *testing.T) {
	ctx := context.Background()
	rowID := uuid.New()
	sourceID := uuid.New()
	countryID := uuid.New()
	companyID := uuid.New()
	domainID := uuid.New()

	payload := []byte(`{
		"financials": [{
			"year": 2024,
			"note": "Annual report missing"
		}]
	}`)

	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT id, name, display_name`).WithArgs("cvr").
		WillReturnRows(sourceRows(sourceID, "cvr", "cvr_company_raw_inputs"))
	mock.ExpectQuery(`SELECT id, source_pull_run_id, source_native_id, cvr_number`).WithArgs(rowID).
		WillReturnRows(cvrApprovalRows(rowID, "12345678", "Dansk ApS", "translated", payload))
	mock.ExpectQuery(`SELECT id FROM countries`).WithArgs("DK").
		WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow(countryID))
	mock.ExpectQuery(`SELECT c.id`).WithArgs(ptrString("12345678"), "DK").
		WillReturnRows(companyRows(companyID, "dansk-aps", "Dansk ApS", countryID))
	mock.ExpectQuery(`INSERT INTO company_emails`).WithArgs(
		companyID, "info@example.dk", pgxmock.AnyArg(), "official", pgxmock.AnyArg(), "cvr", pgxmock.AnyArg(), pgxmock.AnyArg(),
	).WillReturnRows(companyEmailRows(companyID, "info@example.dk", "cvr"))
	mock.ExpectQuery(`INSERT INTO company_phones`).WithArgs(
		companyID, "+45 12 34 56 78", pgxmock.AnyArg(), "official", "cvr", pgxmock.AnyArg(), pgxmock.AnyArg(),
	).WillReturnRows(companyPhoneRows(companyID, "+45 12 34 56 78", "cvr"))
	mock.ExpectQuery(`INSERT INTO company_financials`).WithArgs(
		companyID, int32(2024), "cvr", (*int32)(nil), (*int64)(nil), (*string)(nil), (*int64)(nil), (*int64)(nil), (*int64)(nil),
		jsonContainsArg{`"source":"cvr"`, `"source_input_id":"` + rowID.String() + `"`, `"source_native_id":"12345678"`, `"kind":"financial"`, `"source_snapshot"`, `"payload_hash":"hash1"`, `"original_fields"`, `"note"`, `"raw_fragments"`, `"Annual report missing"`},
	).WillReturnRows(companyFinancialRows(companyID, 2024, "cvr"))
	mock.ExpectQuery(`INSERT INTO domains`).WithArgs("example.dk", "registry").
		WillReturnRows(domainRows(domainID, "example.dk", "registry"))
	mock.ExpectQuery(`INSERT INTO company_domains`).WithArgs(
		companyID, domainID, "official_site", "needs_review", "registry_website", int16(95),
		jsonContainsArg{`"source":"cvr"`, `"source_input_id":"` + rowID.String() + `"`, `"domain":"example.dk"`},
	).WillReturnRows(companyDomainRows(companyID, domainID, "registry_website", 95))
	mock.ExpectQuery(`INSERT INTO domains`).WithArgs("example.dk", "registry").
		WillReturnRows(domainRows(domainID, "example.dk", "registry"))
	mock.ExpectQuery(`INSERT INTO company_domains`).WithArgs(
		companyID, domainID, "candidate", "needs_review", "registry_email", int16(45),
		jsonContainsArg{`"source":"cvr"`, `"source_input_id":"` + rowID.String() + `"`, `"domain":"example.dk"`},
	).WillReturnRows(companyDomainRows(companyID, domainID, "registry_email", 45))
	mock.ExpectQuery(`UPDATE companies SET`).WithArgs(
		(*string)(nil), (*string)(nil), (*string)(nil), ptrString("https://example.dk"), (*int32)(nil),
		[]byte(nil), []byte(nil), []byte(nil), (*int32)(nil), (*int64)(nil), companyID,
	).WillReturnRows(companyRows(companyID, "dansk-aps", "Dansk ApS", countryID))
	mock.ExpectExec(`UPDATE cvr_company_raw_inputs`).WithArgs(rowID).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))
	mock.ExpectCommit()

	company, err := service.ApproveCompanyRawInput(ctx, mock, "cvr", rowID, "ops", "")
	require.NoError(t, err)
	assert.Equal(t, companyID, company.ID)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestApproveCompanyRawInput_CVRDoesNotInferFinancialsFromIndicatorFragments(t *testing.T) {
	ctx := context.Background()
	rowID := uuid.New()
	sourceID := uuid.New()
	countryID := uuid.New()
	companyID := uuid.New()
	domainID := uuid.New()

	payload := []byte(`{
		"financials": [{
			"year": 2024,
			"indicator": "Revenue",
			"value": 1250000
		}]
	}`)

	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT id, name, display_name`).WithArgs("cvr").
		WillReturnRows(sourceRows(sourceID, "cvr", "cvr_company_raw_inputs"))
	mock.ExpectQuery(`SELECT id, source_pull_run_id, source_native_id, cvr_number`).WithArgs(rowID).
		WillReturnRows(cvrApprovalRows(rowID, "12345678", "Dansk ApS", "translated", payload))
	mock.ExpectQuery(`SELECT id FROM countries`).WithArgs("DK").
		WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow(countryID))
	mock.ExpectQuery(`SELECT c.id`).WithArgs(ptrString("12345678"), "DK").
		WillReturnRows(companyRows(companyID, "dansk-aps", "Dansk ApS", countryID))
	mock.ExpectQuery(`INSERT INTO company_emails`).WithArgs(
		companyID, "info@example.dk", pgxmock.AnyArg(), "official", pgxmock.AnyArg(), "cvr", pgxmock.AnyArg(), pgxmock.AnyArg(),
	).WillReturnRows(companyEmailRows(companyID, "info@example.dk", "cvr"))
	mock.ExpectQuery(`INSERT INTO company_phones`).WithArgs(
		companyID, "+45 12 34 56 78", pgxmock.AnyArg(), "official", "cvr", pgxmock.AnyArg(), pgxmock.AnyArg(),
	).WillReturnRows(companyPhoneRows(companyID, "+45 12 34 56 78", "cvr"))
	mock.ExpectQuery(`INSERT INTO company_financials`).WithArgs(
		companyID, int32(2024), "cvr", (*int32)(nil), (*int64)(nil), (*string)(nil), (*int64)(nil), (*int64)(nil), (*int64)(nil),
		jsonContainsArg{`"source":"cvr"`, `"source_input_id":"` + rowID.String() + `"`, `"indicator"`, `"value"`},
	).WillReturnRows(companyFinancialRows(companyID, 2024, "cvr"))
	mock.ExpectQuery(`INSERT INTO domains`).WithArgs("example.dk", "registry").
		WillReturnRows(domainRows(domainID, "example.dk", "registry"))
	mock.ExpectQuery(`INSERT INTO company_domains`).WithArgs(
		companyID, domainID, "official_site", "needs_review", "registry_website", int16(95),
		jsonContainsArg{`"source":"cvr"`, `"source_input_id":"` + rowID.String() + `"`, `"domain":"example.dk"`},
	).WillReturnRows(companyDomainRows(companyID, domainID, "registry_website", 95))
	mock.ExpectQuery(`INSERT INTO domains`).WithArgs("example.dk", "registry").
		WillReturnRows(domainRows(domainID, "example.dk", "registry"))
	mock.ExpectQuery(`INSERT INTO company_domains`).WithArgs(
		companyID, domainID, "candidate", "needs_review", "registry_email", int16(45),
		jsonContainsArg{`"source":"cvr"`, `"source_input_id":"` + rowID.String() + `"`, `"domain":"example.dk"`},
	).WillReturnRows(companyDomainRows(companyID, domainID, "registry_email", 45))
	mock.ExpectQuery(`UPDATE companies SET`).WithArgs(
		(*string)(nil), (*string)(nil), (*string)(nil), ptrString("https://example.dk"), (*int32)(nil),
		[]byte(nil), []byte(nil), []byte(nil), (*int32)(nil), (*int64)(nil), companyID,
	).WillReturnRows(companyRows(companyID, "dansk-aps", "Dansk ApS", countryID))
	mock.ExpectExec(`UPDATE cvr_company_raw_inputs`).WithArgs(rowID).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))
	mock.ExpectCommit()

	company, err := service.ApproveCompanyRawInput(ctx, mock, "cvr", rowID, "ops", "")
	require.NoError(t, err)
	assert.Equal(t, companyID, company.ID)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestApproveCompanyRawInput_CVRPersistsRegistryWebsiteDomainSignal(t *testing.T) {
	ctx := context.Background()
	rowID := uuid.New()
	sourceID := uuid.New()
	countryID := uuid.New()
	companyID := uuid.New()
	domainID := uuid.New()

	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT id, name, display_name`).WithArgs("cvr").
		WillReturnRows(sourceRows(sourceID, "cvr", "cvr_company_raw_inputs"))
	mock.ExpectQuery(`SELECT id, source_pull_run_id, source_native_id, cvr_number`).WithArgs(rowID).
		WillReturnRows(cvrApprovalRows(rowID, "12345678", "Dansk ApS", "translated", []byte(`{}`)))
	mock.ExpectQuery(`SELECT id FROM countries`).WithArgs("DK").
		WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow(countryID))
	mock.ExpectQuery(`SELECT c.id`).WithArgs(ptrString("12345678"), "DK").
		WillReturnError(pgx.ErrNoRows)
	mock.ExpectQuery(`SELECT id, lei, name`).WithArgs("dansk-aps").
		WillReturnError(pgx.ErrNoRows)
	mock.ExpectQuery(`INSERT INTO companies`).
		WithArgs(
			"dansk-aps", "Dansk ApS", countryID, ptrString("12345678"), (*string)(nil),
			"active", ptrString("https://example.dk"), pgUUID(sourceID), (*string)(nil), (*string)(nil), pgxmock.AnyArg(),
		).
		WillReturnRows(companyRows(companyID, "dansk-aps", "Dansk ApS", countryID))
	mock.ExpectQuery(`INSERT INTO company_emails`).WithArgs(
		companyID, "info@example.dk", pgxmock.AnyArg(), "official", pgxmock.AnyArg(), "cvr", pgxmock.AnyArg(), pgxmock.AnyArg(),
	).WillReturnRows(companyEmailRows(companyID, "info@example.dk", "cvr"))
	mock.ExpectQuery(`INSERT INTO company_phones`).WithArgs(
		companyID, "+45 12 34 56 78", pgxmock.AnyArg(), "official", "cvr", pgxmock.AnyArg(), pgxmock.AnyArg(),
	).WillReturnRows(companyPhoneRows(companyID, "+45 12 34 56 78", "cvr"))
	mock.ExpectQuery(`INSERT INTO domains`).WithArgs("example.dk", "registry").
		WillReturnRows(domainRows(domainID, "example.dk", "registry"))
	mock.ExpectQuery(`INSERT INTO company_domains`).WithArgs(
		companyID, domainID, "official_site", "needs_review", "registry_website", int16(95),
		jsonContainsArg{`"source":"cvr"`, `"source_input_id":"` + rowID.String() + `"`, `"domain":"example.dk"`},
	).WillReturnRows(companyDomainRows(companyID, domainID, "registry_website", 95))
	mock.ExpectQuery(`INSERT INTO domains`).WithArgs("example.dk", "registry").
		WillReturnRows(domainRows(domainID, "example.dk", "registry"))
	mock.ExpectQuery(`INSERT INTO company_domains`).WithArgs(
		companyID, domainID, "candidate", "needs_review", "registry_email", int16(45),
		jsonContainsArg{`"source":"cvr"`, `"source_input_id":"` + rowID.String() + `"`, `"domain":"example.dk"`},
	).WillReturnRows(companyDomainRows(companyID, domainID, "registry_email", 45))
	mock.ExpectExec(`UPDATE cvr_company_raw_inputs`).WithArgs(rowID).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))
	mock.ExpectCommit()

	company, err := service.ApproveCompanyRawInput(ctx, mock, "cvr", rowID, "ops", "")
	require.NoError(t, err)
	assert.Equal(t, "dansk-aps", company.CanonicalSlug)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestApproveCompanyRawInput_CVRSkipsPublicEmailDomainSignal(t *testing.T) {
	ctx := context.Background()
	rowID := uuid.New()
	sourceID := uuid.New()
	countryID := uuid.New()
	companyID := uuid.New()

	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT id, name, display_name`).WithArgs("cvr").
		WillReturnRows(sourceRows(sourceID, "cvr", "cvr_company_raw_inputs"))
	mock.ExpectQuery(`SELECT id, source_pull_run_id, source_native_id, cvr_number`).WithArgs(rowID).
		WillReturnRows(cvrApprovalRowsWithContacts(rowID, "12345678", "Dansk ApS", "translated", []byte(`{}`), nil, ptrString("owner@gmail.com"), nil))
	mock.ExpectQuery(`SELECT id FROM countries`).WithArgs("DK").
		WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow(countryID))
	mock.ExpectQuery(`SELECT c.id`).WithArgs(ptrString("12345678"), "DK").
		WillReturnRows(companyRows(companyID, "dansk-aps", "Dansk ApS", countryID))
	mock.ExpectQuery(`INSERT INTO company_emails`).WithArgs(
		companyID, "owner@gmail.com", pgxmock.AnyArg(), "official", pgxmock.AnyArg(), "cvr", pgxmock.AnyArg(), pgxmock.AnyArg(),
	).WillReturnRows(companyEmailRows(companyID, "owner@gmail.com", "cvr"))
	mock.ExpectExec(`UPDATE cvr_company_raw_inputs`).WithArgs(rowID).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))
	mock.ExpectCommit()

	company, err := service.ApproveCompanyRawInput(ctx, mock, "cvr", rowID, "ops", "")
	require.NoError(t, err)
	assert.Equal(t, companyID, company.ID)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestApproveCompanyRawInput_CVRPersistsPrivateEmailDomainSignal(t *testing.T) {
	ctx := context.Background()
	rowID := uuid.New()
	sourceID := uuid.New()
	countryID := uuid.New()
	companyID := uuid.New()
	domainID := uuid.New()

	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT id, name, display_name`).WithArgs("cvr").
		WillReturnRows(sourceRows(sourceID, "cvr", "cvr_company_raw_inputs"))
	mock.ExpectQuery(`SELECT id, source_pull_run_id, source_native_id, cvr_number`).WithArgs(rowID).
		WillReturnRows(cvrApprovalRowsWithContacts(rowID, "12345678", "Dansk ApS", "translated", []byte(`{}`), nil, ptrString("billing@corp-example.dk"), nil))
	mock.ExpectQuery(`SELECT id FROM countries`).WithArgs("DK").
		WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow(countryID))
	mock.ExpectQuery(`SELECT c.id`).WithArgs(ptrString("12345678"), "DK").
		WillReturnRows(companyRows(companyID, "dansk-aps", "Dansk ApS", countryID))
	mock.ExpectQuery(`INSERT INTO company_emails`).WithArgs(
		companyID, "billing@corp-example.dk", pgxmock.AnyArg(), "official", pgxmock.AnyArg(), "cvr", pgxmock.AnyArg(), pgxmock.AnyArg(),
	).WillReturnRows(companyEmailRows(companyID, "billing@corp-example.dk", "cvr"))
	mock.ExpectQuery(`INSERT INTO domains`).WithArgs("corp-example.dk", "registry").
		WillReturnRows(domainRows(domainID, "corp-example.dk", "registry"))
	mock.ExpectQuery(`INSERT INTO company_domains`).WithArgs(
		companyID, domainID, "candidate", "needs_review", "registry_email", int16(45),
		jsonContainsArg{`"source":"cvr"`, `"source_input_id":"` + rowID.String() + `"`, `"domain":"corp-example.dk"`},
	).WillReturnRows(companyDomainRows(companyID, domainID, "registry_email", 45))
	mock.ExpectExec(`UPDATE cvr_company_raw_inputs`).WithArgs(rowID).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))
	mock.ExpectCommit()

	company, err := service.ApproveCompanyRawInput(ctx, mock, "cvr", rowID, "ops", "")
	require.NoError(t, err)
	assert.Equal(t, companyID, company.ID)
	require.NoError(t, mock.ExpectationsWereMet())
}

func ptrInt32(v int32) *int32 { return &v }
func ptrInt64(v int64) *int64 { return &v }

func cvrApprovalRowsWithContacts(rowID uuid.UUID, cvrNumber, companyName, translationStatus string, rawPayloadEn []byte, website, email, phone *string) *pgxmock.Rows {
	return pgxmock.NewRows([]string{
		"id", "source_pull_run_id", "source_native_id", "cvr_number", "company_name",
		"registration_status", "company_type", "website", "email", "phone", "country_iso2",
		"source_updated_at", "raw_payload", "raw_payload_en", "payload_hash",
		"first_seen_at", "last_seen_at", "processing_status", "processing_attempts", "processing_error",
		"processing_lease_by", "processing_lease_until", "processed_at", "run_id",
		"translation_status", "translation_attempts", "translation_error", "translation_model",
		"translation_prompt_version", "translated_at", "translation_lease_by", "translation_lease_until",
		"translation_fx_source", "translation_fx_rate_date", "created_at", "updated_at",
	}).AddRow(
		rowID, pgtype.UUID{}, cvrNumber, cvrNumber, &companyName,
		ptrString("registered"), ptrString("aps"), website, email, phone, (*string)(nil),
		pgtype.Timestamptz{}, []byte(`{"cvrNummer":"`+cvrNumber+`"}`), rawPayloadEn, "hash1",
		time.Time{}, time.Time{}, "pending", int32(0), (*string)(nil),
		(*string)(nil), pgtype.Timestamptz{}, pgtype.Timestamptz{}, (*string)(nil),
		translationStatus, int32(0), (*string)(nil), (*string)(nil),
		(*string)(nil), pgtype.Timestamptz{}, (*string)(nil), pgtype.Timestamptz{},
		(*string)(nil), pgtype.Date{}, time.Time{}, time.Time{},
	)
}

func companyEmailRows(companyID uuid.UUID, email, source string) *pgxmock.Rows {
	return pgxmock.NewRows([]string{
		"id", "company_id", "email", "description", "purpose", "name", "source", "confidence",
		"evidence", "metadata", "removed_at", "created_at", "updated_at",
	}).AddRow(
		uuid.New(), companyID, email, (*string)(nil), "official", (*string)(nil), source, (*float32)(nil),
		json.RawMessage(`{}`), json.RawMessage(nil), pgtype.Timestamptz{}, time.Time{}, time.Time{},
	)
}

func companyPhoneRows(companyID uuid.UUID, phone, source string) *pgxmock.Rows {
	return pgxmock.NewRows([]string{
		"id", "company_id", "phone", "description", "purpose", "source", "confidence",
		"evidence", "metadata", "removed_at", "created_at", "updated_at",
	}).AddRow(
		uuid.New(), companyID, phone, (*string)(nil), "official", source, (*float32)(nil),
		json.RawMessage(`{}`), json.RawMessage(nil), pgtype.Timestamptz{}, time.Time{}, time.Time{},
	)
}

func companyFinancialRows(companyID uuid.UUID, year int32, source string) *pgxmock.Rows {
	return pgxmock.NewRows([]string{
		"id", "company_id", "year", "source_name", "employee_count", "revenue_amount",
		"revenue_currency", "revenue_usd", "profit_amount", "profit_usd", "status",
		"reviewed_by", "reviewed_at", "created_at", "updated_at", "evidence",
	}).AddRow(
		uuid.New(), companyID, year, source, (*int32)(nil), (*int64)(nil),
		(*string)(nil), (*int64)(nil), (*int64)(nil), (*int64)(nil), "suggested",
		(*string)(nil), pgtype.Timestamptz{}, time.Time{}, time.Time{}, json.RawMessage(`{}`),
	)
}

func domainRows(domainID uuid.UUID, domain, importSource string) *pgxmock.Rows {
	return pgxmock.NewRows([]string{
		"id", "domain", "first_seen_at", "last_verified_at", "import_source",
	}).AddRow(
		domainID, domain, time.Time{}, pgtype.Timestamptz{}, importSource,
	)
}

func companyDomainRows(companyID, domainID uuid.UUID, signal string, confidence int16) *pgxmock.Rows {
	return pgxmock.NewRows([]string{
		"id", "company_id", "domain_id", "relationship_type", "status", "signal",
		"confidence", "evidence", "first_seen_at", "last_seen_at",
	}).AddRow(
		uuid.New(), companyID, domainID, "official_site", "needs_review", signal,
		confidence, json.RawMessage(`{}`), time.Time{}, time.Time{},
	)
}

type jsonContainsArg []string

func (a jsonContainsArg) Match(v interface{}) bool {
	var b []byte
	switch value := v.(type) {
	case []byte:
		b = value
	case json.RawMessage:
		b = value
	case string:
		b = []byte(value)
	default:
		return false
	}
	text := string(b)
	for _, fragment := range a {
		if !strings.Contains(text, fragment) {
			return false
		}
	}
	return true
}
