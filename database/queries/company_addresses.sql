-- name: UpsertCompanyAddress :exec
INSERT INTO company_addresses (
    native_id,
    source,
    address_line_1,
    address_line_2,
    locality,
    postal_code,
    country,
    region
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8
)
ON CONFLICT (native_id, source) DO UPDATE SET
    address_line_1 = EXCLUDED.address_line_1,
    address_line_2 = EXCLUDED.address_line_2,
    locality       = EXCLUDED.locality,
    postal_code    = EXCLUDED.postal_code,
    country        = EXCLUDED.country,
    region         = EXCLUDED.region,
    last_seen_at   = now();

-- name: GetCompanyAddressesByNativeID :many
SELECT * FROM company_addresses
WHERE native_id = $1
ORDER BY source;
