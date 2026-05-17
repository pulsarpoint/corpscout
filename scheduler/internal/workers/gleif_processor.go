package workers

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
	pgx "github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	db "github.com/pulsarpoint/corpscout/scheduler/internal/db/gen"
)

const gleifBatchSize = int32(50)
const gleifLeaseSecs = 120

type GLEIFProcessor struct {
	db db.Querier
}

func NewGLEIFProcessor(q db.Querier) *GLEIFProcessor {
	return &GLEIFProcessor{db: q}
}

func (p *GLEIFProcessor) ProcessBatch(ctx context.Context, sourceName string) error {
	src, err := p.db.GetSourceByName(ctx, sourceName)
	if err != nil {
		return errors.Wrap(err, "get source")
	}

	leaseBy := "gleif-processor"
	for {
		rows, err := p.db.ClaimPendingGLEIFRawInputs(ctx, db.ClaimPendingGLEIFRawInputsParams{
			ProcessingLeaseBy: &leaseBy,
			Column2:           gleifLeaseSecs,
			Limit:             gleifBatchSize,
		})
		if err != nil {
			return errors.Wrap(err, "claim gleif rows")
		}
		if len(rows) == 0 {
			break
		}
		for _, row := range rows {
			if err := p.processOne(ctx, src, row); err != nil {
				slog.Error("gleif processor: row failed", "row_id", row.ID, "lei", row.Lei, "error", err)
				errMsg := err.Error()
				_ = p.db.MarkGLEIFRawInputFailed(ctx, db.MarkGLEIFRawInputFailedParams{
					ID:              row.ID,
					ProcessingError: &errMsg,
				})
			}
		}
	}
	return nil
}

func (p *GLEIFProcessor) processOne(ctx context.Context, src db.DataSource, row db.GleifCompanyRawInput) error {
	existing, err := p.db.GetCompanyByLEI(ctx, &row.Lei)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return errors.Wrap(err, "lookup company by lei")
	}

	if errors.Is(err, pgx.ErrNoRows) {
		displayName := row.Lei
		if row.LegalName != nil {
			displayName = *row.LegalName
		}
		profile, _ := json.Marshal(map[string]any{"lei": row.Lei})
		var countryID pgtype.UUID
		if row.HeadquartersCountryCode != nil && *row.HeadquartersCountryCode != "" {
			if cid, err := p.db.GetCountryIDByISO2(ctx, *row.HeadquartersCountryCode); err == nil {
				countryID = pgtype.UUID{Bytes: cid, Valid: true}
			}
		}
		sug, err := p.db.InsertCompanySuggestion(ctx, db.InsertCompanySuggestionParams{
			ProposedDisplayName: displayName,
			ProposedLegalName:   row.LegalName,
			ProposedProfile:     profile,
			ProposedCountryID:   countryID,
			Confidence:          ptrFloat32(0.7),
		})
		if err != nil {
			return errors.Wrap(err, "insert company suggestion")
		}
		if err := p.linkSuggestion(ctx, src, row, "company_suggestions", sug.ID); err != nil {
			return err
		}
	} else {
		// Existing company — emit status suggestion if registration status changed.
		if row.RegistrationStatus != nil {
			current, _ := json.Marshal(map[string]any{"registration_status": existing.Status})
			proposed, _ := json.Marshal(map[string]any{"registration_status": *row.RegistrationStatus})
			sug, err := p.db.InsertCompanyStatusSuggestion(ctx, db.InsertCompanyStatusSuggestionParams{
				CompanyID:       pgtype.UUID{Bytes: existing.ID, Valid: true},
				Operation:       "update",
				StatusField:     "registration_status",
				CurrentValue:    &existing.Status,
				ProposedValue:   row.RegistrationStatus,
				CurrentPayload:  current,
				ProposedPayload: proposed,
				Confidence:      ptrFloat32(0.8),
			})
			if err != nil {
				return errors.Wrap(err, "insert status suggestion")
			}
			if err := p.linkSuggestion(ctx, src, row, "company_status_suggestions", sug.ID); err != nil {
				return err
			}
		}
	}

	return p.db.MarkGLEIFRawInputProcessed(ctx, row.ID)
}

func (p *GLEIFProcessor) linkSuggestion(ctx context.Context, src db.DataSource, row db.GleifCompanyRawInput, table string, sugID uuid.UUID) error {
	_, err := p.db.InsertSuggestionSourceLink(ctx, db.InsertSuggestionSourceLinkParams{
		SuggestionTable:  table,
		SuggestionID:     sugID,
		SourceID:         src.ID,
		SourceInputTable: "gleif_company_raw_inputs",
		SourceInputKey:   row.ID.String(),
		SourcePullRunID:  pgtype.UUID{Bytes: row.SourcePullRunID, Valid: true},
	})
	return errors.Wrap(err, "insert source link")
}

func ptrFloat32(f float32) *float32 { return &f }
