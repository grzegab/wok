CREATE TYPE application_status AS ENUM ('applied', 'progress', 'failed', 'abandon');

CREATE TABLE applications (
    id                BIGSERIAL PRIMARY KEY,
    company_name      TEXT NOT NULL,
    application_date  DATE NOT NULL,
    had_chat          BOOLEAN NOT NULL DEFAULT false,
    starting_price    INTEGER,
    ending_price      INTEGER,
    my_price          INTEGER,
    outcome           TEXT,
    offer_link        TEXT,
    fixes_after_chat  TEXT,
    stack             TEXT,
    additional_info   TEXT,
    things_to_fix     TEXT,
    status            application_status NOT NULL DEFAULT 'applied',
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);
