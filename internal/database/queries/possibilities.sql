-- name: GetPossibility :one
SELECT * FROM possibilities WHERE id = $1;

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

-- name: TechTrendByCategory :many
WITH offers AS (
    SELECT id, tech, created_at >= sqlc.arg(current_start)::timestamptz AS is_current
    FROM possibilities
    WHERE category = sqlc.arg(category)
      AND duplicate_of IS NULL
      AND created_at >= sqlc.arg(previous_start)::timestamptz
),
totals AS (
    SELECT
        COUNT(*) FILTER (WHERE is_current) AS current_total,
        COUNT(*) FILTER (WHERE NOT is_current) AS previous_total
    FROM offers
),
counts AS (
    SELECT
        lower(trim(t)) AS technology,
        COUNT(DISTINCT id) FILTER (WHERE is_current) AS current_count,
        COUNT(DISTINCT id) FILTER (WHERE NOT is_current) AS previous_count
    FROM offers,
         LATERAL unnest(string_to_array(tech, ',')) AS t
    WHERE trim(t) <> ''
    GROUP BY 1
)
SELECT
    c.technology::text AS technology,
    c.current_count,
    c.previous_count,
    t.current_total,
    t.previous_total,
    COALESCE(round(100.0 * c.current_count / NULLIF(t.current_total, 0), 1), 0)::float8 AS current_share,
    COALESCE(round(100.0 * c.previous_count / NULLIF(t.previous_total, 0), 1), 0)::float8 AS previous_share
FROM counts c
CROSS JOIN totals t
ORDER BY c.current_count DESC, c.previous_count DESC, c.technology;

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

-- name: ListPossibilitiesForDuplicates :many
SELECT id, category, company_name, tech
FROM possibilities
WHERE duplicate_of IS NULL AND tech IS NOT NULL
ORDER BY id;

-- name: MarkPossibilityDuplicate :exec
UPDATE possibilities SET tech = $2, duplicate_of = $3 WHERE id = $1;

-- name: ListDuplicatesByCategory :many
SELECT
    o.id AS original_id,
    o.url AS original_url,
    o.company_name,
    o.job_title AS original_job_title,
    o.location AS original_location,
    o.source AS original_source,
    o.tech AS original_tech,
    d.id,
    d.url,
    d.job_title,
    d.location,
    d.source
FROM possibilities d
JOIN possibilities o ON o.id = d.duplicate_of
WHERE d.category = $1
ORDER BY o.id, d.id;
