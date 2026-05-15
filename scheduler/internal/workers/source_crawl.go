package workers

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/pulsarpoint/corpscout/scheduler/internal/crawlerclient"
	db "github.com/pulsarpoint/corpscout/scheduler/internal/db/gen"
	"github.com/riverqueue/river"
)

// SourceCrawlWorker processes source crawl jobs by fetching company records
// from the crawler service and upserting them into the database.
type SourceCrawlWorker struct {
	river.WorkerDefaults[SourceCrawlArgs]
	db          db.Querier
	crawler     *crawlerclient.Client
	riverClient *river.Client[pgx.Tx]
}

// NewSourceCrawlWorker creates a new SourceCrawlWorker.
// riverClient may be nil (e.g. in tests) — domain resolve jobs will simply be skipped.
func NewSourceCrawlWorker(q db.Querier, crawler *crawlerclient.Client, riverClient *river.Client[pgx.Tx]) *SourceCrawlWorker {
	return &SourceCrawlWorker{
		db:          q,
		crawler:     crawler,
		riverClient: riverClient,
	}
}

// SetRiverClient injects the river client after construction to break the
// chicken-and-egg dependency: workers must be registered before the client exists.
func (w *SourceCrawlWorker) SetRiverClient(rc *river.Client[pgx.Tx]) {
	w.riverClient = rc
}

// Work executes a source crawl job.
func (w *SourceCrawlWorker) Work(ctx context.Context, job *river.Job[SourceCrawlArgs]) error {
	sourceName := job.Args.SourceName
	since := job.Args.Since

	// 1. Get source by name.
	source, err := w.db.GetSourceByName(ctx, sourceName)
	if err != nil {
		slog.Error("source crawl: get source failed", "source", sourceName, "job_id", job.ID, "error", err)
		return errors.Wrap(err, "get source by name")
	}

	// 2. Create pull run.
	jobID := job.ID
	pullRun, err := w.db.CreatePullRun(ctx, db.CreatePullRunParams{
		SourceID:    source.ID,
		RiverJobID:  &jobID,
		CursorStart: source.LastCursor,
	})
	if err != nil {
		slog.Error("source crawl: create pull run failed", "source", sourceName, "job_id", job.ID, "error", err)
		return errors.Wrap(err, "create pull run")
	}

	// Mark last_crawled_at now so scheduleOnce won't re-enqueue this source
	// while the crawl is still running (which can take hours for large sources).
	if err := w.db.UpdateSourceCursor(ctx, db.UpdateSourceCursorParams{
		ID:            source.ID,
		LastCursor:    source.LastCursor,
		LastCrawledAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}); err != nil {
		slog.Error("source crawl: stamp last_crawled_at failed", "source", sourceName, "job_id", job.ID, "error", err)
		return errors.Wrap(err, "stamp last_crawled_at")
	}

	pullRunUUID := pgtype.UUID{Bytes: pullRun.ID, Valid: true}
	sourceUUID := pgtype.UUID{Bytes: source.ID, Valid: true}

	var (
		cursor          = source.LastCursor
		page            = 1
		totalFetched    int32
		totalUpserted   int32
		lastCursor      *string
	)

	// 3. Crawl loop.
	for {
		resp, err := w.crawler.Crawl(ctx, sourceName, since, cursor, page)
		if err != nil {
			slog.Error("source crawl: crawl failed", "source", sourceName, "job_id", job.ID, "page", page, "error", err)
			errMsg := err.Error()
			_ = w.db.FailPullRun(ctx, db.FailPullRunParams{
				ID:           pullRun.ID,
				ErrorMessage: &errMsg,
			})
			return errors.Wrap(err, fmt.Sprintf("crawl page %d", page))
		}

		// Store snapshot.
		payload := mustJSON(resp.Records)
		hash := fmt.Sprintf("%x", sha256.Sum256(payload))
		_ = w.db.InsertSourceSnapshot(ctx, db.InsertSourceSnapshotParams{
			SourceID:    source.ID,
			PullRunID:   pullRun.ID,
			PayloadHash: hash,
			Payload:     json.RawMessage(payload),
		})

		totalFetched += int32(len(resp.Records))

		// Process each record.
		for _, rec := range resp.Records {
			country, err := w.db.GetCountryByISO2(ctx, strings.ToUpper(rec.CountryISO2))
			if err != nil {
				// Skip record if country not found.
				slog.Warn("source crawl: country not found, skipping record",
					"source", sourceName, "country_iso2", rec.CountryISO2, "company", rec.Name)
				continue
			}

			company, err := w.upsertCompany(ctx, rec, country.ID, sourceUUID)
			if err != nil {
				if errors.Is(err, errNoIdentifier) {
					slog.Warn("source crawl: skipping company with no stable identifier",
						"source", sourceName, "company", rec.Name, "country", rec.CountryISO2)
				} else {
					slog.Error("source crawl: upsert company failed",
						"source", sourceName, "company", rec.Name, "error", err)
				}
				continue
			}

			// Upsert company source link.
			if err := w.db.UpsertCompanySource(ctx, db.UpsertCompanySourceParams{
				CompanyID:  company.ID,
				SourceID:   source.ID,
				ExternalID: companyExternalID(rec),
				PullRunID:  pullRunUUID,
				RawData:    mustJSON(rec.RawData),
				FetchedAt:  time.Now(),
			}); err != nil {
				slog.Error("source crawl: upsert company source failed",
					"source", sourceName, "company", rec.Name, "error", err)
			}

			// Upsert aliases.
			for _, alias := range rec.Aliases {
				if alias == "" {
					continue
				}
				if err := w.db.UpsertCompanyAlias(ctx, db.UpsertCompanyAliasParams{
					CompanyID: company.ID,
					Alias:     alias,
					AliasType: "trading_name",
					SourceID:  sourceUUID,
				}); err != nil {
					slog.Error("source crawl: upsert alias failed",
						"source", sourceName, "company", rec.Name, "alias", alias, "error", err)
				}
			}

			totalUpserted++

			// Enqueue domain resolve job if river client is available.
			if w.riverClient != nil {
				if _, err := w.riverClient.Insert(ctx, DomainResolveArgs{
					CompanyID: company.ID.String(),
				}, &river.InsertOpts{Queue: "domain_resolve"}); err != nil {
					slog.Error("source crawl: insert domain resolve job failed",
						"source", sourceName, "company_id", company.ID, "error", err)
				}
			}
		}

		lastCursor = resp.NextCursor
		if !resp.HasMore {
			break
		}
		cursor = resp.NextCursor
		page++
	}

	// 4. Complete pull run.
	if err := w.db.CompletePullRun(ctx, db.CompletePullRunParams{
		ID:              pullRun.ID,
		CursorEnd:       lastCursor,
		RecordsFetched:  totalFetched,
		RecordsUpserted: totalUpserted,
	}); err != nil {
		slog.Error("source crawl: complete pull run failed", "source", sourceName, "job_id", job.ID, "error", err)
		return errors.Wrap(err, "complete pull run")
	}

	// 5. Update source cursor.
	if err := w.db.UpdateSourceCursor(ctx, db.UpdateSourceCursorParams{
		ID:            source.ID,
		LastCursor:    lastCursor,
		LastCrawledAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}); err != nil {
		slog.Error("source crawl: update source cursor failed", "source", sourceName, "job_id", job.ID, "error", err)
		return errors.Wrap(err, "update source cursor")
	}

	// 6. Log completion.
	slog.Info("source crawl complete",
		"source", sourceName,
		"job_id", job.ID,
		"records_fetched", totalFetched,
		"records_upserted", totalUpserted,
	)
	return nil
}

// errNoIdentifier is returned by upsertCompany when a record lacks both a LEI
// and a registration number. The unique indexes cannot prevent duplicates for
// such records, so the company is skipped rather than silently duplicated.
var errNoIdentifier = errors.New("no stable identifier")

// upsertCompany inserts or updates a company record by LEI or registration number.
func (w *SourceCrawlWorker) upsertCompany(ctx context.Context, rec crawlerclient.CompanyRecord, countryID uuid.UUID, primarySourceID pgtype.UUID) (db.Company, error) {
	if rec.LEI != nil && *rec.LEI != "" {
		return w.db.UpsertCompanyByLEI(ctx, db.UpsertCompanyByLEIParams{
			Lei:                rec.LEI,
			Name:               rec.Name,
			CountryID:          countryID,
			RegistrationNumber: rec.RegistrationNumber,
			Status:             rec.Status,
			PrimarySourceID:    primarySourceID,
		})
	}
	if rec.RegistrationNumber != nil && *rec.RegistrationNumber != "" {
		return w.db.UpsertCompanyByRegNumber(ctx, db.UpsertCompanyByRegNumberParams{
			Name:               rec.Name,
			CountryID:          countryID,
			RegistrationNumber: rec.RegistrationNumber,
			Status:             rec.Status,
			PrimarySourceID:    primarySourceID,
		})
	}
	return db.Company{}, errNoIdentifier
}

// companyExternalID returns the best available external identifier for a company record.
func companyExternalID(rec crawlerclient.CompanyRecord) string {
	if rec.LEI != nil && *rec.LEI != "" {
		return *rec.LEI
	}
	if rec.RegistrationNumber != nil && *rec.RegistrationNumber != "" {
		return *rec.RegistrationNumber
	}
	return rec.Name
}

// mustJSON marshals v to JSON, returning an empty object on error.
func mustJSON(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		return []byte("{}")
	}
	return b
}
