package service

import (
	"context"
	"encoding/json"
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

// ApproveCompanyStatusSuggestion applies a status field change and marks the suggestion approved.
func ApproveCompanyStatusSuggestion(ctx context.Context, pool TxPool, suggestionID uuid.UUID, reviewedBy, reviewNote string) error {
	q := db.New(pool)
	sug, err := q.GetCompanyStatusSuggestionByID(ctx, suggestionID)
	if err != nil {
		return errors.Wrap(err, "get status suggestion")
	}
	if sug.Status != "pending" {
		return fmt.Errorf("status suggestion %s is not pending", suggestionID)
	}
	if !sug.CompanyID.Valid {
		return fmt.Errorf("status suggestion %s has no company_id", suggestionID)
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return errors.Wrap(err, "begin tx")
	}
	defer func() { _ = tx.Rollback(ctx) }()
	qtx := db.New(tx)

	if sug.ProposedValue != nil && sug.StatusField == "lifecycle_status" {
		if err := qtx.UpdateCompanyStatus(ctx, db.UpdateCompanyStatusParams{
			ID:     uuid.UUID(sug.CompanyID.Bytes),
			Status: *sug.ProposedValue,
		}); err != nil {
			return errors.Wrap(err, "update company status")
		}
	}
	rb, rn := reviewedBy, reviewNote
	if err := qtx.UpdateCompanyStatusSuggestionApproved(ctx, db.UpdateCompanyStatusSuggestionApprovedParams{
		ID: suggestionID, ReviewedBy: &rb, ReviewNote: &rn,
	}); err != nil {
		return errors.Wrap(err, "mark status suggestion approved")
	}
	return errors.Wrap(tx.Commit(ctx), "commit")
}

// RejectCompanyStatusSuggestion marks the suggestion rejected without touching resolved tables.
func RejectCompanyStatusSuggestion(ctx context.Context, pool TxPool, suggestionID uuid.UUID, reviewedBy, reviewNote string) error {
	q := db.New(pool)
	sug, err := q.GetCompanyStatusSuggestionByID(ctx, suggestionID)
	if err != nil {
		return errors.Wrap(err, "get status suggestion")
	}
	if sug.Status != "pending" {
		return fmt.Errorf("status suggestion %s is not pending", suggestionID)
	}
	rb, rn := reviewedBy, reviewNote
	return q.UpdateCompanyStatusSuggestionRejected(ctx, db.UpdateCompanyStatusSuggestionRejectedParams{
		ID: suggestionID, ReviewedBy: &rb, ReviewNote: &rn,
	})
}

// ApproveCompanyContactSuggestion applies a contact-kind change and marks the suggestion approved.
// Only "website" kind is applied to the resolved companies table today.
func ApproveCompanyContactSuggestion(ctx context.Context, pool TxPool, suggestionID uuid.UUID, reviewedBy, reviewNote string) error {
	q := db.New(pool)
	sug, err := q.GetCompanyContactSuggestionByID(ctx, suggestionID)
	if err != nil {
		return errors.Wrap(err, "get contact suggestion")
	}
	if sug.Status != "pending" {
		return fmt.Errorf("contact suggestion %s is not pending", suggestionID)
	}
	if !sug.CompanyID.Valid {
		return fmt.Errorf("contact suggestion %s has no company_id", suggestionID)
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return errors.Wrap(err, "begin tx")
	}
	defer func() { _ = tx.Rollback(ctx) }()
	qtx := db.New(tx)

	if sug.ContactKind == "website" {
		var proposed struct {
			URL string `json:"url"`
		}
		_ = json.Unmarshal(sug.ProposedPayload, &proposed)
		if proposed.URL != "" {
			if err := qtx.UpdateCompanyWebsite(ctx, db.UpdateCompanyWebsiteParams{
				ID:      uuid.UUID(sug.CompanyID.Bytes),
				Website: &proposed.URL,
			}); err != nil {
				return errors.Wrap(err, "update company website")
			}
		}
	}
	rb, rn := reviewedBy, reviewNote
	if err := qtx.UpdateCompanyContactSuggestionApproved(ctx, db.UpdateCompanyContactSuggestionApprovedParams{
		ID: suggestionID, ReviewedBy: &rb, ReviewNote: &rn,
	}); err != nil {
		return errors.Wrap(err, "mark contact suggestion approved")
	}
	return errors.Wrap(tx.Commit(ctx), "commit")
}

// RejectCompanyContactSuggestion marks the suggestion rejected without touching resolved tables.
func RejectCompanyContactSuggestion(ctx context.Context, pool TxPool, suggestionID uuid.UUID, reviewedBy, reviewNote string) error {
	q := db.New(pool)
	sug, err := q.GetCompanyContactSuggestionByID(ctx, suggestionID)
	if err != nil {
		return errors.Wrap(err, "get contact suggestion")
	}
	if sug.Status != "pending" {
		return fmt.Errorf("contact suggestion %s is not pending", suggestionID)
	}
	rb, rn := reviewedBy, reviewNote
	return q.UpdateCompanyContactSuggestionRejected(ctx, db.UpdateCompanyContactSuggestionRejectedParams{
		ID: suggestionID, ReviewedBy: &rb, ReviewNote: &rn,
	})
}

// ApproveCompanyWithSections creates a company from a root suggestion and atomically approves
// all listed child section suggestions in a single transaction.
func ApproveCompanyWithSections(ctx context.Context, pool TxPool, rootID uuid.UUID, children []ChildSuggestionRef, reviewedBy, reviewNote string) (db.Company, error) {
	q := db.New(pool)

	sug, err := q.GetCompanySuggestionByID(ctx, rootID)
	if err != nil {
		return db.Company{}, errors.Wrap(err, "get suggestion")
	}
	if sug.Status != "pending" {
		return db.Company{}, fmt.Errorf("suggestion %s is not pending (status=%s)", rootID, sug.Status)
	}
	if !sug.ProposedCountryID.Valid {
		return db.Company{}, fmt.Errorf("suggestion %s has no proposed_country_id", rootID)
	}

	canonicalSlug := slug.Generate(sug.ProposedDisplayName)
	if canonicalSlug == "" {
		canonicalSlug = "company-" + rootID.String()[:12]
	}
	if _, err := q.GetCompanyBySlug(ctx, canonicalSlug); err == nil {
		canonicalSlug = canonicalSlug + "-" + rootID.String()[:12]
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

	rb, rn := reviewedBy, reviewNote
	if err := qtx.UpdateCompanySuggestionApproved(ctx, db.UpdateCompanySuggestionApprovedParams{
		ID:               rootID,
		CreatedCompanyID: pgtype.UUID{Bytes: company.ID, Valid: true},
		ReviewedBy:       &rb,
		ReviewNote:       &rn,
	}); err != nil {
		return db.Company{}, errors.Wrap(err, "approve root suggestion")
	}

	for _, child := range children {
		switch child.Table {
		case "company_status_suggestions":
			if err := approveCompanyStatusTx(ctx, qtx, child.ID, rootID, company.ID, reviewedBy, reviewNote); err != nil {
				return db.Company{}, errors.Wrapf(err, "approve child status %s", child.ID)
			}
		case "company_contact_suggestions":
			if err := approveCompanyContactTx(ctx, qtx, child.ID, rootID, company.ID, reviewedBy, reviewNote); err != nil {
				return db.Company{}, errors.Wrapf(err, "approve child contact %s", child.ID)
			}
		default:
			return db.Company{}, fmt.Errorf("unknown child suggestion table: %s", child.Table)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return db.Company{}, errors.Wrap(err, "commit")
	}
	return company, nil
}

// resolveChildSuggestionCompanyID resolves the target company ID for a child suggestion.
// If the suggestion has a direct company_id (existing company), use it.
// If it's linked to the approved root suggestion, use the newly created company.
func resolveChildSuggestionCompanyID(companyID, companySuggestionID pgtype.UUID, approvedRootID, createdCompanyID, childID uuid.UUID) (uuid.UUID, error) {
	if companyID.Valid {
		return uuid.UUID(companyID.Bytes), nil
	}
	if companySuggestionID.Valid && uuid.UUID(companySuggestionID.Bytes) == approvedRootID {
		return createdCompanyID, nil
	}
	return uuid.Nil, fmt.Errorf("child suggestion %s is not attached to an existing company or the approved root suggestion", childID)
}

func approveCompanyStatusTx(ctx context.Context, qtx *db.Queries, suggestionID, approvedRootID, createdCompanyID uuid.UUID, reviewedBy, reviewNote string) error {
	sug, err := qtx.GetCompanyStatusSuggestionByID(ctx, suggestionID)
	if err != nil {
		return errors.Wrap(err, "get status suggestion")
	}
	if sug.Status != "pending" {
		return fmt.Errorf("status suggestion %s is not pending", suggestionID)
	}
	targetID, err := resolveChildSuggestionCompanyID(sug.CompanyID, sug.CompanySuggestionID, approvedRootID, createdCompanyID, suggestionID)
	if err != nil {
		return err
	}
	if sug.ProposedValue != nil && sug.StatusField == "lifecycle_status" {
		if err := qtx.UpdateCompanyStatus(ctx, db.UpdateCompanyStatusParams{
			ID:     targetID,
			Status: *sug.ProposedValue,
		}); err != nil {
			return errors.Wrap(err, "update company status")
		}
	}
	rb, rn := reviewedBy, reviewNote
	return errors.Wrap(qtx.UpdateCompanyStatusSuggestionApproved(ctx, db.UpdateCompanyStatusSuggestionApprovedParams{
		ID: suggestionID, ReviewedBy: &rb, ReviewNote: &rn,
	}), "mark status suggestion approved")
}

func approveCompanyContactTx(ctx context.Context, qtx *db.Queries, suggestionID, approvedRootID, createdCompanyID uuid.UUID, reviewedBy, reviewNote string) error {
	sug, err := qtx.GetCompanyContactSuggestionByID(ctx, suggestionID)
	if err != nil {
		return errors.Wrap(err, "get contact suggestion")
	}
	if sug.Status != "pending" {
		return fmt.Errorf("contact suggestion %s is not pending", suggestionID)
	}
	targetID, err := resolveChildSuggestionCompanyID(sug.CompanyID, sug.CompanySuggestionID, approvedRootID, createdCompanyID, suggestionID)
	if err != nil {
		return err
	}
	if sug.ContactKind == "website" {
		var proposed struct {
			URL string `json:"url"`
		}
		_ = json.Unmarshal(sug.ProposedPayload, &proposed)
		if proposed.URL != "" {
			if err := qtx.UpdateCompanyWebsite(ctx, db.UpdateCompanyWebsiteParams{
				ID: targetID, Website: &proposed.URL,
			}); err != nil {
				return errors.Wrap(err, "update company website")
			}
		}
	}
	rb, rn := reviewedBy, reviewNote
	return errors.Wrap(qtx.UpdateCompanyContactSuggestionApproved(ctx, db.UpdateCompanyContactSuggestionApprovedParams{
		ID: suggestionID, ReviewedBy: &rb, ReviewNote: &rn,
	}), "mark contact suggestion approved")
}
