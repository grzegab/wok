-- name: GetApplication :one
SELECT * FROM applications WHERE id = $1;

-- name: ListApplications :many
SELECT * FROM applications ORDER BY application_date DESC;

-- name: CreateApplication :one
INSERT INTO applications (
    company_name, application_date, had_chat, starting_price, ending_price,
    my_price, outcome, offer_link, fixes_after_chat, stack, additional_info,
    things_to_fix, status
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13
) RETURNING *;

-- name: UpdateApplication :one
UPDATE applications SET
    company_name = $2,
    application_date = $3,
    had_chat = $4,
    starting_price = $5,
    ending_price = $6,
    my_price = $7,
    outcome = $8,
    offer_link = $9,
    fixes_after_chat = $10,
    stack = $11,
    additional_info = $12,
    things_to_fix = $13,
    status = $14,
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: DeleteApplication :exec
DELETE FROM applications WHERE id = $1;
