package workers

import (
	"context"
	"log/slog"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/pulsarpoint/corpscout/scheduler/internal/crawlerclient"
	db "github.com/pulsarpoint/corpscout/scheduler/internal/db/gen"
	"github.com/riverqueue/river"
)

// GLEIFEnrichWorker fetches direct and ultimate parent LEI codes for a GLEIF
// company and stores them on the companies row. The GLEIF relationship API
// returns 404 for companies with no reported parent, which is treated as a
// successful no-op (parent LEI stays NULL).
type GLEIFEnrichWorker struct {
	river.WorkerDefaults[GLEIFEnrichArgs]
	db      db.Querier
	crawler *crawlerclient.Client
}

func NewGLEIFEnrichWorker(q db.Querier, crawler *crawlerclient.Client) *GLEIFEnrichWorker {
	return &GLEIFEnrichWorker{db: q, crawler: crawler}
}

func (w *GLEIFEnrichWorker) Timeout(*river.Job[GLEIFEnrichArgs]) time.Duration {
	return 60 * time.Second
}

func (w *GLEIFEnrichWorker) Work(ctx context.Context, job *river.Job[GLEIFEnrichArgs]) error {
	companyID, err := uuid.Parse(job.Args.CompanyID)
	if err != nil {
		return errors.Wrap(err, "parse company_id")
	}

	rel, err := w.crawler.GLEIFRelationship(ctx, job.Args.LEI)
	if err != nil {
		slog.Error("gleif enrich: relationship lookup failed",
			"company_id", job.Args.CompanyID,
			"lei", job.Args.LEI,
			"job_id", job.ID,
			"error", err,
		)
		return errors.Wrap(err, "gleif relationship")
	}

	if err := w.db.UpdateCompanyParentLEI(ctx, db.UpdateCompanyParentLEIParams{
		ID:               companyID,
		ParentLei:        rel.DirectParentLEI,
		UltimateParentLei: rel.UltimateParentLEI,
	}); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return errors.Wrap(err, "update company parent lei")
	}

	slog.Info("gleif enrich: parent LEI stored",
		"company_id", job.Args.CompanyID,
		"lei", job.Args.LEI,
		"direct_parent_lei", rel.DirectParentLEI,
		"ultimate_parent_lei", rel.UltimateParentLEI,
	)
	return nil
}
