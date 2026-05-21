-- name: GetStats :one
SELECT
  (SELECT COUNT(*) FROM companies)::bigint                                     AS total_companies,
  (SELECT COUNT(*) FROM domains)::bigint                                       AS total_domains,
  (SELECT COUNT(*) FROM company_domains WHERE status = 'active')::bigint       AS active_domains,
  (SELECT COUNT(*) FROM company_domains WHERE status = 'needs_review')::bigint AS pending_review,
  (SELECT COUNT(*) FROM data_sources WHERE enabled = true)::bigint             AS enabled_sources,
  (SELECT COUNT(*) FROM source_pull_runs
   WHERE status = 'succeeded' AND finished_at >= now() - interval '24 hours')::bigint AS pull_runs_completed_today,
  (SELECT COUNT(*) FROM source_pull_runs
   WHERE status = 'failed' AND finished_at >= now() - interval '24 hours')::bigint    AS pull_runs_failed_today,
  (SELECT COALESCE(SUM(raw_rows_inserted), 0) FROM source_pull_runs
   WHERE finished_at >= now() - interval '24 hours')::bigint AS records_upserted_24h,
  (SELECT COALESCE(SUM(raw_rows_inserted), 0) FROM source_pull_runs
   WHERE finished_at >= now() - interval '7 days')::bigint   AS records_upserted_7d,
  (
    (SELECT COUNT(*) FROM companies_house_company_raw_inputs WHERE processing_status = 'pending') +
    (SELECT COUNT(*) FROM brreg_company_raw_inputs           WHERE processing_status = 'pending')
  )::bigint AS pending_raw_inputs;
