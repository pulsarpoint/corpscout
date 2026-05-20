package workers

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/cockroachdb/errors"
	"github.com/riverqueue/river"
	"go.temporal.io/sdk/client"

	db "github.com/pulsarpoint/corpscout/scheduler/internal/db/gen"
)

// DataTaskWorker starts a Temporal PullCompanies workflow and records its ID.
// The River job exits immediately — Temporal runs the pipeline independently.
type DataTaskWorker struct {
	river.WorkerDefaults[DataTaskArgs]
	db       db.Querier
	temporal client.Client
}

func NewDataTaskWorker(q db.Querier, tc client.Client) *DataTaskWorker {
	return &DataTaskWorker{db: q, temporal: tc}
}

func (w *DataTaskWorker) Work(ctx context.Context, job *river.Job[DataTaskArgs]) error {
	args := job.Args

	// 1. Insert a tracking row (status = starting).
	country := args.Country
	riverJobID := job.ID
	exec, err := w.db.CreateTemporalExecution(ctx, db.CreateTemporalExecutionParams{
		WorkflowType: "PullCompanies",
		SourceName:   args.Source,
		Country:      &country,
		InputIds:     args.IDs,
		RiverJobID:   &riverJobID,
	})
	if err != nil {
		return errors.Wrap(err, "create temporal execution record")
	}

	// 2. Start the Temporal workflow.
	workflowID := fmt.Sprintf("pull-%s-%s-%d", args.Source, args.Country, job.ID)
	we, err := w.temporal.ExecuteWorkflow(ctx,
		client.StartWorkflowOptions{
			ID:        workflowID,
			TaskQueue: "corpscout-pipelines",
		},
		"PullCompanies",
		map[string]any{
			"source":           args.Source,
			"country":          args.Country,
			"ids":              args.IDs,
			"corpscout_run_id": exec.ID.String(),
		},
	)
	if err != nil {
		errMsg := err.Error()
		dbErr := w.db.UpdateTemporalExecutionFailed(ctx, db.UpdateTemporalExecutionFailedParams{
			ID:           exec.ID,
			ErrorMessage: &errMsg,
		})
		if dbErr != nil {
			slog.Warn("data_task: mark execution failed after workflow start error", "error", dbErr)
		}
		return errors.Wrap(err, "start temporal workflow")
	}

	// 3. Record the workflow ID so the UI can track it.
	runID := we.GetRunID()
	if err := w.db.UpdateTemporalExecutionStarted(ctx, db.UpdateTemporalExecutionStartedParams{
		ID:            exec.ID,
		WorkflowID:    &workflowID,
		WorkflowRunID: &runID,
	}); err != nil {
		slog.Warn("data_task: update temporal execution started", "error", err)
	}

	slog.Info("data_task: Temporal workflow started",
		"workflow_id", workflowID,
		"source", args.Source,
		"country", args.Country,
	)
	return nil // River job done — Temporal handles the rest.
}
