-- migrations/006_payments.sql

CREATE TABLE IF NOT EXISTS payments (
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    amount NUMERIC(12,2) NOT NULL,         -- сумма платежа
    currency VARCHAR(10) NOT NULL,         -- валюта, например 'RUB'
    payment_method VARCHAR(50),            -- способ оплаты, например 'yookassa'
    status VARCHAR(50) NOT NULL,           -- 'pending', 'succeeded', 'failed'
    transaction_id VARCHAR(255) UNIQUE,    -- внешний идентификатор платежа
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);