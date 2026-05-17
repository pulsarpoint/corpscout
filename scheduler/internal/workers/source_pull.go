package workers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
	pgx "github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"

	"github.com/pulsarpoint/corpscout/scheduler/internal/crawlerclient"
	db "github.com/pulsarpoint/corpscout/scheduler/internal/db/gen"
)

type SourcePullWorker struct {
	river.WorkerDefaults[SourcePullArgs]
	db      db.Querier
	crawler *crawlerclient.Client
	pool    *pgxpool.Pool
	rv      *river.Client[pgx.Tx]
}

func NewSourcePullWorker(q db.Querier, crawler *crawlerclient.Client, pool *pgxpool.Pool) *SourcePullWorker {
	return &SourcePullWorker{db: q, crawler: crawler, pool: pool}
}

func (w *SourcePullWorker) SetRiverClient(rc *river.Client[pgx.Tx]) {
	w.rv = rc
}

func (w *SourcePullWorker) Work(ctx context.Context, job *river.Job[SourcePullArgs]) error {
	src, err := w.db.GetSourceByName(ctx, job.Args.SourceName)
	if err != nil {
		return errors.Wrap(err, "get source")
	}

	if err := w.db.UpdateSourcePullStarted(ctx, src.Name); err != nil {
		slog.Warn("source pull: update started_at", "source", src.Name, "error", err)
	}

	riverJobID := job.ID
	run, err := w.db.CreatePullRun(ctx, db.CreatePullRunParams{
		Name:        src.Name,
		RiverJobID:  &riverJobID,
		TaskType:    job.Args.Kind(),
		TriggerType: job.Args.TriggerType,
	})
	if err != nil {
		return errors.Wrap(err, "create pull run")
	}

	inserted, updated, unchanged, pullErr := w.pullAndInsert(ctx, src, run.ID)

	if pullErr != nil {
		errMsg := pullErr.Error()
		_ = w.db.FailPullRun(ctx, db.FailPullRunParams{
			ID:           run.ID,
			ErrorMessage: &errMsg,
		})
		_ = w.db.UpdateSourcePullFailed(ctx, db.UpdateSourcePullFailedParams{
			Name:      src.Name,
			LastError: &errMsg,
		})
		slog.Error("source pull failed", "source", src.Name, "job_id", job.ID, "error", pullErr)
		return pullErr
	}

	_ = w.db.SucceedPullRun(ctx, db.SucceedPullRunParams{
		ID:               run.ID,
		RowsSeen:         int32(inserted + updated + unchanged),
		RawRowsInserted:  int32(inserted),
		RawRowsUpdated:   int32(updated),
		RawRowsUnchanged: int32(unchanged),
	})
	_ = w.db.UpdateSourcePullSucceeded(ctx, db.UpdateSourcePullSucceededParams{
		Name:                 src.Name,
		LastSourceMarkerType: nil,
		LastSourceMarker:     nil,
		LastSourceModifiedAt: pgtype.Timestamptz{},
	})

	if src.ProcessorTaskType != nil && *src.ProcessorTaskType == "source_process" && w.rv != nil {
		if _, err := w.rv.Insert(ctx, SourceProcessArgs{
			SourceName: src.Name,
			PullRunID:  run.ID.String(),
		}, &river.InsertOpts{Queue: "source_process"}); err != nil {
			slog.Error("source pull: enqueue source_process job", "source", src.Name, "error", err)
			return errors.Wrap(err, "enqueue source_process job")
		}
	}
	return nil
}

func (w *SourcePullWorker) pullAndInsert(ctx context.Context, src db.DataSource, runID uuid.UUID) (inserted, updated, unchanged int, err error) {
	page := 1
	var rowErrs int
	for {
		resp, err := w.crawler.Crawl(ctx, src.Name, time.Time{}, nil, page)
		if err != nil {
			return inserted, updated, unchanged, errors.Wrap(err, "crawl page")
		}
		for _, rec := range resp.Records {
			raw, _ := json.Marshal(rec.RawData)
			i, u, unch, e := w.upsertRecord(ctx, src.Name, runID, rec, raw)
			inserted += i
			updated += u
			unchanged += unch
			if e != nil {
				slog.Warn("source pull: upsert row", "source", src.Name, "error", e)
				rowErrs++
			}
		}
		if !resp.HasMore {
			break
		}
		page++
	}
	if rowErrs > 0 {
		return inserted, updated, unchanged, fmt.Errorf("%d record(s) failed to upsert", rowErrs)
	}
	return
}

func (w *SourcePullWorker) upsertRecord(ctx context.Context, sourceName string, runID uuid.UUID, rec crawlerclient.CompanyRecord, raw []byte) (inserted, updated, unchanged int, err error) {
	hash := rec.SnapshotHash
	switch sourceName {
	case "gleif":
		if rec.LEI == nil || *rec.LEI == "" {
			return 0, 0, 0, errors.New("gleif record missing lei")
		}
		lei := *rec.LEI
		regStatus, _ := rec.RawData["registration_status"].(string)
		hqCountry, _ := rec.RawData["headquarters_country_code"].(string)
		parentLEI, _ := rec.RawData["direct_parent_lei"].(string)
		ultimateLEI, _ := rec.RawData["ultimate_parent_lei"].(string)
		row, err := w.db.UpsertGLEIFCompanyRawInput(ctx, db.UpsertGLEIFCompanyRawInputParams{
			SourcePullRunID:         runID,
			SourceNativeID:          lei,
			Lei:                     lei,
			LegalName:               ptrStr(rec.Name),
			RegistrationStatus:      ptrStr(regStatus),
			HeadquartersCountryCode: ptrStr(hqCountry),
			ParentLei:               ptrStr(parentLEI),
			UltimateParentLei:       ptrStr(ultimateLEI),
			SourceUpdatedAt:         pgtype.Timestamptz{},
			RawPayload:              raw,
			PayloadHash:             hash,
		})
		if err != nil {
			return 0, 0, 0, errors.Wrap(err, "upsert gleif")
		}
		if row.LastSeenAt.Equal(row.FirstSeenAt) {
			return 1, 0, 0, nil
		}
		if row.ProcessingStatus == "pending" {
			return 0, 1, 0, nil
		}
		return 0, 0, 1, nil

	case "companies_house":
		if rec.RegistrationNumber == nil || *rec.RegistrationNumber == "" {
			return 0, 0, 0, errors.New("companies_house record missing registration_number")
		}
		num := *rec.RegistrationNumber
		companyType, _ := rec.RawData["type"].(string)
		row, err := w.db.UpsertCompaniesHouseRawInput(ctx, db.UpsertCompaniesHouseRawInputParams{
			SourcePullRunID: runID,
			SourceNativeID:  num,
			CompanyName:     ptrStr(rec.Name),
			CompanyStatus:   ptrStr(rec.Status),
			CompanyType:     ptrStr(companyType),
			SourceUpdatedAt: pgtype.Timestamptz{},
			RawPayload:      raw,
			PayloadHash:     hash,
		})
		if err != nil {
			return 0, 0, 0, errors.Wrap(err, "upsert companies_house")
		}
		_ = row
		return 1, 0, 0, nil

	case "brreg":
		if rec.RegistrationNumber == nil || *rec.RegistrationNumber == "" {
			return 0, 0, 0, errors.New("brreg record missing registration_number")
		}
		num := *rec.RegistrationNumber
		website := ""
		if rec.Website != nil {
			website = *rec.Website
		}
		row, err := w.db.UpsertBrregRawInput(ctx, db.UpsertBrregRawInputParams{
			SourcePullRunID:  runID,
			SourceNativeID:   num,
			OrganizationName: ptrStr(rec.Name),
			Website:          ptrStr(website),
			SourceUpdatedAt:  pgtype.Timestamptz{},
			RawPayload:       raw,
			PayloadHash:      hash,
		})
		if err != nil {
			return 0, 0, 0, errors.Wrap(err, "upsert brreg")
		}
		_ = row
		return 1, 0, 0, nil

	default:
		return 0, 0, 0, fmt.Errorf("unknown source: %s", sourceName)
	}
}

func ptrStr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
