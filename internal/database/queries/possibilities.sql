-- name: GetPossibility :one
SELECT * FROM possibilities WHERE id = $1;

-- name: ListPossibilitiesByCategory :many
SELECT * FROM possibilities
WHERE category = $1
ORDER BY created_at DESC, id DESC;

-- name: ExistingPossibilityUrls :many
SELECT url FROM possibilities WHERE url = ANY(@urls::text[]);

-- name: ExistingPossibilityUrl :one
SELECT url FROM possibilities WHERE url = $1;

-- name: CreatePossibility :one
INSERT INTO possibilities (
category, url, company_name, job_title, location, is_remote, tech, description, source, posted_at
) VALUES (
$1, $2, $3, $4, $5, $6, $7, $8, $9, $10
) ON CONFLICT (url) DO UPDATE
SET posted_at = COALESCE(EXCLUDED.posted_at, possibilities.posted_at)
RETURNING *;

-- name: StatsPossibilityByCategory :many
SELECT
    trim(t) AS technology,
    COUNT(*)
FROM possibilities,
     LATERAL unnest(string_to_array(tech, ',')) AS t
WHERE category = $1
GROUP BY technology
ORDER BY COUNT(*) DESC;

-- name: ListPossibilitiesByCategoryPaged :many
SELECT id, url, category, company_name, job_title, location, is_remote, tech, source, posted_at, created_at
FROM possibilities
WHERE category = $1
ORDER BY created_at DESC, id DESC
LIMIT $2 OFFSET $3;

-- name: ListPossibilitiesWithJsonFields :many
SELECT id, location, tech
FROM possibilities
WHERE location LIKE '{%' OR tech LIKE '{%';

-- name: UpdatePossibilityLocationTech :exec
UPDATE possibilities SET location = $2, tech = $3 WHERE id = $1;

-- name: ListPossibilitiesTech :many
SELECT id, tech
FROM possibilities
WHERE tech IS NOT NULL AND tech NOT LIKE '{%';

-- name: UpdatePossibilityTech :exec
UPDATE possibilities SET tech = $2 WHERE id = $1;
