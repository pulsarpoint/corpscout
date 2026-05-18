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
	retry      func(context.Context, uuid.UUID) (uuid.UUID, error)
	ignore     func(context.Context, uuid.UUID) (uuid.UUID, error)
}

func (h *Handlers) rawInputSupport(src db.DataSource) rawInputSupport {
	switch src.InputTableName {
	case "gleif_company_raw_inputs":
		return rawInputSupport{
			canProcess: true,
			retry:      h.db.RetryGLEIFRawInput,
			ignore:     h.db.IgnoreGLEIFRawInput,
		}
	case "companies_house_company_raw_inputs":
		return rawInputSupport{
			canProcess: true,
			retry:      h.db.RetryCompaniesHouseRawInput,
			ignore:     h.db.IgnoreCompaniesHouseRawInput,
		}
	case "brreg_company_raw_inputs":
		return rawInputSupport{
			canProcess: true,
			retry:      h.db.RetryBrregRawInput,
			ignore:     h.db.IgnoreBrregRawInput,
		}
	case "ai_company_profile_raw_inputs":
		return rawInputSupport{
			retry:  h.db.RetryAIRawInput,
			ignore: h.db.IgnoreAIRawInput,
		}
	case "domain_discovery_raw_inputs":
		return rawInputSupport{
			retry:  h.db.RetryDomainDiscoveryRawInput,
			ignore: h.db.IgnoreDomainDiscoveryRawInput,
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
	if _, err := support.retry(r.Context(), rowID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusUnprocessableEntity, "raw input row is not retryable")
			return
		}
		slog.Error("retry raw input", "source", src.Name, "id", rowID, "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if support.canProcess {
		if _, err := h.rv.Insert(r.Context(), workers.SourceProcessArgs{
			SourceName: src.Name,
		}, &river.InsertOpts{
			Queue: "source_process",
			UniqueOpts: river.UniqueOpts{
				ByArgs:  true,
				ByState: []rivertype.JobState{rivertype.JobStateAvailable, rivertype.JobStateRunning, rivertype.JobStateScheduled},
			},
		}); err != nil {
			slog.Error("enqueue raw input retry processor", "source", src.Name, "id", rowID, "error", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "retried"})
}

func (h *Handlers) handleIgnoreRawInput(w http.ResponseWriter, r *http.Request) {
	_, rowID, support, ok := h.resolveRawInputAction(w, r)
	if !ok {
		return
	}
	if _, err := support.ignore(r.Context(), rowID); err != nil {
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
		writeError(w, http.StatusNotFound, "source not found")
		return db.DataSource{}, uuid.UUID{}, rawInputSupport{}, false
	}

	support := h.rawInputSupport(src)
	if support.retry == nil || support.ignore == nil {
		writeError(w, http.StatusUnprocessableEntity, "raw input retry not supported for this source")
		return db.DataSource{}, uuid.UUID{}, rawInputSupport{}, false
	}

	return src, id, support, true
}
