package app

import (
	"context"
	"log/slog"

	pgx "github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivermigrate"

	"github.com/pulsarpoint/corpscout/scheduler/internal/config"
	"github.com/pulsarpoint/corpscout/scheduler/internal/crawlerclient"
	db "github.com/pulsarpoint/corpscout/scheduler/internal/db/gen"
	"github.com/pulsarpoint/corpscout/scheduler/internal/s3client"
	"github.com/pulsarpoint/corpscout/scheduler/internal/workers"
)

func setupRiver(ctx context.Context, pool *pgxpool.Pool, cfg config.Config, q db.Querier, crawler *crawlerclient.Client, s3 *s3client.Client) (*river.Client[pgx.Tx], error) {
	migrator, err := rivermigrate.New(riverpgxv5.New(pool), nil)
	if err != nil {
		return nil, err
	}
	res, err := migrator.Migrate(ctx, rivermigrate.DirectionUp, nil)
	if err != nil {
		return nil, err
	}
	for _, v := range res.Versions {
		slog.Info("river migration applied", "version", v.Version, "direction", "up")
	}

	sourcePullWorker := workers.NewSourcePullWorker(q, crawler, pool)
	sourceProcessWorker := workers.NewSourceProcessWorker(q, pool)
	domainCrawlWorker := workers.NewDomainCrawlWorker(q, crawler, s3)
	domainImportWorker := workers.NewDomainImportWorker(q, s3)

	w := river.NewWorkers()
	river.AddWorker(w, sourcePullWorker)
	river.AddWorker(w, sourceProcessWorker)
	river.AddWorker(w, domainCrawlWorker)
	river.AddWorker(w, domainImportWorker)

	riverCfg := &river.Config{
		Queues: map[string]river.QueueConfig{
			"source_pull":    {MaxWorkers: cfg.CrawlConcurrency},
			"source_process": {MaxWorkers: cfg.DomainConcurrency},
			"domain_crawl":   {MaxWorkers: 3},
			"domain_import":  {MaxWorkers: 2},
		},
		Workers: w,
	}

	rc, err := river.NewClient(riverpgxv5.New(pool), riverCfg)
	if err != nil {
		return nil, err
	}
	sourcePullWorker.SetRiverClient(rc)
	return rc, nil
}
