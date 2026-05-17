package workers

import (
	"context"
	"log/slog"

	"github.com/cockroachdb/errors"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"

	db "github.com/pulsarpoint/corpscout/scheduler/internal/db/gen"
)

type SourceProcessWorker struct {
	river.WorkerDefaults[SourceProcessArgs]
	db   db.Querier
	pool *pgxpool.Pool
}

func NewSourceProcessWorker(q db.Querier, pool *pgxpool.Pool) *SourceProcessWorker {
	return &SourceProcessWorker{db: q, pool: pool}
}

func (w *SourceProcessWorker) Work(ctx context.Context, job *river.Job[SourceProcessArgs]) error {
	switch job.Args.SourceName {
	case "gleif":
		proc := NewGLEIFProcessor(w.db)
		if err := proc.ProcessBatch(ctx, job.Args.SourceName); err != nil {
			slog.Error("source process: gleif", "job_id", job.ID, "error", err)
			return err
		}
	case "companies_house":
		proc := NewCompaniesHouseProcessor(w.db)
		if err := proc.ProcessBatch(ctx, job.Args.SourceName); err != nil {
			slog.Error("source process: companies_house", "job_id", job.ID, "error", err)
			return err
		}
	case "brreg":
		proc := NewBrregProcessor(w.db)
		if err := proc.ProcessBatch(ctx, job.Args.SourceName); err != nil {
			slog.Error("source process: brreg", "job_id", job.ID, "error", err)
			return err
		}
	default:
		return errors.Newf("unknown source for processing: %s", job.Args.SourceName)
	}
	return nil
}
