package service

import (
	"context"
	"fmt"

	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
	pgx "github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	db "github.com/pulsarpoint/corpscout/scheduler/internal/db/gen"
	"github.com/pulsarpoint/corpscout/scheduler/internal/slug"
)

// TxPool abstracts *pgxpool.Pool to allow injection of pgxmock in tests.
// *pgxpool.Pool satisfies this interface — no change needed in callers.
type TxPool interface {
	db.DBTX
	Begin(ctx context.Context) (pgx.Tx, error)
}

// ChildSuggestionRef identifies a section suggestion to approve alongside a root company suggestion.
type ChildSuggestionRef struct {
	Table string
	ID    uuid.UUID
}

// ApproveCompanySuggestion creates a company from the suggestion and marks it approved.
// It is the only path that writes to the companies table from source-derived data.
// proposed_country_id must be set on the suggestion; approval fails without it.
func ApproveCompanySuggestion(ctx context.Context, pool TxPool, suggestionID uuid.UUID, reviewedBy, reviewNote string) (db.Company, error) {
	q := db.New(pool)

	sug, err := q.GetCompanySuggestionByID(ctx, suggestionID)
	if err != nil {
		return db.Company{}, errors.Wrap(err, "get suggestion")
	}
	if sug.Status != "pending" {
		return db.Company{}, fmt.Errorf("suggestion %s is not pending (status=%s)", suggestionID, sug.Status)
	}
	if !sug.ProposedCountryID.Valid {
		return db.Company{}, fmt.Errorf("suggestion %s has no proposed_country_id; cannot create company", suggestionID)
	}

	canonicalSlug := slug.Generate(sug.ProposedDisplayName)
	if canonicalSlug == "" {
		canonicalSlug = "company-" + suggestionID.String()[:12]
	}

	// Check for slug collision; append suggestion UUID prefix as suffix.
	if _, err := q.GetCompanyBySlug(ctx, canonicalSlug); err == nil {
		canonicalSlug = canonicalSlug + "-" + suggestionID.String()[:12]
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return db.Company{}, errors.Wrap(err, "check slug collision")
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return db.Company{}, errors.Wrap(err, "begin tx")
	}
	defer func() { _ = tx.Rollback(ctx) }()

	qtx := db.New(tx)

	company, err := qtx.InsertCompany(ctx, db.InsertCompanyParams{
		CanonicalSlug: canonicalSlug,
		Name:          sug.ProposedDisplayName,
		CountryID:     uuid.UUID(sug.ProposedCountryID.Bytes),
		Column4:       "active",
	})
	if err != nil {
		return db.Company{}, errors.Wrap(err, "insert company")
	}

	rb := reviewedBy
	rn := reviewNote
	if err := qtx.UpdateCompanySuggestionApproved(ctx, db.UpdateCompanySuggestionApprovedParams{
		ID:               suggestionID,
		CreatedCompanyID: pgtype.UUID{Bytes: company.ID, Valid: true},
		ReviewedBy:       &rb,
		ReviewNote:       &rn,
	}); err != nil {
		return db.Company{}, errors.Wrap(err, "update suggestion approved")
	}

	if err := tx.Commit(ctx); err != nil {
		return db.Company{}, errors.Wrap(err, "commit")
	}
	return company, nil
}

// RejectCompanySuggestion marks the suggestion rejected without touching resolved tables.
func RejectCompanySuggestion(ctx context.Context, pool TxPool, suggestionID uuid.UUID, reviewedBy, reviewNote string) error {
	q := db.New(pool)

	sug, err := q.GetCompanySuggestionByID(ctx, suggestionID)
	if err != nil {
		return errors.Wrap(err, "get suggestion")
	}
	if sug.Status != "pending" {
		return fmt.Errorf("suggestion %s is not pending (status=%s)", suggestionID, sug.Status)
	}

	rb := reviewedBy
	rn := reviewNote
	return q.UpdateCompanySuggestionRejected(ctx, db.UpdateCompanySuggestionRejectedParams{
		ID:         suggestionID,
		ReviewedBy: &rb,
		ReviewNote: &rn,
	})
}
