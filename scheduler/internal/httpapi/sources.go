package httpapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/go-chi/chi/v5"
	pgx "github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
	"go.temporal.io/api/serviceerror"
	"go.temporal.io/sdk/client"

	db "github.com/pulsarpoint/corpscout/scheduler/internal/db/gen"
	"github.com/pulsarpoint/corpscout/scheduler/internal/workers"
)

type syncCheckpointView struct {
	Cursor          string     `json:"cursor"`
	LastCompletedAt *time.Time `json:"last_completed_at,omitempty"`
	UpdatedAt       time.Time  `json:"updated_at"`
	// Derived fields parsed from cursor.
	Mode     string `json:"mode"`                // "bulk" | "incremental" | "none"
	BulkDate string `json:"bulk_date,omitempty"` // date of last bulk, e.g. "2026-05-21"
}

type sourceView struct {
	db.DataSource
	Config         json.RawMessage     `json:"config"`
	SyncCheckpoint *syncCheckpointView `json:"sync_checkpoint,omitempty"`
}

func toSourceView(s db.DataSource) sourceView {
	cfg := json.RawMessage(s.Config)
	if len(cfg) == 0 {
		cfg = json.RawMessage("null")
	}
	return sourceView{DataSource: s, Config: cfg}
}

func toSourceViewWithCheckpoint(s db.DataSource, cp *db.SourceSyncCheckpoint) sourceView {
	v := toSourceView(s)
	if cp == nil {
		return v
	}
	scv := &syncCheckpointView{
		Cursor:    cp.Cursor,
		UpdatedAt: cp.UpdatedAt,
	}
	if cp.LastCompletedAt.Valid {
		t := cp.LastCompletedAt.Time
		scv.LastCompletedAt = &t
	}
	if strings.HasPrefix(cp.Cursor, "bulk:") {
		scv.Mode = "incremental" // bulk done, now doing incremental
		scv.BulkDate = strings.TrimPrefix(cp.Cursor, "bulk:")
	} else if cp.Cursor != "" {
		scv.Mode = "incremental"
	} else {
		scv.Mode = "none"
	}
	v.SyncCheckpoint = scv
	return v
}

func toSourceViews(sources []db.DataSource) []sourceView {
	out := make([]sourceView, len(sources))
	for i, s := range sources {
		out[i] = toSourceView(s)
	}
	return out
}

func (h *Handlers) handleListSources(w http.ResponseWriter, r *http.Request) {
	sources, err := h.db.ListSources(r.Context())
	if err != nil {
		slog.Error("list sources", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, toSourceViews(sources))
}

func (h *Handlers) handleGetSource(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	source, err := h.db.GetSourceByName(r.Context(), name)
	if err != nil {
		writeError(w, http.StatusNotFound, "source not found")
		return
	}
	cp, err := h.db.GetSyncCheckpoint(r.Context(), name)
	if err != nil {
		writeJSON(w, http.StatusOK, toSourceView(source))
		return
	}
	writeJSON(w, http.StatusOK, toSourceViewWithCheckpoint(source, &cp))
}

type patchSourceRequest struct {
	Enabled            *bool                      `json:"enabled"`
	ScheduleEnabled    *bool                      `json:"schedule_enabled"`
	ScheduleKind       *string                    `json:"schedule_kind"`
	ScheduleExpression *string                    `json:"schedule_expression"`
	Config             map[string]json.RawMessage `json:"config"`
}

var forbiddenConfigKey = regexp.MustCompile(`(?i)(key|secret|token|password)`)

func (h *Handlers) handlePatchSource(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	var req patchSourceRequest
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	ctx := r.Context()
	configRequested := req.Config != nil && len(req.Config) > 0
	hasWrites := req.Enabled != nil || req.ScheduleEnabled != nil || req.ScheduleKind != nil || req.ScheduleExpression != nil || configRequested
	if !hasWrites {
		writeError(w, http.StatusBadRequest, "empty patch request")
		return
	}

	src, err := h.db.GetSourceByName(ctx, name)
	if err != nil {
		writeError(w, http.StatusNotFound, "source not found")
		return
	}

	scheduleKind := src.ScheduleKind
	scheduleExpr := src.ScheduleExpression
	if req.ScheduleKind != nil {
		scheduleKind = *req.ScheduleKind
	}
	if req.ScheduleExpression != nil {
		scheduleExpr = req.ScheduleExpression
	}
	if req.ScheduleKind != nil || req.ScheduleExpression != nil {
		if !validScheduleKind(scheduleKind) {
			writeError(w, http.StatusUnprocessableEntity, "invalid schedule kind")
			return
		}
		if scheduleKind == "interval" && scheduleExpr != nil {
			if _, err := parsePositiveDuration(*scheduleExpr); err != nil {
				writeError(w, http.StatusUnprocessableEntity, "invalid schedule expression")
				return
			}
		}
	}

	var mergedConfig json.RawMessage
	if configRequested {
		if err := validateConfigPatch(req.Config); err != nil {
			writeError(w, http.StatusUnprocessableEntity, "invalid config patch")
			return
		}
		config, err := mergeConfig(src.Config, req.Config)
		if err != nil {
			slog.Error("merge source config", "name", name, "error", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		mergedConfig = config
	}

	writeDB := h.db
	var tx pgx.Tx
	if h.pool != nil && hasWrites {
		var err error
		tx, err = h.pool.Begin(ctx)
		if err != nil {
			slog.Error("begin source patch transaction", "name", name, "error", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		defer func() {
			if tx == nil {
				return
			}
			if err := tx.Rollback(ctx); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
				slog.Error("rollback source patch transaction", "name", name, "error", err)
			}
		}()
		writeDB = db.New(tx)
	}

	if req.Enabled != nil {
		if err := writeDB.UpdateSourceEnabled(ctx, db.UpdateSourceEnabledParams{
			Name: name, Enabled: *req.Enabled,
		}); err != nil {
			slog.Error("update source enabled", "name", name, "error", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
	}
	if req.ScheduleEnabled != nil {
		if err := writeDB.UpdateSourceScheduleEnabled(ctx, db.UpdateSourceScheduleEnabledParams{
			Name: name, ScheduleEnabled: *req.ScheduleEnabled,
		}); err != nil {
			slog.Error("update source schedule enabled", "name", name, "error", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
	}
	if req.ScheduleKind != nil || req.ScheduleExpression != nil {
		if err := writeDB.UpdateSourceSchedule(ctx, db.UpdateSourceScheduleParams{
			Name:               name,
			ScheduleKind:       scheduleKind,
			ScheduleExpression: scheduleExpr,
		}); err != nil {
			slog.Error("update source schedule", "name", name, "error", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
	}
	if configRequested {
		if err := writeDB.UpdateSourceConfig(ctx, db.UpdateSourceConfigParams{
			Name:   name,
			Config: mergedConfig,
		}); err != nil {
			slog.Error("update source config", "name", name, "error", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
	}
	if tx != nil {
		if err := tx.Commit(ctx); err != nil {
			slog.Error("commit source patch transaction", "name", name, "error", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		tx = nil
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func validateConfigPatch(config map[string]json.RawMessage) error {
	for key, value := range config {
		if forbiddenConfigKey.MatchString(key) {
			return errors.Newf("forbidden config key %q", key)
		}
		if !json.Valid(value) {
			return errors.Newf("invalid json for config key %q", key)
		}
		var decoded any
		dec := json.NewDecoder(bytes.NewReader(value))
		dec.UseNumber()
		if err := dec.Decode(&decoded); err != nil {
			return errors.Wrapf(err, "decode config key %q", key)
		}
		if err := validateNestedConfigKeys(key, decoded); err != nil {
			return errors.Wrapf(err, "validate config key %q", key)
		}
	}
	return nil
}

func validateNestedConfigKeys(path string, value any) error {
	switch typed := value.(type) {
	case map[string]any:
		for key, nestedValue := range typed {
			nestedPath := path + "." + key
			if forbiddenConfigKey.MatchString(key) {
				return errors.Newf("forbidden nested config key %q", nestedPath)
			}
			if err := validateNestedConfigKeys(nestedPath, nestedValue); err != nil {
				return errors.Wrapf(err, "validate nested config key %q", nestedPath)
			}
		}
	case []any:
		for i, nestedValue := range typed {
			nestedPath := fmt.Sprintf("%s[%d]", path, i)
			if err := validateNestedConfigKeys(nestedPath, nestedValue); err != nil {
				return errors.Wrapf(err, "validate nested config item %q", nestedPath)
			}
		}
	}
	return nil
}

func validScheduleKind(kind string) bool {
	switch kind {
	case "manual", "interval", "cron", "event":
		return true
	default:
		return false
	}
}

func parsePositiveDuration(expr string) (time.Duration, error) {
	duration, err := time.ParseDuration(expr)
	if err != nil {
		return 0, errors.Wrap(err, "parse schedule expression")
	}
	if duration <= 0 {
		return 0, errors.Newf("schedule expression must be positive")
	}
	return duration, nil
}

func mergeConfig(existing json.RawMessage, patch map[string]json.RawMessage) (json.RawMessage, error) {
	merged := map[string]json.RawMessage{}
	if len(bytes.TrimSpace(existing)) > 0 && string(bytes.TrimSpace(existing)) != "null" {
		if err := json.Unmarshal(existing, &merged); err != nil {
			return nil, errors.Wrap(err, "decode existing source config")
		}
	}
	for key, value := range patch {
		copied := make(json.RawMessage, len(value))
		copy(copied, value)
		merged[key] = copied
	}
	out, err := json.Marshal(merged)
	if err != nil {
		return nil, errors.Wrap(err, "encode merged source config")
	}
	return json.RawMessage(out), nil
}

func (h *Handlers) handleTriggerSource(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	source, err := h.db.GetSourceByName(r.Context(), name)
	if err != nil {
		writeError(w, http.StatusNotFound, "source not found")
		return
	}
	// Sources with a Temporal pipeline are dispatched as DataTask River jobs.
	if cfg, ok := workers.TemporalWorkflowForSource(name); ok {
		if h.rv == nil {
			writeError(w, http.StatusServiceUnavailable, "scheduler not available")
			return
		}
		if _, err := h.rv.Insert(r.Context(), workers.DataTaskArgs{
			Source:  name,
			Country: cfg.Country,
		}, &river.InsertOpts{
			Queue: "data_task",
		}); err != nil {
			slog.Error("trigger source (temporal)", "name", name, "error", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "queued"})
		return
	}

	// Legacy River-based sources.
	if source.PullTaskType != "source_pull" {
		writeError(w, http.StatusUnprocessableEntity, "pull task type not supported for manual trigger")
		return
	}
	if h.rv == nil {
		writeError(w, http.StatusServiceUnavailable, "scheduler not available")
		return
	}
	if _, err := h.rv.Insert(r.Context(), workers.SourcePullArgs{
		SourceName:  name,
		TriggerType: "manual",
	}, &river.InsertOpts{
		Queue: "source_pull",
		UniqueOpts: river.UniqueOpts{
			ByArgs:  true,
			ByState: []rivertype.JobState{rivertype.JobStatePending, rivertype.JobStateAvailable, rivertype.JobStateRunning, rivertype.JobStateScheduled},
		},
	}); err != nil {
		slog.Error("trigger source", "name", name, "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "queued"})
}

type translateBrregRequest struct {
	IDs        []string `json:"ids,omitempty"`
	FXRateDate string   `json:"fx_rate_date,omitempty"`
}

type brregTranslationStats struct {
	Pending        int64 `json:"pending"`
	Translating    int64 `json:"translating"`
	Translated     int64 `json:"translated"`
	Failed         int64 `json:"failed"`
	ReadyToProcess int64 `json:"ready_to_process"`
	Total          int64 `json:"total"`
}

func (h *Handlers) handleTranslateBrreg(w http.ResponseWriter, r *http.Request) {
	var req translateBrregRequest
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil && !errors.Is(err, io.EOF) {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.FXRateDate != "" {
		if _, err := time.Parse("2006-01-02", req.FXRateDate); err != nil {
			writeError(w, http.StatusBadRequest, "invalid fx_rate_date")
			return
		}
	}
	if h.temporal == nil {
		writeError(w, http.StatusServiceUnavailable, "temporal client not available")
		return
	}

	workflowID := "translate-brreg-all"
	if len(req.IDs) > 0 {
		workflowID = fmt.Sprintf("translate-brreg-ids-%d", time.Now().Unix())
	}
	input := map[string]any{
		"ids":            req.IDs,
		"prompt_version": envWithDefault("LLM_PROMPT_VERSION", "v1"),
		"model":          envWithDefault("LLM_MODEL", "qwen3:6b"),
		"fx_rate_date":   req.FXRateDate,
	}

	run, err := h.temporal.ExecuteWorkflow(r.Context(),
		client.StartWorkflowOptions{
			ID:        workflowID,
			TaskQueue: "corpscout-pipelines",
		},
		"TranslateBrregRawInputs",
		input,
	)
	if err != nil {
		var alreadyStarted *serviceerror.WorkflowExecutionAlreadyStarted
		if errors.As(err, &alreadyStarted) {
			writeError(w, http.StatusConflict, "translation workflow already running")
			return
		}
		slog.Error("start brreg translation workflow", "workflow_id", workflowID, "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"workflow_id":     workflowID,
		"workflow_run_id": run.GetRunID(),
		"status":          "started",
	})
}

func (h *Handlers) handleProcessSource(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if _, err := h.db.GetSourceByName(r.Context(), name); err != nil {
		writeError(w, http.StatusNotFound, "source not found")
		return
	}
	if h.rv == nil {
		writeError(w, http.StatusServiceUnavailable, "scheduler not available")
		return
	}

	result, err := h.rv.Insert(r.Context(), workers.SourceProcessArgs{
		SourceName: name,
	}, &river.InsertOpts{
		Queue: "source_process",
		UniqueOpts: river.UniqueOpts{
			ByArgs: true,
			ByState: []rivertype.JobState{
				rivertype.JobStatePending,
				rivertype.JobStateAvailable,
				rivertype.JobStateRunning,
				rivertype.JobStateScheduled,
			},
		},
	})
	if err != nil {
		slog.Error("manual source process", "source", name, "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if result.UniqueSkippedAsDuplicate {
		writeError(w, http.StatusConflict, "source process already running")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"job_id": result.Job.ID, "status": "enqueued"})
}

func (h *Handlers) handleBrregTranslationStats(w http.ResponseWriter, r *http.Request) {
	if h.pool == nil {
		writeError(w, http.StatusServiceUnavailable, "database pool not available")
		return
	}
	var stats brregTranslationStats
	if err := h.pool.QueryRow(r.Context(), `
		SELECT
			COUNT(*) FILTER (WHERE translation_status = 'pending') AS pending,
			COUNT(*) FILTER (WHERE translation_status = 'translating') AS translating,
			COUNT(*) FILTER (WHERE translation_status = 'translated') AS translated,
			COUNT(*) FILTER (WHERE translation_status = 'failed') AS failed,
			COUNT(*) FILTER (WHERE translation_status = 'translated' AND processing_status = 'pending') AS ready_to_process,
			COUNT(*) AS total
		FROM brreg_company_raw_inputs
	`).Scan(
		&stats.Pending,
		&stats.Translating,
		&stats.Translated,
		&stats.Failed,
		&stats.ReadyToProcess,
		&stats.Total,
	); err != nil {
		slog.Error("brreg translation stats", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, stats)
}

func envWithDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func (h *Handlers) handleProbeSource(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if _, err := h.db.GetSourceByName(r.Context(), name); err != nil {
		writeError(w, http.StatusNotFound, "source not found")
		return
	}
	if h.crawler == nil {
		writeError(w, http.StatusServiceUnavailable, "crawler not available")
		return
	}
	start := time.Now()
	resp, err := h.crawler.Crawl(r.Context(), name, time.Time{}, nil, 1)
	durationMs := time.Since(start).Milliseconds()
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"records_count": 0, "total": 0, "has_more": false,
			"sample": nil, "error": err.Error(), "duration_ms": durationMs,
		})
		return
	}
	var sample any
	if len(resp.Records) > 0 {
		sample = resp.Records[0]
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"records_count": len(resp.Records), "total": resp.Total,
		"has_more": resp.HasMore, "sample": sample,
		"error": nil, "duration_ms": durationMs,
	})
}
