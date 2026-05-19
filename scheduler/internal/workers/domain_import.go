package workers

import (
	"bytes"
	"context"
	"encoding/csv"
	"log/slog"
	"strings"

	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
	pgx "github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"

	db "github.com/pulsarpoint/corpscout/scheduler/internal/db/gen"
)

const evidenceJSON = `{"source":"csv_upload"}`

// s3Downloader is the subset of s3client.Client that DomainImportWorker needs.
type s3Downloader interface {
	Download(ctx context.Context, key string) ([]byte, string, error)
}

// DomainImportWorker processes a CSV domain import batch.
type DomainImportWorker struct {
	river.WorkerDefaults[DomainImportArgs]
	db db.Querier
	s3 s3Downloader
}

// NewDomainImportWorker constructs a DomainImportWorker.
func NewDomainImportWorker(q db.Querier, s3 s3Downloader) *DomainImportWorker {
	return &DomainImportWorker{db: q, s3: s3}
}

// Work reads the CSV from S3, processes each row, and updates the batch record.
func (w *DomainImportWorker) Work(ctx context.Context, job *river.Job[DomainImportArgs]) error {
	args := job.Args
	batchID, err := uuid.Parse(args.BatchID)
	if err != nil {
		return errors.Wrap(err, "parse batch_id")
	}

	data, _, err := w.s3.Download(ctx, args.CsvS3Key)
	if err != nil {
		return errors.Wrap(err, "download csv from s3")
	}

	r := csv.NewReader(bytes.NewReader(data))
	r.TrimLeadingSpace = true
	r.FieldsPerRecord = -1
	records, err := r.ReadAll()
	if err != nil {
		return errors.Wrap(err, "parse csv")
	}

	rows := records
	if len(records) > 0 {
		rows = records[1:]
	}

	if err := w.db.UpdateImportBatchStarted(ctx, db.UpdateImportBatchStartedParams{
		ID:        batchID,
		RowsTotal: int32(len(rows)),
	}); err != nil {
		slog.Warn("update import batch started", "batch_id", batchID, "error", err)
	}

	var imported, skipped, failed int32
	for _, rec := range rows {
		if len(rec) < 2 {
			failed++
			continue
		}
		domainStr := strings.ToLower(strings.TrimSpace(rec[1]))
		if domainStr == "" {
			skipped++
			continue
		}
		companyName := ""
		if len(rec) >= 3 {
			companyName = strings.TrimSpace(rec[2])
		}

		if processErr := w.processRow(ctx, domainStr, companyName); processErr != nil {
			slog.Warn("import row failed", "domain", domainStr, "error", processErr)
			failed++
			continue
		}
		imported++
	}

	finalStatus := "completed"
	if imported == 0 && failed > 0 {
		finalStatus = "failed"
	}

	if err := w.db.UpdateImportBatchCompleted(ctx, db.UpdateImportBatchCompletedParams{
		ID:           batchID,
		Status:       finalStatus,
		RowsImported: imported,
		RowsSkipped:  skipped,
		RowsFailed:   failed,
		ErrorMessage: nil,
	}); err != nil {
		slog.Error("update import batch completed", "batch_id", batchID, "error", err)
		return errors.Wrap(err, "update import batch")
	}

	slog.Info("domain import batch completed",
		"batch_id", batchID,
		"imported", imported,
		"skipped", skipped,
		"failed", failed,
	)
	return nil
}

func (w *DomainImportWorker) processRow(ctx context.Context, domainStr, companyName string) error {
	d, err := w.db.UpsertDomainWithSource(ctx, db.UpsertDomainWithSourceParams{
		Domain:       domainStr,
		ImportSource: "manual_upload",
	})
	if err != nil {
		return errors.Wrap(err, "upsert domain")
	}

	if companyName == "" {
		return nil
	}

	company, err := w.db.GetCompanyByExactName(ctx, companyName)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		return errors.Wrap(err, "lookup company by name")
	}

	_, err = w.db.UpsertCompanyDomain(ctx, db.UpsertCompanyDomainParams{
		CompanyID:        company.ID,
		DomainID:         d.ID,
		RelationshipType: "candidate",
		Status:           "needs_review",
		Signal:           "manual_upload",
		Confidence:       int16(90),
		Evidence:         []byte(evidenceJSON),
	})
	if err != nil {
		return errors.Wrap(err, "upsert company domain")
	}
	return nil
}
