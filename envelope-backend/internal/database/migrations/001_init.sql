CREATE TABLE envelopes (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(80) NOT NULL,
    category VARCHAR(40) NOT NULL,
    allocation_percent SMALLINT NOT NULL DEFAULT 0 CHECK (allocation_percent BETWEEN 0 AND 100),
    balance_kopecks BIGINT NOT NULL DEFAULT 0 CHECK (balance_kopecks >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE transactions (
    id BIGSERIAL PRIMARY KEY,
    type VARCHAR(16) NOT NULL CHECK (type IN ('income', 'expense')),
    amount_kopecks BIGINT NOT NULL CHECK (amount_kopecks > 0),
    description VARCHAR(200) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE transaction_allocations (
    id BIGSERIAL PRIMARY KEY,
    transaction_id BIGINT NOT NULL REFERENCES transactions(id) ON DELETE RESTRICT,
    envelope_id BIGINT NOT NULL REFERENCES envelopes(id) ON DELETE RESTRICT,
    amount_kopecks BIGINT NOT NULL CHECK (amount_kopecks > 0),
    UNIQUE (transaction_id, envelope_id)
);

CREATE INDEX idx_transactions_created_at ON transactions (created_at DESC, id DESC);
CREATE INDEX idx_transaction_allocations_envelope_id ON transaction_allocations (envelope_id);

-- Starter data for the championship demo. The percentages sum to 100%.
INSERT INTO envelopes (name, category, allocation_percent)
VALUES
    ('Рестораны', 'Еда вне дома', 40),
    ('Такси', 'Транспорт', 25),
    ('Продукты', 'Покупки', 35);
