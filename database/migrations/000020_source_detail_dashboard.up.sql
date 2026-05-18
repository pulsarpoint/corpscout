ALTER TABLE data_sources
    ADD COLUMN schedule_enabled BOOLEAN NOT NULL DEFAULT TRUE;

CREATE OR REPLACE VIEW v_source_raw_inputs AS
  SELECT
    id,
    'gleif' AS source_name,
    'gleif_company_raw_inputs' AS source_input_table,
    lei AS source_native_id,
    processing_status,
    processing_attempts,
    processing_error,
    first_seen_at,
    last_seen_at,
    payload_hash,
    EXISTS (
      SELECT 1 FROM suggestion_source_links ssl
      WHERE ssl.source_input_table = 'gleif_company_raw_inputs'
        AND ssl.source_input_key = id::text
    ) AS has_suggestion
  FROM gleif_company_raw_inputs

  UNION ALL

  SELECT
    id,
    'companies_house' AS source_name,
    'companies_house_company_raw_inputs' AS source_input_table,
    company_number AS source_native_id,
    processing_status,
    processing_attempts,
    processing_error,
    first_seen_at,
    last_seen_at,
    payload_hash,
    EXISTS (
      SELECT 1 FROM suggestion_source_links ssl
      WHERE ssl.source_input_table = 'companies_house_company_raw_inputs'
        AND ssl.source_input_key = id::text
    ) AS has_suggestion
  FROM companies_house_company_raw_inputs

  UNION ALL

  SELECT
    id,
    'brreg' AS source_name,
    'brreg_company_raw_inputs' AS source_input_table,
    organization_number AS source_native_id,
    processing_status,
    processing_attempts,
    processing_error,
    first_seen_at,
    last_seen_at,
    payload_hash,
    EXISTS (
      SELECT 1 FROM suggestion_source_links ssl
      WHERE ssl.source_input_table = 'brreg_company_raw_inputs'
        AND ssl.source_input_key = id::text
    ) AS has_suggestion
  FROM brreg_company_raw_inputs

  UNION ALL

  SELECT
    id,
    'ai_company_profile' AS source_name,
    'ai_company_profile_raw_inputs' AS source_input_table,
    COALESCE(normalized_domain, '') AS source_native_id,
    processing_status,
    processing_attempts,
    processing_error,
    first_seen_at,
    last_seen_at,
    payload_hash,
    EXISTS (
      SELECT 1 FROM suggestion_source_links ssl
      WHERE ssl.source_input_table = 'ai_company_profile_raw_inputs'
        AND ssl.source_input_key = id::text
    ) AS has_suggestion
  FROM ai_company_profile_raw_inputs

  UNION ALL

  SELECT
    id,
    'domain_discovery' AS source_name,
    'domain_discovery_raw_inputs' AS source_input_table,
    domain AS source_native_id,
    processing_status,
    processing_attempts,
    processing_error,
    first_seen_at,
    last_seen_at,
    payload_hash,
    EXISTS (
      SELECT 1 FROM suggestion_source_links ssl
      WHERE ssl.source_input_table = 'domain_discovery_raw_inputs'
        AND ssl.source_input_key = id::text
    ) AS has_suggestion
  FROM domain_discovery_raw_inputs;

GRANT SELECT ON v_source_raw_inputs TO corpscout_anon;
GRANT SELECT ON gleif_company_raw_inputs TO corpscout_anon;
GRANT SELECT ON companies_house_company_raw_inputs TO corpscout_anon;
GRANT SELECT ON brreg_company_raw_inputs TO corpscout_anon;
GRANT SELECT ON ai_company_profile_raw_inputs TO corpscout_anon;
GRANT SELECT ON domain_discovery_raw_inputs TO corpscout_anon;
GRANT SELECT ON suggestion_source_links TO corpscout_anon;
