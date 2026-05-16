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
	"github.com/pulsarpoint/corpscout/scheduler/internal/workers"
)

func setupRiver(ctx context.Context, pool *pgxpool.Pool, cfg config.Config, q db.Querier, crawler *crawlerclient.Client) (*river.Client[pgx.Tx], error) {
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

	sourceCrawlWorker := workers.NewSourceCrawlWorker(q, crawler, nil)
	domainResolveWorker := workers.NewDomainResolveWorker(q, crawler)
	gleifEnrichWorker := workers.NewGLEIFEnrichWorker(q, crawler)

	w := river.NewWorkers()
	river.AddWorker(w, sourceCrawlWorker)
	river.AddWorker(w, domainResolveWorker)
	river.AddWorker(w, gleifEnrichWorker)

	riverCfg := &river.Config{
		Queues: map[string]river.QueueConfig{
			"source_crawl":   {MaxWorkers: cfg.CrawlConcurrency},
			"domain_resolve": {MaxWorkers: cfg.DomainConcurrency},
			"gleif_enrich":   {MaxWorkers: cfg.GLEIFEnrichConcurrency},
		},
		Workers: w,
	}

	rc, err := river.NewClient(riverpgxv5.New(pool), riverCfg)
	if err != nil {
		return nil, err
	}

	sourceCrawlWorker.SetRiverClient(rc)
	return rc, nil
}
