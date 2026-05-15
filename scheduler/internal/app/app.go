package app

import (
	"context"
	"net/http"

	"github.com/cockroachdb/errors"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	pgx "github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"

	"github.com/pulsarpoint/corpscout/scheduler/internal/config"
	"github.com/pulsarpoint/corpscout/scheduler/internal/httpapi"
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

	riverClient, err := setupRiver(ctx, pool, cfg)
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
