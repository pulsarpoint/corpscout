package workers

import (
	"context"

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
}

func NewSourcePullWorker(q db.Querier, crawler *crawlerclient.Client, pool *pgxpool.Pool) *SourcePullWorker {
	return &SourcePullWorker{db: q, crawler: crawler, pool: pool}
}

func (w *SourcePullWorker) Work(ctx context.Context, job *river.Job[SourcePullArgs]) error {
	return nil
}
