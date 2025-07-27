-- migrations/004_leads.sql

CREATE TABLE IF NOT EXISTS leads (
    id SERIAL PRIMARY KEY,
    landing_id INTEGER NOT NULL REFERENCES landings(id) ON DELETE CASCADE,
    name VARCHAR(255),
    email VARCHAR(255),
    phone VARCHAR(50),
    message TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);