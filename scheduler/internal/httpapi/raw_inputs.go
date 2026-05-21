package httpapi

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	pgx "github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

	db "github.com/pulsarpoint/corpscout/scheduler/internal/db/gen"
	"github.com/pulsarpoint/corpscout/scheduler/internal/workers"
)

type rawInputRow struct {
	ID        string    `json:"id"`
	Source    string    `json:"source"`
	Name      string    `json:"name"`
	NativeID  string    `json:"native_id"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

// handleListRawInputs returns a unified paginated view of all raw_inputs tables.
// Query params: source, status, q (name search), sort (name|source|created_at|status), dir (asc|desc), page, limit.
func (h *Handlers) handleListRawInputs(w http.ResponseWriter, r *http.Request) {
	if h.pool == nil {
		writeError(w, http.StatusServiceUnavailable, "database pool not available")
		return
	}

	page := queryInt(r, "page", 1)
	pageSize := min(queryInt(r, "limit", 50), 200)
	offset := (page - 1) * pageSize
	srcFilter := r.URL.Query().Get("source")
	statusFilter := r.URL.Query().Get("status")
	nameQ := r.URL.Query().Get("q")
	sortBy := r.URL.Query().Get("sort")
	sortDir := r.URL.Query().Get("dir")

	if sortDir != "asc" {
		sortDir = "desc"
	}
	validSort := map[string]bool{"name": true, "source": true, "created_at": true, "status": true}
	if !validSort[sortBy] {
		sortBy = "created_at"
	}

	// Build shared positional args and WHERE expressions.
	var args []any
	var commonWhere []string

	if statusFilter != "" {
		args = append(args, statusFilter)
		commonWhere = append(commonWhere, fmt.Sprintf("processing_status = $%d", len(args)))
	}

	var chNameExpr, brregNameExpr string
	if nameQ != "" {
		args = append(args, "%"+nameQ+"%")
		ref := fmt.Sprintf("$%d", len(args))
		chNameExpr = fmt.Sprintf("company_name ILIKE %s", ref)
		brregNameExpr = fmt.Sprintf("organization_name ILIKE %s", ref)
	}

	buildWhere := func(extra string) string {
		parts := append([]string{}, commonWhere...)
		if extra != "" {
			parts = append(parts, extra)
		}
		if len(parts) == 0 {
			return ""
		}
		return "WHERE " + strings.Join(parts, " AND ")
	}

	chSub := fmt.Sprintf(
		`SELECT id::text, 'companies_house' AS source, company_name AS name, company_number AS native_id, processing_status AS status, created_at FROM companies_house_company_raw_inputs %s`,
		buildWhere(chNameExpr),
	)
	brregSub := fmt.Sprintf(
		`SELECT id::text, 'brreg' AS source, organization_name AS name, organization_number AS native_id, processing_status AS status, created_at FROM brreg_company_raw_inputs %s`,
		buildWhere(brregNameExpr),
	)

	var subs []string
	switch srcFilter {
	case "companies_house":
		subs = []string{chSub}
	case "brreg":
		subs = []string{brregSub}
	default:
		subs = []string{chSub, brregSub}
	}
	union := strings.Join(subs, " UNION ALL ")

	// Count.
	var total int64
	if err := h.pool.QueryRow(r.Context(), fmt.Sprintf("SELECT COUNT(*) FROM (%s) t", union), args...).Scan(&total); err != nil {
		slog.Error("list raw inputs count", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	// Paginated, sorted rows.
	dataArgs := append(args, pageSize, offset)
	dataSQL := fmt.Sprintf(
		"SELECT id, source, name, native_id, status, created_at FROM (%s) t ORDER BY %s %s LIMIT $%d OFFSET $%d",
		union, sortBy, sortDir, len(args)+1, len(args)+2,
	)
	rows, err := h.pool.Query(r.Context(), dataSQL, dataArgs...)
	if err != nil {
		slog.Error("list raw inputs", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	defer rows.Close()

	items := []rawInputRow{}
	for rows.Next() {
		var row rawInputRow
		if err := rows.Scan(&row.ID, &row.Source, &row.Name, &row.NativeID, &row.Status, &row.CreatedAt); err != nil {
			slog.Error("scan raw input row", "error", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		items = append(items, row)
	}
	if err := rows.Err(); err != nil {
		slog.Error("raw input rows iter", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"items": items,
		"total": total,
		"page":  page,
		"limit": pageSize,
	})
}

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
