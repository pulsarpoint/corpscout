package service

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
	pgx "github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	db "github.com/pulsarpoint/corpscout/scheduler/internal/db/gen"
	"github.com/pulsarpoint/corpscout/scheduler/internal/slug"
)

var (
	ErrRawInputNotFound            = errors.New("raw input not found")
	ErrRawInputNotApprovable       = errors.New("raw input is not approvable")
	ErrRawInputRequiresTranslation = errors.New("raw input translation is required before approval")
	ErrRawInputUnsupportedSource   = errors.New("raw input source is not supported for company approval")
	ErrRawInputCountryNotFound     = errors.New("raw input country not found")
)

type rawCompanyCandidate struct {
	id                 uuid.UUID
	sourceName         string
	sourceNativeID     string
	displayName        string
	countryISO2        string
	registrationNumber *string
	lei                *string
	website            *string
	registrationStatus *string
	parentLei          *string
	ultimateParentLei  *string
	processingStatus   string
	translated         bool
}

// ApproveCompanyRawInput creates or returns a resolved company directly from a
// source raw input and marks that raw input processed. This replaces the retired
// source_process/company_suggestion path for registry raw inputs.
func ApproveCompanyRawInput(ctx context.Context, pool TxPool, sourceName string, rawInputID uuid.UUID, reviewedBy, reviewNote string) (db.Company, error) {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return db.Company{}, errors.Wrap(err, "begin tx")
	}
	defer func() { _ = tx.Rollback(ctx) }()

	qtx := db.New(tx)
	src, err := qtx.GetSourceByName(ctx, sourceName)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.Company{}, ErrRawInputUnsupportedSource
		}
		return db.Company{}, errors.Wrap(err, "get source")
	}

	candidate, err := loadRawCompanyCandidate(ctx, qtx, src, rawInputID)
	if err != nil {
		return db.Company{}, err
	}
	if !isRawInputStatusApprovable(candidate.processingStatus) {
		return db.Company{}, ErrRawInputNotApprovable
	}
	if candidate.countryISO2 == "" {
		return db.Company{}, ErrRawInputCountryNotFound
	}

	countryID, err := qtx.GetCountryIDByISO2(ctx, candidate.countryISO2)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.Company{}, ErrRawInputCountryNotFound
		}
		return db.Company{}, errors.Wrap(err, "get raw input country")
	}

	existing, err := findExistingRawInputCompany(ctx, qtx, candidate)
	if err == nil {
		if err := markRawInputApproved(ctx, qtx, src.InputTableName, rawInputID); err != nil {
			return db.Company{}, err
		}
		if err := tx.Commit(ctx); err != nil {
			return db.Company{}, errors.Wrap(err, "commit")
		}
		return existing, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return db.Company{}, errors.Wrap(err, "lookup existing company")
	}

	canonicalSlug := slug.Generate(candidate.displayName)
	if canonicalSlug == "" {
		canonicalSlug = "company-" + rawInputID.String()[:12]
	}
	if _, err := qtx.GetCompanyBySlug(ctx, canonicalSlug); err == nil {
		canonicalSlug = canonicalSlug + "-" + rawInputID.String()[:12]
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return db.Company{}, errors.Wrap(err, "check slug collision")
	}

	evidence, err := rawInputApprovalEvidence(candidate, reviewedBy, reviewNote)
	if err != nil {
		return db.Company{}, errors.Wrap(err, "build approval evidence")
	}
	status := normalizeResolvedCompanyStatus(candidate.registrationStatus)

	company, err := qtx.InsertCompanyFromRawInput(ctx, db.InsertCompanyFromRawInputParams{
		CanonicalSlug:      canonicalSlug,
		Name:               candidate.displayName,
		CountryID:          countryID,
		RegistrationNumber: candidate.registrationNumber,
		Lei:                candidate.lei,
		Column6:            status,
		Website:            candidate.website,
		PrimarySourceID:    pgtype.UUID{Bytes: src.ID, Valid: true},
		ParentLei:          candidate.parentLei,
		UltimateParentLei:  candidate.ultimateParentLei,
		Evidence:           evidence,
	})
	if err != nil {
		return db.Company{}, errors.Wrap(err, "insert company from raw input")
	}

	if err := markRawInputApproved(ctx, qtx, src.InputTableName, rawInputID); err != nil {
		return db.Company{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return db.Company{}, errors.Wrap(err, "commit")
	}
	return company, nil
}

func loadRawCompanyCandidate(ctx context.Context, q *db.Queries, src db.DataSource, rawInputID uuid.UUID) (rawCompanyCandidate, error) {
	switch src.InputTableName {
	case "gleif_company_raw_inputs":
		row, err := q.GetGLEIFRawInputForCompanyApproval(ctx, rawInputID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return rawCompanyCandidate{}, ErrRawInputNotFound
			}
			return rawCompanyCandidate{}, errors.Wrap(err, "get gleif raw input")
		}
		displayName := fallbackString(row.LegalName, row.Lei)
		return rawCompanyCandidate{
			id:                 row.ID,
			sourceName:         src.Name,
			sourceNativeID:     row.SourceNativeID,
			displayName:        displayName,
			countryISO2:        fallbackString(row.HeadquartersCountryCode, ""),
			lei:                &row.Lei,
			registrationStatus: row.RegistrationStatus,
			parentLei:          row.ParentLei,
			ultimateParentLei:  row.UltimateParentLei,
			processingStatus:   row.ProcessingStatus,
		}, nil
	case "companies_house_company_raw_inputs":
		row, err := q.GetCompaniesHouseRawInputForCompanyApproval(ctx, rawInputID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return rawCompanyCandidate{}, ErrRawInputNotFound
			}
			return rawCompanyCandidate{}, errors.Wrap(err, "get companies house raw input")
		}
		displayName := fallbackString(row.CompanyName, row.CompanyNumber)
		countryISO2 := fallbackString(row.CountryIso2, "GB")
		return rawCompanyCandidate{
			id:                 row.ID,
			sourceName:         src.Name,
			sourceNativeID:     row.SourceNativeID,
			displayName:        displayName,
			countryISO2:        countryISO2,
			registrationNumber: &row.CompanyNumber,
			registrationStatus: row.CompanyStatus,
			processingStatus:   row.ProcessingStatus,
		}, nil
	case "brreg_company_raw_inputs":
		row, err := q.GetBrregRawInputForCompanyApproval(ctx, rawInputID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return rawCompanyCandidate{}, ErrRawInputNotFound
			}
			return rawCompanyCandidate{}, errors.Wrap(err, "get brreg raw input")
		}
		if row.TranslationStatus != "translated" || len(row.RawPayloadEn) == 0 {
			return rawCompanyCandidate{}, ErrRawInputRequiresTranslation
		}
		displayName := fallbackString(row.OrganizationName, row.OrganizationNumber)
		countryISO2 := fallbackString(row.CountryIso2, "NO")
		return rawCompanyCandidate{
			id:                 row.ID,
			sourceName:         src.Name,
			sourceNativeID:     row.SourceNativeID,
			displayName:        displayName,
			countryISO2:        countryISO2,
			registrationNumber: &row.OrganizationNumber,
			website:            row.Website,
			registrationStatus: row.RegistrationStatus,
			processingStatus:   row.ProcessingStatus,
			translated:         true,
		}, nil
	default:
		return rawCompanyCandidate{}, ErrRawInputUnsupportedSource
	}
}

func findExistingRawInputCompany(ctx context.Context, q *db.Queries, candidate rawCompanyCandidate) (db.Company, error) {
	if candidate.lei != nil && *candidate.lei != "" {
		company, err := q.GetCompanyByLEI(ctx, candidate.lei)
		if err == nil || !errors.Is(err, pgx.ErrNoRows) {
			return company, err
		}
	}
	if candidate.registrationNumber != nil && *candidate.registrationNumber != "" {
		return q.GetCompanyByRegistrationAndCountry(ctx, db.GetCompanyByRegistrationAndCountryParams{
			RegistrationNumber: candidate.registrationNumber,
			IsoAlpha2:          candidate.countryISO2,
		})
	}
	return db.Company{}, pgx.ErrNoRows
}

func markRawInputApproved(ctx context.Context, q *db.Queries, inputTableName string, rawInputID uuid.UUID) error {
	switch inputTableName {
	case "gleif_company_raw_inputs":
		return errors.Wrap(q.MarkGLEIFRawInputProcessed(ctx, rawInputID), "mark gleif raw input processed")
	case "companies_house_company_raw_inputs":
		return errors.Wrap(q.MarkCompaniesHouseRawInputProcessed(ctx, rawInputID), "mark companies house raw input processed")
	case "brreg_company_raw_inputs":
		return errors.Wrap(q.MarkBrregRawInputProcessed(ctx, rawInputID), "mark brreg raw input processed")
	default:
		return ErrRawInputUnsupportedSource
	}
}

func rawInputApprovalEvidence(candidate rawCompanyCandidate, reviewedBy, reviewNote string) (json.RawMessage, error) {
	payload := map[string]any{
		"source":           candidate.sourceName,
		"source_input_id":  candidate.id.String(),
		"source_native_id": candidate.sourceNativeID,
		"reviewed_by":      reviewedBy,
		"translated":       candidate.translated,
	}
	if reviewNote != "" {
		payload["review_note"] = reviewNote
	}
	b, err := json.Marshal(payload)
	return json.RawMessage(b), err
}

func isRawInputStatusApprovable(status string) bool {
	return status == "pending" || status == "failed"
}

func normalizeResolvedCompanyStatus(rawStatus *string) string {
	if rawStatus == nil {
		return "active"
	}
	status := strings.ToLower(strings.TrimSpace(*rawStatus))
	switch {
	case status == "":
		return "active"
	case strings.Contains(status, "active"):
		return "active"
	case strings.Contains(status, "registered"):
		return "active"
	case strings.Contains(status, "dissolved"):
		return "dissolved"
	case strings.Contains(status, "deleted"):
		return "dissolved"
	case strings.Contains(status, "closed"):
		return "dissolved"
	default:
		return "inactive"
	}
}

func fallbackString(value *string, fallback string) string {
	if value == nil || strings.TrimSpace(*value) == "" {
		return fallback
	}
	return *value
}
