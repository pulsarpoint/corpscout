package config_test

import (
	"testing"

	"github.com/pulsarpoint/corpscout/scheduler/internal/config"
)

func TestLoad_defaults(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://test")
	cfg := config.Load()

	if cfg.ListenAddr != ":8090" {
		t.Errorf("want :8090, got %s", cfg.ListenAddr)
	}
	if cfg.CrawlConcurrency != 5 {
		t.Errorf("want 5, got %d", cfg.CrawlConcurrency)
	}
	if cfg.DomainConcurrency != 10 {
		t.Errorf("want 10, got %d", cfg.DomainConcurrency)
	}
}

func TestLoad_overrides(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://test")
	t.Setenv("CORPSCOUT_LISTEN_ADDR", ":9000")
	t.Setenv("CORPSCOUT_CRAWL_CONCURRENCY", "3")

	cfg := config.Load()

	if cfg.ListenAddr != ":9000" {
		t.Errorf("want :9000, got %s", cfg.ListenAddr)
	}
	if cfg.CrawlConcurrency != 3 {
		t.Errorf("want 3, got %d", cfg.CrawlConcurrency)
	}
}
