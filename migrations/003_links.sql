-- migrations/003_links.sql

CREATE TABLE IF NOT EXISTS links (
    id SERIAL PRIMARY KEY,
    landing_id INTEGER NOT NULL REFERENCES landings(id) ON DELETE CASCADE,
    type VARCHAR(50) NOT NULL,        -- e.g. 'button', 'social'
    title VARCHAR(255) NOT NULL,
    url TEXT NOT NULL,
    position INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);