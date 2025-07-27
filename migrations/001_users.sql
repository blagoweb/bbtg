-- migrations/001_users.sql

CREATE TABLE IF NOT EXISTS users (
    id SERIAL PRIMARY KEY,
    telegram_id BIGINT NOT NULL UNIQUE,
    username VARCHAR(255),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);