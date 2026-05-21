package workers

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/client"

	db "github.com/pulsarpoint/corpscout/scheduler/internal/db/gen"
)

func TestTemporalWorkflowForSource(t *testing.T) {
	cases := []struct {
		source    string
		workflow  string
		country   string
		firstMode string
		nextMode  string
		bulkFirst bool
	}{
		{"companies_house", "PullCompaniesHouse", "GB", "", "", false},
		{"brreg", "PullBrreg", "NO", "bulk", "incremental", true},
		{"gleif", "PullGLEIF", "", "bulk", "delta", true},
		{"cvr", "PullCVR", "DK", "bulk", "incremental", true},
		{"ariregister", "PullAriregister", "EE", "bulk", "refresh", true},
	}

	for _, tc := range cases {
		t.Run(tc.source, func(t *testing.T) {
			cfg, ok := TemporalWorkflowForSource(tc.source)
			require.True(t, ok)
			require.Equal(t, tc.workflow, cfg.WorkflowType)
			require.Equal(t, tc.country, cfg.Country)
			require.Equal(t, tc.firstMode, cfg.FirstMode)
			require.Equal(t, tc.nextMode, cfg.NextMode)
			require.Equal(t, tc.bulkFirst, cfg.BulkFirst)
		})
	}
}

func TestTemporalWorkflowForSource_unknown(t *testing.T) {
	cfg, ok := TemporalWorkflowForSource("unknown")
	require.False(t, ok)
	require.Empty(t, cfg)
}

func TestDataTaskWorkerWork_buildsWorkflowInput(t *testing.T) {
	cases := []struct {
		name                    string
		args                    DataTaskArgs
		checkpoint              string
		checkpointExists        bool
		expectedWorkflow        string
		expectedWorkflowID      string
		expectedCountry         string
		expectedMode            string
		expectedCursor          string
		expectedIncrementalFrom string
		expectedForce           bool
	}{
		{
			name:               "gleif first bulk without country",
			args:               DataTaskArgs{Source: "gleif", Force: true},
			expectedWorkflow:   "PullGLEIF",
			expectedWorkflowID: "pull-gleif-42",
			expectedMode:       "bulk",
			expectedForce:      true,
		},
		{
			name:                    "brreg checkpoint switches to incremental",
			args:                    DataTaskArgs{Source: "brreg"},
			checkpoint:              "bulk:2026-05-20",
			checkpointExists:        true,
			expectedWorkflow:        "PullBrreg",
			expectedWorkflowID:      "pull-brreg-NO-42",
			expectedCountry:         "NO",
			expectedMode:            "incremental",
			expectedCursor:          "bulk:2026-05-20",
			expectedIncrementalFrom: "2026-05-20,0",
		},
		{
			name:               "ariregister checkpoint switches to refresh",
			args:               DataTaskArgs{Source: "ariregister"},
			checkpoint:         "snapshot:2026-05-20",
			checkpointExists:   true,
			expectedWorkflow:   "PullAriregister",
			expectedWorkflowID: "pull-ariregister-EE-42",
			expectedCountry:    "EE",
			expectedMode:       "refresh",
			expectedCursor:     "snapshot:2026-05-20",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			q := &dataTaskFakeDB{
				checkpoint:       tc.checkpoint,
				checkpointExists: tc.checkpointExists,
				executionID:      uuid.New(),
			}
			tcClient := &dataTaskFakeTemporal{runID: "temporal-run-id"}
			worker := NewDataTaskWorker(q, tcClient)

			err := worker.Work(context.Background(), &river.Job[DataTaskArgs]{
				JobRow: &rivertype.JobRow{ID: 42},
				Args:   tc.args,
			})
			require.NoError(t, err)

			require.Equal(t, tc.expectedWorkflow, q.createParams.WorkflowType)
			require.Equal(t, tc.args.Source, q.createParams.SourceName)
			require.NotNil(t, q.createParams.Country)
			require.Equal(t, tc.expectedCountry, *q.createParams.Country)
			require.NotNil(t, q.startedParams.WorkflowID)
			require.Equal(t, tc.expectedWorkflowID, *q.startedParams.WorkflowID)
			require.NotNil(t, q.startedParams.WorkflowRunID)
			require.Equal(t, "temporal-run-id", *q.startedParams.WorkflowRunID)

			require.Equal(t, tc.expectedWorkflowID, tcClient.options.ID)
			require.Equal(t, "corpscout-pipelines", tcClient.options.TaskQueue)
			require.Equal(t, tc.expectedWorkflow, tcClient.workflow)
			require.Len(t, tcClient.args, 1)

			input, ok := tcClient.args[0].(map[string]any)
			require.True(t, ok)
			require.Equal(t, tc.expectedCountry, input["country"])
			require.Equal(t, tc.expectedMode, input["mode"])
			require.Equal(t, tc.expectedCursor, input["cursor"])
			require.Equal(t, tc.expectedIncrementalFrom, input["incremental_from"])
			require.Equal(t, q.executionID.String(), input["corpscout_run_id"])
			require.Equal(t, tc.expectedWorkflowID, input["run_id"])
			require.Equal(t, tc.expectedForce, input["force"])
		})
	}
}

type dataTaskFakeDB struct {
	db.Querier
	checkpoint       string
	checkpointExists bool
	executionID      uuid.UUID
	createParams     db.CreateTemporalExecutionParams
	startedParams    db.UpdateTemporalExecutionStartedParams
}

func (d *dataTaskFakeDB) GetSyncCheckpoint(context.Context, string) (db.SourceSyncCheckpoint, error) {
	if !d.checkpointExists {
		return db.SourceSyncCheckpoint{}, errors.New("not found")
	}
	return db.SourceSyncCheckpoint{Cursor: d.checkpoint}, nil
}

func (d *dataTaskFakeDB) CreateTemporalExecution(_ context.Context, arg db.CreateTemporalExecutionParams) (db.TemporalExecution, error) {
	d.createParams = arg
	return db.TemporalExecution{ID: d.executionID}, nil
}

func (d *dataTaskFakeDB) UpdateTemporalExecutionStarted(_ context.Context, arg db.UpdateTemporalExecutionStartedParams) error {
	d.startedParams = arg
	return nil
}

func (d *dataTaskFakeDB) UpdateTemporalExecutionFailed(context.Context, db.UpdateTemporalExecutionFailedParams) error {
	return nil
}

type dataTaskFakeTemporal struct {
	client.Client
	options  client.StartWorkflowOptions
	workflow any
	args     []interface{}
	runID    string
}

func (d *dataTaskFakeTemporal) ExecuteWorkflow(_ context.Context, options client.StartWorkflowOptions, workflow interface{}, args ...interface{}) (client.WorkflowRun, error) {
	d.options = options
	d.workflow = workflow
	d.args = args
	return dataTaskFakeWorkflowRun{id: options.ID, runID: d.runID}, nil
}

type dataTaskFakeWorkflowRun struct {
	id    string
	runID string
}

func (d dataTaskFakeWorkflowRun) GetID() string {
	return d.id
}

func (d dataTaskFakeWorkflowRun) GetRunID() string {
	return d.runID
}

func (d dataTaskFakeWorkflowRun) Get(context.Context, interface{}) error {
	return nil
}

func (d dataTaskFakeWorkflowRun) GetWithOptions(context.Context, interface{}, client.WorkflowRunGetOptions) error {
	return nil
}
