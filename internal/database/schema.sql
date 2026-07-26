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

CREATE TABLE possibilities (
    id BIGSERIAL PRIMARY KEY,
    url TEXT NOT NULL,
    category TEXT NOT NULL,
    company_name TEXT NOT NULL,
    job_title TEXT NOT NULL,
    location TEXT,
    is_remote BOOLEAN NOT NULL DEFAULT false,
    tech TEXT,
    description TEXT,
    posted_at TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
