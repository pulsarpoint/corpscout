-- name: ListCountries :many
SELECT * FROM countries ORDER BY name;

-- name: GetCountryByISO2 :one
SELECT * FROM countries WHERE iso_alpha2 = $1;

-- name: GetCountryByID :one
SELECT * FROM countries WHERE id = $1;
