package workers_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulsarpoint/corpscout/scheduler/internal/crawlerclient"
	db "github.com/pulsarpoint/corpscout/scheduler/internal/db/gen"
	"github.com/pulsarpoint/corpscout/scheduler/internal/workers"
)

// stubPullQuerier implements db.Querier for SourcePullWorker tests.
type stubPullQuerier struct {
	db.Querier
	getSourceByNameFn           func(name string) (db.DataSource, error)
	updateSourcePullStartedFn   func() error
	createPullRunFn             func() (db.SourcePullRun, error)
	upsertGLEIFFn               func() (db.GleifCompanyRawInput, error)
	failPullRunFn               func() error
	succeedPullRunFn            func() error
	updateSourcePullSucceededFn func() error
	updateSourcePullFailedFn    func() error
	insertCompanyFn             func() (db.Company, error)
}

func (q *stubPullQuerier) GetSourceByName(ctx context.Context, name string) (db.DataSource, error) {
	return q.getSourceByNameFn(name)
}
func (q *stubPullQuerier) UpdateSourcePullStarted(ctx context.Context, name string) error {
	if q.updateSourcePullStartedFn != nil {
		return q.updateSourcePullStartedFn()
	}
	return nil
}
func (q *stubPullQuerier) CreatePullRun(ctx context.Context, arg db.CreatePullRunParams) (db.SourcePullRun, error) {
	return q.createPullRunFn()
}
func (q *stubPullQuerier) UpsertGLEIFCompanyRawInput(ctx context.Context, arg db.UpsertGLEIFCompanyRawInputParams) (db.GleifCompanyRawInput, error) {
	return q.upsertGLEIFFn()
}
func (q *stubPullQuerier) FailPullRun(ctx context.Context, arg db.FailPullRunParams) error {
	if q.failPullRunFn != nil {
		return q.failPullRunFn()
	}
	return nil
}
func (q *stubPullQuerier) SucceedPullRun(ctx context.Context, arg db.SucceedPullRunParams) error {
	if q.succeedPullRunFn != nil {
		return q.succeedPullRunFn()
	}
	return nil
}
func (q *stubPullQuerier) UpdateSourcePullSucceeded(ctx context.Context, arg db.UpdateSourcePullSucceededParams) error {
	if q.updateSourcePullSucceededFn != nil {
		return q.updateSourcePullSucceededFn()
	}
	return nil
}
func (q *stubPullQuerier) UpdateSourcePullFailed(ctx context.Context, arg db.UpdateSourcePullFailedParams) error {
	if q.updateSourcePullFailedFn != nil {
		return q.updateSourcePullFailedFn()
	}
	return nil
}
func (q *stubPullQuerier) InsertCompany(ctx context.Context, arg db.InsertCompanyParams) (db.Company, error) {
	if q.insertCompanyFn != nil {
		return q.insertCompanyFn()
	}
	return db.Company{}, nil
}

func TestSourcePullWorker_WritesRawInputsOnly(t *testing.T) {
	ctx := context.Background()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"records": []any{map[string]any{
				"name":          "Test Corp",
				"country_iso2":  "GB",
				"lei":           "TEST123456789012345678",
				"status":        "active",
				"snapshot_hash": "abc123",
				"raw_data":      map[string]any{"lei": "TEST123456789012345678"},
			}},
			"has_more": false,
			"total":    1,
		})
	}))
	defer srv.Close()

	runID := uuid.New()
	sourceID := uuid.New()
	calls := map[string]int{}

	q := &stubPullQuerier{
		getSourceByNameFn: func(name string) (db.DataSource, error) {
			return db.DataSource{
				ID:                sourceID,
				Name:              name,
				PullTaskType:      "source_pull",
				ScheduleKind:      "interval",
				ProcessorTaskType: pullPtrString("source_process"),
				Enabled:           true,
			}, nil
		},
		createPullRunFn: func() (db.SourcePullRun, error) {
			calls["createPullRun"]++
			return db.SourcePullRun{ID: runID}, nil
		},
		upsertGLEIFFn: func() (db.GleifCompanyRawInput, error) {
			calls["upsertGLEIF"]++
			now := time.Now()
			return db.GleifCompanyRawInput{
				ID:               uuid.New(),
				FirstSeenAt:      now,
				LastSeenAt:       now,
				ProcessingStatus: "pending",
			}, nil
		},
		insertCompanyFn: func() (db.Company, error) {
			calls["insertCompany"]++
			return db.Company{}, nil
		},
	}

	crawler := crawlerclient.New(srv.URL)
	w := workers.NewSourcePullWorker(q, crawler)

	job := &river.Job[workers.SourcePullArgs]{
		JobRow: &rivertype.JobRow{ID: 1, Kind: "source_pull"},
		Args:   workers.SourcePullArgs{SourceName: "gleif", TriggerType: "manual"},
	}

	require.NoError(t, w.Work(ctx, job))

	assert.Equal(t, 0, calls["insertCompany"], "must not write resolved companies")
	assert.Equal(t, 1, calls["createPullRun"], "must create pull run row")
	assert.GreaterOrEqual(t, calls["upsertGLEIF"], 1, "must write to gleif raw input table")
}

func pullPtrString(s string) *string { return &s }
