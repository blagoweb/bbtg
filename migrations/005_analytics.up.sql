-- migrations/005_analytics.sql

CREATE TABLE IF NOT EXISTS analytics (
    id SERIAL PRIMARY KEY,
    landing_id INTEGER NOT NULL REFERENCES landings(id) ON DELETE CASCADE,
    event_type VARCHAR(50) NOT NULL,       -- e.g. 'view', 'click'
    geo_country VARCHAR(100),              -- страна пользователя
    geo_city VARCHAR(100),                 -- город пользователя
    ip_address INET,                       -- IP-адрес
    user_agent TEXT,                       -- User-Agent браузера
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);