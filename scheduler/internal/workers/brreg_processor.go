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

const brregBatchSize = int32(50)
const brregLeaseSecs = 120

type BrregProcessor struct {
	db db.Querier
}

func NewBrregProcessor(q db.Querier) *BrregProcessor {
	return &BrregProcessor{db: q}
}

func (p *BrregProcessor) ProcessBatch(ctx context.Context, sourceName string) error {
	src, err := p.db.GetSourceByName(ctx, sourceName)
	if err != nil {
		return errors.Wrap(err, "get source")
	}

	leaseBy := "brreg-processor"
	for {
		rows, err := p.db.ClaimPendingBrregRawInputs(ctx, db.ClaimPendingBrregRawInputsParams{
			ProcessingLeaseBy: &leaseBy,
			Column2:           brregLeaseSecs,
			Limit:             brregBatchSize,
		})
		if err != nil {
			return errors.Wrap(err, "claim brreg rows")
		}
		if len(rows) == 0 {
			break
		}
		for _, row := range rows {
			if err := p.processOne(ctx, src, row); err != nil {
				slog.Error("brreg processor: row failed", "row_id", row.ID, "org_number", row.OrganizationNumber, "error", err)
				errMsg := err.Error()
				_ = p.db.MarkBrregRawInputFailed(ctx, db.MarkBrregRawInputFailedParams{
					ID:              row.ID,
					ProcessingError: &errMsg,
				})
			}
		}
	}
	return nil
}

func (p *BrregProcessor) processOne(ctx context.Context, src db.DataSource, row db.BrregCompanyRawInput) error {
	orgNum := row.OrganizationNumber
	existing, err := p.db.GetCompanyByRegistrationAndCountry(ctx, db.GetCompanyByRegistrationAndCountryParams{
		RegistrationNumber: &orgNum,
		IsoAlpha2:          "NO",
	})

	if errors.Is(err, pgx.ErrNoRows) {
		displayName := row.OrganizationNumber
		if row.OrganizationName != nil {
			displayName = *row.OrganizationName
		}
		profile, _ := json.Marshal(map[string]any{"organization_number": row.OrganizationNumber, "country": "NO"})
		var countryID pgtype.UUID
		if cid, err := p.db.GetCountryIDByISO2(ctx, "NO"); err == nil {
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
			SourceInputTable: "brreg_company_raw_inputs",
			SourceInputKey:   row.ID.String(),
			SourcePullRunID:  row.SourcePullRunID,
		}); err != nil {
			return errors.Wrap(err, "insert source link")
		}
		// Website → contact suggestion attached to root suggestion.
		if row.Website != nil {
			proposed, _ := json.Marshal(map[string]any{"url": *row.Website})
			cSug, err := p.db.InsertCompanyContactSuggestion(ctx, db.InsertCompanyContactSuggestionParams{
				CompanySuggestionID: pgtype.UUID{Bytes: sug.ID, Valid: true},
				Operation:           "add",
				ContactKind:         "website",
				CurrentPayload:      json.RawMessage("{}"),
				ProposedPayload:     proposed,
				Confidence:          ptrFloat32(0.75),
			})
			if err != nil {
				return errors.Wrap(err, "insert contact suggestion")
			}
			if _, err := p.db.InsertSuggestionSourceLink(ctx, db.InsertSuggestionSourceLinkParams{
				SuggestionTable:  "company_contact_suggestions",
				SuggestionID:     cSug.ID,
				SourceID:         src.ID,
				SourceInputTable: "brreg_company_raw_inputs",
				SourceInputKey:   row.ID.String(),
			}); err != nil {
				return errors.Wrap(err, "insert contact source link")
			}
		}
	} else if err != nil {
		return errors.Wrap(err, "lookup company")
	} else if row.Website != nil && existing.Website == nil {
		proposed, _ := json.Marshal(map[string]any{"url": *row.Website})
		sug, err := p.db.InsertCompanyContactSuggestion(ctx, db.InsertCompanyContactSuggestionParams{
			CompanyID:       pgtype.UUID{Bytes: existing.ID, Valid: true},
			Operation:       "add",
			ContactKind:     "website",
			CurrentPayload:  json.RawMessage("{}"),
			ProposedPayload: proposed,
			Confidence:      ptrFloat32(0.75),
		})
		if err != nil {
			return errors.Wrap(err, "insert contact suggestion for existing")
		}
		if _, err := p.db.InsertSuggestionSourceLink(ctx, db.InsertSuggestionSourceLinkParams{
			SuggestionTable:  "company_contact_suggestions",
			SuggestionID:     sug.ID,
			SourceID:         src.ID,
			SourceInputTable: "brreg_company_raw_inputs",
			SourceInputKey:   row.ID.String(),
		}); err != nil {
			return errors.Wrap(err, "insert contact source link")
		}
	}

	return p.db.MarkBrregRawInputProcessed(ctx, row.ID)
}
