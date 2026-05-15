// Package workers contains River job worker implementations.
package workers

import (
	"context"
	"fmt"
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
	return fmt.Errorf("source_crawl worker not implemented")
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
	return fmt.Errorf("domain_resolve worker not implemented")
}
