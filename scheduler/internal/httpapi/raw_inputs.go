package httpapi

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/cockroachdb/errors"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	pgx "github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

	db "github.com/pulsarpoint/corpscout/scheduler/internal/db/gen"
	"github.com/pulsarpoint/corpscout/scheduler/internal/workers"
)

type rawInputSupport struct {
	canProcess bool
	retry      func(db.Querier, context.Context, uuid.UUID) (uuid.UUID, error)
	ignore     func(db.Querier, context.Context, uuid.UUID) (uuid.UUID, error)
}

func (h *Handlers) rawInputSupport(src db.DataSource) rawInputSupport {
	switch src.InputTableName {
	case "gleif_company_raw_inputs":
		return rawInputSupport{
			canProcess: true,
			retry: func(q db.Querier, ctx context.Context, id uuid.UUID) (uuid.UUID, error) {
				return q.RetryGLEIFRawInput(ctx, id)
			},
			ignore: func(q db.Querier, ctx context.Context, id uuid.UUID) (uuid.UUID, error) {
				return q.IgnoreGLEIFRawInput(ctx, id)
			},
		}
	case "companies_house_company_raw_inputs":
		return rawInputSupport{
			canProcess: true,
			retry: func(q db.Querier, ctx context.Context, id uuid.UUID) (uuid.UUID, error) {
				return q.RetryCompaniesHouseRawInput(ctx, id)
			},
			ignore: func(q db.Querier, ctx context.Context, id uuid.UUID) (uuid.UUID, error) {
				return q.IgnoreCompaniesHouseRawInput(ctx, id)
			},
		}
	case "brreg_company_raw_inputs":
		return rawInputSupport{
			canProcess: true,
			retry: func(q db.Querier, ctx context.Context, id uuid.UUID) (uuid.UUID, error) {
				return q.RetryBrregRawInput(ctx, id)
			},
			ignore: func(q db.Querier, ctx context.Context, id uuid.UUID) (uuid.UUID, error) {
				return q.IgnoreBrregRawInput(ctx, id)
			},
		}
	case "ai_company_profile_raw_inputs":
		return rawInputSupport{
			retry: func(q db.Querier, ctx context.Context, id uuid.UUID) (uuid.UUID, error) {
				return q.RetryAIRawInput(ctx, id)
			},
			ignore: func(q db.Querier, ctx context.Context, id uuid.UUID) (uuid.UUID, error) {
				return q.IgnoreAIRawInput(ctx, id)
			},
		}
	case "domain_discovery_raw_inputs":
		return rawInputSupport{
			retry: func(q db.Querier, ctx context.Context, id uuid.UUID) (uuid.UUID, error) {
				return q.RetryDomainDiscoveryRawInput(ctx, id)
			},
			ignore: func(q db.Querier, ctx context.Context, id uuid.UUID) (uuid.UUID, error) {
				return q.IgnoreDomainDiscoveryRawInput(ctx, id)
			},
		}
	default:
		return rawInputSupport{}
	}
}

func (h *Handlers) handleRetryRawInput(w http.ResponseWriter, r *http.Request) {
	src, rowID, support, ok := h.resolveRawInputAction(w, r)
	if !ok {
		return
	}
	if support.canProcess && h.rv == nil {
		writeError(w, http.StatusServiceUnavailable, "scheduler not available")
		return
	}
	if support.canProcess {
		if h.pool == nil {
			writeError(w, http.StatusServiceUnavailable, "database pool not available")
			return
		}
		if err := h.retryRawInputWithProcessJob(r.Context(), src, rowID, support); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				writeError(w, http.StatusUnprocessableEntity, "raw input row is not retryable")
				return
			}
			slog.Error("retry raw input with processor", "source", src.Name, "id", rowID, "error", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "retried"})
		return
	}
	if _, err := support.retry(h.db, r.Context(), rowID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusUnprocessableEntity, "raw input row is not retryable")
			return
		}
		slog.Error("retry raw input", "source", src.Name, "id", rowID, "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "retried"})
}

func (h *Handlers) retryRawInputWithProcessJob(ctx context.Context, src db.DataSource, rowID uuid.UUID, support rawInputSupport) error {
	tx, err := h.pool.Begin(ctx)
	if err != nil {
		return errors.Wrap(err, "begin raw input retry transaction")
	}
	defer func() {
		if err := tx.Rollback(ctx); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
			slog.Error("rollback raw input retry transaction", "source", src.Name, "id", rowID, "error", err)
		}
	}()

	qtx := db.New(tx)
	if _, err := support.retry(qtx, ctx, rowID); err != nil {
		return err
	}
	if _, err := h.rv.InsertTx(ctx, tx, workers.SourceProcessArgs{
		SourceName: src.Name,
	}, &river.InsertOpts{
		Queue: "source_process",
		UniqueOpts: river.UniqueOpts{
			ByArgs:  true,
			ByState: []rivertype.JobState{rivertype.JobStateAvailable, rivertype.JobStateScheduled},
		},
	}); err != nil {
		return errors.Wrap(err, "enqueue raw input retry processor")
	}
	if err := tx.Commit(ctx); err != nil {
		return errors.Wrap(err, "commit raw input retry transaction")
	}
	return nil
}

func (h *Handlers) handleIgnoreRawInput(w http.ResponseWriter, r *http.Request) {
	_, rowID, support, ok := h.resolveRawInputAction(w, r)
	if !ok {
		return
	}
	if _, err := support.ignore(h.db, r.Context(), rowID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusUnprocessableEntity, "raw input row cannot be ignored")
			return
		}
		sourceName := chi.URLParam(r, "name")
		slog.Error("ignore raw input", "source", sourceName, "id", rowID, "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ignored"})
}

func (h *Handlers) resolveRawInputAction(w http.ResponseWriter, r *http.Request) (db.DataSource, uuid.UUID, rawInputSupport, bool) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid raw input id")
		return db.DataSource{}, uuid.UUID{}, rawInputSupport{}, false
	}

	name := chi.URLParam(r, "name")
	src, err := h.db.GetSourceByName(r.Context(), name)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "source not found")
			return db.DataSource{}, uuid.UUID{}, rawInputSupport{}, false
		}
		slog.Error("resolve raw input source", "source", name, "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return db.DataSource{}, uuid.UUID{}, rawInputSupport{}, false
	}

	support := h.rawInputSupport(src)
	if support.retry == nil || support.ignore == nil {
		writeError(w, http.StatusUnprocessableEntity, "raw input retry not supported for this source")
		return db.DataSource{}, uuid.UUID{}, rawInputSupport{}, false
	}

	return src, id, support, true
}
