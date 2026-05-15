package workers

import (
	"context"
	"fmt"

	"github.com/riverqueue/river"
)

// DomainResolveWorker processes domain resolution jobs.
// TODO(task4): replace stub with full implementation.
type DomainResolveWorker struct {
	river.WorkerDefaults[DomainResolveArgs]
}

// Work executes a domain resolve job.
func (w *DomainResolveWorker) Work(_ context.Context, job *river.Job[DomainResolveArgs]) error {
	return fmt.Errorf("domain_resolve worker not implemented: company_id=%s", job.Args.CompanyID)
}
