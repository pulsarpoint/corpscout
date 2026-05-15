package workers

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
	"github.com/pulsarpoint/corpscout/scheduler/internal/crawlerclient"
	db "github.com/pulsarpoint/corpscout/scheduler/internal/db/gen"
	"github.com/riverqueue/river"
)

// DomainResolveWorker resolves candidate domains for a company by calling the
// crawler's domain resolution pipeline and persisting results.
type DomainResolveWorker struct {
	river.WorkerDefaults[DomainResolveArgs]
	db      db.Querier
	crawler *crawlerclient.Client
}

// NewDomainResolveWorker creates a new DomainResolveWorker.
func NewDomainResolveWorker(q db.Querier, crawler *crawlerclient.Client) *DomainResolveWorker {
	return &DomainResolveWorker{
		db:      q,
		crawler: crawler,
	}
}

// Work executes a domain resolve job.
func (w *DomainResolveWorker) Work(ctx context.Context, job *river.Job[DomainResolveArgs]) error {
	// 1. Parse company ID.
	companyID, err := uuid.Parse(job.Args.CompanyID)
	if err != nil {
		return errors.Wrap(err, "parse company_id")
	}

	// 2. Load company.
	company, err := w.db.GetCompany(ctx, companyID)
	if err != nil {
		slog.Error("domain resolve: get company failed",
			"company_id", companyID, "job_id", job.ID, "error", err)
		return errors.Wrap(err, "get company")
	}

	// 3. Load country.
	country, err := w.db.GetCountryByID(ctx, company.CountryID)
	if err != nil {
		slog.Error("domain resolve: get country failed",
			"company_id", companyID, "country_id", company.CountryID, "job_id", job.ID, "error", err)
		return errors.Wrap(err, "get country")
	}

	// 4. Build LEI string (empty if nil).
	lei := ""
	if company.Lei != nil {
		lei = *company.Lei
	}

	// 5. Resolve domain candidates from crawler.
	resp, err := w.crawler.ResolveDomain(ctx, company.Name, lei, country.IsoAlpha2)
	if err != nil {
		slog.Error("domain resolve: resolve domain failed",
			"company_id", companyID, "company_name", company.Name, "job_id", job.ID, "error", err)
		return errors.Wrap(err, "resolve domain")
	}

	// 6. Persist each candidate.
	for _, candidate := range resp.Candidates {
		// a. Upsert the domain itself.
		domainRow, err := w.db.UpsertDomain(ctx, candidate.Domain)
		if err != nil {
			slog.Error("domain resolve: upsert domain failed",
				"company_id", companyID, "domain", candidate.Domain, "error", err)
			continue
		}

		// b. Determine status and relationship type by confidence threshold.
		status := "needs_review"
		relType := "candidate"
		if candidate.Confidence >= 85 {
			status = "active"
			relType = "official_site"
		}

		// c. Marshal evidence; ignore error per spec.
		evidenceBytes, _ := json.Marshal(candidate.Evidence)

		// d. Upsert company-domain linkage.
		_, err = w.db.UpsertCompanyDomain(ctx, db.UpsertCompanyDomainParams{
			CompanyID:        companyID,
			DomainID:         domainRow.ID,
			RelationshipType: relType,
			Status:           status,
			Signal:           candidate.Signal,
			Confidence:       int16(candidate.Confidence),
			Evidence:         evidenceBytes,
		})
		if err != nil {
			slog.Error("domain resolve: upsert company domain failed",
				"company_id", companyID, "domain", candidate.Domain, "error", err)
			continue
		}
	}

	return nil
}
