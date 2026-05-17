package workers

import (
	"context"

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
	return nil
}
