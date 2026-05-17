package service_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	pgx "github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	pgxmock "github.com/pashagolub/pgxmock/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulsarpoint/corpscout/scheduler/internal/service"
)

func pgUUID(id uuid.UUID) pgtype.UUID { return pgtype.UUID{Bytes: id, Valid: true} }

// sugRows returns a pgxmock.Rows matching the GetCompanySuggestionByID scan order:
// id, proposed_display_name, proposed_legal_name, proposed_website, proposed_canonical_slug,
// proposed_country_id, proposed_profile, confidence, status, created_company_id,
// reviewed_by, reviewed_at, review_note, created_at, updated_at
func sugRows(suggestionID, countryID uuid.UUID, displayName, status string) *pgxmock.Rows {
	return pgxmock.NewRows([]string{
		"id", "proposed_display_name", "proposed_legal_name", "proposed_website",
		"proposed_canonical_slug", "proposed_country_id", "proposed_profile",
		"confidence", "status", "created_company_id",
		"reviewed_by", "reviewed_at", "review_note", "created_at", "updated_at",
	}).AddRow(
		suggestionID, displayName, (*string)(nil), (*string)(nil),
		(*string)(nil), pgUUID(countryID), []byte("{}"),
		(*float32)(nil), status, pgtype.UUID{},
		(*string)(nil), pgtype.Timestamptz{}, (*string)(nil), time.Time{}, time.Time{},
	)
}

// companyRows returns a pgxmock.Rows matching the InsertCompany/GetCompanyBySlug scan order:
// id, lei, name, country_id, registration_number, status, primary_source_id,
// created_at, updated_at, short_name, short_description, description, website,
// founded_year, employee_estimate, revenue_estimate, ownership, parent_lei,
// ultimate_parent_lei, canonical_slug, display_name, resolution_status, evidence
func companyRows(companyID uuid.UUID, slug, name string, countryID uuid.UUID) *pgxmock.Rows {
	return pgxmock.NewRows([]string{
		"id", "lei", "name", "country_id", "registration_number", "status", "primary_source_id",
		"created_at", "updated_at", "short_name", "short_description", "description", "website",
		"founded_year", "employee_estimate", "revenue_estimate", "ownership", "parent_lei",
		"ultimate_parent_lei", "canonical_slug", "display_name", "resolution_status", "evidence",
	}).AddRow(
		companyID, (*string)(nil), name, countryID, (*string)(nil), "active", pgtype.UUID{},
		time.Time{}, time.Time{}, (*string)(nil), (*string)(nil), (*string)(nil), (*string)(nil),
		(*int32)(nil), []byte(nil), []byte(nil), []byte(nil), (*string)(nil),
		(*string)(nil), slug, (*string)(nil), "resolved", []byte(nil),
	)
}

func TestApproveCompanySuggestion_CreatesCompanyAndApprovesSuggestion(t *testing.T) {
	ctx := context.Background()
	suggestionID := uuid.New()
	countryID := uuid.New()
	companyID := uuid.New()

	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	mock.ExpectQuery(`SELECT`).WithArgs(suggestionID).
		WillReturnRows(sugRows(suggestionID, countryID, "Test Company", "pending"))
	mock.ExpectQuery(`SELECT`).WithArgs("test-company").
		WillReturnError(pgx.ErrNoRows)
	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO companies`).
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnRows(companyRows(companyID, "test-company", "Test Company", countryID))
	mock.ExpectExec(`UPDATE company_suggestions`).
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))
	mock.ExpectCommit()

	company, err := service.ApproveCompanySuggestion(ctx, mock, suggestionID, "admin", "looks good")
	require.NoError(t, err)
	assert.Equal(t, "test-company", company.CanonicalSlug)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestApproveCompanySuggestion_SlugCollision_UsesSuffix(t *testing.T) {
	ctx := context.Background()
	suggestionID := uuid.New()
	countryID := uuid.New()
	companyID := uuid.New()
	existingID := uuid.New()
	expectedSlug := "test-company-" + suggestionID.String()[:12]

	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	mock.ExpectQuery(`SELECT`).WithArgs(suggestionID).
		WillReturnRows(sugRows(suggestionID, countryID, "Test Company", "pending"))
	mock.ExpectQuery(`SELECT`).WithArgs("test-company").
		WillReturnRows(companyRows(existingID, "test-company", "Existing Co", countryID))
	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO companies`).
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnRows(companyRows(companyID, expectedSlug, "Test Company", countryID))
	mock.ExpectExec(`UPDATE company_suggestions`).
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))
	mock.ExpectCommit()

	company, err := service.ApproveCompanySuggestion(ctx, mock, suggestionID, "admin", "")
	require.NoError(t, err)
	assert.Equal(t, expectedSlug, company.CanonicalSlug, "slug collision must append UUID suffix")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRejectCompanySuggestion_DoesNotWriteResolvedTables(t *testing.T) {
	ctx := context.Background()
	suggestionID := uuid.New()

	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	mock.ExpectQuery(`SELECT`).WithArgs(suggestionID).
		WillReturnRows(sugRows(suggestionID, uuid.Nil, "Test Corp", "pending"))
	mock.ExpectExec(`UPDATE company_suggestions`).
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	err = service.RejectCompanySuggestion(ctx, mock, suggestionID, "admin", "not relevant")
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet(), "no company writes should occur on reject")
}
