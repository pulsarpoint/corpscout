-- name: InsertDomainCrawlJob :one
INSERT INTO domain_crawl_jobs (domain_id, mode, max_pages)
VALUES ($1, $2, $3)
RETURNING *;

-- name: SetDomainCrawlJobRiverID :exec
UPDATE domain_crawl_jobs SET river_job_id = $2 WHERE id = $1;

-- name: SetDomainCrawlJobS3Prefix :exec
UPDATE domain_crawl_jobs SET s3_prefix = $2 WHERE id = $1;

-- name: SetDomainCrawlJobFavicon :exec
UPDATE domain_crawl_jobs SET favicon_s3_key = $2, favicon_url = $3 WHERE id = $1;

-- name: InsertDomainCrawlJobPage :exec
INSERT INTO domain_crawl_job_pages (job_id, page_num, url, title, status_code, content_type, md_s3_key, html_s3_key, headers_s3_key)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
ON CONFLICT (job_id, page_num) DO NOTHING;

-- name: ListDomainCrawlJobs :many
SELECT
    j.id,
    j.domain_id,
    j.river_job_id,
    j.mode,
    j.max_pages,
    j.s3_prefix,
    j.favicon_s3_key,
    j.favicon_url,
    j.created_at,
    rj.state        AS river_state,
    rj.finalized_at AS river_finalized_at,
    rj.errors       AS river_errors
FROM domain_crawl_jobs j
LEFT JOIN river_job rj ON rj.id = j.river_job_id
WHERE j.domain_id = $1
ORDER BY j.created_at DESC;

-- name: ListDomainCrawlJobPages :many
SELECT *
FROM domain_crawl_job_pages
WHERE job_id = $1
ORDER BY page_num;

-- name: GetDomainCrawlJobPage :one
SELECT *
FROM domain_crawl_job_pages
WHERE job_id = $1 AND page_num = $2;

-- name: GetDomainCrawlJob :one
SELECT
    j.id,
    j.domain_id,
    j.river_job_id,
    j.mode,
    j.max_pages,
    j.s3_prefix,
    j.favicon_s3_key,
    j.favicon_url,
    j.created_at
FROM domain_crawl_jobs j
WHERE j.id = $1 AND j.domain_id = $2;
