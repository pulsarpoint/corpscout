package service_test

import (
	"context"
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
