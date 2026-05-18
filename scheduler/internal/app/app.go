package app

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	pgx "github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

	"github.com/pulsarpoint/corpscout/scheduler/internal/config"
	"github.com/pulsarpoint/corpscout/scheduler/internal/crawlerclient"
	db "github.com/pulsarpoint/corpscout/scheduler/internal/db/gen"
	"github.com/pulsarpoint/corpscout/scheduler/internal/httpapi"
	"github.com/pulsarpoint/corpscout/scheduler/internal/workers"
)

type Server struct {
	cfg   config.Config
	pool  *pgxpool.Pool
	river *river.Client[pgx.Tx]
	http  *http.Server
}

func NewServer(ctx context.Context, cfg config.Config) (*Server, error) {
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return nil, errors.Wrap(err, "create pgx pool")
	}
	if err := pool.Ping(ctx); err != nil {
		return nil, errors.Wrap(err, "ping database")
	}

	queries := db.New(pool)
	crawler := crawlerclient.New(cfg.CrawlerURL)

	if err := queries.InterruptStalePullRuns(ctx); err != nil {
		slog.Warn("startup: could not interrupt stale pull runs", "error", err)
	}

	riverClient, err := setupRiver(ctx, pool, cfg, queries, crawler)
	if err != nil {
		return nil, errors.Wrap(err, "setup river")
	}
	if err := riverClient.Start(ctx); err != nil {
		return nil, errors.Wrap(err, "start river client")
	}

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.Recoverer)
	r.Get("/health", httpapi.HandleHealth)
	httpapi.NewHandlers(queries, riverClient, pool, crawler, cfg.PostgRESTURL).RegisterRoutes(r)

	go scheduleSources(ctx, queries, riverClient)

	return &Server{
		cfg:   cfg,
		pool:  pool,
		river: riverClient,
		http:  &http.Server{Addr: cfg.ListenAddr, Handler: r},
	}, nil
}

func (s *Server) ListenAndServe() error {
	return s.http.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	if err := s.http.Shutdown(ctx); err != nil {
		return errors.Wrap(err, "http shutdown")
	}
	if err := s.river.Stop(ctx); err != nil {
		return errors.Wrap(err, "river stop")
	}
	s.pool.Close()
	return nil
}

// scheduleSources enqueues SourceCrawlArgs jobs for any enabled source that is
// due for its next crawl. It runs once on startup, then every 5 minutes.
func scheduleSources(ctx context.Context, q db.Querier, rc *river.Client[pgx.Tx]) {
	scheduleOnce(ctx, q, rc)

	tick := time.NewTicker(5 * time.Minute)
	defer tick.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			scheduleOnce(ctx, q, rc)
		}
	}
}

func scheduleOnce(ctx context.Context, q db.Querier, rc *river.Client[pgx.Tx]) {
	sources, err := q.ListSources(ctx)
	if err != nil {
		slog.Error("schedule sources: list sources", "error", err)
		return
	}

	now := time.Now()
	for _, src := range sources {
		due, err := sourceScheduleDue(src, now)
		if err != nil {
			slog.Warn("schedule sources: source schedule due", "source", src.Name, "error", err)
			continue
		}
		if !due {
			continue
		}
		if _, err := rc.Insert(ctx, workers.SourcePullArgs{
			SourceName:  src.Name,
			TriggerType: "scheduled",
		}, &river.InsertOpts{
			Queue: "source_pull",
			UniqueOpts: river.UniqueOpts{
				ByArgs:  true,
				ByState: []rivertype.JobState{rivertype.JobStateAvailable, rivertype.JobStateRunning, rivertype.JobStateScheduled},
			},
		}); err != nil {
			slog.Error("schedule sources: insert job", "source", src.Name, "error", err)
		}
	}
}

func sourceScheduleDue(src db.DataSource, now time.Time) (bool, error) {
	if !src.Enabled {
		return false, nil
	}
	if !src.ScheduleEnabled {
		return false, nil
	}
	if src.ScheduleKind != "interval" {
		return false, nil
	}
	if src.ScheduleExpression == nil {
		return false, nil
	}
	interval, err := parsePositiveDuration(*src.ScheduleExpression)
	if err != nil {
		return false, err
	}
	if src.LastStartedAt.Valid && now.Before(src.LastStartedAt.Time.Add(interval)) {
		return false, nil
	}
	return true, nil
}

func parsePositiveDuration(expr string) (time.Duration, error) {
	duration, err := time.ParseDuration(expr)
	if err != nil {
		return 0, errors.Wrap(err, "parse schedule expression")
	}
	if duration <= 0 {
		return 0, errors.Newf("schedule expression must be positive")
	}
	return duration, nil
}
