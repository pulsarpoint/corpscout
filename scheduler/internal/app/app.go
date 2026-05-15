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
	httpapi.NewHandlers(queries, riverClient, pool, crawler).RegisterRoutes(r)

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

	for _, src := range sources {
		if !src.Enabled {
			continue
		}

		if src.LastCrawledAt.Valid {
			due := src.LastCrawledAt.Time.Add(time.Duration(src.CrawlIntervalHours) * time.Hour)
			if time.Now().Before(due) {
				continue
			}
		}

		since := time.Time{}
		if src.LastCrawledAt.Valid {
			since = src.LastCrawledAt.Time
		}

		_, err := rc.Insert(ctx, workers.SourceCrawlArgs{
			SourceName: src.Name,
			Since:      since,
		}, &river.InsertOpts{Queue: "source_crawl"})
		if err != nil {
			slog.Error("schedule sources: insert job", "source", src.Name, "error", err)
		}
	}
}
