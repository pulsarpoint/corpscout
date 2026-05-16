package workers

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/pulsarpoint/corpscout/scheduler/internal/crawlerclient"
	db "github.com/pulsarpoint/corpscout/scheduler/internal/db/gen"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
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

// Timeout gives source crawl jobs 12 hours — large sources (brreg ~1M records)
// take several hours per full crawl and must not be cut off by the default 1h limit.
func (w *SourceCrawlWorker) Timeout(*river.Job[SourceCrawlArgs]) time.Duration {
	return 12 * time.Hour
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
			// Use context.Background() — the job context may already be cancelled
			// (e.g. deadline exceeded), so we must use an independent context to
			// ensure the status update actually reaches the database.
			_ = w.db.FailPullRun(context.Background(), db.FailPullRunParams{
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

			// Upsert enrichment (locations, phones, emails, industries, etc.)
			if err := w.upsertEnrichment(ctx, company.ID, rec, sourceName); err != nil {
				slog.Error("source crawl: upsert enrichment failed",
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

			// If the registry provided a website URL, persist it directly as a
			// registry_website signal (confidence 90) and skip the domain_resolve
			// job — no need to call the external signal pipeline.
			if rec.Website != nil && *rec.Website != "" {
				if domain, err := w.persistRegistryWebsite(ctx, company.ID, *rec.Website, sourceName); err != nil {
					slog.Error("source crawl: persist registry website failed",
						"source", sourceName, "company_id", company.ID, "website", *rec.Website, "error", err)
				} else if domain != "" {
					slog.Info("source crawl: persisted registry website",
						"source", sourceName, "company_id", company.ID, "domain", domain)
					continue
				}
			}

			// Enqueue domain resolve job if river client is available.
			if w.riverClient != nil {
				if _, err := w.riverClient.Insert(ctx, DomainResolveArgs{
					CompanyID: company.ID.String(),
				}, &river.InsertOpts{
					Queue: "domain_resolve",
					UniqueOpts: river.UniqueOpts{
						ByArgs:  true,
						ByState: []rivertype.JobState{rivertype.JobStateAvailable, rivertype.JobStatePending, rivertype.JobStateRunning, rivertype.JobStateRetryable, rivertype.JobStateScheduled},
					},
				}); err != nil {
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

// upsertEnrichment persists profile data extracted from the raw API response:
// website, founded year, employee estimate, locations, phones, emails, and industries.
func (w *SourceCrawlWorker) upsertEnrichment(ctx context.Context, companyID uuid.UUID, rec crawlerclient.CompanyRecord, sourceName string) error {
	// Update scalar enrichment fields (COALESCE — only overwrites nulls).
	var empJSON []byte
	if len(rec.EmployeeEstimate) > 0 {
		empJSON = mustJSON(rec.EmployeeEstimate)
	}
	if _, err := w.db.UpdateCompanyEnrichment(ctx, db.UpdateCompanyEnrichmentParams{
		ID:               companyID,
		Website:          rec.Website,
		FoundedYear:      rec.FoundedYear,
		EmployeeEstimate: empJSON,
	}); err != nil {
		return errors.Wrap(err, "update company enrichment")
	}

	evidence := json.RawMessage(mustJSON(map[string]any{"source": sourceName}))

	for _, loc := range rec.Locations {
		if _, err := w.db.UpsertCompanyLocation(ctx, db.UpsertCompanyLocationParams{
			CompanyID:    companyID,
			LocationType: loc.LocationType,
			AddressLine1: loc.AddressLine1,
			AddressLine2: loc.AddressLine2,
			City:         loc.City,
			Region:       loc.Region,
			PostalCode:   loc.PostalCode,
			Country:      loc.Country,
			CountryCode:  loc.CountryCode,
			Source:       sourceName,
			Evidence:     evidence,
		}); err != nil {
			slog.Warn("source crawl: upsert location failed", "source", sourceName, "company_id", companyID, "error", err)
		}
	}

	for _, ph := range rec.Phones {
		if ph.Phone == "" {
			continue
		}
		if _, err := w.db.UpsertCompanyPhone(ctx, db.UpsertCompanyPhoneParams{
			CompanyID: companyID,
			Phone:     ph.Phone,
			Purpose:   ph.Purpose,
			Source:    sourceName,
			Evidence:  evidence,
		}); err != nil {
			slog.Warn("source crawl: upsert phone failed", "source", sourceName, "company_id", companyID, "error", err)
		}
	}

	for _, em := range rec.Emails {
		if em.Email == "" {
			continue
		}
		if _, err := w.db.UpsertCompanyEmail(ctx, db.UpsertCompanyEmailParams{
			CompanyID: companyID,
			Email:     em.Email,
			Purpose:   em.Purpose,
			Source:    sourceName,
			Evidence:  evidence,
		}); err != nil {
			slog.Warn("source crawl: upsert email failed", "source", sourceName, "company_id", companyID, "error", err)
		}
	}

	for _, ind := range rec.Industries {
		if ind == "" {
			continue
		}
		if _, err := w.db.UpsertCompanyIndustry(ctx, db.UpsertCompanyIndustryParams{
			CompanyID: companyID,
			Industry:  ind,
			Source:    sourceName,
			Evidence:  evidence,
		}); err != nil {
			slog.Warn("source crawl: upsert industry failed", "source", sourceName, "company_id", companyID, "error", err)
		}
	}

	return nil
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

// persistRegistryWebsite upserts a domain from a registry-provided website URL and
// links it to the company as a registry_website signal (confidence 90).
// Returns the domain string on success, empty string if the URL is unusable.
func (w *SourceCrawlWorker) persistRegistryWebsite(ctx context.Context, companyID uuid.UUID, rawURL string, source string) (string, error) {
	domain := extractDomain(rawURL)
	if domain == "" {
		return "", nil
	}

	domainRow, err := w.db.UpsertDomain(ctx, domain)
	if err != nil {
		return "", errors.Wrap(err, "upsert domain")
	}

	evidence, _ := json.Marshal(map[string]any{"source": source, "raw_url": rawURL})
	_, err = w.db.UpsertCompanyDomain(ctx, db.UpsertCompanyDomainParams{
		CompanyID:        companyID,
		DomainID:         domainRow.ID,
		RelationshipType: "official_site",
		Status:           "active",
		Signal:           "registry_website",
		Confidence:       90,
		Evidence:         evidence,
	})
	if err != nil {
		return "", errors.Wrap(err, "upsert company domain")
	}
	return domain, nil
}

// extractDomain parses a raw URL or bare hostname from a registry website field
// and returns the lowercase hostname with no port, or empty string if unusable.
func extractDomain(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	// Add scheme if missing so url.Parse works correctly.
	if !strings.Contains(raw, "://") {
		raw = "http://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	host := strings.ToLower(u.Hostname())
	host = strings.TrimSuffix(host, ".")
	for strings.HasPrefix(host, "*.") {
		host = host[2:]
	}
	if host == "" || host == "localhost" || !strings.Contains(host, ".") {
		return ""
	}
	// Reject raw IP addresses.
	if isIPAddress(host) {
		return ""
	}
	return host
}

// isIPAddress returns true if s is a valid IPv4 or IPv6 address literal.
func isIPAddress(s string) bool {
	// IPv6 literal inside brackets is already stripped by url.Hostname().
	// Simple heuristic: if all segments of dot-split are numeric, it's an IPv4.
	parts := strings.Split(s, ".")
	if len(parts) == 4 {
		allDigits := true
		for _, p := range parts {
			if len(p) == 0 || len(p) > 3 {
				allDigits = false
				break
			}
			for _, c := range p {
				if c < '0' || c > '9' {
					allDigits = false
					break
				}
			}
		}
		if allDigits {
			return true
		}
	}
	// IPv6: contains colons.
	return strings.Contains(s, ":")
}

// mustJSON marshals v to JSON, returning an empty object on error.
func mustJSON(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		return []byte("{}")
	}
	return b
}
