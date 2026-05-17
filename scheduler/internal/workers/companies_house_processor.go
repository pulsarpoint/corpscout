package workers

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/cockroachdb/errors"
	pgx "github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	db "github.com/pulsarpoint/corpscout/scheduler/internal/db/gen"
)

const chBatchSize = int32(50)
const chLeaseSecs = 120

type CompaniesHouseProcessor struct {
	db db.Querier
}

func NewCompaniesHouseProcessor(q db.Querier) *CompaniesHouseProcessor {
	return &CompaniesHouseProcessor{db: q}
}

func (p *CompaniesHouseProcessor) ProcessBatch(ctx context.Context, sourceName string) error {
	src, err := p.db.GetSourceByName(ctx, sourceName)
	if err != nil {
		return errors.Wrap(err, "get source")
	}

	leaseBy := "ch-processor"
	rows, err := p.db.ClaimPendingCompaniesHouseRawInputs(ctx, db.ClaimPendingCompaniesHouseRawInputsParams{
		ProcessingLeaseBy: &leaseBy,
		Column2:           chLeaseSecs,
		Limit:             chBatchSize,
	})
	if err != nil {
		return errors.Wrap(err, "claim ch rows")
	}

	for _, row := range rows {
		if err := p.processOne(ctx, src, row); err != nil {
			slog.Error("ch processor: row failed", "row_id", row.ID, "company_number", row.CompanyNumber, "error", err)
			errMsg := err.Error()
			_ = p.db.MarkCompaniesHouseRawInputFailed(ctx, db.MarkCompaniesHouseRawInputFailedParams{
				ID:              row.ID,
				ProcessingError: &errMsg,
			})
		}
	}
	return nil
}

func (p *CompaniesHouseProcessor) processOne(ctx context.Context, src db.DataSource, row db.CompaniesHouseCompanyRawInput) error {
	regNum := row.CompanyNumber
	existing, err := p.db.GetCompanyByRegistrationAndCountry(ctx, db.GetCompanyByRegistrationAndCountryParams{
		RegistrationNumber: &regNum,
		IsoAlpha2:          "GB",
	})

	if errors.Is(err, pgx.ErrNoRows) {
		displayName := row.CompanyNumber
		if row.CompanyName != nil {
			displayName = *row.CompanyName
		}
		profile, _ := json.Marshal(map[string]any{"company_number": row.CompanyNumber, "country": "GB"})
		var countryID pgtype.UUID
		if cid, err := p.db.GetCountryIDByISO2(ctx, "GB"); err == nil {
			countryID = pgtype.UUID{Bytes: cid, Valid: true}
		}
		sug, err := p.db.InsertCompanySuggestion(ctx, db.InsertCompanySuggestionParams{
			ProposedDisplayName: displayName,
			ProposedProfile:     profile,
			ProposedCountryID:   countryID,
			Confidence:          ptrFloat32(0.75),
		})
		if err != nil {
			return errors.Wrap(err, "insert company suggestion")
		}
		if _, err := p.db.InsertSuggestionSourceLink(ctx, db.InsertSuggestionSourceLinkParams{
			SuggestionTable:  "company_suggestions",
			SuggestionID:     sug.ID,
			SourceID:         src.ID,
			SourceInputTable: "companies_house_company_raw_inputs",
			SourceInputKey:   row.ID.String(),
			SourcePullRunID:  pgtype.UUID{Bytes: row.SourcePullRunID, Valid: true},
		}); err != nil {
			return errors.Wrap(err, "insert source link")
		}
	} else if err != nil {
		return errors.Wrap(err, "lookup company")
	} else if row.CompanyStatus != nil {
		current, _ := json.Marshal(map[string]any{"lifecycle_status": existing.Status})
		proposed, _ := json.Marshal(map[string]any{"registration_status": *row.CompanyStatus})
		sug, err := p.db.InsertCompanyStatusSuggestion(ctx, db.InsertCompanyStatusSuggestionParams{
			CompanyID:       pgtype.UUID{Bytes: existing.ID, Valid: true},
			Operation:       "update",
			StatusField:     "registration_status",
			ProposedValue:   row.CompanyStatus,
			CurrentPayload:  current,
			ProposedPayload: proposed,
			Confidence:      ptrFloat32(0.8),
		})
		if err != nil {
			return errors.Wrap(err, "insert status suggestion")
		}
		if _, err := p.db.InsertSuggestionSourceLink(ctx, db.InsertSuggestionSourceLinkParams{
			SuggestionTable:  "company_status_suggestions",
			SuggestionID:     sug.ID,
			SourceID:         src.ID,
			SourceInputTable: "companies_house_company_raw_inputs",
			SourceInputKey:   row.ID.String(),
		}); err != nil {
			return errors.Wrap(err, "insert source link")
		}
	}

	return p.db.MarkCompaniesHouseRawInputProcessed(ctx, row.ID)
}
