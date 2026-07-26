-- name: GetPossibility :one
SELECT * FROM possibilities WHERE id = $1;

-- name: ListPossibilities :many
SELECT * FROM possibilities ORDER BY created_at DESC;

-- name: CreatePossibility :one
INSERT INTO possibilities (
category, url, company_name, job_title, location, is_remote, tech, description, posted_at
) VALUES (
$1, $2, $3, $4, $5, $6, $7, $8, $9
) ON CONFLICT (url) DO NOTHING
RETURNING *;

-- name: StatsPossibility :many
SELECT
    trim(t) AS technology,
    COUNT(*)
FROM possibilities,
     LATERAL unnest(string_to_array(tech, ',')) AS t
GROUP BY technology
ORDER BY COUNT(*) DESC;
