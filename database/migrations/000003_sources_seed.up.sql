-- Seed known data sources. Sources without a country are global (country_id NULL).
INSERT INTO data_sources (name, source_type, adapter_type, country_id, enabled, crawl_interval_hours, config)
VALUES
    ('gleif',          'corporate_registry', 'api', NULL,                                                     true,  24,  '{}'),
    ('wikidata',       'corporate_registry', 'api', NULL,                                                     true,  168, '{}'),
    ('opencorporates', 'corporate_registry', 'api', NULL,                                                     false, 72,  '{}'),
    ('companies_house','corporate_registry', 'api', (SELECT id FROM countries WHERE iso_alpha2 = 'GB'),       true,  24,  '{}'),
    ('brreg',          'corporate_registry', 'api', (SELECT id FROM countries WHERE iso_alpha2 = 'NO'),       true,  24,  '{}'),
    ('cvr',            'corporate_registry', 'api', (SELECT id FROM countries WHERE iso_alpha2 = 'DK'),       true,  24,  '{}'),
    ('ariregister',    'corporate_registry', 'api', (SELECT id FROM countries WHERE iso_alpha2 = 'EE'),       true,  24,  '{}')
ON CONFLICT (name) DO NOTHING;
