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

type TemporalSourceWorkflow struct {
	WorkflowType string
	Country      string
	FirstMode    string
	NextMode     string
	BulkFirst    bool
}

var sourceWorkflows = map[string]TemporalSourceWorkflow{
	"companies_house": {WorkflowType: "PullCompaniesHouse", Country: "GB"},
	"brreg":           {WorkflowType: "PullBrreg", Country: "NO", FirstMode: "bulk", NextMode: "incremental", BulkFirst: true},
	"gleif":           {WorkflowType: "PullGLEIF", FirstMode: "bulk", NextMode: "delta", BulkFirst: true},
	"cvr":             {WorkflowType: "PullCVR", Country: "DK", FirstMode: "bulk", NextMode: "incremental", BulkFirst: true},
	"ariregister":     {WorkflowType: "PullAriregister", Country: "EE", FirstMode: "bulk", NextMode: "refresh", BulkFirst: true},
}

func TemporalWorkflowForSource(source string) (TemporalSourceWorkflow, bool) {
	cfg, ok := sourceWorkflows[source]
	return cfg, ok
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

	cfg, ok := TemporalWorkflowForSource(args.Source)
	if !ok {
		return fmt.Errorf("no workflow registered for source %q", args.Source)
	}

	// 1. Read saved checkpoint to determine pipeline mode.
	savedCursor := ""
	checkpointExists := false
	if checkpoint, err := w.db.GetSyncCheckpoint(ctx, args.Source); err == nil {
		checkpointExists = true
		savedCursor = checkpoint.Cursor
		slog.Info("data_task: checkpoint found", "source", args.Source, "cursor", savedCursor)
	}

	mode := ""
	if cfg.BulkFirst {
		mode = cfg.FirstMode
		if checkpointExists {
			mode = cfg.NextMode
		}
	}
	incrementalFrom := ""
	if args.Source == "brreg" && strings.HasPrefix(savedCursor, "bulk:") {
		incrementalFrom = strings.TrimPrefix(savedCursor, "bulk:") + ",0"
	}
	if mode != "" {
		slog.Info("data_task: source mode", "source", args.Source, "mode", mode, "incremental_from", incrementalFrom)
	}

	// 2. Insert a tracking row (status = starting).
	country := args.Country
	if country == "" {
		country = cfg.Country
	}
	riverJobID := job.ID
	exec, err := w.db.CreateTemporalExecution(ctx, db.CreateTemporalExecutionParams{
		WorkflowType: cfg.WorkflowType,
		SourceName:   args.Source,
		Country:      &country,
		InputIds:     args.IDs,
		RiverJobID:   &riverJobID,
	})
	if err != nil {
		return errors.Wrap(err, "create temporal execution record")
	}

	// 3. Start the Temporal workflow. Pass saved cursor for incremental pull.
	workflowID := fmt.Sprintf("pull-%s-%s-%d", args.Source, country, job.ID)
	we, err := w.temporal.ExecuteWorkflow(ctx,
		client.StartWorkflowOptions{
			ID:        workflowID,
			TaskQueue: "corpscout-pipelines",
		},
		cfg.WorkflowType,
		map[string]any{
			"country":          country,
			"ids":              args.IDs,
			"corpscout_run_id": exec.ID.String(),
			"run_id":           workflowID,
			"cursor":           savedCursor,
			"mode":             mode,
			"incremental_from": incrementalFrom,
			"force":            args.Force,
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
		"country", country,
	)
	return nil // River job done — Temporal handles the rest.
}
