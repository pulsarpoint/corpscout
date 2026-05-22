package httpapi

import (
	"context"
	"database/sql"
	"encoding/json"
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
	ID                string    `json:"id"`
	Source            string    `json:"source"`
	Name              string    `json:"name"`
	NativeID          string    `json:"native_id"`
	Status            string    `json:"status"`
	TranslationStatus *string   `json:"translation_status,omitempty"`
	CreatedAt         time.Time `json:"created_at"`
}

type rawInputListSource struct {
	source       string
	tableName    string
	nameColumn   string
	nativeColumn string
	translated   bool
}

var rawInputListSources = []rawInputListSource{
	{source: "gleif", tableName: "gleif_company_raw_inputs", nameColumn: "legal_name", nativeColumn: "lei"},
	{source: "companies_house", tableName: "companies_house_company_raw_inputs", nameColumn: "company_name", nativeColumn: "company_number"},
	{source: "brreg", tableName: "brreg_company_raw_inputs", nameColumn: "organization_name", nativeColumn: "organization_number", translated: true},
	{source: "cvr", tableName: "cvr_company_raw_inputs", nameColumn: "company_name", nativeColumn: "cvr_number", translated: true},
	{source: "ariregister", tableName: "ariregister_company_raw_inputs", nameColumn: "legal_name", nativeColumn: "registry_code", translated: true},
}

// handleListRawInputs returns a unified paginated view of all raw_inputs tables.
// Query params: source, status, translation_status, q (name search), sort (name|source|created_at|status), dir (asc|desc), page, limit.
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
	translationStatusFilter := r.URL.Query().Get("translation_status")
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

	var nameExpr string
	if nameQ != "" {
		args = append(args, "%"+nameQ+"%")
		nameExpr = fmt.Sprintf("$%d", len(args))
	}
	var translationExpr string
	if translationStatusFilter != "" {
		args = append(args, translationStatusFilter)
		translationExpr = fmt.Sprintf("translation_status = $%d", len(args))
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

	var subs []string
	for _, src := range rawInputListSources {
		if srcFilter != "" && srcFilter != src.source {
			continue
		}
		if translationStatusFilter != "" && !src.translated {
			continue
		}

		var extra []string
		if nameExpr != "" {
			extra = append(extra, fmt.Sprintf("%s ILIKE %s", src.nameColumn, nameExpr))
		}
		translationSelect := "NULL::text AS translation_status"
		if src.translated {
			translationSelect = "translation_status"
			if translationExpr != "" {
				extra = append(extra, translationExpr)
			}
		}
		subs = append(subs, fmt.Sprintf(
			`SELECT id::text, '%s' AS source, COALESCE(%s, '') AS name, %s AS native_id, processing_status AS status, %s, created_at FROM %s %s`,
			src.source,
			src.nameColumn,
			src.nativeColumn,
			translationSelect,
			src.tableName,
			buildWhere(strings.Join(extra, " AND ")),
		))
	}
	if len(subs) == 0 {
		writeJSON(w, http.StatusOK, map[string]any{
			"items": []rawInputRow{},
			"total": int64(0),
			"page":  page,
			"limit": pageSize,
		})
		return
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
		"SELECT id, source, name, native_id, status, translation_status, created_at FROM (%s) t ORDER BY %s %s LIMIT $%d OFFSET $%d",
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
		var translationStatus sql.NullString
		if err := rows.Scan(&row.ID, &row.Source, &row.Name, &row.NativeID, &row.Status, &translationStatus, &row.CreatedAt); err != nil {
			slog.Error("scan raw input row", "error", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		if translationStatus.Valid {
			row.TranslationStatus = &translationStatus.String
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
	case "cvr_company_raw_inputs":
		return rawInputSupport{
			retry: func(q db.Querier, ctx context.Context, id uuid.UUID) (uuid.UUID, error) {
				return q.RetryCVRRawInput(ctx, id)
			},
			ignore: func(q db.Querier, ctx context.Context, id uuid.UUID) (uuid.UUID, error) {
				return q.IgnoreCVRRawInput(ctx, id)
			},
		}
	case "ariregister_company_raw_inputs":
		return rawInputSupport{
			retry: func(q db.Querier, ctx context.Context, id uuid.UUID) (uuid.UUID, error) {
				return q.RetryAriregisterRawInput(ctx, id)
			},
			ignore: func(q db.Querier, ctx context.Context, id uuid.UUID) (uuid.UUID, error) {
				return q.IgnoreAriregisterRawInput(ctx, id)
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

type rawInputDetail struct {
	ID                       string          `json:"id"`
	Source                   string          `json:"source"`
	Name                     string          `json:"name"`
	NativeID                 string          `json:"native_id"`
	Status                   string          `json:"status"`
	CompanyType              string          `json:"company_type,omitempty"`
	RegistrationStatus       string          `json:"registration_status,omitempty"`
	Website                  string          `json:"website,omitempty"`
	CountryISO2              string          `json:"country_iso2,omitempty"`
	RunID                    string          `json:"run_id,omitempty"`
	ProcessingAttempts       int             `json:"processing_attempts"`
	ProcessingError          string          `json:"processing_error,omitempty"`
	PayloadHash              string          `json:"payload_hash"`
	RawPayload               json.RawMessage `json:"raw_payload"`
	RawPayloadEn             json.RawMessage `json:"raw_payload_en,omitempty"`
	TranslationStatus        string          `json:"translation_status,omitempty"`
	TranslationAttempts      int             `json:"translation_attempts,omitempty"`
	TranslationError         string          `json:"translation_error,omitempty"`
	TranslationModel         string          `json:"translation_model,omitempty"`
	TranslationPromptVersion string          `json:"translation_prompt_version,omitempty"`
	TranslationFxSource      string          `json:"translation_fx_source,omitempty"`
	TranslationFxRateDate    string          `json:"translation_fx_rate_date,omitempty"`
	TranslatedAt             *time.Time      `json:"translated_at,omitempty"`
	FirstSeenAt              time.Time       `json:"first_seen_at"`
	LastSeenAt               time.Time       `json:"last_seen_at"`
	ProcessedAt              *time.Time      `json:"processed_at,omitempty"`
	CreatedAt                time.Time       `json:"created_at"`
	UpdatedAt                time.Time       `json:"updated_at"`
}

// handleGetRawInput returns full detail for a single raw input row.
// URL: GET /api/v1/raw-inputs/{source}/{id}
func (h *Handlers) handleGetRawInput(w http.ResponseWriter, r *http.Request) {
	if h.pool == nil {
		writeError(w, http.StatusServiceUnavailable, "database pool not available")
		return
	}
	source := chi.URLParam(r, "source")
	idStr := chi.URLParam(r, "id")

	var row rawInputDetail
	var err error

	switch source {
	case "gleif":
		err = h.pool.QueryRow(r.Context(), `
			SELECT id::text, 'gleif', COALESCE(legal_name,''), lei,
			       COALESCE(processing_status,''), '', COALESCE(registration_status,''), '', COALESCE(headquarters_country_code,''),
			       COALESCE(run_id,''), processing_attempts, COALESCE(processing_error,''),
			       COALESCE(payload_hash,''), raw_payload,
			       first_seen_at, last_seen_at, processed_at, created_at, updated_at
			FROM gleif_company_raw_inputs WHERE id = $1
		`, idStr).Scan(
			&row.ID, &row.Source, &row.Name, &row.NativeID,
			&row.Status, &row.CompanyType, &row.RegistrationStatus, &row.Website, &row.CountryISO2,
			&row.RunID, &row.ProcessingAttempts, &row.ProcessingError,
			&row.PayloadHash, &row.RawPayload,
			&row.FirstSeenAt, &row.LastSeenAt, &row.ProcessedAt, &row.CreatedAt, &row.UpdatedAt,
		)
	case "companies_house":
		err = h.pool.QueryRow(r.Context(), `
			SELECT id::text, 'companies_house', company_name, company_number,
			       processing_status, company_type, '', '', COALESCE(country_iso2,''),
			       COALESCE(run_id,''), processing_attempts, COALESCE(processing_error,''),
			       payload_hash, raw_payload,
			       first_seen_at, last_seen_at, processed_at, created_at, updated_at
			FROM companies_house_company_raw_inputs WHERE id = $1
		`, idStr).Scan(
			&row.ID, &row.Source, &row.Name, &row.NativeID,
			&row.Status, &row.CompanyType, &row.RegistrationStatus, &row.Website, &row.CountryISO2,
			&row.RunID, &row.ProcessingAttempts, &row.ProcessingError,
			&row.PayloadHash, &row.RawPayload,
			&row.FirstSeenAt, &row.LastSeenAt, &row.ProcessedAt, &row.CreatedAt, &row.UpdatedAt,
		)
	case "brreg":
		var rawPayloadEn []byte
		err = h.pool.QueryRow(r.Context(), `
			SELECT id::text, 'brreg', organization_name, organization_number,
			       processing_status, '', registration_status, COALESCE(website,''), COALESCE(country_iso2,''),
			       COALESCE(run_id,''), processing_attempts, COALESCE(processing_error,''),
			       payload_hash, raw_payload, raw_payload_en,
			       translation_status, translation_attempts, COALESCE(translation_error,''), COALESCE(translation_model,''),
			       COALESCE(translation_prompt_version,''), COALESCE(translation_fx_source,''), COALESCE(translation_fx_rate_date::text,''),
			       translated_at, first_seen_at, last_seen_at, processed_at, created_at, updated_at
			FROM brreg_company_raw_inputs WHERE id = $1
		`, idStr).Scan(
			&row.ID, &row.Source, &row.Name, &row.NativeID,
			&row.Status, &row.CompanyType, &row.RegistrationStatus, &row.Website, &row.CountryISO2,
			&row.RunID, &row.ProcessingAttempts, &row.ProcessingError,
			&row.PayloadHash, &row.RawPayload, &rawPayloadEn,
			&row.TranslationStatus, &row.TranslationAttempts, &row.TranslationError, &row.TranslationModel,
			&row.TranslationPromptVersion, &row.TranslationFxSource, &row.TranslationFxRateDate,
			&row.TranslatedAt,
			&row.FirstSeenAt, &row.LastSeenAt, &row.ProcessedAt, &row.CreatedAt, &row.UpdatedAt,
		)
		if len(rawPayloadEn) > 0 {
			row.RawPayloadEn = json.RawMessage(rawPayloadEn)
		}
	case "cvr":
		row, err = h.getTranslatedRawInputDetail(r.Context(), translatedRawInputDetailQuery{
			source:       "cvr",
			tableName:    "cvr_company_raw_inputs",
			nameColumn:   "company_name",
			nativeColumn: "cvr_number",
			typeColumn:   "company_type",
		}, idStr)
	case "ariregister":
		row, err = h.getTranslatedRawInputDetail(r.Context(), translatedRawInputDetailQuery{
			source:       "ariregister",
			tableName:    "ariregister_company_raw_inputs",
			nameColumn:   "legal_name",
			nativeColumn: "registry_code",
			typeColumn:   "legal_form",
		}, idStr)
	default:
		writeError(w, http.StatusBadRequest, "unknown source")
		return
	}

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "raw input not found")
			return
		}
		slog.Error("get raw input detail", "source", source, "id", idStr, "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusOK, row)
}

type translatedRawInputDetailQuery struct {
	source       string
	tableName    string
	nameColumn   string
	nativeColumn string
	typeColumn   string
}

func (h *Handlers) getTranslatedRawInputDetail(ctx context.Context, cfg translatedRawInputDetailQuery, id string) (rawInputDetail, error) {
	var row rawInputDetail
	var rawPayloadEn []byte
	err := h.pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT id::text, '%s', COALESCE(%s,''), COALESCE(%s,''),
		       COALESCE(processing_status,''), COALESCE(%s,''), COALESCE(registration_status,''), COALESCE(website,''), COALESCE(country_iso2,''),
		       COALESCE(run_id,''), processing_attempts, COALESCE(processing_error,''),
		       COALESCE(payload_hash,''), raw_payload, raw_payload_en,
		       COALESCE(translation_status,''), translation_attempts, COALESCE(translation_error,''), COALESCE(translation_model,''),
		       COALESCE(translation_prompt_version,''), COALESCE(translation_fx_source,''), COALESCE(translation_fx_rate_date::text,''),
		       translated_at, first_seen_at, last_seen_at, processed_at, created_at, updated_at
		FROM %s WHERE id = $1
	`, cfg.source, cfg.nameColumn, cfg.nativeColumn, cfg.typeColumn, cfg.tableName), id).Scan(
		&row.ID, &row.Source, &row.Name, &row.NativeID,
		&row.Status, &row.CompanyType, &row.RegistrationStatus, &row.Website, &row.CountryISO2,
		&row.RunID, &row.ProcessingAttempts, &row.ProcessingError,
		&row.PayloadHash, &row.RawPayload, &rawPayloadEn,
		&row.TranslationStatus, &row.TranslationAttempts, &row.TranslationError, &row.TranslationModel,
		&row.TranslationPromptVersion, &row.TranslationFxSource, &row.TranslationFxRateDate,
		&row.TranslatedAt,
		&row.FirstSeenAt, &row.LastSeenAt, &row.ProcessedAt, &row.CreatedAt, &row.UpdatedAt,
	)
	if len(rawPayloadEn) > 0 {
		row.RawPayloadEn = json.RawMessage(rawPayloadEn)
	}
	return row, err
}
