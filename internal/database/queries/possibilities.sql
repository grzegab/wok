-- name: GetPossibility :one
SELECT * FROM possibilities WHERE id = $1;

-- name: CheckPossibility :one
SELECT url FROM possibilities WHERE url = $1 LIMIT 1;

-- name: ListPossibilities :many
SELECT * FROM possibilities ORDER BY created_at DESC;

-- name: CreatePossibility :one
INSERT INTO possibilities (
category, url, company_name, job_title, location, is_remote, tech, description, posted_at
) VALUES (
$1, $2, $3, $4, $5, $6, $7, $8, $9
) RETURNING *;

