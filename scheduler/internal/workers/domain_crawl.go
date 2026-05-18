package workers

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"

	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
	"github.com/riverqueue/river"

	"github.com/pulsarpoint/corpscout/scheduler/internal/crawlerclient"
	db "github.com/pulsarpoint/corpscout/scheduler/internal/db/gen"
	"github.com/pulsarpoint/corpscout/scheduler/internal/s3client"
)

// DomainCrawlArgs are the arguments for a domain crawl River job.
type DomainCrawlArgs struct {
	DomainCrawlJobID string `json:"domain_crawl_job_id"`
	DomainID         string `json:"domain_id"`
	Domain           string `json:"domain"`
	Mode             string `json:"mode"`
	MaxPages         int    `json:"max_pages"`
}

// Kind returns the River job kind identifier.
func (DomainCrawlArgs) Kind() string { return "domain_crawl" }

// DomainCrawlWorker is the River worker that crawls a domain and persists results to S3 and the database.
type DomainCrawlWorker struct {
	river.WorkerDefaults[DomainCrawlArgs]
	db      db.Querier
	crawler *crawlerclient.Client
	s3      *s3client.Client
}

// NewDomainCrawlWorker constructs a DomainCrawlWorker with the given dependencies.
func NewDomainCrawlWorker(q db.Querier, crawler *crawlerclient.Client, s3 *s3client.Client) *DomainCrawlWorker {
	return &DomainCrawlWorker{db: q, crawler: crawler, s3: s3}
}

// Work executes the domain crawl job: links the River job ID, sets the S3 prefix, calls the
// Python crawler, uploads all page content and the favicon, then inserts page rows.
func (w *DomainCrawlWorker) Work(ctx context.Context, job *river.Job[DomainCrawlArgs]) error {
	args := job.Args
	jobID, err := uuid.Parse(args.DomainCrawlJobID)
	if err != nil {
		return errors.Wrap(err, "parse domain_crawl_job_id")
	}

	// 1. Link river job ID
	riverJobID := job.ID
	if err := w.db.SetDomainCrawlJobRiverID(ctx, db.SetDomainCrawlJobRiverIDParams{
		ID:         jobID,
		RiverJobID: &riverJobID,
	}); err != nil {
		slog.Error("set river job id", "error", err, "job_id", jobID)
		return errors.Wrap(err, "set river job id")
	}

	// 2. Set S3 prefix
	prefix := fmt.Sprintf("%s/%s/", args.Domain, args.DomainCrawlJobID)
	if err := w.db.SetDomainCrawlJobS3Prefix(ctx, db.SetDomainCrawlJobS3PrefixParams{
		ID:       jobID,
		S3Prefix: &prefix,
	}); err != nil {
		return errors.Wrap(err, "set s3 prefix")
	}

	// 3. Call Python crawler
	resp, err := w.crawler.CrawlDomain(ctx, args.Domain, args.Mode, args.MaxPages)
	if err != nil {
		slog.Error("domain crawl failed", "domain", args.Domain, "job_id", jobID, "error", err)
		return errors.Wrap(err, "crawl domain")
	}

	// 4. Upload favicon
	if resp.FaviconBytes != nil && resp.FaviconURL != nil {
		decoded, decErr := base64.StdEncoding.DecodeString(*resp.FaviconBytes)
		if decErr == nil {
			ext := filepath.Ext(*resp.FaviconURL)
			if ext == "" {
				ext = ".ico"
			}
			faviconKey := prefix + "favicon" + ext
			ct := faviconContentType(ext)
			if uploadErr := w.s3.Upload(ctx, faviconKey, decoded, ct); uploadErr != nil {
				slog.Error("upload favicon", "error", uploadErr, "key", faviconKey)
			} else {
				if dbErr := w.db.SetDomainCrawlJobFavicon(ctx, db.SetDomainCrawlJobFaviconParams{
					ID:           jobID,
					FaviconS3Key: &faviconKey,
					FaviconUrl:   resp.FaviconURL,
				}); dbErr != nil {
					return errors.Wrap(dbErr, "set favicon s3 key")
				}
			}
		}
	}

	// 5. Upload pages and insert page rows
	for i, page := range resp.Pages {
		pageNum := i + 1
		mdKey := fmt.Sprintf("%spage_%d.md", prefix, pageNum)
		htmlKey := fmt.Sprintf("%spage_%d.html", prefix, pageNum)
		headersKey := fmt.Sprintf("%spage_%d_headers.json", prefix, pageNum)

		headersJSON, _ := json.Marshal(page.Headers)

		if err := w.s3.Upload(ctx, mdKey, []byte(page.Markdown), "text/markdown"); err != nil {
			slog.Error("upload markdown", "error", err, "key", mdKey)
			return errors.Wrap(err, "upload page markdown")
		}
		if err := w.s3.Upload(ctx, htmlKey, []byte(page.HTML), "text/html"); err != nil {
			slog.Error("upload html", "error", err, "key", htmlKey)
			return errors.Wrap(err, "upload page html")
		}
		if err := w.s3.Upload(ctx, headersKey, headersJSON, "application/json"); err != nil {
			slog.Error("upload headers", "error", err, "key", headersKey)
			return errors.Wrap(err, "upload page headers")
		}

		statusCode := int32(page.StatusCode)
		if err := w.db.InsertDomainCrawlJobPage(ctx, db.InsertDomainCrawlJobPageParams{
			JobID:        jobID,
			PageNum:      int32(pageNum),
			Url:          page.URL,
			Title:        page.Title,
			StatusCode:   &statusCode,
			ContentType:  page.ContentType,
			MdS3Key:      mdKey,
			HtmlS3Key:    htmlKey,
			HeadersS3Key: headersKey,
		}); err != nil {
			return errors.Wrap(err, "insert page row")
		}
	}

	return nil
}

// faviconContentType returns an appropriate MIME type for the given file extension.
func faviconContentType(ext string) string {
	switch strings.ToLower(ext) {
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".svg":
		return "image/svg+xml"
	case ".gif":
		return "image/gif"
	default:
		return "image/x-icon"
	}
}
