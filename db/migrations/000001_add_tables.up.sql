CREATE TABLE IF NOT EXISTS codes (
    id SERIAL PRIMARY KEY,
    code TEXT UNIQUE NOT NULL,
    batch_id UUID NOT NULL,
    client_id VARCHAR NOT NULL,
    customer_id UUID,
    used_at TIMESTAMP
);

CREATE TABLE IF NOT EXISTS batches (
    id UUID PRIMARY KEY,
    rules JSONB NOT NULL
);

CREATE TABLE IF NOT EXISTS code_usage (
    id SERIAL PRIMARY KEY,
    code TEXT NOT NULL,
    batch_id UUID NOT NULL,
    client_id UUID NOT NULL,
    customer_id UUID NOT NULL,
    used_at TIMESTAMP NOT NULL
);
