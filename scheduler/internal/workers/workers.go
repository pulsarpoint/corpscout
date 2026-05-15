// Package workers contains River job worker implementations.
package workers

import (
	"context"
	"log/slog"
	"time"

	"github.com/riverqueue/river"
)

// SourceCrawlArgs is the job argument for a source crawl task.
type SourceCrawlArgs struct {
	SourceName string    `json:"source_name"`
	Since      time.Time `json:"since"`
}

func (SourceCrawlArgs) Kind() string { return "source_crawl" }

// SourceCrawlWorker processes source crawl jobs.
type SourceCrawlWorker struct {
	river.WorkerDefaults[SourceCrawlArgs]
}

// Work executes a source crawl job.
func (w *SourceCrawlWorker) Work(ctx context.Context, job *river.Job[SourceCrawlArgs]) error {
	slog.Info("source crawl job received", "source", job.Args.SourceName, "since", job.Args.Since, "job_id", job.ID)
	return nil
}

// DomainResolveArgs is the job argument for a domain resolution task.
type DomainResolveArgs struct {
	CompanyID string `json:"company_id"`
}

func (DomainResolveArgs) Kind() string { return "domain_resolve" }

// DomainResolveWorker processes domain resolution jobs.
type DomainResolveWorker struct {
	river.WorkerDefaults[DomainResolveArgs]
}

// Work executes a domain resolve job.
func (w *DomainResolveWorker) Work(ctx context.Context, job *river.Job[DomainResolveArgs]) error {
	slog.Info("domain resolve job received", "company_id", job.Args.CompanyID, "job_id", job.ID)
	return nil
}
