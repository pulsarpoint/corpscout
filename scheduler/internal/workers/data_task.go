package workers

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/cockroachdb/errors"
	"github.com/riverqueue/river"
	"go.temporal.io/sdk/client"

	db "github.com/pulsarpoint/corpscout/scheduler/internal/db/gen"
)

// sourceWorkflowType maps a source name to its Temporal workflow type.
// Add new sources here as they are implemented.
var sourceWorkflowType = map[string]string{
	"companies_house": "PullCompaniesHouse",
	"brreg":           "PullBrreg",
}

// sourceDefaultCountry maps a source name to its default ISO-3166 country code.
var sourceDefaultCountry = map[string]string{
	"companies_house": "GB",
	"brreg":           "NO",
}

// TemporalWorkflowForSource returns the workflow type and default country for a
// source that has a Temporal pipeline, or ("", "") if none is registered.
func TemporalWorkflowForSource(source string) (workflowType, country string) {
	return sourceWorkflowType[source], sourceDefaultCountry[source]
}

// DataTaskWorker starts a source-specific Temporal workflow and records its ID.
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

	wfType, ok := sourceWorkflowType[args.Source]
	if !ok {
		return fmt.Errorf("no workflow registered for source %q", args.Source)
	}

	// 1. Read saved checkpoint to determine pipeline mode.
	// Brreg: "bulk:YYYY-MM-DD" cursor means bulk was done → incremental.
	// No checkpoint or any other cursor → bulk first.
	savedCursor := ""
	brregMode := ""          // only set for brreg
	brregIncrementalFrom := ""
	if checkpoint, err := w.db.GetSyncCheckpoint(ctx, args.Source); err == nil {
		savedCursor = checkpoint.Cursor
		slog.Info("data_task: checkpoint found", "source", args.Source, "cursor", savedCursor)
	}
	if args.Source == "brreg" {
		if strings.HasPrefix(savedCursor, "bulk:") {
			brregMode = "incremental"
			brregIncrementalFrom = strings.TrimPrefix(savedCursor, "bulk:") + ",0"
		} else {
			brregMode = "bulk"
		}
		slog.Info("data_task: brreg mode", "mode", brregMode, "incremental_from", brregIncrementalFrom)
	}

	// 2. Insert a tracking row (status = starting).
	country := args.Country
	riverJobID := job.ID
	exec, err := w.db.CreateTemporalExecution(ctx, db.CreateTemporalExecutionParams{
		WorkflowType: wfType,
		SourceName:   args.Source,
		Country:      &country,
		InputIds:     args.IDs,
		RiverJobID:   &riverJobID,
	})
	if err != nil {
		return errors.Wrap(err, "create temporal execution record")
	}

	// 3. Start the Temporal workflow. Pass saved cursor for incremental pull.
	workflowID := fmt.Sprintf("pull-%s-%s-%d", args.Source, args.Country, job.ID)
	we, err := w.temporal.ExecuteWorkflow(ctx,
		client.StartWorkflowOptions{
			ID:        workflowID,
			TaskQueue: "corpscout-pipelines",
		},
		wfType,
		map[string]any{
			"country":          args.Country,
			"ids":              args.IDs,
			"corpscout_run_id": exec.ID.String(),
			"cursor":           savedCursor,
			"mode":             brregMode,
			"incremental_from": brregIncrementalFrom,
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

	// 4. Record the workflow ID so the UI can track it.
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
